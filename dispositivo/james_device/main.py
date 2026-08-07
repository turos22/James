"""Ponto de entrada do james-device: liga wake word, voz e tela numa
conversa contínua com o jamesd. Substitui processador_voz.py da raiz.
Depende de microfone real — verificado à mão (Step 11), não em pytest.
"""

import logging
import os
import threading

import speech_recognition as sr
from playsound import playsound

from james_device.conexao import ConexaoJames
from james_device.tela import executar_acao
from james_device.wake import contem_wake_word, quase_acerto, registrar_quase_acerto

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

URL_JAMESD = os.environ.get("JAMES_WS_URL", "ws://127.0.0.1:8080/ws/dispositivo")
PALAVRA_PARADA = "enviar"


def _ouvir_comando(recognizer, source) -> str:
    buffer = []
    while True:
        try:
            audio = recognizer.listen(source, timeout=5, phrase_time_limit=10)
            parcial = recognizer.recognize_google(audio, language="pt-BR").lower()
        except (sr.WaitTimeoutError, sr.UnknownValueError):
            continue

        if PALAVRA_PARADA in parcial:
            limpo = parcial.replace(PALAVRA_PARADA, "").strip()
            if limpo:
                buffer.append(limpo)
            playsound("dispositivo/assets/faaah.wav")
            return " ".join(buffer).strip()
        buffer.append(parcial)


def escutar_e_enviar(conexao: ConexaoJames):
    recognizer = sr.Recognizer()
    recognizer.dynamic_energy_threshold = True

    try:
        microfone = sr.Microphone()
    except Exception:
        # PyAudio ausente ou sem dispositivo de entrada — sr.Microphone()
        # levanta na hora, sem log nenhum por padrão. Falhar alto continua
        # certo aqui (não dá pra escutar sem microfone), mas sem isso a
        # única pista que sobrava era o traceback cru no console.
        logger.exception("não consegui abrir o microfone — PyAudio instalado? dispositivo de entrada disponível?")
        raise

    with microfone as source:
        recognizer.adjust_for_ambient_noise(source, duration=1)
        logger.info("escuta ativa iniciada")

        while True:
            try:
                audio = recognizer.listen(source, phrase_time_limit=3)
                texto = recognizer.recognize_google(audio, language="pt-BR").lower()
            except (sr.UnknownValueError, sr.WaitTimeoutError):
                continue
            except sr.RequestError:
                logger.error("erro de conexão com o Google Speech")
                continue

            if contem_wake_word(texto):
                playsound("dispositivo/assets/entrada.mp3")
                comando = _ouvir_comando(recognizer, source)
                if comando:
                    conexao.enviar_entrada_voz(comando)
            else:
                candidato = quase_acerto(texto)
                if candidato:
                    registrar_quase_acerto(candidato)


def main():
    conexao = ConexaoJames(URL_JAMESD, manipulador_acao=executar_acao)
    threading.Thread(target=conexao.executar, daemon=True).start()
    escutar_e_enviar(conexao)


if __name__ == "__main__":
    main()
