"""O STT do comando: Whisper local (padrão) ou Google, por JAMES_STT.

O contrato é o de sempre, Callable[[bytes], str]: PCM 16 kHz mono 16 bits
entra, texto sai. Falha devolve "" e loga; quem decide o que fazer com um
comando vazio é o escutar (que manda o James pedir pra repetir).

Sem fallback para o Google quando o Whisper falha (decisão da Parte A): o áudio
do comando não volta a sair da máquina por baixo dos panos.
"""

import io
import logging
import re
import wave
from collections.abc import Mapping

import requests

logger = logging.getLogger(__name__)

TAXA_AMOSTRAGEM = 16000
LARGURA_AMOSTRA = 2

URL_WHISPER_PADRAO = "http://127.0.0.1:8179"
TIMEOUT_WHISPER_S = 10.0

# O Whisper "alucina" frases-padrão em áudio sem fala (cresceram de legendas de
# vídeo no treino). Estas, sozinhas, não são comando de ninguém. A lista é só o
# que se sabe com certeza; cresce com o que o uso real mostrar.
FRASE_VAZIA = frozenset({
    "legendas pela comunidade amara.org",
    "obrigado por assistir",
    "obrigado por assistir!",
})

_TEXTO_ENTRE_COLCHETES = re.compile(r"^[\[\(].*[\]\)]\.?$")
_TEM_CONTEUDO = re.compile(r"[\w]", re.UNICODE)


def _e_alucinacao(texto: str) -> bool:
    t = texto.strip()
    if not t or not _TEM_CONTEUDO.search(t):
        return True
    if _TEXTO_ENTRE_COLCHETES.match(t):  # [BLANK_AUDIO], [Música], (silêncio)
        return True
    return t.lower() in FRASE_VAZIA


def _wav(audio: bytes) -> bytes:
    buffer = io.BytesIO()
    with wave.open(buffer, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(LARGURA_AMOSTRA)
        w.setframerate(TAXA_AMOSTRAGEM)
        w.writeframes(audio)
    return buffer.getvalue()


def transcritor_whisper(url: str = URL_WHISPER_PADRAO):
    """Fala com o whisper-server (whisper.cpp, GPU), que já está com o modelo
    carregado: o modelo nunca é carregado por comando."""

    def transcrever(audio: bytes) -> str:
        if not audio:
            return ""
        try:
            resp = requests.post(
                f"{url}/inference",
                files={"file": ("comando.wav", _wav(audio), "audio/wav")},
                data={"response_format": "json", "temperature": "0.0"},
                timeout=TIMEOUT_WHISPER_S,
            )
            if resp.status_code != 200:
                logger.warning("whisper-server devolveu HTTP %d, comando sem texto", resp.status_code)
                return ""
            texto = resp.json()["text"]
        except requests.RequestException as erro:
            logger.warning("whisper-server fora do ar ou lento (%s), comando sem texto", erro)
            return ""
        except (ValueError, KeyError, TypeError):
            logger.warning("resposta ilegível do whisper-server, comando sem texto")
            return ""
        if not isinstance(texto, str):
            logger.warning("whisper-server devolveu texto que não é string, comando sem texto")
            return ""
        texto = texto.strip()
        if _e_alucinacao(texto):
            logger.info("whisper devolveu %r, que não é fala: tratado como vazio", texto)
            return ""
        return texto

    return transcrever


def transcritor_google():
    """O STT de antes: API web gratuita do Google, sem SLA, e o áudio sai da
    máquina. Fica como opção explícita (JAMES_STT=google)."""
    import speech_recognition as sr

    reconhecedor = sr.Recognizer()

    def transcrever(audio: bytes) -> str:
        if not audio:
            return ""
        dados = sr.AudioData(audio, TAXA_AMOSTRAGEM, LARGURA_AMOSTRA)
        try:
            return reconhecedor.recognize_google(dados, language="pt-BR")
        except sr.UnknownValueError:
            logger.info("não entendi o que foi falado na janela")
            return ""

    return transcrever


def criar_transcritor(env: Mapping):
    """Escolhe pelo JAMES_STT (whisper por padrão). Valor inválido falha no boot:
    é melhor do que cair num STT que ninguém escolheu."""
    escolha = (env.get("JAMES_STT") or "whisper").strip().lower()
    if escolha == "whisper":
        return transcritor_whisper(env.get("JAMES_WHISPER_URL") or URL_WHISPER_PADRAO)
    if escolha == "google":
        return transcritor_google()
    raise ValueError(f"JAMES_STT={escolha!r} inválido: use 'whisper' ou 'google'")
