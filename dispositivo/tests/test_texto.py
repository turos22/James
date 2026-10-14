"""Escrever texto sem depender do layout do teclado (D6 da spec 2026-10-13 B2,
com a área de transferência pelo Klipper: ver o registro do plano)."""

import pytest

from james_device.texto import ErroDeTexto, campo_em_foco, digitar


class Campo:
    def __init__(self, texto="", aceita_insercao=True):
        self.texto = texto
        self.aceita = aceita_insercao
        self.inseridos = []

    def ler(self):
        return self.texto

    def inserir(self, texto):
        self.inseridos.append(texto)
        if self.aceita:
            self.texto += texto
        return True  # o Firefox devolve True e não muda nada


class Area:
    def __init__(self, tipos=("text/plain;charset=utf-8",), conteudo="o que o Arthur copiou"):
        self._tipos = list(tipos)
        self.conteudo = conteudo if tipos else None
        self.historico = []

    def tipos(self):
        return list(self._tipos)

    def ler(self):
        return self.conteudo

    def gravar(self, texto):
        self.historico.append(("gravar", texto))
        self.conteudo = texto

    def limpar(self):
        self.historico.append(("limpar", None))
        self.conteudo = None


def _colar_em(campo, area, falhar=None):
    def colar():
        if falhar:
            raise falhar
        if campo is not None:
            campo.texto += area.conteudo
    return colar


def _esperar(condicao, segundos):
    return condicao()


FRASE = "Olá, ação, coração e maçã — 100% 🎉\nsegunda linha"


def test_insercao_pelo_atspi_nao_toca_na_area_de_transferencia():
    campo, area = Campo("busca: "), Area()

    saida = digitar(FRASE, campo, area, _colar_em(campo, area), esperar=_esperar, dormir=lambda s: None)

    assert campo.texto == "busca: " + FRASE
    assert area.historico == []
    assert "AT-SPI" in saida


def test_insercao_ignorada_cai_para_colar_e_devolve_a_area():
    campo, area = Campo("trin", aceita_insercao=False), Area()

    saida = digitar(FRASE, campo, area, _colar_em(campo, area), esperar=_esperar, dormir=lambda s: None)

    assert campo.texto == "trin" + FRASE
    assert area.historico == [("gravar", FRASE), ("gravar", "o que o Arthur copiou")]
    assert area.conteudo == "o que o Arthur copiou"
    assert "colei" in saida


def test_colar_que_nao_aparece_e_erro_e_a_area_volta_mesmo_assim():
    campo, area = Campo("x", aceita_insercao=False), Area()

    with pytest.raises(ErroDeTexto, match="não apareceu"):
        digitar(FRASE, campo, area, lambda: None, esperar=_esperar, dormir=lambda s: None)

    assert area.conteudo == "o que o Arthur copiou"


def test_area_vazia_antes_fica_vazia_depois():
    campo, area = Campo("", aceita_insercao=False), Area(tipos=())

    digitar(FRASE, campo, area, _colar_em(campo, area), esperar=_esperar, dormir=lambda s: None)

    assert area.historico[-1] == ("limpar", None)


def test_area_com_imagem_nao_e_sobrescrita():
    campo, area = Campo("", aceita_insercao=False), Area(tipos=("image/png",), conteudo=None)
    colados = []

    with pytest.raises(ErroDeTexto, match="não é texto"):
        digitar(FRASE, campo, area, lambda: colados.append(1), esperar=_esperar, dormir=lambda s: None)

    assert area.historico == []
    assert colados == []


def test_area_com_imagem_nao_importa_quando_o_atspi_escreve():
    campo, area = Campo(""), Area(tipos=("image/png",), conteudo=None)

    digitar(FRASE, campo, area, _colar_em(campo, area), esperar=_esperar, dormir=lambda s: None)

    assert campo.texto == FRASE


def test_sem_campo_legivel_cola_espera_e_so_entao_devolve():
    area = Area()
    ordem = []

    def colar():
        ordem.append(("colar", area.conteudo))

    saida = digitar(FRASE, None, area, colar, esperar=_esperar, dormir=lambda s: ordem.append(("dormir", s)))

    assert ordem[0] == ("colar", FRASE)
    assert ordem[1][0] == "dormir" and ordem[1][1] >= 0.3
    assert area.conteudo == "o que o Arthur copiou"
    assert "sem confirmação" in saida


def test_falha_ao_colar_propaga_e_a_area_volta():
    campo, area = Campo("", aceita_insercao=False), Area()

    with pytest.raises(RuntimeError, match="sem permissão"):
        digitar(FRASE, campo, area, _colar_em(campo, area, falhar=RuntimeError("sem permissão")),
                esperar=_esperar, dormir=lambda s: None)

    assert area.conteudo == "o que o Arthur copiou"


