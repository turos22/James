"""O fim de fala por VAD com transcrição especulativa (spec 2026-10-10, §6.1).

Tudo injetado: fonte de áudio, VAD, transcritor, executor e medidor. O tempo é
o TEMPO DE ÁUDIO (cada bloco de 0,1 s conta 0,1 s), então nenhum teste depende
do relógio de parede. Nada aqui abre microfone, Vosk, Whisper ou rede."""

import concurrent.futures
import json
import logging

import pytest

from james_device import escuta
from james_device.escuta import (
    Coleta,
    ParametrosDeFim,
    coletar_comando,
    cobertura_do_audio_ctx,
    parametros_do_ambiente,
    parece_incompleta,
    validar_teto,
)

BLOCO = 3200  # 0,1 s de PCM 16 kHz mono 16 bits


def bloco(i):
    return bytes([i % 256]) * BLOCO


class VadFalso:
    """'V' = voz, '.' = silêncio, um por bloco, na ordem das chamadas."""

    def __init__(self, roteiro):
        self.roteiro = list(roteiro)
        self.resets = 0

    def tem_voz(self, _bloco):
        return (self.roteiro.pop(0) if self.roteiro else ".") == "V"

    def reset(self):
        self.resets += 1


class ExecutorSincrono:
    """Roda na hora: a especulação 'termina' antes de qualquer decisão."""

    def submit(self, funcao, *args):
        futuro = concurrent.futures.Future()
        try:
            futuro.set_result(funcao(*args))
        except Exception as erro:  # noqa: BLE001 — igual a um executor de verdade
            futuro.set_exception(erro)
        return futuro


