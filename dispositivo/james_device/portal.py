"""Teclado, mouse e captura pelo portal RemoteDesktop + ScreenCast
(D5, D5a e D8 da spec 2026-10-13 B2).

No Wayland, programa nenhum injeta clique ou tecla nos outros por conta
própria; quem libera é o compositor, pelo portal, depois que o Arthur clica
em Permitir no aviso do KDE. Com `persist_mode=2` o portal devolve um
`restore_token`, e as próximas sessões abrem sem perguntar (0,01 s no
spike).

O token dá controle de teclado e mouse SEM PERGUNTAR. É segredo local:
arquivo 0600 numa pasta 0700, fora do repositório, nunca em log.

O ScreenCast de todos os monitores entra na mesma sessão por dois motivos:
mouse em posição ABSOLUTA só existe relativo a um fluxo de tela, e a captura
do `ver_tela` sai do mesmo fluxo, sem diálogo nenhum a mais.

Permissão é sempre do Arthur, nunca do James: sem ela, toda ação devolve
MENSAGEM_SEM_PERMISSAO, nada trava e a voz segue (D5a). Uma nova tentativa
(que reabre o aviso do KDE) acontece em segundo plano, no máximo uma vez por
minuto, para o aviso não ficar pipocando a cada pedido.

A lógica é testada com um canal falso; o `CanalPortal` é quem fala D-Bus,
sempre pelo laço GLib (laco_glib.py).
"""

import logging
import os
import tempfile
import threading
import time
import uuid
from dataclasses import dataclass
from pathlib import Path

logger = logging.getLogger(__name__)

RD = "org.freedesktop.portal.RemoteDesktop"
SC = "org.freedesktop.portal.ScreenCast"
BTN_ESQUERDO = 0x110  # BTN_LEFT do kernel (linux/input-event-codes.h)
MENSAGEM_SEM_PERMISSAO = (
    "preciso de permissão: clique em Permitir no aviso do KDE para eu controlar teclado, mouse e ver a tela"
)
TIMEOUT_DIALOGO = 300.0  # o Arthur pode demorar para ver o aviso
TIMEOUT_REABRIR = 20.0  # com token, reabrir leva centésimos
INTERVALO_NOVA_TENTATIVA = 60.0
_TIPOS_TECLADO_E_MOUSE = 3
_TIPO_MONITOR = 1
_PERSISTIR_ATE_REVOGAR = 2


class SemPermissao(Exception):
    def __init__(self) -> None:
        super().__init__(MENSAGEM_SEM_PERMISSAO)


class RecusaDoPortal(Exception):
    """Resposta do portal com código ≠ 0: 1 = o Arthur cancelou, 2 = outro motivo."""

    def __init__(self, codigo: int) -> None:
        super().__init__(f"o portal recusou o pedido (código {codigo})")
        self.codigo = codigo


def caminho_do_token() -> Path:
    base = os.environ.get("XDG_DATA_HOME") or os.path.expanduser("~/.local/share")
    return Path(base) / "james" / "portal-token"


class Token:
    def __init__(self, caminho: Path) -> None:
        self.caminho = Path(caminho)

    def ler(self) -> str | None:
        try:
            valor = self.caminho.read_text(encoding="utf-8").strip()
        except FileNotFoundError:
            return None
        return valor or None

    def gravar(self, valor: str) -> None:
        """Escrita atômica: arquivo temporário 0600 na mesma pasta e
        os.replace. Queda no meio nunca deixa um token pela metade."""
        pasta = self.caminho.parent
        pasta.mkdir(mode=0o700, parents=True, exist_ok=True)
        os.chmod(pasta, 0o700)  # o mkdir respeita o umask; aqui não
        descritor, temporario = tempfile.mkstemp(dir=pasta, prefix=".token-")
        try:
            with os.fdopen(descritor, "w", encoding="utf-8") as saida:
                saida.write(valor.strip())
            os.replace(temporario, self.caminho)
        except BaseException:
            os.unlink(temporario)
            raise


@dataclass(frozen=True)
class Fluxo:
    """Um monitor no ScreenCast, em coordenada lógica global."""

    node: int
    x: float
    y: float
    largura: float
    altura: float


def casar_fluxos(fluxos: list[Fluxo], telas) -> dict[str, Fluxo]:
    """Fluxo de cada tela do KWin, casado PELA POSIÇÃO: o node_id muda entre
    sessões (no spike, 138 e 142 trocaram de monitor)."""
    casados = {}
    for tela in telas:
        fluxo = next((f for f in fluxos if abs(f.x - tela.x) < 1 and abs(f.y - tela.y) < 1), None)
        if fluxo is None:
            raise ValueError(f"nenhum fluxo do portal na posição da tela {tela.nome} ({tela.x}, {tela.y})")
        casados[tela.nome] = fluxo
    return casados


