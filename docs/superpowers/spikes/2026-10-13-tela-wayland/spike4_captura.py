"""Spike B2 ver_tela: um quadro de UM monitor pelo ScreenCast da sessão do portal.

Sem diálogo novo: a sessão do RemoteDesktop já inclui o ScreenCast dos monitores.
    /usr/bin/python3 docs/superpowers/spikes/2026-10-13-tela-wayland/spike4_captura.py
"""
import os
import struct
import sys
import tempfile
import time

import gi

gi.require_version("Gst", "1.0")
from gi.repository import Gio, GLib, Gst  # noqa: E402

sys.path.insert(0, __file__.rsplit("/", 1)[0])
import spike2_portal_entrada as s2  # noqa: E402


def abrir_pipewire(sessao) -> int:
    resultado, fds = s2.BUS.call_with_unix_fd_list_sync(
        s2.PORTAL, s2.CAMINHO_PORTAL, "org.freedesktop.portal.ScreenCast", "OpenPipeWireRemote",
        GLib.Variant("(oa{sv})", (sessao, {})), None, 0, 5000, None, None)
    return fds.get(resultado.unpack()[0])


def capturar(fd: int, no: int, destino: str) -> float:
    inicio = time.monotonic()
    pipeline = Gst.parse_launch(
        f"pipewiresrc fd={os.dup(fd)} path={no} num-buffers=2 always-copy=true ! videoconvert ! "
        f"video/x-raw,format=RGB ! pngenc snapshot=true ! filesink location={destino}")
    pipeline.set_state(Gst.State.PLAYING)
    msg = pipeline.get_bus().timed_pop_filtered(10 * Gst.SECOND, Gst.MessageType.EOS | Gst.MessageType.ERROR)
    pipeline.set_state(Gst.State.NULL)
    if msg is None or msg.type == Gst.MessageType.ERROR:
        raise RuntimeError(f"captura falhou: {msg.parse_error() if msg else 'timeout'}")
    return time.monotonic() - inicio


def main():
    Gst.init(None)
    sessao, fluxos = s2.abrir_sessao()
    fd = abrir_pipewire(sessao)
    for no, px, py, lx, ly in fluxos:
        for tentativa in range(3):
            with tempfile.NamedTemporaryFile(suffix=".png", delete=False) as f:
                caminho = f.name
            dur = capturar(fd, no, caminho)
            with open(caminho, "rb") as f:
                largura, altura = struct.unpack(">II", f.read(24)[16:24])
            tamanho = os.path.getsize(caminho)
            os.unlink(caminho)
            print(f"fluxo {no} (tela em {px},{py} lógico {lx}x{ly}): {dur:.3f}s, png {largura}x{altura}, {tamanho // 1024} KiB")


if __name__ == "__main__":
    main()
