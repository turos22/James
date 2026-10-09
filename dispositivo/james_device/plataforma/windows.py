"""Windows: tudo o que fala com o Win32 (monitores, DPI, janela em foco).

Movido de superficies.py na fatia B1 do porte pro Fedora. Importar este
módulo TEM efeito colateral, deliberado: ativa o DPI awareness per-monitor, o
que só funciona se rodar antes do pyautogui (tela.py e captura.py importam
este módulo antes dele). Nunca importar fora do Windows: ctypes.windll e
WINFUNCTYPE não existem.
"""

import ctypes
import logging
from ctypes import wintypes

from james_device.superficies import Superficie

logger = logging.getLogger(__name__)

_MONITORINFOF_PRIMARY = 1
_MDT_EFFECTIVE_DPI = 0
_DPI_PADRAO = 96.0
_PROCESS_PER_MONITOR_DPI_AWARE = 2


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


# Efeito colateral de import, deliberado: para que a detecção de falha
# funcione, este módulo precisa ser importado ANTES de qualquer código que
# use pyautogui (ver Task 2 do plano da Fase 5) — senão outra biblioteca já
# terá fixado um awareness diferente, e ativar_dpi_awareness() só vai
# conseguir logar o erro, não corrigir. Hoje (Task 1) nada em produção
# importa este módulo ainda.
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
ctypes.windll.shcore.GetDpiForMonitor.argtypes = [
    wintypes.HANDLE, ctypes.c_int, ctypes.POINTER(ctypes.c_uint), ctypes.POINTER(ctypes.c_uint)
]


def _escala_de(handle_monitor) -> float:
    """Escala do monitor (1.25 para 125%). Cai no padrão de 1.0 se o shcore
    não responder — monitor com escala desconhecida é melhor tratado como
    100% do que fazer a enumeração inteira falhar, mas o log garante que essa
    degradação não passe despercebida (mesma lição do Critical de
    ativar_dpi_awareness, aplicada aqui: HRESULT não levanta, tem que ser
    checado)."""
    dpi_x = ctypes.c_uint()
    dpi_y = ctypes.c_uint()
    try:
        hr = ctypes.windll.shcore.GetDpiForMonitor(
            handle_monitor, _MDT_EFFECTIVE_DPI, ctypes.byref(dpi_x), ctypes.byref(dpi_y)
        )
    except Exception:
        logger.warning("GetDpiForMonitor levantou exceção para o handle %s, assumindo escala 100%%", handle_monitor)
        return 1.0
    if hr != 0 or dpi_x.value == 0:
        logger.warning("GetDpiForMonitor falhou (hr=%#x) para o handle %s, assumindo escala 100%%", hr & 0xFFFFFFFF, handle_monitor)
        return 1.0
    return dpi_x.value / _DPI_PADRAO


def listar_superficies() -> list[Superficie]:
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


def handle_janela_em_foco() -> int:
    """Handle cru da janela em foco (0 se não houver). Vive junto com o
    retângulo (retangulo_da_janela_em_foco) porque as duas perguntas são o
    mesmo GetForegroundWindow: o retângulo diz ONDE a janela está, o handle
    diz QUEM ela é, e Mapa precisa do handle pra identidade."""
    return int(ctypes.windll.user32.GetForegroundWindow())
