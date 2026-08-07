"""Geometria de monitor: enumera as superfícies (D14 da spec da Fase 5),
converte coordenada local↔global (D15) e decide qual superfície contém uma
janela.

Único lugar do device que conhece origem e escala de monitor — captura.py,
elementos.py e tela.py perguntam aqui em vez de calcular por conta.

Divisão importante: listar() fala com o SO e não tem teste automatizado;
tudo abaixo dela é aritmética pura sobre uma lista de Superficie recebida
por parâmetro, e é testável sem hardware.
"""

import ctypes
import logging
from ctypes import wintypes
from dataclasses import dataclass

logger = logging.getLogger(__name__)

_MONITORINFOF_PRIMARY = 1
_MDT_EFFECTIVE_DPI = 0
_DPI_PADRAO = 96.0
_PROCESS_PER_MONITOR_DPI_AWARE = 2


@dataclass(frozen=True)
class Superficie:
    """Um monitor. `origem_x`/`origem_y` são a posição do canto superior
    esquerdo no espaço global do desktop; `escala` é 1.0 para 100%."""

    id: str
    largura: int
    altura: int
    origem_x: int
    origem_y: int
    escala: float
    primaria: bool


def ativar_dpi_awareness() -> None:
    """Declara o processo como per-monitor DPI aware. Precisa rodar antes de
    qualquer uso do pyautogui: sem isso, numa tela a 150% o Windows mente a
    resolução (reporta 1280x720 numa tela de 1920x1080) e todo clique cai
    deslocado.

    SetProcessDpiAwareness DEVOLVE um HRESULT, não levanta — chamar depois
    que outra biblioteca (ex: pyautogui) já setou um awareness diferente
    falha em silêncio (E_ACCESSDENIED) sem nenhuma exceção pra pegar. Por
    isso a confirmação é feita relendo o estado real com
    GetProcessDpiAwareness, não confiando em ausência de exceção."""
    try:
        ctypes.windll.shcore.SetProcessDpiAwareness(_PROCESS_PER_MONITOR_DPI_AWARE)
    except Exception:
        try:
            ctypes.windll.user32.SetProcessDPIAware()
        except Exception:
            logger.warning("não consegui ativar DPI awareness — coordenadas podem sair deslocadas em tela escalada")
            return

    atual = ctypes.c_int()
    try:
        ctypes.windll.shcore.GetProcessDpiAwareness(None, ctypes.byref(atual))
    except Exception:
        return
    if atual.value != _PROCESS_PER_MONITOR_DPI_AWARE:
        logger.error(
            "DPI awareness ficou em %s, não per-monitor (%s) — provavelmente outra biblioteca "
            "(ex: pyautogui) já tinha setado um awareness diferente antes deste import. "
            "Coordenadas em monitor escalado vão sair erradas.",
            atual.value, _PROCESS_PER_MONITOR_DPI_AWARE,
        )


# Efeito colateral de import, deliberado: DPI awareness precisa valer antes
# do primeiro uso do pyautogui, e qualquer módulo que mexa com tela importa
# este aqui.
ativar_dpi_awareness()


class _RECT(ctypes.Structure):
    _fields_ = [
        ("left", ctypes.c_long),
        ("top", ctypes.c_long),
        ("right", ctypes.c_long),
        ("bottom", ctypes.c_long),
    ]


class _MONITORINFOEXW(ctypes.Structure):
    _fields_ = [
        ("cbSize", ctypes.c_ulong),
        ("rcMonitor", _RECT),
        ("rcWork", _RECT),
        ("dwFlags", ctypes.c_ulong),
        ("szDevice", ctypes.c_wchar * 32),
    ]


_MONITORENUMPROC = ctypes.WINFUNCTYPE(
    ctypes.c_int,
    wintypes.HANDLE,
    wintypes.HANDLE,
    ctypes.POINTER(_RECT),
    wintypes.LPARAM,
)

# Sem argtypes/restype explícitos, ctypes assume o default c_long (32 bits)
# pro retorno — um handle de janela de 64 bits pode truncar silenciosamente.
ctypes.windll.user32.GetForegroundWindow.restype = wintypes.HWND
ctypes.windll.user32.GetWindowRect.argtypes = [wintypes.HWND, ctypes.POINTER(_RECT)]
ctypes.windll.user32.GetMonitorInfoW.argtypes = [wintypes.HANDLE, ctypes.POINTER(_MONITORINFOEXW)]


def _escala_de(handle_monitor) -> float:
    """Escala do monitor (1.25 para 125%). Cai no padrão de 1.0 se o shcore
    não responder — monitor com escala desconhecida é melhor tratado como
    100% do que fazer a enumeração inteira falhar."""
    dpi_x = ctypes.c_uint()
    dpi_y = ctypes.c_uint()
    try:
        ctypes.windll.shcore.GetDpiForMonitor(
            handle_monitor, _MDT_EFFECTIVE_DPI, ctypes.byref(dpi_x), ctypes.byref(dpi_y)
        )
    except Exception:
        return 1.0
    if dpi_x.value == 0:
        return 1.0
    return dpi_x.value / _DPI_PADRAO


