"""A fila de fala do device: recebe frases do jamesd, toca na ordem, e diz à
escuta quando ela pode voltar a ouvir. Sintetizador e reprodutor são falsos:
nenhum teste aqui toca som nem carrega o Piper."""

import logging
import threading

import pytest

from james_device.fala import Fala, FRASE_NAO_ENTENDI


class SintetizadorFalso:
    def __init__(self, falhar_em=()):
        self.falhar_em = set(falhar_em)

    def sintetizar(self, texto):
        if texto in self.falhar_em:
            raise RuntimeError(f"síntese quebrou em {texto!r}")
        return texto.encode(), 22050


class ReprodutorFalso:
    def __init__(self, falhar_em=()):
        self.tocadas = []
        self.falhar_em = set(falhar_em)
        self.turnos_fechados = 0
        self.travar = None  # threading.Event: segura o tocar() até o teste soltar
        self.tocando = threading.Event()

    def tocar(self, pcm, taxa):
        texto = pcm.decode()
        self.tocando.set()
        if self.travar is not None:
            self.travar.wait(timeout=5)
        if texto in self.falhar_em:
            raise OSError(f"alto-falante quebrou em {texto!r}")
        self.tocadas.append(texto)

    def fim_do_turno(self):
        self.turnos_fechados += 1


class MedidorFalso:
    def __init__(self):
        self.marcas = []

    def marca(self, nome):
        self.marcas.append(nome)


@pytest.fixture
def montar():
    criadas = []

    def _montar(sintetizador=None, reprodutor=None, medidor=None, relogio=None, timeout=30.0):
        reprodutor = reprodutor or ReprodutorFalso()
        medidor = medidor or MedidorFalso()
        kwargs = {"relogio": relogio} if relogio else {}
        fala = Fala(sintetizador or SintetizadorFalso(), reprodutor, medidor=medidor,
                    timeout_resposta=timeout, **kwargs)
        criadas.append(fala)
        return fala, reprodutor, medidor

    yield _montar
    for f in criadas:
        f.parar()


def test_frases_fora_de_ordem_tocam_na_ordem(montar):
    fala, rep, _ = montar()

    fala.comando_enviado()
    fala.frase(1, "segunda")
    fala.frase(0, "primeira")
    fala.frase(2, "terceira")
    fala.fim()

    assert fala.aguardar_fim_do_turno(3)
    assert rep.tocadas == ["primeira", "segunda", "terceira"]


def test_ocupado_do_comando_enviado_ate_a_ultima_frase_tocar(montar):
    fala, rep, _ = montar()
    rep.travar = threading.Event()

    assert not fala.ocupado(), "ocioso antes de haver turno"
    fala.comando_enviado()
    assert fala.ocupado(), "esperando a resposta: a escuta não pode voltar ainda"

    fala.frase(0, "só uma")
    fala.fim()
    assert rep.tocando.wait(3)
    assert fala.ocupado(), "ainda tocando"

    rep.travar.set()
    assert fala.aguardar_fim_do_turno(3)
    assert not fala.ocupado()


def test_resposta_vazia_libera_na_hora(montar):
    fala, rep, _ = montar()

    fala.comando_enviado()
    fala.fim()

    assert fala.aguardar_fim_do_turno(3)
    assert rep.tocadas == []


def test_erro_toca_a_mensagem_e_libera(montar):
    fala, rep, _ = montar()

    fala.comando_enviado()
    fala.erro("não consegui completar esse turno")

    assert fala.aguardar_fim_do_turno(3)
    assert rep.tocadas == ["não consegui completar esse turno"]
    assert not fala.ocupado()


def test_sem_fala_fim_o_ocupado_libera_com_log(montar, caplog):
    tempo = [100.0]
    fala, _, _ = montar(relogio=lambda: tempo[0], timeout=30.0)

    fala.comando_enviado()
    tempo[0] = 129.0
    assert fala.ocupado(), "29 s ainda está dentro do prazo"

    tempo[0] = 131.0
    with caplog.at_level(logging.WARNING):
        assert not fala.ocupado(), "passou do prazo sem resposta: o James ficaria surdo pra sempre"
    assert any("resposta" in r.message for r in caplog.records)