def para_fluxo(fluxos: list[Fluxo], gx: float, gy: float) -> tuple[int, float, float]:
    """Ponto global lógico → (node, x e y relativos ao fluxo), que é o que
    o NotifyPointerMotionAbsolute aceita."""
    for f in fluxos:
        if f.x <= gx < f.x + f.largura and f.y <= gy < f.y + f.altura:
            return f.node, float(gx - f.x), float(gy - f.y)
    raise ValueError(f"({gx}, {gy}) fora de todos os monitores")


def _fluxos_do_resultado(resultado: dict) -> list[Fluxo]:
    fluxos = []
    for node, propriedades in resultado.get("streams", []):
        (x, y), (largura, altura) = propriedades["position"], propriedades["size"]
        fluxos.append(Fluxo(node, x, y, largura, altura))
    return fluxos


def _em_thread(funcao) -> None:
    threading.Thread(target=funcao, name="portal-nova-tentativa", daemon=True).start()


class Portal:
    """Estados: fechado → abrindo → aberto | sem_permissao. Sessão fechada
    por fora volta a fechado, e a próxima ação reabre com o token."""

    def __init__(self, canal, token: Token, relogio=time.monotonic, em_segundo_plano=_em_thread) -> None:
        self._canal = canal
        self._token = token
        self._relogio = relogio
        self._em_segundo_plano = em_segundo_plano
        self._trava = threading.Lock()
        self._ultima_tentativa = float("-inf")
        self.estado = "fechado"
        self.sessao: str | None = None
        self.fluxos: list[Fluxo] = []

    def abrir(self, timeout: float = TIMEOUT_DIALOGO) -> None:
        """Abre a sessão. Sem token válido, o KDE mostra o aviso e isto
        espera o Arthur responder (até `timeout`). Nunca levanta: o
        resultado fica em `estado`."""
        with self._trava:
            self.estado = "abrindo"
            self._ultima_tentativa = self._relogio()
            try:
                self._abrir(timeout)
                self.estado = "aberto"
                logger.info("portal aberto: %d monitor(es)", len(self.fluxos))
            except (RecusaDoPortal, TimeoutError) as erro:
                self.estado = "sem_permissao"
                logger.warning("portal sem permissão: %s", erro)
            except Exception:  # noqa: BLE001 — qualquer outra falha também não pode derrubar o device
                self.estado = "sem_permissao"
                logger.exception("falha ao abrir o portal")

    def _abrir(self, timeout: float) -> None:
        sessao = self._canal.pedir(
            RD, "CreateSession", "(a{sv})",
            ({"session_handle_token": ("s", "james" + uuid.uuid4().hex[:8])},), timeout,
        )["session_handle"]
        opcoes = {"types": ("u", _TIPOS_TECLADO_E_MOUSE), "persist_mode": ("u", _PERSISTIR_ATE_REVOGAR)}
        guardado = self._token.ler()
        if guardado:
            opcoes["restore_token"] = ("s", guardado)
        self._canal.pedir(RD, "SelectDevices", "(oa{sv})", (sessao, opcoes), timeout)
        self._canal.pedir(SC, "SelectSources", "(oa{sv})",
                          (sessao, {"types": ("u", _TIPO_MONITOR), "multiple": ("b", True)}), timeout)
        resultado = self._canal.pedir(RD, "Start", "(osa{sv})", (sessao, "", {}), timeout)
        if resultado.get("restore_token"):
            self._token.gravar(resultado["restore_token"])
        self.sessao = sessao
        self.fluxos = _fluxos_do_resultado(resultado)
        self._canal.assinar_fechamento(sessao, self._sessao_fechada)

    def _sessao_fechada(self) -> None:
        logger.warning("a sessão do portal foi fechada por fora; a próxima ação reabre")
        self.estado = "fechado"

    def _garantir(self) -> None:
        if self.estado == "aberto":
            return
        if self.estado == "fechado":
            self.abrir(TIMEOUT_REABRIR)
            if self.estado == "aberto":
                return
            raise SemPermissao()
        if self.estado == "sem_permissao" and self._relogio() - self._ultima_tentativa >= INTERVALO_NOVA_TENTATIVA:
            self._ultima_tentativa = self._relogio()
            self._em_segundo_plano(self.abrir)
        raise SemPermissao()

    def mover(self, gx: float, gy: float) -> None:
        self._garantir()
        node, x, y = para_fluxo(self.fluxos, gx, gy)
        self._canal.chamar(RD, "NotifyPointerMotionAbsolute", "(oa{sv}udd)", (self.sessao, {}, node, x, y))

    def clicar(self, botao: int = BTN_ESQUERDO) -> None:
        self._garantir()
        for estado in (1, 0):
            self._canal.chamar(RD, "NotifyPointerButton", "(oa{sv}iu)", (self.sessao, {}, botao, estado))

    def teclas(self, keysyms: list[int]) -> None:
        """Aperta em ordem e solta ao contrário: [Ctrl, v] é Ctrl+V."""
        self._garantir()
        for keysym in keysyms:
            self._canal.chamar(RD, "NotifyKeyboardKeysym", "(oa{sv}iu)", (self.sessao, {}, keysym, 1))
        for keysym in reversed(keysyms):
            self._canal.chamar(RD, "NotifyKeyboardKeysym", "(oa{sv}iu)", (self.sessao, {}, keysym, 0))

    def abrir_pipewire(self) -> int:
        """Descritor do PipeWire para capturar os fluxos (captura_linux.py)."""
        self._garantir()
        return self._canal.abrir_pipewire(self.sessao)