def listar() -> list[Superficie]:
    """Enumera os monitores conectados. Fala com o SO — sem teste
    automatizado, coberto por verificação ao vivo (CA4/CA8)."""
    encontradas: list[Superficie] = []

    def callback(handle_monitor, _hdc, _lprect, _lparam):
        info = _MONITORINFOEXW()
        info.cbSize = ctypes.sizeof(_MONITORINFOEXW)
        if not ctypes.windll.user32.GetMonitorInfoW(handle_monitor, ctypes.byref(info)):
            return 1
        retangulo = info.rcMonitor
        encontradas.append(
            Superficie(
                id=info.szDevice.replace("\\\\.\\", ""),
                largura=retangulo.right - retangulo.left,
                altura=retangulo.bottom - retangulo.top,
                origem_x=retangulo.left,
                origem_y=retangulo.top,
                escala=_escala_de(handle_monitor),
                primaria=bool(info.dwFlags & _MONITORINFOF_PRIMARY),
            )
        )
        return 1

    resultado = ctypes.windll.user32.EnumDisplayMonitors(None, None, _MONITORENUMPROC(callback), 0)
    if not resultado:
        logger.error("EnumDisplayMonitors falhou — nenhum monitor será reportado")
    return encontradas


def retangulo_da_janela_em_foco() -> tuple[int, int, int, int] | None:
    """Retângulo global da janela em foco, ou None se não houver.
    Fala com o SO — sem teste automatizado."""
    handle = ctypes.windll.user32.GetForegroundWindow()
    if not handle:
        return None
    retangulo = _RECT()
    if not ctypes.windll.user32.GetWindowRect(handle, ctypes.byref(retangulo)):
        return None
    return (retangulo.left, retangulo.top, retangulo.right, retangulo.bottom)


def para_global(superficie: Superficie, x: int, y: int) -> tuple[int, int]:
    """Coordenada local da superfície para coordenada global do desktop.
    Esquecer esta soma faz o clique cair no monitor errado — e o feedback de
    tela.py reporta com honestidade o elemento errado, o que torna o bug
    plausível e difícil de rastrear."""
    return (superficie.origem_x + x, superficie.origem_y + y)


def para_local(superficie: Superficie, x: int, y: int) -> tuple[int, int]:
    """Inverso de para_global."""
    return (x - superficie.origem_x, y - superficie.origem_y)


def contendo(superficies: list[Superficie], x: int, y: int) -> Superficie | None:
    """Qual superfície contém o ponto global, ou None. O None é real e
    acontece: com monitores desalinhados, existem pontos dentro do retângulo
    que engloba tudo mas fora de qualquer monitor."""
    for superficie in superficies:
        dentro_x = superficie.origem_x <= x < superficie.origem_x + superficie.largura
        dentro_y = superficie.origem_y <= y < superficie.origem_y + superficie.altura
        if dentro_x and dentro_y:
            return superficie
    return None


def por_id(superficies: list[Superficie], identificador: str) -> Superficie | None:
    """Busca por id (ex: "DISPLAY1")."""
    for superficie in superficies:
        if superficie.id == identificador:
            return superficie
    return None


def primaria(superficies: list[Superficie]) -> Superficie:
    """A superfície primária, ou a primeira da lista se nenhuma se declarar.
    Levanta se a lista vier vazia — não devolve None, porque quem chama vai
    capturar alguma tela de qualquer jeito, e não ter resposta aqui só
    empurra o problema pra mais longe da causa real."""
    if not superficies:
        raise RuntimeError("nenhuma superfície disponível — listar() não encontrou monitor nenhum")
    for superficie in superficies:
        if superficie.primaria:
            return superficie
    return superficies[0]


def da_janela(superficies: list[Superficie], retangulo: tuple[int, int, int, int]) -> Superficie:
    """Superfície que contém o centro do retângulo da janela (regra de R6).
    Determinística e barata; conhecidamente incompleta para janela cortada
    quase ao meio entre dois monitores. Centro na área morta cai na
    primária."""
    esquerda, topo, direita, base = retangulo
    centro_x = (esquerda + direita) // 2
    centro_y = (topo + base) // 2
    return contendo(superficies, centro_x, centro_y) or primaria(superficies)


def alvo_padrao(superficies: list[Superficie], retangulo_em_foco: tuple[int, int, int, int] | None) -> Superficie:
    """A superfície que `ver_tela` sem argumento captura (D14): a que contém
    a janela em foco, ou a primária se não houver janela em foco."""
    if retangulo_em_foco is None:
        return primaria(superficies)
    return da_janela(superficies, retangulo_em_foco)
