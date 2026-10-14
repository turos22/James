"""Linux (Fedora, KDE, Wayland): voz e, desde a fatia B2, tela.

A tela liga só com sessão Wayland + KDE Plasma + PyGObject (spec 2026-10-13
B2): é o que foi medido e provado. Em qualquer outro caso (X11, GNOME, sem
`gi`), a ponte responde "indisponível" com o motivo, e a voz segue.

Ao montar a ponte, o device liga a acessibilidade (D11), prende a libatspi
ao laço GLib (o cache da árvore depende disso) e abre a sessão do portal EM
SEGUNDO PLANO: sem permissão guardada, o KDE mostra um aviso e espera o
Arthur clicar, e isso não pode segurar a voz (D5a).
"""

import logging
import os
import subprocess
import threading

logger = logging.getLogger(__name__)

MENSAGEM_TELA_INDISPONIVEL = "tela indisponível neste sistema (Linux)"


def _importar_gi() -> None:
    import gi

    gi.require_version("Atspi", "2.0")
    gi.require_version("Gst", "1.0")
    from gi.repository import Atspi, Gio, GLib, Gst  # noqa: F401


def motivo_sem_tela(env=os.environ, importar_gi=_importar_gi) -> str | None:
    """None quando dá para ligar a tela; senão, o motivo, para o log e para
    a resposta ao modelo."""
    sessao = env.get("XDG_SESSION_TYPE", "")
    if sessao != "wayland":
        return f"a sessão não é Wayland (XDG_SESSION_TYPE={sessao or 'vazio'})"
    desktops = [d.strip().upper() for d in env.get("XDG_CURRENT_DESKTOP", "").split(":")]
    if "KDE" not in desktops:
        return f"o desktop não é o KDE Plasma (XDG_CURRENT_DESKTOP={env.get('XDG_CURRENT_DESKTOP') or 'vazio'})"
    try:
        importar_gi()
    except (ImportError, ValueError) as erro:
        return f"PyGObject (gi) indisponível: {erro}"
    return None


MOTIVO_SEM_TELA = motivo_sem_tela()
TELA_DISPONIVEL = MOTIVO_SEM_TELA is None


def tocar_som(caminho: str) -> None:
    """Bloqueia até o som acabar; quem chama já roda numa thread
    (main.tocar_em_paralelo). pw-play lê via libsndfile, que toca WAV e MP3."""
    try:
        subprocess.run(["pw-play", caminho], check=False)
    except FileNotFoundError:
        logger.warning("pw-play não encontrado, som %s não tocou (pipewire-utils instalado?)", caminho)


def _recusar(motivo: str):
    mensagem = f"{MENSAGEM_TELA_INDISPONIVEL}: {motivo}"

    def recusar(dados: dict) -> dict:
        return {"erro": mensagem}

    return recusar


def _montar_ponte():
    from gi.repository import Gio

    from james_device import acessibilidade
    from james_device.abrir import Abrir, LancadorGio
    from james_device.kwin import ExecutorKWin, Kwin
    from james_device.laco_glib import LacoGlib
    from james_device.midia import BarramentoMpris, Midia
    from james_device.portal import CanalPortal, Portal, Token, caminho_do_token
    from james_device.tela_linux import TelaLinux
    from james_device.texto import AreaKlipper

    laco = LacoGlib()
    laco.iniciar()
    acessibilidade.vincular_ao_laco(laco)
    conexao = laco.executar(lambda: Gio.bus_get_sync(Gio.BusType.SESSION, None))
    if not acessibilidade.ligar(acessibilidade.propriedade_da_acessibilidade(laco, conexao)):
        logger.warning("liguei a acessibilidade do desktop agora: apps abertos ANTES do James precisam ser "
                       "reabertos para mostrar botões e campos")
    portal = Portal(CanalPortal(laco, conexao), Token(caminho_do_token()))
    threading.Thread(target=portal.abrir, name="portal-abrir", daemon=True).start()
    tela = TelaLinux(laco=laco, kwin=Kwin(ExecutorKWin(laco, conexao)), portal=portal,
                     area=AreaKlipper(laco, conexao), midia=Midia(BarramentoMpris(laco, conexao)),
                     abrir=Abrir(LancadorGio()))
    logger.info("ponte de tela do KDE Wayland montada")
    return tela.executar_acao


def ponte_de_tela():
    if not TELA_DISPONIVEL:
        return _recusar(MOTIVO_SEM_TELA)
    try:
        return _montar_ponte()
    except Exception as erro:  # noqa: BLE001 — sem tela, a voz segue
        logger.exception("a ponte de tela não montou; a voz segue")
        return _recusar(f"a ponte não montou ({erro})")