def test_atividade_renova_o_prazo(montar):
    tempo = [0.0]
    fala, _, _ = montar(relogio=lambda: tempo[0], timeout=30.0)

    fala.comando_enviado()
    tempo[0] = 25.0
    fala.frase(0, "chegou algo")
    assert fala.aguardar_ocioso(3)
    tempo[0] = 50.0  # 25 s depois da frase: dentro do prazo
    assert fala.ocupado()


def test_nao_entendi_fala_localmente_e_libera(montar):
    fala, rep, _ = montar()

    fala.nao_entendi()

    assert fala.aguardar_fim_do_turno(3)
    assert rep.tocadas == [FRASE_NAO_ENTENDI]
    assert FRASE_NAO_ENTENDI == "Não entendi, pode repetir?"


def test_falha_do_alto_falante_ou_da_sintese_nao_derruba_a_fila(montar, caplog):
    fala, rep, _ = montar(sintetizador=SintetizadorFalso(falhar_em={"quebra a síntese"}),
                          reprodutor=ReprodutorFalso(falhar_em={"quebra o som"}))

    with caplog.at_level(logging.ERROR):
        fala.comando_enviado()
        fala.frase(0, "ok um")
        fala.frase(1, "quebra a síntese")
        fala.frase(2, "quebra o som")
        fala.frase(3, "ok dois")
        fala.fim()
        assert fala.aguardar_fim_do_turno(3)

    assert rep.tocadas == ["ok um", "ok dois"]
    assert not fala.ocupado()
    assert len([r for r in caplog.records if r.levelno >= logging.ERROR]) >= 2


def test_primeiro_audio_e_marcado_uma_vez_por_turno(montar):
    fala, _, medidor = montar()

    fala.comando_enviado()
    for i, t in enumerate(["a", "b", "c"]):
        fala.frase(i, t)
    fala.fim()
    assert fala.aguardar_fim_do_turno(3)
    assert medidor.marcas == ["primeiro_audio"]

    fala.comando_enviado()
    fala.frase(0, "d")
    fala.fim()
    assert fala.aguardar_fim_do_turno(3)
    assert medidor.marcas == ["primeiro_audio", "primeiro_audio"]


def test_fecha_o_reprodutor_no_fim_de_cada_turno(montar):
    fala, rep, _ = montar()

    fala.comando_enviado()
    fala.frase(0, "a")
    fala.fim()
    assert fala.aguardar_fim_do_turno(3)

    assert rep.turnos_fechados == 1


def test_novo_comando_descarta_o_que_sobrou_do_turno_anterior(montar):
    fala, rep, _ = montar()

    fala.comando_enviado()
    fala.frase(5, "perdida de um turno que nunca terminou")
    fala.comando_enviado()  # o Arthur falou de novo
    fala.frase(0, "do turno novo")
    fala.fim()

    assert fala.aguardar_fim_do_turno(3)
    assert rep.tocadas == ["do turno novo"]


def test_frase_longa_tocando_nao_solta_a_escuta_mesmo_passando_do_prazo(montar):
    tempo = [0.0]
    fala, rep, _ = montar(relogio=lambda: tempo[0], timeout=30.0)
    rep.travar = threading.Event()

    fala.comando_enviado()
    fala.frase(0, "uma frase bem longa que ainda está sendo dita")  # sem fim: o jamesd ainda escreve
    assert rep.tocando.wait(3)
    tempo[0] = 100.0  # bem além do prazo de silêncio do jamesd

    # Enquanto há som saindo, a escuta tem de seguir fechada: ouvir agora é ouvir o James.
    assert fala.ocupado()

    rep.travar.set()
