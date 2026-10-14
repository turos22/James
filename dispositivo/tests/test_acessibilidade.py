"""Varredura de elementos pelo AT-SPI (D7, D11 da spec 2026-10-13 B2).

A árvore é falsa (NoFalso imita o adaptador NoAtspi). O caminho real só roda
com JAMES_TESTE_DESKTOP=1.
"""

import os
import subprocess
import sys
import time

import pytest

from james_device.acessibilidade import NoIlegivel, casar_janela, ligar, revalidar, tipo_exibido, varrer
from james_device.kwin import Janela
from james_device.superficies import Superficie

HDMI1 = Superficie("HDMI-A-1", 1280, 720, 0, 0, 1.0, True, fator=1.5)
HDMI2 = Superficie("HDMI-A-2", 1280, 720, 1920, 588, 1.35, False, fator=1422 / 1280)


class NoFalso:
    def __init__(self, papel, nome="", ret=(0, 0, 0, 0), filhos=(), visivel=True, habilitado=True,
                 editavel=False, pid=0, ilegivel=False):
        self._papel, self._nome, self._ret = papel, nome, ret
        self._filhos = list(filhos)
        self._estados = {e for e, sim in (("showing", visivel), ("enabled", habilitado), ("editable", editavel)) if sim}
        self._pid = pid
        self.ilegivel = ilegivel
        self.lidos = 0

    def _ler(self):
        self.lidos += 1
        if self.ilegivel:
            raise NoIlegivel("sumiu")

    def papel(self):
        self._ler()
        return self._papel

    def nome(self):
        self._ler()
        return self._nome

    def estados(self):
        self._ler()
        return set(self._estados)

    def retangulo(self):
        self._ler()
        return self._ret

    def filhos(self):
        self._ler()
        return list(self._filhos)

    def pid(self):
        return self._pid


def _janela(id_, app, titulo, pid, cliente, ativa=False, normal=True, minimizada=False):
    return Janela(id=id_, app=app, titulo=titulo, pid=pid, ativa=ativa, minimizada=minimizada, normal=normal,
                  monitor="", frame=cliente, cliente=cliente)


ID_KW = "{11111111-1111-4111-8111-111111111111}"
ID_FF = "{22222222-2222-4222-8222-222222222222}"


def _kwrite(pid=500, botao_visivel=True):
    novo = NoFalso("button", "Novo", (6, 6, 74, 34), visivel=botao_visivel)
    abrir = NoFalso("push button menu", "Abrir", (80, 6, 93, 34))
    rotulo = NoFalso("label", "Bem-vindo", (10, 100, 200, 20))
    painel = NoFalso("panel", "", (0, 0, 1414, 734), filhos=[novo, abrir, rotulo])
    frame = NoFalso("frame", "Bem-vindo — KWrite", (0, 0, 1414, 734), filhos=[painel])
    return NoFalso("application", "kwrite", filhos=[frame], pid=pid)


def _firefox(pid=600):
    busca = NoFalso("entry", "Pesquisar", (325, 49, 1332, 32), editavel=True)
    link = NoFalso("link", "Gmail", (1100, 10, 40, 20))
    frame = NoFalso("frame", "Google — Mozilla Firefox", (0, 0, 1422, 800), filhos=[busca, link])
    return NoFalso("application", "Firefox", filhos=[frame], pid=pid)


def _painel():
    botao = NoFalso("button", "Mostrar ícones ocultos", (1896, 2, 22, 22))
    frame = NoFalso("frame", "", (0, 0, 1920, 26), filhos=[botao])
    return NoFalso("application", "plasmashell", filhos=[frame], pid=5000)


JANELAS = [
    _janela(ID_KW, "org.kde.kwrite", "Bem-vindo — KWrite", 500, (253.0, 200.0, 1414.0, 734.0), ativa=True),
    _janela(ID_FF, "org.mozilla.firefox", "Google — Mozilla Firefox", 600, (1920.0, 588.0, 1422.0, 800.0)),
]


# --- tipo exibido ---------------------------------------------------------------

@pytest.mark.parametrize("papel, tipo", [
    ("button", "Button"), ("push button", "Button"), ("push button menu", "Button"), ("toggle button", "Button"),
    ("entry", "Edit"), ("password text", "Edit"), ("combo box", "ComboBox"), ("link", "Hyperlink"),
    ("menu item", "MenuItem"), ("page tab", "TabItem"), ("check box", "CheckBox"),
    ("radio button", "RadioButton"), ("list item", "ListItem"), ("tree item", "TreeItem"), ("slider", "Slider"),
])
def test_tipo_exibido_segue_o_vocabulario_do_windows(papel, tipo):
    assert tipo_exibido(papel, editavel=False) == tipo


def test_papel_que_nao_interessa_nao_tem_tipo():
    assert tipo_exibido("label", editavel=False) is None
    assert tipo_exibido("panel", editavel=False) is None


