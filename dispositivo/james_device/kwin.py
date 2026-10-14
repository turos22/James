"""Janelas pelo script do KWin (D2 da spec 2026-10-13 B2).

No Wayland nenhum programa enxerga as janelas dos outros; quem sabe é o
compositor. O KWin aceita um script JS carregado por D-Bus
(`org.kde.KWin /Scripting`), e o script tem `workspace.windowList()` com
geometria GLOBAL em coordenada lógica, pid, monitor e foco. `wmctrl` é X11 e
não veria janela Wayland nativa; nada a instalar.

Dois detalhes medidos no spike:
- `print()` do script cai numa categoria de log desligada e some. A saída
  volta por `callDBus` para um objeto que o device registra no barramento.
- O id estável da janela é o `internalId` (UUID).

SEGURANÇA: texto vindo do modelo NUNCA entra no JS. O casamento por alvo
(`por_alvo`) roda aqui em Python, sobre a lista; o JS de focar e fechar só
recebe um `internalId` que passou pela regex de UUID.

A lógica é pura e testável com um executor falso; quem fala com o KWin é o
`ExecutorKWin`, que importa `gi` só ao ser criado.
"""

import json
import logging
import os
import re
import tempfile
import threading
import unicodedata
import uuid
from dataclasses import dataclass

logger = logging.getLogger(__name__)

SEPARADOR = "\x1f"  # não aparece em título de janela
_SEPARADOR_JS = "\\u001f"
_FIM = "\x04FIM"
_ID_VALIDO = re.compile(r"^\{?[0-9a-fA-F-]{36}\}?$")
_CAMPOS_JANELA = 16
TIMEOUT_SCRIPT = 3.0


@dataclass(frozen=True)
class Janela:
    id: str
    app: str
    titulo: str
    pid: int
    ativa: bool
    minimizada: bool
    normal: bool
    monitor: str
    frame: tuple[float, float, float, float]
    cliente: tuple[float, float, float, float]


@dataclass(frozen=True)
class Tela:
    nome: str
    x: float
    y: float
    largura: float
    altura: float
    escala: float


_JS_LISTAR = f"""
const SEP = "{_SEPARADOR_JS}";
for (const w of workspace.windowList()) {{
  const f = w.frameGeometry, c = w.clientGeometry;
  devolver([String(w.internalId), w.resourceClass, w.caption, w.pid, w.active, w.minimized,
            w.normalWindow, w.output ? w.output.name : "", f.x, f.y, f.width, f.height,
            c.x, c.y, c.width, c.height].join(SEP));
}}
"""

_JS_TELAS = f"""
const SEP = "{_SEPARADOR_JS}";
for (const o of workspace.screens) {{
  const g = o.geometry;
  devolver([o.name, g.x, g.y, g.width, g.height, o.devicePixelRatio].join(SEP));
}}
"""


def _verificar_id(identificador: str) -> str:
    if not isinstance(identificador, str) or not _ID_VALIDO.match(identificador):
        raise ValueError(f"id de janela inválido: {identificador!r}")
    return identificador


def _js_focar(identificador: str) -> str:
    alvo = json.dumps(_verificar_id(identificador))
    return f"""
const alvo = {alvo};
for (const w of workspace.windowList()) {{
  if (String(w.internalId) === alvo) {{
    if (w.minimized) w.minimized = false;
    workspace.activeWindow = w;
  }}
}}
devolver(workspace.activeWindow ? String(workspace.activeWindow.internalId) : "");
"""


def _js_fechar(identificador: str) -> str:
    alvo = json.dumps(_verificar_id(identificador))
    return f"""
const alvo = {alvo};
let achou = false;
for (const w of workspace.windowList()) {{
  if (String(w.internalId) === alvo) {{ w.closeWindow(); achou = true; }}
}}
devolver(achou ? "ok" : "nao_achei");
"""


def _booleano(texto: str) -> bool:
    return texto == "true"


def _ler_janela(linha: str) -> Janela | None:
    campos = linha.split(SEPARADOR)
    if len(campos) != _CAMPOS_JANELA:
        logger.warning("linha ilegível do script do KWin (%d campos): %r", len(campos), linha[:80])
        return None
    try:
        numeros = [float(c) for c in campos[8:]]
        return Janela(
            id=campos[0], app=campos[1], titulo=campos[2], pid=int(campos[3]),
            ativa=_booleano(campos[4]), minimizada=_booleano(campos[5]), normal=_booleano(campos[6]),
            monitor=campos[7], frame=tuple(numeros[:4]), cliente=tuple(numeros[4:]),
        )
    except ValueError:
        logger.warning("linha ilegível do script do KWin: %r", linha[:80])
        return None


