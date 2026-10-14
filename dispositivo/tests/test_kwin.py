"""Janelas pelo script do KWin (D2 da spec 2026-10-13 B2).

A lógica (montar o JS, ler a saída, casar alvo) é testada com um executor
falso. O caminho real (carregar o script no KWin e receber a saída por
D-Bus) só roda com JAMES_TESTE_DESKTOP=1.
"""

import os
import subprocess
import sys
import time

import pytest

from james_device.kwin import SEPARADOR, Janela, Kwin, por_alvo

ID_FIREFOX = "{cfdea265-2348-4381-9f29-f9b5db7369da}"
ID_DOLPHIN = "{0b6e1c2a-1111-4222-8333-944455556666}"


def _linha(*campos) -> str:
    return SEPARADOR.join(str(c) for c in campos)


LINHA_FIREFOX = _linha(ID_FIREFOX, "org.mozilla.firefox", "Google | Busca — Mozilla Firefox", 8240,
                       "true", "false", "true", "HDMI-A-2", 1920, 588, 1422.2, 800, 1920, 588, 1422.2, 800)
LINHA_DOLPHIN = _linha(ID_DOLPHIN, "org.kde.dolphin", 'Word "anotações" — Dolphin', 671102,
                       "false", "false", "true", "HDMI-A-1", 555.1, 309.4, 937, 527, 559.1, 343.4, 929, 489)
LINHA_PAINEL = _linha("{aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee}", "org.kde.plasmashell", "Painel", 5000,
                      "false", "false", "false", "HDMI-A-1", 0, 1040, 1920, 40, 0, 1040, 1920, 40)


class ExecutorFalso:
    def __init__(self, respostas=None):
        self.scripts: list[str] = []
        self.respostas = list(respostas or [])

    def rodar(self, js: str) -> list[str]:
        self.scripts.append(js)
        return self.respostas.pop(0) if self.respostas else []


def test_listar_monta_o_script_e_le_cada_janela():
    executor = ExecutorFalso([[LINHA_FIREFOX, LINHA_DOLPHIN]])

    janelas = Kwin(executor).listar()

    assert "workspace.windowList()" in executor.scripts[0]
    firefox, dolphin = janelas
    assert firefox == Janela(
        id=ID_FIREFOX, app="org.mozilla.firefox", titulo="Google | Busca — Mozilla Firefox", pid=8240,
        ativa=True, minimizada=False, normal=True, monitor="HDMI-A-2",
        frame=(1920.0, 588.0, 1422.2, 800.0), cliente=(1920.0, 588.0, 1422.2, 800.0),
    )
    assert dolphin.titulo == 'Word "anotações" — Dolphin'
    assert dolphin.cliente == (559.1, 343.4, 929.0, 489.0)


def test_linha_com_campos_faltando_e_ignorada_com_log(caplog):
    executor = ExecutorFalso([[LINHA_FIREFOX, "quebrada" + SEPARADOR + "x"]])

    janelas = Kwin(executor).listar()

    assert [j.id for j in janelas] == [ID_FIREFOX]
    assert "ilegível" in caplog.text


def test_ativa_devolve_a_janela_em_foco_ou_none():
    assert Kwin(ExecutorFalso([[LINHA_FIREFOX, LINHA_DOLPHIN]])).ativa().id == ID_FIREFOX
    assert Kwin(ExecutorFalso([[LINHA_DOLPHIN]])).ativa() is None


def test_telas_le_geometria_logica_e_escala():
    executor = ExecutorFalso([[_linha("HDMI-A-1", 0, 0, 1920, 1080, 1), _linha("HDMI-A-2", 1920, 588, 1422, 800, 1.35)]])

    telas = Kwin(executor).telas()

    assert "workspace.screens" in executor.scripts[0]
    assert [(t.nome, t.x, t.y, t.largura, t.altura, t.escala) for t in telas] == [
        ("HDMI-A-1", 0.0, 0.0, 1920.0, 1080.0, 1.0),
        ("HDMI-A-2", 1920.0, 588.0, 1422.0, 800.0, 1.35),
    ]