class CanalPortal:
    """O canal de verdade: D-Bus pelo laço GLib.

    Valores chegam como tuplas Python; dicionário de opções vem como
    {chave: (tipo, valor)} e vira a{sv} aqui. Assim o Portal não precisa
    de gi e é testável sem barramento."""

    _DESTINO = "org.freedesktop.portal.Desktop"
    _CAMINHO = "/org/freedesktop/portal/desktop"

    def __init__(self, laco, conexao) -> None:
        self._laco = laco
        self._conexao = conexao
        self._remetente = laco.executar(conexao.get_unique_name)[1:].replace(".", "_")

    @staticmethod
    def _variante(assinatura: str, valores: tuple):
        from gi.repository import GLib

        convertidos = tuple(
            {chave: GLib.Variant(tipo, valor) for chave, (tipo, valor) in v.items()} if isinstance(v, dict) else v
            for v in valores
        )
        return GLib.Variant(assinatura, convertidos)

    def pedir(self, interface: str, metodo: str, assinatura: str, valores: tuple, timeout: float) -> dict:
        """Método que responde pelo sinal Request.Response. A assinatura do
        sinal entra ANTES da chamada: a resposta pode chegar antes de a
        chamada voltar."""
        token = "james" + uuid.uuid4().hex[:8]
        opcoes = dict(valores[-1])
        opcoes["handle_token"] = ("s", token)
        valores = (*valores[:-1], opcoes)
        caminho = f"{self._CAMINHO}/request/{self._remetente}/{token}"
        espera = self._laco.assinar_sinal(self._conexao, self._DESTINO, "org.freedesktop.portal.Request",
                                          "Response", caminho)
        try:
            variante = self._variante(assinatura, valores)
            self._laco.executar(lambda: self._conexao.call_sync(
                self._DESTINO, self._CAMINHO, interface, metodo, variante, None, 0, 10000, None))
        except BaseException:
            espera.cancelar()
            raise
        codigo, resultados = espera.aguardar(timeout)
        if codigo != 0:
            raise RecusaDoPortal(codigo)
        return resultados

    def chamar(self, interface: str, metodo: str, assinatura: str, valores: tuple) -> None:
        variante = self._variante(assinatura, valores)
        self._laco.executar(lambda: self._conexao.call_sync(
            self._DESTINO, self._CAMINHO, interface, metodo, variante, None, 0, 5000, None))

    def abrir_pipewire(self, sessao: str) -> int:
        variante = self._variante("(oa{sv})", (sessao, {}))

        def abrir():
            resultado, fds = self._conexao.call_with_unix_fd_list_sync(
                self._DESTINO, self._CAMINHO, SC, "OpenPipeWireRemote", variante, None, 0, 5000, None, None)
            return fds.get(resultado.unpack()[0])

        return self._laco.executar(abrir)

    def assinar_fechamento(self, sessao: str, callback) -> None:
        def assinar():
            self._conexao.signal_subscribe(self._DESTINO, "org.freedesktop.portal.Session", "Closed", sessao,
                                           None, 0, lambda *_: callback())

        self._laco.executar(assinar)