class Kwin:
    def __init__(self, executor) -> None:
        self._executor = executor

    def listar(self) -> list[Janela]:
        janelas = (_ler_janela(linha) for linha in self._executor.rodar(_JS_LISTAR))
        return [j for j in janelas if j is not None]

    def ativa(self) -> Janela | None:
        return next((j for j in self.listar() if j.ativa), None)

    def telas(self) -> list[Tela]:
        telas = []
        for linha in self._executor.rodar(_JS_TELAS):
            campos = linha.split(SEPARADOR)
            try:
                nome, *numeros = campos
                x, y, largura, altura, escala = (float(n) for n in numeros)
            except ValueError:
                logger.warning("tela ilegível do script do KWin: %r", linha[:80])
                continue
            telas.append(Tela(nome, x, y, largura, altura, escala))
        return telas

    def focar(self, identificador: str) -> bool:
        """Traz a janela para frente (e desminimiza). True se ela ficou ativa."""
        resposta = self._executor.rodar(_js_focar(identificador))
        return resposta == [identificador]

    def fechar(self, identificador: str) -> bool:
        """Pede para fechar, como o X da janela: o app ainda pode perguntar
        se quer salvar. True se a janela existia."""
        return self._executor.rodar(_js_fechar(identificador)) == ["ok"]


def _normalizar(texto: str) -> str:
    sem_acento = unicodedata.normalize("NFKD", texto)
    return "".join(c for c in sem_acento if not unicodedata.combining(c)).casefold().strip()


def por_alvo(janelas: list[Janela], alvo: str) -> list[Janela]:
    """Janelas normais cujo app ou título contém o alvo, sem caixa e sem
    acento. Devolve todas as que casam: escolher entre duas é decisão de
    quem pediu, nunca chute."""
    procurado = _normalizar(alvo)
    if not procurado:
        return []
    return [j for j in janelas if j.normal and (procurado in _normalizar(j.app) or procurado in _normalizar(j.titulo))]


class ExecutorKWin:
    """Carrega o script no KWin, roda, recebe a saída por D-Bus e descarrega.

    Todas as chamadas D-Bus passam pelo laço GLib (laco_glib.py). Cada
    execução tem uma chave própria, então dois scripts ao mesmo tempo não
    misturam as linhas."""

    _INTERFACE = "org.james.Device.KWin"
    _XML = (f"<node><interface name='{_INTERFACE}'><method name='receber'>"
            "<arg type='s' direction='in'/></method></interface></node>")

    def __init__(self, laco, conexao) -> None:
        from gi.repository import Gio

        self._laco = laco
        self._conexao = conexao
        # Caminho único por instância: a conexão com o barramento da sessão é
        # compartilhada, e dois executores no mesmo processo não podem
        # registrar o mesmo objeto.
        self._CAMINHO = "/org/james/kwin/" + uuid.uuid4().hex
        self._pendentes: dict[str, tuple[list[str], threading.Event]] = {}
        self._trava = threading.Lock()
        info = Gio.DBusNodeInfo.new_for_xml(self._XML).interfaces[0]
        self._laco.executar(lambda: conexao.register_object(self._CAMINHO, info, self._receber, None, None))
        self._destino = self._laco.executar(conexao.get_unique_name)

    def _receber(self, conexao, remetente, caminho, interface, metodo, parametros, invocacao) -> None:
        texto = parametros.unpack()[0]
        invocacao.return_value(None)
        chave, _, linha = texto.partition(SEPARADOR)
        with self._trava:
            pendente = self._pendentes.get(chave)
        if pendente is None:
            return
        linhas, fim = pendente
        if linha == _FIM:
            fim.set()
        else:
            linhas.append(linha)

    def _preambulo(self, chave: str) -> str:
        destino, caminho, interface = (json.dumps(v) for v in (self._destino, self._CAMINHO, self._INTERFACE))
        return (f'function devolver(s) {{ callDBus({destino}, {caminho}, {interface}, "receber", '
                f'{json.dumps(chave + SEPARADOR)} + String(s)); }}\n')

    def rodar(self, js: str) -> list[str]:
        from gi.repository import GLib

        chave = uuid.uuid4().hex
        nome = "james-" + chave[:12]
        linhas: list[str] = []
        fim = threading.Event()
        with self._trava:
            self._pendentes[chave] = (linhas, fim)
        descritor, arquivo = tempfile.mkstemp(prefix="james-kwin-", suffix=".js")
        carregado = False
        try:
            with os.fdopen(descritor, "w", encoding="utf-8") as saida:
                saida.write(self._preambulo(chave) + js + f"\ndevolver({json.dumps(_FIM)});\n")

            def carregar_e_rodar():
                nonlocal carregado
                ident = self._conexao.call_sync(
                    "org.kde.KWin", "/Scripting", "org.kde.kwin.Scripting", "loadScript",
                    GLib.Variant("(ss)", (arquivo, nome)), None, 0, 3000, None).unpack()[0]
                carregado = True
                self._conexao.call_sync(
                    "org.kde.KWin", f"/Scripting/Script{ident}", "org.kde.kwin.Script", "run",
                    None, None, 0, 3000, None)

            self._laco.executar(carregar_e_rodar)
            if not fim.wait(TIMEOUT_SCRIPT):
                raise TimeoutError(f"o script do KWin não respondeu em {TIMEOUT_SCRIPT}s")
            return linhas
        finally:
            with self._trava:
                self._pendentes.pop(chave, None)
            if carregado:
                try:
                    self._laco.executar(lambda: self._conexao.call_sync(
                        "org.kde.KWin", "/Scripting", "org.kde.kwin.Scripting", "unloadScript",
                        GLib.Variant("(s)", (nome,)), None, 0, 3000, None))
                except Exception:  # noqa: BLE001 — limpeza; o erro principal (se houver) já está subindo
                    logger.warning("não consegui descarregar o script %s do KWin", nome, exc_info=True)
            os.unlink(arquivo)