class TranscritorFalso:
    """Devolve os textos roteirizados, um por chamada, e anota quantos blocos
    de áudio recebeu em cada uma."""

    def __init__(self, textos=None, erro=None):
        self.textos = list(textos or ["texto"])
        self.erro = erro
        self.blocos = []

    def __call__(self, audio):
        self.blocos.append(len(audio) // BLOCO)
        if self.erro:
            raise self.erro
        return self.textos.pop(0) if len(self.textos) > 1 else self.textos[0]


class MedidorFalso:
    def __init__(self):
        self.marcas = []

    def marca(self, nome, em=None):
        self.marcas.append((nome, em))

    def nomes(self):
        return [n for n, _ in self.marcas]


class ParadaNoBloco:
    """Reconhecedor que 'ouve' a palavra de parada no bloco de índice dado."""

    def __init__(self, indice):
        self.indice = indice
        self.n = -1

    def AcceptWaveform(self, _chunk):  # noqa: N802
        self.n += 1
        return self.n == self.indice

    def Result(self):  # noqa: N802
        return json.dumps({"text": "abre o navegador enviar"})

    def PartialResult(self):  # noqa: N802
        return json.dumps({"partial": ""})

    def Reset(self):  # noqa: N802
        pass


class Contador:
    """Relógio que anda 1.0 a cada consulta: serve de 'agora' injetado."""

    def __init__(self):
        self.t = 0.0

    def __call__(self):
        self.t += 1.0
        return self.t


def coletar(roteiro, textos=None, total_blocos=None, params=None, reconhecedor=None,
            palavra_parada="", transcritor=None, medidor=None, executor=None):
    n = total_blocos if total_blocos is not None else len(roteiro) + 5
    fonte = iter([bloco(i) for i in range(n)])
    transcritor = transcritor or TranscritorFalso(textos)
    medidor = medidor if medidor is not None else MedidorFalso()
    coleta = coletar_comando(
        fonte, VadFalso(roteiro), reconhecedor, transcritor,
        params or ParametrosDeFim(), medidor=medidor, executor=executor or ExecutorSincrono(),
        palavra_parada=palavra_parada, agora=Contador(),
    )
    return coleta, fonte, transcritor, medidor


def proximo(fonte):
    return next(fonte, None)


def test_voz_e_silencio_de_700ms_fecha_com_a_especulacao_sem_transcrever_de_novo():
    coleta, fonte, transcritor, _ = coletar("VVVVV" + "." * 15, textos=["abre o navegador"])

    assert coleta == Coleta(texto="abre o navegador", desistiu=False)
    # A especulação saiu no 1º bloco de silêncio: 5 de voz + 1 de silêncio.
    assert transcritor.blocos == [6]
    # Fechou no 7º bloco de silêncio (0,7 s), deixando o resto da fonte intacto.
    assert proximo(fonte) == bloco(12)


def test_voz_que_volta_antes_de_700ms_descarta_a_especulacao():
    roteiro = "VVVVV" + "..." + "VVV" + "." * 15

    coleta, _, transcritor, _ = coletar(roteiro, textos=["primeira", "segunda"])

    assert transcritor.blocos == [6, 12], "a segunda especulação tem que cobrir TODA a fala até ali"
    assert coleta.texto == "segunda", "valeu a transcrição da fala completa, não a da pausa no meio"


def test_palavra_de_ligacao_estica_a_espera_para_1500ms():
    coleta, fonte, _, _ = coletar("VVVVV" + "." * 30, textos=["abre o navegador e"])

    assert coleta.texto == "abre o navegador e"
    assert proximo(fonte) == bloco(20), "fechou em 1,5 s de silêncio (15 blocos), não em 0,7 s"


def test_frase_completa_fecha_em_700ms_mesmo_com_espera_longa_disponivel():
    _, fonte, _, _ = coletar("VVVVV" + "." * 30, textos=["abre o navegador."])
    assert proximo(fonte) == bloco(12)


def test_teto_de_dez_segundos_fecha_com_voz_continua():
    coleta, fonte, transcritor, _ = coletar("V" * 120, textos=["fala sem parar"], total_blocos=130)

    assert coleta.texto == "fala sem parar"
    assert transcritor.blocos == [100], "uma só transcrição, da fala inteira até o teto"
    assert proximo(fonte) == bloco(100)


def test_especulacao_velha_nao_vale_no_teto_quando_a_voz_voltou():
    # Pausa curta (gera uma especulação só com o começo), a voz volta e segue
    # até o teto: o que vale é a transcrição da fala INTEIRA, não a do começo.
    coleta, _, transcritor, _ = coletar("VV" + "." + "V" * 120, textos=["so o comeco", "a fala inteira"],
                                        total_blocos=130)

    assert transcritor.blocos == [3, 100]
    assert coleta.texto == "a fala inteira"


def test_o_teto_conta_a_partir_da_primeira_voz_e_nao_do_silencio_inicial():
    coleta, _, transcritor, _ = coletar("." * 30 + "V" * 5 + "." * 10, textos=["ok"], total_blocos=60)

    assert coleta.texto == "ok"
    assert not coleta.desistiu


def test_enviar_fecha_na_hora_e_o_texto_vai_cru_pro_chamador_limpar():
    reconhecedor = ParadaNoBloco(indice=8)

    coleta, fonte, transcritor, _ = coletar("V" * 30, textos=["abre o navegador enviar"],
                                            reconhecedor=reconhecedor, palavra_parada="enviar")

    assert transcritor.blocos == [9]
    assert coleta.texto == "abre o navegador enviar", "tirar a palavra de parada é do escutar, como sempre foi"
    assert proximo(fonte) == bloco(9)


def test_sem_voz_em_cinco_segundos_desiste_sem_transcrever():
    coleta, fonte, transcritor, _ = coletar("." * 80)

    assert coleta == Coleta(texto="", desistiu=True)
    assert transcritor.blocos == []
    assert proximo(fonte) == bloco(50)


def test_fonte_que_acaba_no_meio_da_fala_transcreve_o_que_veio():
    coleta, _, transcritor, _ = coletar("VVV", textos=["cortou"], total_blocos=3)

    assert coleta.texto == "cortou"
    assert transcritor.blocos == [3]


def test_fonte_vazia_sem_voz_desiste():
    coleta, _, transcritor, _ = coletar("", total_blocos=0)
    assert coleta.desistiu and transcritor.blocos == []


def test_transcritor_que_quebra_devolve_texto_vazio_sem_derrubar(caplog):
    with caplog.at_level(logging.ERROR):
        coleta, _, _, _ = coletar("VVVVV" + "." * 15, transcritor=TranscritorFalso(erro=RuntimeError("whisper fora")))

    assert coleta == Coleta(texto="", desistiu=False)
    assert any("transcrever" in r.message for r in caplog.records)


def test_medidor_recebe_fim_da_fala_no_silencio_que_fechou_e_o_instante_do_stt():
    coleta, _, _, medidor = coletar("VV" + "." + "V" + "." * 10, textos=["a", "b"])

    fins = [em for n, em in medidor.marcas if n == "fim_fala"]
    assert len(fins) == 2, "um começo de silêncio por pausa; vale o último"
    assert fins[1] > fins[0]
    stt = [em for n, em in medidor.marcas if n == "stt_pronto"]
    assert len(stt) == 1 and isinstance(stt[0], float), "o instante vem do término da transcrição usada"
    assert coleta.texto == "b"


# ---------- parece_incompleta ----------

@pytest.mark.parametrize("texto,esperado", [
    ("abre o navegador e", True),
    ("abre o navegador e,", True),
    ("abre o navegador então", True),
    ("manda pra", True),
    ("abre o navegador...", True),
    ("abre o navegador…", True),
    ("abre o navegador", False),
    ("abre o navegador.", False),
    ("que horas são?", False),
    ("pra.", False),
    ("", False),
    ("   ", False),
])
def test_parece_incompleta(texto, esperado):
    assert parece_incompleta(texto) is esperado


# ---------- teto x contexto do Whisper ----------

def test_cobertura_do_audio_ctx():
    assert cobertura_do_audio_ctx(512) == pytest.approx(10.24)
    assert cobertura_do_audio_ctx(768) == pytest.approx(15.36)


def test_teto_dentro_da_cobertura_passa():
    validar_teto(10.0, 512)
    validar_teto(10.2, 512)


def test_teto_acima_da_cobertura_do_ac_falha_com_mensagem_clara():
    with pytest.raises(ValueError, match=r"(?s)15.*-ac 512|-ac 512.*15"):
        validar_teto(15.0, 512)


def test_parametros_padrao_do_ambiente():
    p = parametros_do_ambiente({})
    assert p == ParametrosDeFim(silencio_s=0.7, silencio_longo_s=1.5, comando_max_s=10.0, espera_voz_s=5.0)


def test_parametros_do_ambiente_em_milissegundos_e_segundos():
    p = parametros_do_ambiente({"JAMES_FIM_SILENCIO_MS": "900", "JAMES_FIM_SILENCIO_LONGO_MS": "2000",
                                "JAMES_COMANDO_MAX_S": "8", "JAMES_ESPERA_VOZ_S": "4"})
    assert p == ParametrosDeFim(silencio_s=0.9, silencio_longo_s=2.0, comando_max_s=8.0, espera_voz_s=4.0)


def test_ambiente_com_teto_maior_que_o_ac_falha_na_construcao():
    with pytest.raises(ValueError, match="-ac"):
        parametros_do_ambiente({"JAMES_COMANDO_MAX_S": "15"}, audio_ctx=512)
    parametros_do_ambiente({"JAMES_COMANDO_MAX_S": "15"}, audio_ctx=768)  # com -ac 768 cabe


def test_ambiente_com_espera_longa_menor_que_a_curta_falha():
    with pytest.raises(ValueError, match="longo"):
        parametros_do_ambiente({"JAMES_FIM_SILENCIO_MS": "900", "JAMES_FIM_SILENCIO_LONGO_MS": "500"})


def test_ambiente_com_valor_ilegivel_falha_nomeando_a_variavel():
    with pytest.raises(ValueError, match="JAMES_COMANDO_MAX_S"):
        parametros_do_ambiente({"JAMES_COMANDO_MAX_S": "dez"})


# ---------- integrado ao laço de escuta ----------

def test_escutar_com_vad_entrega_o_comando_transcrito_na_especulacao():
    from tests.test_escuta import ReconhecedorFalso

    # 1 bloco com a wake word, depois a fala (5 de voz) e silêncio.
    fonte = [b"wake"] + [bloco(i) for i in range(20)]
    reconhecedor = ReconhecedorFalso([("parcial", "james")])
    comandos = []

    escuta.escutar(
        fonte_audio=fonte,
        reconhecedor=reconhecedor,
        transcrever_comando=lambda audio: "abre o navegador",
        ao_acordar=lambda: None,
        ao_comando=comandos.append,
        palavra_parada="",
        vad=VadFalso("VVVVV" + "." * 15),
        parametros_fim=ParametrosDeFim(),
        executor=ExecutorSincrono(),
    )

    assert comandos == ["abre o navegador"]
