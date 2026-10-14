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


class AreaLenta(Area):
    """O Klipper leva um instante para assumir a área de transferência: as
    primeiras leituras depois de gravar ainda devolvem o conteúdo antigo
    (medido ao vivo: o Ctrl+V cedo demais colou o texto velho)."""

    def __init__(self, atraso_leituras=2, **kw):
        super().__init__(**kw)
        self._velho = self.conteudo
        self._faltam = 0
        self._atraso = atraso_leituras

    def gravar(self, texto):
        self._velho = self.conteudo
        super().gravar(texto)
        self._faltam = self._atraso

    def ler(self):
        if self._faltam > 0:
            self._faltam -= 1
            return self._velho
        return self.conteudo


def _esperar_de_verdade(condicao, segundos):
    for _ in range(50):
        if condicao():
            return True
    return False


def test_so_cola_depois_que_a_area_tem_o_texto_do_james():
    campo, area = Campo("", aceita_insercao=False), AreaLenta()
    vistos_no_colar = []

    def colar():
        vistos_no_colar.append(area.ler())
        campo.texto += area.conteudo

    digitar(FRASE, campo, area, colar, esperar=_esperar_de_verdade, dormir=lambda s: None)

    assert vistos_no_colar == [FRASE]
    assert campo.texto == FRASE
    assert area.conteudo == "o que o Arthur copiou"


def test_area_que_nao_aceita_o_texto_e_erro_e_nao_cola():
    class Teimosa(Area):
        def ler(self):
            return "o que o Arthur copiou"

    campo, area = Campo("", aceita_insercao=False), Teimosa()
    colados = []

    with pytest.raises(ErroDeTexto, match="não aceitou"):
        digitar(FRASE, campo, area, lambda: colados.append(1), esperar=_esperar, dormir=lambda s: None)
    assert colados == []


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

        # A tela de boas-vindas tem duas versões: "Novo" na barra, ou "Novo
        # arquivo" quando há arquivos recentes. Só vale o que está visível.
        novo = laco.executar(lambda: achar(app, lambda n: n.nome() in ("Novo arquivo", "Novo")
                                           and n.papel() in ("button", "push button") and "showing" in n.estados()))
        x, y, w, h = laco.executar(novo.retangulo)
        cx, cy = minha.cliente[:2]
        portal.mover(cx + x + w / 2, cy + y + h / 2)
        portal.clicar()
        time.sleep(0.8)

        campo, fim = None, time.monotonic() + 3
        while campo is None and time.monotonic() < fim:
            campo = laco.executar(lambda: campo_em_foco(app, envolver=lambda no: CampoAtspi(no, laco.executar)))
            time.sleep(0.2)
        assert campo is not None, "nenhum campo editável em foco depois de clicar em Novo"
        saida = digitar(frase, campo, area, lambda: portal.teclas(para_keysyms("ctrl+v")))
        print(saida)
        assert campo.ler().strip() == frase
        # Sem lixo: com o comprimento em bytes, o Qt inseria caracteres nulos.
        from james_device.texto import _DocumentoAtspi
        documento = _DocumentoAtspi(campo)
        assert laco.executar(documento.contar) == len(frase)
        for _ in range(2):  # inserções seguidas no mesmo campo (falhavam antes)
            assert "AT-SPI" in digitar(" mais", campo, area, lambda: portal.teclas(para_keysyms("ctrl+v")))
        assert laco.executar(documento.contar) == len(frase) + 2 * len(" mais")

        class SemInsercao(CampoAtspi):
            def inserir(self, conteudo):
                return True  # imita o Firefox: força o caminho de colar

        campo_forcado = SemInsercao(campo._no, laco.executar)
        portal.teclas(para_keysyms("ctrl+a"))
        portal.teclas(para_keysyms("delete"))
        time.sleep(0.3)
        so_texto = not tipos_antes or any(t.startswith("text/") or t in ("STRING", "UTF8_STRING") for t in tipos_antes)
        if not so_texto:
            # Imagem ou arquivo copiado: o James recusa colar por cima (e o teste
            # não destrói o que o Arthur copiou só para provar o caminho).
            with pytest.raises(ErroDeTexto, match="não é texto"):
                digitar(frase, campo_forcado, area, lambda: portal.teclas(para_keysyms("ctrl+v")))
            print("área de transferência com imagem: colar recusado, como deve")
        else:
            saida = digitar(frase, campo_forcado, area, lambda: portal.teclas(para_keysyms("ctrl+v")))
            print(saida)
            assert campo.ler().strip() == frase
            assert "colei" in saida

        if area_antes is None:
            assert area.tipos() == [] or area.ler() == ""
        else:
            assert area.ler() == area_antes, "a área de transferência do Arthur não voltou"
    finally:
        editor.kill()  # documento alterado: o KWrite demora a sair com SIGTERM
        editor.wait(5)
        laco.encerrar()


