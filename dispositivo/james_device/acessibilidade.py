"""Elementos da tela pelo AT-SPI, no Linux (D7 e D11 da spec 2026-10-13 B2).

É o equivalente do elementos.varrer do Windows (uiautomation): a posição de
cada botão sai do sistema, não de um palpite do modelo. O resto da cadeia
(Elemento, Mapa, formatar) é o mesmo de elementos.py.

Três coisas que o Wayland muda, todas medidas no spike:
- O app NÃO sabe onde a própria janela está: o retângulo do AT-SPI é
  relativo à janela, e a origem vem do clientGeometry do KWin. A exceção é o
  plasmashell (painel), que já dá coordenada global.
- O AT-SPI não sabe se uma janela está coberta por outra. Por isso cada
  elemento guarda a janela dona (`donos`), e quem clica foca a dona antes.
- App aberto antes de ligar a acessibilidade não monta a árvore; VS Code e
  Spotify não montam nunca. `em_foco_sem_elementos` é o que permite ao
  ver_tela avisar em vez de dizer "não há nada aqui".

Fronteira: o Atspi só é tocado em NoAtspi e listar_apps. Erro de leitura de
um nó (GLib.Error, o app fechou no meio) vira NoIlegivel e pula o nó; erro
de programação (AttributeError, TypeError) NÃO é engolido: no spike, um
`except` largo escondeu um AttributeError e a varredura pulou a árvore
inteira em silêncio.
"""

import logging
import time
from dataclasses import dataclass, field

from james_device import superficies
from james_device.elementos import Elemento, Mapa

logger = logging.getLogger(__name__)

TETO_TEMPO = 0.4
TETO_NOS = 4000
_TOLERANCIA_PX = 2

_TIPOS = {
    "button": "Button", "push button": "Button", "push button menu": "Button", "toggle button": "Button",
    "entry": "Edit", "password text": "Edit", "spin button": "Edit",
    "combo box": "ComboBox",
    "link": "Hyperlink",
    "menu item": "MenuItem", "check menu item": "MenuItem", "radio menu item": "MenuItem",
    "page tab": "TabItem",
    "check box": "CheckBox",
    "radio button": "RadioButton",
    "list item": "ListItem", "table cell": "ListItem",
    "tree item": "TreeItem",
    "slider": "Slider",
}
_APPS_COM_COORDENADA_GLOBAL = frozenset({"plasmashell"})


class NoIlegivel(Exception):
    """O nó não pôde ser lido (sumiu, o app fechou). Pula o nó."""


def tipo_exibido(papel: str, editavel: bool) -> str | None:
    """Papel do AT-SPI → tipo no vocabulário do Windows (sem o sufixo
    Control), ou None se o papel não interessa. "text" é rótulo estático na
    maior parte das vezes; só conta quando é editável."""
    if papel == "text":
        return "Edit" if editavel else None
    return _TIPOS.get(papel)


def casar_janela(titulo: str, pid: int, janelas) -> object | None:
    """Janela do KWin que corresponde a uma janela do AT-SPI. Pid + título;
    só título (app flatpak tem o pid do sandbox no AT-SPI); só pid, quando o
    app tem uma janela só. Minimizada não conta: não está na tela."""
    candidatas = [j for j in janelas if j.normal and not j.minimizada]
    for criterio in (
        lambda j: j.pid == pid and j.titulo == titulo,
        lambda j: j.titulo == titulo,
    ):
        achadas = [j for j in candidatas if criterio(j)]
        if achadas:
            return achadas[0]
    do_pid = [j for j in candidatas if j.pid == pid]
    return do_pid[0] if len(do_pid) == 1 else None


@dataclass
class Dono:
    no: object
    janela_id: str | None
    coordenada_global: bool


@dataclass
class Varredura:
    mapa: Mapa
    donos: dict[str, Dono] = field(default_factory=dict)
    avisos: list[str] = field(default_factory=list)
    ignorados: int = 0
    cortada: bool = False
    em_foco_sem_elementos: bool = False


