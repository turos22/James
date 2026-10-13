"""Síntese de voz local com o Piper.

A voz é carregada UMA vez e reaproveitada: a carga leva ~0,7 s e cada frase
depois sai em dezenas de milissegundos (RTF 0,017 medido no spike). Carregar
por frase jogaria fora o motivo de usar o Piper.

Usado pelo device (que fala as respostas de voz) e pelo servidor de TTS (que
gera o áudio das respostas de texto): a mesma voz nos dois caminhos.
"""

import threading
import wave
from pathlib import Path


def _carregar_piper(caminho):
    # Import tardio: carregar o piper/onnxruntime custa, e os testes que não
    # tocam em voz real não devem pagar isso.
    from piper import PiperVoice

    return PiperVoice.load(caminho)


class Sintetizador:
    """Texto -> (PCM int16 mono, taxa de amostragem)."""

    def __init__(self, caminho_voz, carregar=None):
        self._caminho = caminho_voz
        self._carregar = carregar or _carregar_piper
        self._voz = None
        self._trava = threading.Lock()

    def carregar_agora(self) -> None:
        """Pré-carrega a voz. O device chama no boot: a primeira frase falada
        não pode pagar a carga."""
        self._voz_carregada()

    def _voz_carregada(self):
        with self._trava:
            if self._voz is None:
                self._voz = self._carregar(self._caminho)
            return self._voz

    def sintetizar(self, texto: str) -> tuple[bytes, int]:
        if not texto.strip():
            return b"", 0
        pedacos = []
        taxa = 0
        for pedaco in self._voz_carregada().synthesize(texto):
            pedacos.append(pedaco.audio_int16_bytes)
            taxa = pedaco.sample_rate
        return b"".join(pedacos), taxa


def para_wav(destino, pcm: bytes, taxa: int) -> None:
    """Grava PCM int16 mono como WAV."""
    destino = Path(destino)
    destino.parent.mkdir(parents=True, exist_ok=True)
    with wave.open(str(destino), "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(taxa)
        w.writeframes(pcm)