# --- inserir conferindo: cada toolkit conta o comprimento de um jeito ---------------
#
# Medido ao vivo (Task 17): o Qt conta o comprimento do insert_text em
# CARACTERES e, com o comprimento em bytes, insere o texto mais caracteres
# nulos; o GTK conta em bytes; a barra do Firefox ignora a inserção.

from james_device.texto import inserir_conferindo  # noqa: E402


class _Documento:
    def __init__(self, texto="", caret=None):
        self.texto = texto
        self._caret = len(texto) if caret is None else caret

    def contar(self):
        return len(self.texto)

    def caret(self):
        return self._caret

    def ler(self, inicio, fim):
        return self.texto[inicio:fim]

    def apagar(self, inicio, fim):
        self.texto = self.texto[:inicio] + self.texto[fim:]


class Qt(_Documento):
    def inserir(self, posicao, conteudo, comprimento):
        trecho = conteudo[:comprimento] + "\0" * max(0, comprimento - len(conteudo))
        self.texto = self.texto[:posicao] + trecho + self.texto[posicao:]
        return True


class Gtk(_Documento):
    def inserir(self, posicao, conteudo, comprimento):
        trecho = conteudo.encode("utf-8")[:comprimento].decode("utf-8", "ignore")
        self.texto = self.texto[:posicao] + trecho + self.texto[posicao:]
        return True


class Ignora(_Documento):
    def inserir(self, posicao, conteudo, comprimento):
        return True


@pytest.mark.parametrize("toolkit", [Qt, Gtk])
@pytest.mark.parametrize("frase", ["ação ", "abc", FRASE])
def test_inserir_conferindo_acerta_no_qt_e_no_gtk_sem_deixar_lixo(toolkit, frase):
    doc = toolkit("antes|depois", caret=6)

    assert inserir_conferindo(doc, frase) is True

    assert doc.texto == "antes|" + frase + "depois"
    assert "\0" not in doc.texto


def test_inserir_conferindo_ignorado_devolve_false_e_nao_mexe():
    doc = Ignora("trin")
    assert inserir_conferindo(doc, "ação") is False
    assert doc.texto == "trin"


def test_inserir_conferindo_sem_caret_insere_no_fim():
    doc = Qt("abc", caret=-1)
    assert inserir_conferindo(doc, "ç") is True
    assert doc.texto == "abcç"


# --- achados do code-review ------------------------------------------------------------

def test_insercao_que_nao_conferiu_nao_gasta_a_espera_e_cai_direto_para_colar():
    class Recusa(Campo):
        def inserir(self, texto):
            self.inseridos.append(texto)
            return False  # inserir_conferindo já provou que não entrou

    campo, area = Recusa("trin"), Area()
    esperas = []

    def esperar(condicao, segundos):
        esperas.append(segundos)
        return condicao()

    digitar(FRASE, campo, area, _colar_em(campo, area), esperar=esperar, dormir=lambda s: None)

    assert 0.3 not in esperas, "esperou o prazo da inserção que já tinha falhado"


@pytest.mark.parametrize("tipos", [
    ["text/plain;charset=utf-8", "text/html"],
    ["text/uri-list", "text/plain"],
    ["text/plain", "image/png"],
    ["text/x-moz-url", "text/plain"],
    ["application/x-qt-image", "text/plain"],
])
def test_area_com_formato_rico_nao_e_sobrescrita(tipos):
    campo, area = Campo("", aceita_insercao=False), Area(tipos=tuple(tipos))

    with pytest.raises(ErroDeTexto, match="formato"):
        digitar(FRASE, campo, area, lambda: None, esperar=_esperar, dormir=lambda s: None)

    assert area.historico == []


def test_area_so_com_texto_simples_continua_valendo():
    campo, area = Campo("", aceita_insercao=False), Area(tipos=("text/plain;charset=utf-8", "STRING", "text/plain"))

    digitar(FRASE, campo, area, _colar_em(campo, area), esperar=_esperar, dormir=lambda s: None)

    assert campo.texto == FRASE