def test_focar_ativa_pelo_id_e_confirma_relendo():
    executor = ExecutorFalso([[ID_DOLPHIN]])

    assert Kwin(executor).focar(ID_DOLPHIN) is True
    js = executor.scripts[0]
    assert "workspace.activeWindow = w" in js
    assert f'"{ID_DOLPHIN}"' in js


def test_focar_id_que_sumiu_devolve_false():
    assert Kwin(ExecutorFalso([[ID_FIREFOX]])).focar(ID_DOLPHIN) is False


def test_fechar_usa_close_window():
    executor = ExecutorFalso([["ok"]])

    assert Kwin(executor).fechar(ID_DOLPHIN) is True
    assert "closeWindow()" in executor.scripts[0]


def test_fechar_janela_que_nao_existe_devolve_false():
    assert Kwin(ExecutorFalso([["nao_achei"]])).fechar(ID_DOLPHIN) is False


@pytest.mark.parametrize("ruim", [
    "firefox",
    '"); workspace.activeWindow.closeWindow(); ("',
    "{cfdea265-2348-4381-9f29-f9b5db7369da}\"; x(); \"",
    "",
])
def test_id_fora_do_formato_nao_gera_script(ruim):
    executor = ExecutorFalso()
    kwin = Kwin(executor)

    with pytest.raises(ValueError, match="id de janela inválido"):
        kwin.focar(ruim)
    with pytest.raises(ValueError, match="id de janela inválido"):
        kwin.fechar(ruim)
    assert executor.scripts == []


def test_texto_do_modelo_nunca_entra_no_script():
    malicioso = '"); workspace.activeWindow.closeWindow(); ("'
    executor = ExecutorFalso([[LINHA_FIREFOX, LINHA_DOLPHIN]])
    kwin = Kwin(executor)

    achadas = por_alvo(kwin.listar(), malicioso)

    assert achadas == []
    assert all(malicioso not in js for js in executor.scripts)


def _janelas():
    return Kwin(ExecutorFalso([[LINHA_FIREFOX, LINHA_DOLPHIN, LINHA_PAINEL]])).listar()


def test_por_alvo_casa_app_ou_titulo_sem_caixa_e_sem_acento():
    janelas = _janelas()
    assert [j.app for j in por_alvo(janelas, "FIREFOX")] == ["org.mozilla.firefox"]
    assert [j.app for j in por_alvo(janelas, "anotacoes")] == ["org.kde.dolphin"]
    assert [j.app for j in por_alvo(janelas, "Anotações")] == ["org.kde.dolphin"]


def test_por_alvo_ignora_janela_que_nao_e_normal():
    assert por_alvo(_janelas(), "painel") == []


def test_por_alvo_devolve_todas_quando_ha_mais_de_uma():
    janelas = _janelas()
    assert len(por_alvo(janelas, "—")) == 2


def test_por_alvo_vazio_nao_casa_tudo():
    assert por_alvo(_janelas(), "   ") == []


# --- desktop de verdade --------------------------------------------------------

desktop = pytest.mark.skipif(
    os.environ.get("JAMES_TESTE_DESKTOP") != "1" or sys.platform != "linux",
    reason="defina JAMES_TESTE_DESKTOP=1 para falar com o KWin de verdade",
)


@desktop
def test_ao_vivo_lista_telas_janelas_e_foca_um_kwrite_proprio():
    from gi.repository import Gio

    from james_device.kwin import ExecutorKWin
    from james_device.laco_glib import LacoGlib

    laco = LacoGlib()
    laco.iniciar()
    editor = subprocess.Popen(["kwrite"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        conexao = laco.executar(lambda: Gio.bus_get_sync(Gio.BusType.SESSION, None))
        kwin = Kwin(ExecutorKWin(laco, conexao))
        assert kwin.telas(), "nenhuma tela"

        fim = time.monotonic() + 15
        minha = None
        while minha is None and time.monotonic() < fim:
            minha = next((j for j in kwin.listar() if j.pid == editor.pid and j.normal), None)
            time.sleep(0.3)
        assert minha is not None, "a janela do KWrite não apareceu"

        inicio = time.monotonic()
        assert kwin.focar(minha.id)
        print(f"focar levou {time.monotonic() - inicio:.3f}s")
        assert kwin.ativa().id == minha.id
    finally:
        editor.terminate()
        editor.wait(5)
        laco.encerrar()