def test_texto_so_conta_quando_editavel():
    assert tipo_exibido("text", editavel=True) == "Edit"
    assert tipo_exibido("text", editavel=False) is None


# --- casar AT-SPI com KWin ------------------------------------------------------

def test_casar_por_pid_e_titulo():
    assert casar_janela("Bem-vindo — KWrite", 500, JANELAS).id == ID_KW


def test_casar_so_por_titulo_quando_o_pid_nao_bate_flatpak():
    assert casar_janela("Google — Mozilla Firefox", 7, JANELAS).id == ID_FF


def test_casar_por_pid_quando_ha_uma_janela_so_desse_pid():
    assert casar_janela("título diferente", 500, JANELAS).id == ID_KW


def test_casar_sem_correspondencia_devolve_none():
    assert casar_janela("nada", 999, JANELAS) is None


def test_casar_ignora_janela_minimizada():
    minimizada = [_janela(ID_KW, "org.kde.kwrite", "Bem-vindo — KWrite", 500, (0, 0, 10, 10), minimizada=True)]
    assert casar_janela("Bem-vindo — KWrite", 500, minimizada) is None


# --- varredura -----------------------------------------------------------------

def test_soma_a_origem_do_cliente_e_so_pega_o_que_interessa():
    resultado = varrer([_kwrite()], JANELAS, HDMI1, ativa_id=ID_KW)

    nomes = [e.nome for e in resultado.mapa.elementos]
    assert nomes == ["Novo", "Abrir"]
    novo = resultado.mapa.elementos[0]
    assert novo.tipo == "Button"
    assert novo.retangulo == (253 + 6, 200 + 6, 253 + 80, 200 + 40)
    assert novo.em_foco is True
    assert novo.id == "e1"


def test_no_invisivel_fica_de_fora():
    resultado = varrer([_kwrite(botao_visivel=False)], JANELAS, HDMI1, ativa_id=ID_KW)
    assert [e.nome for e in resultado.mapa.elementos] == ["Abrir"]


def test_so_entra_elemento_com_centro_na_superficie():
    resultado = varrer([_kwrite(), _firefox()], JANELAS, HDMI1, ativa_id=ID_KW)
    assert all(e.nome in ("Novo", "Abrir") for e in resultado.mapa.elementos)

    no_outro = varrer([_kwrite(), _firefox()], JANELAS, HDMI2, ativa_id=ID_KW)
    assert sorted(e.nome for e in no_outro.mapa.elementos) == ["Gmail", "Pesquisar"]


def test_plasmashell_ja_vem_global():
    resultado = varrer([_painel()], JANELAS, HDMI1, ativa_id=ID_KW)

    (botao,) = resultado.mapa.elementos
    assert botao.retangulo == (1896, 2, 1918, 24)
    assert botao.em_foco is False


def test_janela_ativa_vem_primeiro_mesmo_se_o_app_vem_depois():
    janelas = [
        _janela(ID_KW, "org.kde.kwrite", "Bem-vindo — KWrite", 500, (253.0, 200.0, 1414.0, 734.0)),
        _janela(ID_FF, "org.mozilla.firefox", "Google — Mozilla Firefox", 600, (100.0, 100.0, 1422.0, 800.0),
                ativa=True),
    ]
    resultado = varrer([_kwrite(), _firefox()], janelas, HDMI1, ativa_id=ID_FF)

    primeiro = resultado.mapa.elementos[0]
    assert primeiro.nome in ("Pesquisar", "Gmail")
    assert primeiro.em_foco
    assert primeiro.id == "e1"
    assert [e.em_foco for e in resultado.mapa.elementos] == sorted(
        (e.em_foco for e in resultado.mapa.elementos), reverse=True)


def test_elemento_guarda_a_janela_dona_para_focar_antes_de_clicar():
    resultado = varrer([_kwrite()], JANELAS, HDMI1, ativa_id=ID_KW)
    assert resultado.donos["e1"].janela_id == ID_KW


def test_mapa_registra_superficie_e_janela_ativa():
    resultado = varrer([_kwrite()], JANELAS, HDMI1, ativa_id=ID_KW)
    assert resultado.mapa.valido_para("HDMI-A-1", ID_KW)


def test_no_ilegivel_e_pulado_e_contado():
    app = _kwrite()
    app._filhos[0]._filhos[0]._filhos[0].ilegivel = True  # o botão Novo some no meio
    resultado = varrer([app], JANELAS, HDMI1, ativa_id=ID_KW)

    assert [e.nome for e in resultado.mapa.elementos] == ["Abrir"]
    assert resultado.ignorados == 1


def test_erro_de_programacao_nao_e_engolido():
    class Quebrado(NoFalso):
        def papel(self):
            raise AttributeError("API mudou")

    frame = NoFalso("frame", "Bem-vindo — KWrite", filhos=[Quebrado("button")])
    app = NoFalso("application", "kwrite", filhos=[frame], pid=500)
    with pytest.raises(AttributeError):
        varrer([app], JANELAS, HDMI1, ativa_id=ID_KW)


