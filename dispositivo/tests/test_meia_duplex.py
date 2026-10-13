"""Meia-duplex e continuação (spec 2026-10-10, D9 e D13).

Enquanto o James fala, o microfone o ouve: a escuta descarta o áudio, sem
alimentar o Vosk nem o VAD (senão a voz dele dispara a wake word). Quando ele
cala, abre uma janela em que a voz do Arthur vira comando SEM wake word."""

import json

from james_device import escuta
from james_device.escuta import ParametrosDeFim
from tests.test_fim_de_fala import (
    BLOCO,
    ExecutorSincrono,
    TranscritorFalso,
    VadFalso,
    bloco,
)


class ReconhecedorWake:
    """Só 'ouve' a wake word no chunk b'wake'; conta quantas vezes foi chamado."""

    def __init__(self):
        self.chamadas = 0
        self.resets = 0
        self._ultimo = ""

    def AcceptWaveform(self, chunk):  # noqa: N802
        self.chamadas += 1
        self._ultimo = "james" if chunk == b"wake" else ""
        return False

    def Result(self):  # noqa: N802
        return json.dumps({"text": self._ultimo})

    def PartialResult(self):  # noqa: N802
        return json.dumps({"partial": self._ultimo})

    def Reset(self):  # noqa: N802
        self.resets += 1
        self._ultimo = ""


class Ocupado:
    """ocupado() devolve True pelos próximos N chunks depois de acionar()."""

    def __init__(self, chunks):
        self.chunks = chunks
        self.restantes = 0

    def acionar(self):
        self.restantes = self.chunks

    def __call__(self):
        if self.restantes > 0:
            self.restantes -= 1
            return True
        return False


class TranscritorQueLembra(TranscritorFalso):
    """Além dos textos, guarda o índice do PRIMEIRO bloco de cada áudio."""

    def __init__(self, textos):
        super().__init__(textos)
        self.primeiros = []

    def __call__(self, audio):
        self.primeiros.append(audio[0])
        return super().__call__(audio)


COMANDO = "VVVVV" + "." * 7  # 5 de voz e 7 de silêncio: fecha em 0,7 s


def rodar(fonte, vad_roteiro, textos, ocupado, continuacao_s=6.0, guarda_eco_s=0.4,
          ao_nao_entendeu=None, transcritor=None):
    reconhecedor = ReconhecedorWake()
    comandos, acordou = [], []
    transcritor = transcritor or TranscritorQueLembra(textos)

    def ao_comando(texto):
        comandos.append(texto)
        ocupado.acionar()  # o James vai responder: o device fica ocupado

    escuta.escutar(
        fonte_audio=fonte,
        reconhecedor=reconhecedor,
        transcrever_comando=transcritor,
        ao_acordar=lambda: acordou.append(1),
        ao_comando=ao_comando,
        palavra_parada="",
        vad=VadFalso(vad_roteiro),
        parametros_fim=ParametrosDeFim(),
        executor=ExecutorSincrono(),
        ocupado=ocupado,
        continuacao_s=continuacao_s,
        guarda_eco_s=guarda_eco_s,
        ao_nao_entendeu=ao_nao_entendeu,
    )
    return comandos, acordou, reconhecedor, transcritor


def fonte_padrao():
    return (
        [b"wake"] + [bloco(i) for i in range(12)]          # wake + comando 1
        + [b"wake"] * 6                                     # o James falando (ocupado)
        + [bloco(100 + i) for i in range(4)]                # eco da sala (guarda)
        + [bloco(200 + i) for i in range(12)]               # comando 2, sem wake word
        + [bloco(250)] * 5                                  # silêncio depois
    )


def test_enquanto_ocupado_descarta_o_audio_e_a_voz_do_james_nao_acorda_a_escuta():
    comandos, acordou, reconhecedor, _ = rodar(fonte_padrao(), COMANDO + COMANDO, ["primeiro", "segundo"], Ocupado(6))

    assert acordou == [1], "a wake word que o James 'disse' enquanto falava acordou a escuta"
    # Só o chunk da wake word passou pelo reconhecedor. Os 6 da fala do James
    # foram descartados (se tivessem passado, "james" acordaria de novo), a
    # continuação não usa o Vosk, e depois do 2º comando o device fica ocupado
    # outra vez e descarta o resto.
    assert reconhecedor.chamadas == 1


def test_depois_que_o_james_cala_a_voz_vira_comando_sem_wake_word():
    comandos, acordou, _, transcritor = rodar(fonte_padrao(), COMANDO + COMANDO, ["primeiro", "segundo"], Ocupado(6))

    assert comandos == ["primeiro", "segundo"]
    assert acordou == [1], "o segundo comando não pode ter pedido wake word"
    # O áudio do 2º comando começa no bloco 200: os 4 de eco (100-103) ficaram na guarda.
    assert transcritor.primeiros == [0, 200]


