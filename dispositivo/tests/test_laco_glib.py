"""Laço GLib numa thread própria (D12 da spec 2026-10-13 B2).

Roda com GLib de verdade e, nos testes de sinal, com um barramento D-Bus
privado (Gio.TestDBus): nada aqui toca no desktop do Arthur.
"""

import sys
import threading
import time

import pytest

if sys.platform != "linux":
    pytest.skip("laço GLib só no Linux", allow_module_level=True)

from gi.repository import Gio, GLib  # noqa: E402

from james_device.laco_glib import LacoGlib  # noqa: E402


@pytest.fixture
def laco():
    laco = LacoGlib()
    laco.iniciar()
    yield laco
    laco.encerrar()


@pytest.fixture
def barramento():
    teste = Gio.TestDBus.new(Gio.TestDBusFlags.NONE)
    teste.up()
    endereco = teste.get_bus_address()
    yield endereco
    teste.down()


def _conectar(endereco):
    return Gio.DBusConnection.new_for_address_sync(
        endereco,
        Gio.DBusConnectionFlags.AUTHENTICATION_CLIENT | Gio.DBusConnectionFlags.MESSAGE_BUS_CONNECTION,
        None,
        None,
    )


def test_executar_roda_na_thread_do_laco_e_devolve_o_valor(laco):
    quem_chamou = threading.get_ident()

    resultado = laco.executar(lambda: (threading.get_ident(), 42))

    thread_do_laco, valor = resultado
    assert valor == 42
    assert thread_do_laco != quem_chamou
    assert thread_do_laco == laco.ident


def test_excecao_chega_a_quem_chamou_com_o_tipo_original(laco):
    def falhar():
        raise ValueError("deu ruim")

    with pytest.raises(ValueError, match="deu ruim"):
        laco.executar(falhar)


def test_funcao_lenta_estoura_o_timeout_e_o_laco_atende_depois(laco):
    inicio = time.monotonic()
    with pytest.raises(TimeoutError):
        laco.executar(lambda: time.sleep(0.3), timeout=0.05)
    assert time.monotonic() - inicio < 0.2, "quem chamou não pode esperar a função lenta acabar"

    # A função lenta trava o laço enquanto roda (é uma thread só); quando ela
    # termina, o laço volta a atender.
    assert laco.executar(lambda: "de volta", timeout=2) == "de volta"


def test_executar_antes_de_iniciar_e_erro_claro():
    laco = LacoGlib()
    with pytest.raises(RuntimeError, match="não foi iniciado"):
        laco.executar(lambda: None)


def test_encerrar_para_a_thread():
    laco = LacoGlib()
    laco.iniciar()
    laco.encerrar()
    assert not laco.vivo()
    with pytest.raises(RuntimeError):
        laco.executar(lambda: None)


def test_sinal_assinado_no_laco_chega_e_quem_espera_recebe_os_parametros(laco, barramento):
    ouvinte = laco.executar(lambda: _conectar(barramento))
    emissor = _conectar(barramento)

    espera = laco.assinar_sinal(ouvinte, None, "org.james.Teste", "Pronto", "/org/james/teste")
    emissor.emit_signal(None, "/org/james/teste", "org.james.Teste", "Pronto", GLib.Variant("(us)", (0, "ok")))
    emissor.flush_sync(None)

    assert espera.aguardar(timeout=2) == (0, "ok")


def test_aguardar_sinal_que_nao_vem_estoura_o_timeout(laco, barramento):
    ouvinte = laco.executar(lambda: _conectar(barramento))

    espera = laco.assinar_sinal(ouvinte, None, "org.james.Teste", "Nunca", "/org/james/teste")

    with pytest.raises(TimeoutError):
        espera.aguardar(timeout=0.1)


def test_aguardar_cancela_a_assinatura(laco, barramento):
    ouvinte = laco.executar(lambda: _conectar(barramento))
    emissor = _conectar(barramento)
    espera = laco.assinar_sinal(ouvinte, None, "org.james.Teste", "Pronto", "/org/james/teste")
    emissor.emit_signal(None, "/org/james/teste", "org.james.Teste", "Pronto", GLib.Variant("(us)", (0, "a")))
    emissor.flush_sync(None)
    espera.aguardar(timeout=2)

    assert espera.assinatura is None
