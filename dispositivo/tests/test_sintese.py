"""A síntese local com o Piper. A voz é carregada UMA vez (0,7 s) e depois cada
frase sai em milissegundos; carregar por frase jogaria fora o ganho todo."""

import wave
from pathlib import Path

import pytest

from james_device.sintese import Sintetizador, para_wav

RAIZ = Path(__file__).resolve().parents[2]
VOZ_REAL = RAIZ / "vozes" / "pt_BR-faber-medium.onnx"


class _Pedaco:
    def __init__(self, dados: bytes, taxa: int):
        self.audio_int16_bytes = dados
        self.sample_rate = taxa


class _VozFalsa:
    def __init__(self):
        self.textos = []

    def synthesize(self, texto):
        self.textos.append(texto)
        yield _Pedaco(b"\x01\x00" * 3, 22050)
        yield _Pedaco(b"\x02\x00" * 2, 22050)


def test_carrega_a_voz_uma_vez_so():
    cargas = []

    def carregar(caminho):
        cargas.append(caminho)
        return _VozFalsa()

    s = Sintetizador("voz.onnx", carregar=carregar)
    s.sintetizar("um.")
    s.sintetizar("dois.")
    s.sintetizar("três.")

    assert cargas == ["voz.onnx"], "a voz foi carregada mais de uma vez"


def test_junta_os_pedacos_e_devolve_a_taxa():
    s = Sintetizador("voz.onnx", carregar=lambda _: _VozFalsa())

    pcm, taxa = s.sintetizar("oi.")

    assert pcm == b"\x01\x00" * 3 + b"\x02\x00" * 2
    assert taxa == 22050


def test_texto_vazio_devolve_pcm_vazio_sem_chamar_a_voz():
    voz = _VozFalsa()
    s = Sintetizador("voz.onnx", carregar=lambda _: voz)

    pcm, _ = s.sintetizar("   ")

    assert pcm == b""
    assert voz.textos == []


def test_carga_so_acontece_no_primeiro_uso():
    cargas = []
    Sintetizador("voz.onnx", carregar=lambda c: cargas.append(c) or _VozFalsa())
    assert cargas == [], "carregar no construtor atrasa o boot do device mesmo sem fala"


def test_para_wav_gera_arquivo_valido(tmp_path):
    destino = tmp_path / "a.wav"
    para_wav(destino, b"\x01\x00" * 100, 22050)

    with wave.open(str(destino), "rb") as w:
        assert (w.getnchannels(), w.getsampwidth(), w.getframerate()) == (1, 2, 22050)
        assert w.getnframes() == 100


@pytest.mark.skipif(not VOZ_REAL.exists(), reason="voz do Piper ausente em vozes/ (ver Task 0 do plano)")
def test_voz_real_sintetiza_uma_frase():
    s = Sintetizador(str(VOZ_REAL))

    pcm, taxa = s.sintetizar("Teste de voz.")

    assert taxa == 22050
    assert len(pcm) > taxa * 2 * 0.5, "menos de meio segundo de áudio para uma frase"