def test_a_guarda_de_eco_descarta_o_rabo_da_voz_do_james():
    fonte = fonte_padrao()
    # Mesmo cenário, mas com guarda de zero: a continuação começaria no eco.
    comandos, _, _, transcritor = rodar(fonte, COMANDO + COMANDO, ["primeiro", "segundo"], Ocupado(6), guarda_eco_s=0.0)

    assert transcritor.primeiros[1] == 100, "sem guarda a continuação começa no eco (é o que a guarda evita)"


def test_sem_voz_na_janela_de_continuacao_volta_a_exigir_a_wake_word():
    fonte = (
        [b"wake"] + [bloco(i) for i in range(12)]
        + [b"wake"] * 6
        + [bloco(100 + i) for i in range(4)]
        + [bloco(150)] * 60                                  # 6 s de silêncio: a janela expira
        + [bloco(200 + i) for i in range(12)]               # voz depois: sem wake word, não vale
    )

    comandos, acordou, _, _ = rodar(fonte, COMANDO + "." * 60 + "VVVVV" + "." * 7, ["primeiro", "fantasma"], Ocupado(6))

    assert comandos == ["primeiro"]
    assert acordou == [1]


def test_continuacao_desligada_nao_abre_janela():
    comandos, acordou, _, _ = rodar(fonte_padrao(), COMANDO + COMANDO, ["primeiro", "segundo"], Ocupado(6), continuacao_s=0)

    assert comandos == ["primeiro"], "com JAMES_CONTINUACAO_S=0 a voz sem wake word não vale"
    assert acordou == [1]


def test_nao_entendi_e_chamado_com_texto_vazio_e_abre_a_continuacao():
    eventos = []
    ocupado = Ocupado(3)

    def nao_entendeu():
        eventos.append("nao_entendeu")
        ocupado.acionar()  # Fala.nao_entendi() deixa o device ocupado

    transcritor = TranscritorQueLembra(["", "repetido"])
    fonte = (
        [b"wake"] + [bloco(i) for i in range(22)]          # 1,5 s de fala que o STT não entende
        + [b"wake"] * 3                                     # "Não entendi, pode repetir?"
        + [bloco(100 + i) for i in range(4)]                # guarda
        + [bloco(200 + i) for i in range(12)]               # o Arthur repete, sem wake word
    )
    comandos = []
    reconhecedor = ReconhecedorWake()
    escuta.escutar(
        fonte_audio=fonte, reconhecedor=reconhecedor, transcrever_comando=transcritor,
        ao_acordar=lambda: None, ao_comando=comandos.append, palavra_parada="",
        vad=VadFalso("V" * 15 + "." * 7 + COMANDO), parametros_fim=ParametrosDeFim(), executor=ExecutorSincrono(),
        ocupado=ocupado, continuacao_s=6.0, guarda_eco_s=0.4, ao_nao_entendeu=nao_entendeu,
    )

    assert eventos == ["nao_entendeu"]
    assert comandos == ["repetido"]


def test_ninguem_falar_depois_do_acordar_nao_diz_nao_entendi():
    eventos = []
    fonte = [b"wake"] + [bloco(i) for i in range(60)]  # acordou e ficou quieto 6 s
    reconhecedor = ReconhecedorWake()
    escuta.escutar(
        fonte_audio=fonte, reconhecedor=reconhecedor, transcrever_comando=TranscritorFalso(["x"]),
        ao_acordar=lambda: None, ao_comando=eventos.append, palavra_parada="",
        vad=VadFalso("." * 60), parametros_fim=ParametrosDeFim(), executor=ExecutorSincrono(),
        ocupado=Ocupado(0), ao_nao_entendeu=lambda: eventos.append("nao_entendeu"),
    )

    assert eventos == [], "silêncio não é 'não entendi': ninguém falou"


def test_sem_ocupado_o_comportamento_antigo_continua_igual():
    comandos = []
    escuta.escutar(
        fonte_audio=[b"wake"] + [bloco(i) for i in range(12)],
        reconhecedor=ReconhecedorWake(), transcrever_comando=lambda audio: "ok",
        ao_acordar=lambda: None, ao_comando=comandos.append, palavra_parada="",
        vad=VadFalso(COMANDO), parametros_fim=ParametrosDeFim(), executor=ExecutorSincrono(),
    )
    assert comandos == ["ok"]