def test_texto_vazio_e_recusado():
    with pytest.raises(ErroDeTexto, match="vazio"):
        digitar("", Campo(), Area(), lambda: None, esperar=_esperar, dormir=lambda s: None)


# --- campo em foco ------------------------------------------------------------------

class No:
    def __init__(self, papel="panel", estados=("showing",), filhos=(), editavel=False):
        self._papel, self._estados, self._filhos = papel, set(estados), list(filhos)
        self.acessivel = object()
        if editavel:
            self._estados.add("editable")

    def papel(self):
        return self._papel

    def estados(self):
        return set(self._estados)

    def filhos(self):
        return list(self._filhos)


def test_campo_em_foco_acha_o_editavel_focado():
    alvo = No("entry", ("showing", "focused"), editavel=True)
    raiz = No(filhos=[No("button", ("showing", "focused")), No(filhos=[alvo])])

    assert campo_em_foco(raiz, envolver=lambda no: no) is alvo


def test_campo_em_foco_sem_editavel_focado_devolve_none():
    raiz = No(filhos=[No("entry", ("showing",), editavel=True)])
    assert campo_em_foco(raiz, envolver=lambda no: no) is None


# --- desktop de verdade ---------------------------------------------------------------

import os  # noqa: E402
import sys  # noqa: E402


@pytest.mark.skipif(os.environ.get("JAMES_TESTE_DESKTOP") != "1" or sys.platform != "linux",
                    reason="defina JAMES_TESTE_DESKTOP=1 para escrever num KWrite de verdade")
def test_ao_vivo_escreve_com_acento_pelo_atspi_e_colando_e_devolve_a_area():
    import subprocess
    import time

    from gi.repository import Gio

    from james_device.acessibilidade import listar_apps, vincular_ao_laco
    from james_device.kwin import ExecutorKWin, Kwin
    from james_device.laco_glib import LacoGlib
    from james_device.portal import CanalPortal, Portal, Token, caminho_do_token
    from james_device.teclas import para_keysyms
    from james_device.texto import AreaKlipper, CampoAtspi

    frase = "Olá, ação, coração e maçã — 100%"
    laco = LacoGlib()
    laco.iniciar()
    editor = subprocess.Popen(["kwrite"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        vincular_ao_laco(laco)
        conexao = laco.executar(lambda: Gio.bus_get_sync(Gio.BusType.SESSION, None))
        kwin = Kwin(ExecutorKWin(laco, conexao))
        portal = Portal(CanalPortal(laco, conexao), Token(caminho_do_token()))
        portal.abrir()
        area = AreaKlipper(laco, conexao)
        tipos_antes = area.tipos()
        area_antes = area.ler() if tipos_antes else None

        fim, minha = time.monotonic() + 15, None
        while minha is None and time.monotonic() < fim:
            minha = next((j for j in kwin.listar() if j.pid == editor.pid and j.normal), None)
            time.sleep(0.3)
        kwin.focar(minha.id)
        time.sleep(1.0)
        app = laco.executar(lambda: next(a for a in listar_apps() if a.pid() == editor.pid))

        def achar(no, criterio):
            pilha = [no]
            while pilha:
                x = pilha.pop()
                if criterio(x):
                    return x
                pilha.extend(x.filhos())

        novo = laco.executar(lambda: achar(app, lambda n: n.nome() == "Novo" and n.papel() in ("button", "push button")))
        x, y, w, h = laco.executar(novo.retangulo)
        cx, cy = minha.cliente[:2]
        portal.mover(cx + x + w / 2, cy + y + h / 2)
        portal.clicar()
        time.sleep(0.8)

        campo = laco.executar(lambda: campo_em_foco(app, envolver=lambda no: CampoAtspi(no, laco.executar)))
        assert campo is not None, "nenhum campo editável em foco depois de clicar em Novo"
        saida = digitar(frase, campo, area, lambda: portal.teclas(para_keysyms("ctrl+v")))
        print(saida)
        assert campo.ler().strip() == frase

        class SemInsercao(CampoAtspi):
            def inserir(self, conteudo):
                return True  # imita o Firefox: força o caminho de colar

        campo_forcado = SemInsercao(campo._no, laco.executar)
        portal.teclas(para_keysyms("ctrl+a"))
        portal.teclas(para_keysyms("delete"))
        time.sleep(0.3)
        saida = digitar(frase, campo_forcado, area, lambda: portal.teclas(para_keysyms("ctrl+v")))
        print(saida)
        assert campo.ler().strip() == frase
        assert "colei" in saida

        if area_antes is None:
            assert area.tipos() == [] or area.ler() == ""
        else:
            assert area.ler() == area_antes, "a área de transferência do Arthur não voltou"
    finally:
        editor.terminate()
        editor.wait(5)
        laco.encerrar()
