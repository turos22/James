"""Captura de um monitor pelo ScreenCast do portal (D9 da spec 2026-10-13 B2)."""

import os
import struct
import sys

import pytest

if sys.platform != "linux":
    pytest.skip("captura pelo PipeWire só no Linux", allow_module_level=True)

from james_device.captura_linux import capturar, montar_pipeline  # noqa: E402


def test_pipeline_le_o_fluxo_reduz_e_codifica_png_na_memoria():
    texto = montar_pipeline(fd=17, node=138, largura=1280, altura=720)

    assert "pipewiresrc fd=17 path=138 num-buffers=2 always-copy=true" in texto
    assert "videoscale" in texto
    assert "video/x-raw,format=RGB,width=1280,height=720" in texto
    assert "pngenc snapshot=true" in texto
    assert "appsink name=saida" in texto
    assert "filesink" not in texto  # a tela do Arthur não toca o disco


def test_pipewire_que_nao_responde_vira_erro_dentro_do_prazo():
    # Descritor que não é um PipeWire: o set_state(PLAYING) do pipewiresrc
    # TRAVA esperando conectar (achado ao escrever este teste). Um descritor
    # velho (sessão do portal fechada, PipeWire reiniciado) faria o mesmo e
    # congelaria a ponte de tela inteira, então o prazo é obrigatório.
    import time

    descritor = os.open(os.devnull, os.O_RDONLY)
    inicio = time.monotonic()
    try:
        with pytest.raises(RuntimeError, match="captura falhou"):
            capturar(descritor, node=999_001, largura=64, altura=36, timeout=1)
    finally:
        os.close(descritor)
    assert time.monotonic() - inicio < 3


def test_captura_presa_num_fluxo_nao_bloqueia_outro_fluxo():
    descritor = os.open(os.devnull, os.O_RDONLY)
    try:
        with pytest.raises(RuntimeError):
            capturar(descritor, node=999_002, largura=64, altura=36, timeout=1)
        # O mesmo fluxo continua preso: recusa na hora, sem empilhar threads.
        with pytest.raises(RuntimeError, match="anterior ainda está presa"):
            capturar(descritor, node=999_002, largura=64, altura=36, timeout=1)
        # Outro fluxo tenta de verdade (e aqui falha pelo prazo, não pela trava).
        with pytest.raises(RuntimeError, match="não respondeu"):
            capturar(descritor, node=999_003, largura=64, altura=36, timeout=1)
    finally:
        os.close(descritor)


@pytest.mark.skipif(os.environ.get("JAMES_TESTE_DESKTOP") != "1",
                    reason="defina JAMES_TESTE_DESKTOP=1 para capturar a tela de verdade")
def test_ao_vivo_captura_cada_monitor_em_1280_sem_quadro_preto():
    import time

    from gi.repository import Gio

    from james_device.kwin import ExecutorKWin, Kwin
    from james_device.laco_glib import LacoGlib
    from james_device.portal import CanalPortal, Portal, Token, caminho_do_token, casar_fluxos
    from james_device.superficies import tamanho_da_imagem

    laco = LacoGlib()
    laco.iniciar()
    try:
        conexao = laco.executar(lambda: Gio.bus_get_sync(Gio.BusType.SESSION, None))
        kwin = Kwin(ExecutorKWin(laco, conexao))
        portal = Portal(CanalPortal(laco, conexao), Token(caminho_do_token()))
        portal.abrir()
        assert portal.estado == "aberto"
        fluxos = casar_fluxos(portal.fluxos, kwin.telas())
        for tela in kwin.telas():
            largura, altura, _ = tamanho_da_imagem(int(tela.largura), int(tela.altura))
            descritor = portal.abrir_pipewire()
            inicio = time.monotonic()
            png = capturar(descritor, fluxos[tela.nome].node, largura, altura)
            duracao = time.monotonic() - inicio
            assert png[:8] == b"\x89PNG\r\n\x1a\n"
            assert struct.unpack(">II", png[16:24]) == (largura, altura)
            # PNG de quadro preto de 1280x720 tem poucos KiB; tela de verdade, dezenas.
            assert len(png) > 20_000, f"{tela.nome}: {len(png)} bytes, parece quadro vazio"
            print(f"{tela.nome}: {largura}x{altura}, {len(png) // 1024} KiB em {duracao:.3f}s")
    finally:
        laco.encerrar()