def varrer(apps, janelas, superficie, ativa_id: str | None,
           relogio=time.monotonic, teto_tempo: float = TETO_TEMPO, teto_nos: int = TETO_NOS) -> Varredura:
    resultado = Varredura(mapa=Mapa(superficie_id=superficie.id, janela_em_foco=ativa_id))

    # (em_foco, raiz, janela_id, origem, global) de cada janela de app que dá
    # para pôr na tela. A janela ativa vai primeiro: o corte por tempo é real.
    alvos = []
    for app in apps:
        try:
            nome_app, pid, janelas_do_app = app.nome(), app.pid(), app.filhos()
        except NoIlegivel:
            resultado.ignorados += 1
            continue
        for raiz in janelas_do_app:
            if nome_app in _APPS_COM_COORDENADA_GLOBAL:
                alvos.append((False, raiz, None, (0.0, 0.0), True))
                continue
            try:
                titulo = raiz.nome()
            except NoIlegivel:
                resultado.ignorados += 1
                continue
            janela = casar_janela(titulo, pid, janelas)
            if janela is None:
                continue
            alvos.append((janela.id == ativa_id, raiz, janela.id, janela.cliente[:2], False))
    alvos.sort(key=lambda alvo: not alvo[0])

    inicio = relogio()
    visitados = 0
    algum_em_foco = any(alvo[0] for alvo in alvos)

    def estourou() -> bool:
        return visitados >= teto_nos or relogio() - inicio > teto_tempo

    for em_foco, raiz, janela_id, (ox, oy), global_ in alvos:
        pilha = [raiz]
        while pilha:
            if estourou():
                resultado.cortada = True
                break
            no = pilha.pop()
            visitados += 1
            try:
                estados = no.estados()
                if "showing" not in estados:
                    continue  # nó escondido não tem filho visível na prática
                tipo = tipo_exibido(no.papel(), "editable" in estados)
                if tipo is not None:
                    x, y, largura, altura = no.retangulo()
                    if largura > 0 and altura > 0:
                        bx, by = (0.0, 0.0) if global_ else (ox, oy)
                        retangulo = (round(bx + x), round(by + y), round(bx + x + largura), round(by + y + altura))
                        centro = ((retangulo[0] + retangulo[2]) // 2, (retangulo[1] + retangulo[3]) // 2)
                        if superficies.contendo([superficie], *centro) is not None:
                            identificador = f"e{len(resultado.mapa.elementos) + 1}"
                            resultado.mapa.elementos.append(Elemento(
                                id=identificador, nome=no.nome(), tipo=tipo, retangulo=retangulo, em_foco=em_foco))
                            resultado.donos[identificador] = Dono(no, janela_id, global_)
                pilha.extend(reversed(no.filhos()))
            except NoIlegivel:
                resultado.ignorados += 1
        if resultado.cortada:
            break

    if resultado.cortada:
        resultado.avisos.append(
            f"varredura cortada em {visitados} nós / {relogio() - inicio:.2f}s: a lista pode não ter tudo "
            "(a janela ativa vem primeiro)")
    if resultado.ignorados:
        logger.warning("varredura pulou %d nós ilegíveis (%d elementos)", resultado.ignorados,
                       len(resultado.mapa.elementos))
    resultado.em_foco_sem_elementos = algum_em_foco and not any(e.em_foco for e in resultado.mapa.elementos)
    if not algum_em_foco and ativa_id is not None:
        resultado.em_foco_sem_elementos = True  # a janela ativa nem aparece no AT-SPI
    return resultado


def revalidar(elemento: Elemento, dono: Dono, origem: tuple[float, float]) -> str | None:
    """Confere o elemento contra a árvore VIVA antes do clique. Devolve o
    motivo da recusa, ou None. `origem` é a origem do cliente da janela
    dona AGORA (o KWin relido na hora do clique)."""
    try:
        estados = dono.no.estados()
        if "showing" not in estados:
            return "o elemento não está mais visível"
        if "enabled" not in estados:
            return "o elemento está desabilitado"
        x, y, largura, altura = dono.no.retangulo()
    except NoIlegivel:
        return "o elemento não está mais na tela"
    ox, oy = (0.0, 0.0) if dono.coordenada_global else origem
    atual = (round(ox + x), round(oy + y), round(ox + x + largura), round(oy + y + altura))
    if max(abs(a - b) for a, b in zip(atual, elemento.retangulo)) > _TOLERANCIA_PX:
        return "o elemento mudou de lugar — a tela mudou desde o ver_tela"
    return None


def ligar(propriedade) -> bool:
    """Liga a acessibilidade do desktop (D11). Devolve se já estava ligada:
    se não estava, apps abertos antes precisam ser reabertos para montar a
    árvore. `propriedade(acao, valor)` lê ou grava org.a11y.Status.IsEnabled."""
    if propriedade("ler"):
        return True
    propriedade("gravar", True)
    return False


def propriedade_da_acessibilidade(laco, conexao):
    """A `propriedade` de verdade para `ligar`: D-Bus pelo laço GLib."""
    from gi.repository import GLib

    def propriedade(acao, valor=None):
        if acao == "ler":
            resposta = laco.executar(lambda: conexao.call_sync(
                "org.a11y.Bus", "/org/a11y/bus", "org.freedesktop.DBus.Properties", "Get",
                GLib.Variant("(ss)", ("org.a11y.Status", "IsEnabled")), None, 0, 3000, None))
            return bool(resposta.unpack()[0])
        laco.executar(lambda: conexao.call_sync(
            "org.a11y.Bus", "/org/a11y/bus", "org.freedesktop.DBus.Properties", "Set",
            GLib.Variant("(ssv)", ("org.a11y.Status", "IsEnabled", GLib.Variant("b", valor))),
            None, 0, 3000, None))
        return None

    return propriedade


class NoAtspi:
    """Adaptador fino sobre Atspi.Accessible. Só GLib.Error vira NoIlegivel."""

    def __init__(self, acessivel) -> None:
        self.acessivel = acessivel

    def _ler(self, funcao):
        from gi.repository import GLib

        try:
            return funcao()
        except GLib.Error as erro:
            raise NoIlegivel(erro.message) from erro

    def papel(self) -> str:
        return self._ler(self.acessivel.get_role_name)

    def nome(self) -> str:
        return self._ler(self.acessivel.get_name) or ""

    def pid(self) -> int:
        return self._ler(self.acessivel.get_process_id)

    def estados(self) -> set[str]:
        from gi.repository import Atspi

        conjunto = self._ler(self.acessivel.get_state_set)
        return {nome for nome, estado in (("showing", Atspi.StateType.SHOWING), ("enabled", Atspi.StateType.ENABLED),
                                          ("editable", Atspi.StateType.EDITABLE), ("focused", Atspi.StateType.FOCUSED))
                if conjunto.contains(estado)}

    def retangulo(self) -> tuple[int, int, int, int]:
        from gi.repository import Atspi

        r = self._ler(lambda: self.acessivel.get_extents(Atspi.CoordType.WINDOW))
        return (r.x, r.y, r.width, r.height)

    def filhos(self) -> list["NoAtspi"]:
        quantos = self._ler(self.acessivel.get_child_count)
        filhos = []
        for i in range(quantos):
            filho = self._ler(lambda: self.acessivel.get_child_at_index(i))
            if filho is not None:
                filhos.append(NoAtspi(filho))
        return filhos


def listar_apps() -> list[NoAtspi]:
    import gi

    gi.require_version("Atspi", "2.0")
    from gi.repository import Atspi

    desktop = Atspi.get_desktop(0)
    return NoAtspi(desktop).filhos()