def test_teto_de_tempo_corta_e_avisa():
    instantes = iter([0.0] + [0.5] * 1000)
    resultado = varrer([_kwrite(), _firefox()], JANELAS, HDMI2, ativa_id=ID_KW, relogio=lambda: next(instantes))

    assert resultado.cortada
    assert any("cortada" in a for a in resultado.avisos)


def test_teto_de_nos_corta_e_avisa():
    resultado = varrer([_kwrite()], JANELAS, HDMI1, ativa_id=ID_KW, teto_nos=3)
    assert resultado.cortada


def test_janela_ativa_sem_elementos_e_detectada():
    vazio = NoFalso("application", "code", filhos=[NoFalso("frame", "main.go - Visual Studio Code")], pid=500)
    resultado = varrer([vazio], JANELAS, HDMI1, ativa_id=ID_KW)

    assert resultado.mapa.elementos == []
    assert resultado.em_foco_sem_elementos


# --- revalidar ------------------------------------------------------------------

def test_revalidar_aceita_o_mesmo_elemento():
    resultado = varrer([_kwrite()], JANELAS, HDMI1, ativa_id=ID_KW)
    dono = resultado.donos["e1"]

    assert revalidar(resultado.mapa.elementos[0], dono, origem=(253.0, 200.0)) is None


def test_revalidar_recusa_quando_some_esconde_desabilita_ou_move():
    resultado = varrer([_kwrite()], JANELAS, HDMI1, ativa_id=ID_KW)
    elemento, dono = resultado.mapa.elementos[0], resultado.donos["e1"]

    assert "mudou de lugar" in revalidar(elemento, dono, origem=(300.0, 200.0))
    dono.no._estados.discard("enabled")
    assert "desabilitado" in revalidar(elemento, dono, origem=(253.0, 200.0))
    dono.no._estados.discard("showing")
    assert "não está mais visível" in revalidar(elemento, dono, origem=(253.0, 200.0))
    dono.no.ilegivel = True
    assert "não está mais na tela" in revalidar(elemento, dono, origem=(253.0, 200.0))


# --- ligar a acessibilidade -----------------------------------------------------------

def test_ligar_liga_e_diz_se_ja_estava_ligada():
    chamadas = []

    def propriedade(acao, valor=None):
        chamadas.append((acao, valor))
        return False if acao == "ler" else None

    assert ligar(propriedade) is False
    assert chamadas == [("ler", None), ("gravar", True)]


def test_ligar_nao_regrava_quando_ja_esta_ligada():
    chamadas = []
    assert ligar(lambda acao, valor=None: chamadas.append(acao) or True) is True
    assert chamadas == ["ler"]


# --- desktop de verdade ---------------------------------------------------------------

@pytest.mark.skipif(os.environ.get("JAMES_TESTE_DESKTOP") != "1" or sys.platform != "linux",
                    reason="defina JAMES_TESTE_DESKTOP=1 para ler o AT-SPI de verdade")
def test_ao_vivo_acha_o_botao_novo_de_um_kwrite_proprio():
    from gi.repository import Gio

    from james_device.acessibilidade import listar_apps
    from james_device.kwin import ExecutorKWin, Kwin
    from james_device.laco_glib import LacoGlib
    from james_device.superficies import tamanho_da_imagem

    laco = LacoGlib()
    laco.iniciar()
    editor = subprocess.Popen(["kwrite"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        conexao = laco.executar(lambda: Gio.bus_get_sync(Gio.BusType.SESSION, None))
        kwin = Kwin(ExecutorKWin(laco, conexao))
        fim, minha = time.monotonic() + 15, None
        while minha is None and time.monotonic() < fim:
            minha = next((j for j in kwin.listar() if j.pid == editor.pid and j.normal), None)
            time.sleep(0.3)
        assert minha is not None
        kwin.focar(minha.id)
        time.sleep(1.0)
        tela = next(t for t in kwin.telas() if t.nome == kwin.ativa().monitor)
        largura, altura, fator = tamanho_da_imagem(int(tela.largura), int(tela.altura))
        superficie = Superficie(tela.nome, largura, altura, int(tela.x), int(tela.y), tela.escala, False, fator)

        inicio = time.monotonic()
        resultado = varrer(listar_apps(), kwin.listar(), superficie, ativa_id=minha.id)
        print(f"varredura: {len(resultado.mapa.elementos)} elementos em {time.monotonic() - inicio:.3f}s, "
              f"cortada={resultado.cortada}")
        novo = next(e for e in resultado.mapa.elementos if e.nome == "Novo")
        assert novo.em_foco
        assert novo.retangulo[0] >= minha.cliente[0]
    finally:
        editor.terminate()
        editor.wait(5)
        laco.encerrar()
