"""Ponto de entrada do james-device: liga wake word, voz e tela numa
conversa contínua com o jamesd.

Duas responsabilidades independentes moram aqui, e a fiação entre elas é
deliberada: a ponte de tela (ConexaoJames) roda na thread PRINCIPAL e a
escuta numa thread secundária. Era o contrário, e o contrário estava errado —
`sr.Microphone()` sem PyAudio, sem placa de entrada, ou um erro de rede na
transcrição levantava na thread principal, e como a thread da conexão era
`daemon`, o processo inteiro saía junto. Problema de microfone derrubava o
controle de tela, que não tem nada a ver com áudio, e o jamesd passava a
responder "máquina física indisponível".

Nesta ordem, o pior caso de um problema de áudio é ficar sem voz: a ponte de
tela continua de pé, e o log diz alto o que caiu.
"""

import logging
import os
import threading

import speech_recognition as sr
import vosk
from playsound import playsound

from james_device.conexao import ConexaoJames
from james_device.escuta import JANELA_PADRAO_SEGUNDOS, PALAVRA_PARADA_PADRAO, escutar
from james_device.tela import executar_acao
from james_device.wake import quase_acerto, registrar_quase_acerto

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

URL_JAMESD = os.environ.get("JAMES_WS_URL", "ws://127.0.0.1:8080/ws/dispositivo")

# Caminho relativo à raiz do repo — mesmo contrato de cwd que os assets já
# tinham (ver executar.bat). O modelo não é versionado (.gitignore).
CAMINHO_MODELO_VOSK = os.environ.get("JAMES_VOSK_MODELO", "vosk/vosk-model-small-pt-0.3")

# 16 kHz mono 16 bits é o que o modelo do Vosk espera. Não é preferência:
# alimentar outra taxa devolve reconhecimento silenciosamente ruim, sem erro.
TAXA_AMOSTRAGEM = 16000
LARGURA_AMOSTRA = 2

# 0,25s por bloco. Menor deixa a ativação mais responsiva e custa mais CPU por
# segundo de áudio; maior atrasa a wake word em até o tamanho do bloco.
QUADROS_POR_BLOCO = TAXA_AMOSTRAGEM // 4

JANELA_SEGUNDOS = float(os.environ.get("JAMES_JANELA_SEGUNDOS", JANELA_PADRAO_SEGUNDOS))

# Fecha a janela na hora em vez de esperar JANELA_SEGUNDOS. String vazia
# desliga e deixa só o tempo mandar.
PALAVRA_PARADA = os.environ.get("JAMES_PALAVRA_PARADA", PALAVRA_PARADA_PADRAO)


def blocos_do_microfone(quadros_por_bloco=QUADROS_POR_BLOCO):
    """Gerador infinito de PCM cru do microfone padrão.

    Stream contínuo, e é o ponto: o desenho antigo alternava entre gravar e
    esperar a rede responder, e tudo que fosse dito durante a espera era
    perdido. Aqui o stream nunca para — quem consome é que decide o que fazer
    com cada bloco.
    """
    import pyaudio

    audio = pyaudio.PyAudio()
    stream = audio.open(
        format=pyaudio.paInt16,
        channels=1,
        rate=TAXA_AMOSTRAGEM,
        input=True,
        frames_per_buffer=quadros_por_bloco,
    )
    try:
        while True:
            # exception_on_overflow=False: um estouro de buffer (a máquina
            # engasgou, outro processo segurou a CPU) custa um bloco de áudio,
            # não a escuta inteira. Levantar aqui mataria a thread de voz por
            # um soluço de 0,25s.
            yield stream.read(quadros_por_bloco, exception_on_overflow=False)
    finally:
        stream.stop_stream()
        stream.close()
        audio.terminate()


def criar_transcritor():
    """Devolve a função que transcreve os bytes da janela em texto.

    Continua sendo o Google: pra frase livre em português o modelo small do
    Vosk perde feio, e aqui a latência de rede não custa nada — a wake word
    (que era o que sofria com ela) já foi resolvida offline antes deste ponto.
    """
    recognizer = sr.Recognizer()

    def transcrever(audio: bytes) -> str:
        if not audio:
            return ""
        dados = sr.AudioData(audio, TAXA_AMOSTRAGEM, LARGURA_AMOSTRA)
        try:
            return recognizer.recognize_google(dados, language="pt-BR")
        except sr.UnknownValueError:
            logger.info("não entendi o que foi falado na janela")
            return ""

    return transcrever


def tocar_em_paralelo(caminho: str):
    """Toca o som numa thread. Bloquear aqui comeria o começo da janela de
    escuta, que abre imediatamente depois — o preço é o bipe vazar um pouco
    pro áudio gravado, e a transcrição lida bem com isso."""
    threading.Thread(target=playsound, args=(caminho,), daemon=True).start()


def escutar_e_enviar(conexao: ConexaoJames):
    """Carrega o modelo, abre o microfone e entrega o laço pro escuta.py."""
    # SetLogLevel(-1) antes de carregar: o Vosk despeja o grafo inteiro em
    # stderr no nível padrão e enterra qualquer log útil do device.
    vosk.SetLogLevel(-1)
    modelo = vosk.Model(CAMINHO_MODELO_VOSK)
    reconhecedor = vosk.KaldiRecognizer(modelo, TAXA_AMOSTRAGEM)

    logger.info(
        "escuta ativa: wake word offline, janela de %.0fs, palavra de parada %r",
        JANELA_SEGUNDOS,
        PALAVRA_PARADA or "(desligada)",
    )

    escutar(
        fonte_audio=blocos_do_microfone(),
        reconhecedor=reconhecedor,
        transcrever_comando=criar_transcritor(),
        # Dois sons, dois significados, iguais aos do laço antigo: entrada.mp3
        # é "acordei, pode falar", faaah.wav é "ouvi, fechei, estou pensando".
        ao_acordar=lambda: tocar_em_paralelo("dispositivo/assets/entrada.mp3"),
        ao_fechar_janela=lambda: tocar_em_paralelo("dispositivo/assets/faaah.wav"),
        ao_comando=conexao.enviar_entrada_voz,
        janela_segundos=JANELA_SEGUNDOS,
        palavra_parada=PALAVRA_PARADA,
        ao_quase_acerto=lambda texto: _registrar_se_quase(texto),
    )


def _registrar_se_quase(texto: str) -> None:
    candidato = quase_acerto(texto)
    if candidato:
        registrar_quase_acerto(candidato)


def _thread_de_escuta(conexao: ConexaoJames):
    try:
        escutar_e_enviar(conexao)
    except Exception:
        # Alto e explícito: sem isso, uma thread que morre no arranque some em
        # silêncio e o sintoma vira "o James não responde quando eu falo", sem
        # nenhuma pista de por quê.
        logger.exception(
            "a escuta caiu — o James ficou surdo, mas a ponte de tela continua de pé. "
            "Microfone conectado? PyAudio instalado? O modelo existe em %s?",
            CAMINHO_MODELO_VOSK,
        )


def main():
    conexao = ConexaoJames(URL_JAMESD, manipulador_acao=executar_acao)
    threading.Thread(target=_thread_de_escuta, args=(conexao,), daemon=True).start()
    # Na thread principal: é a parte que não pode morrer por causa de áudio.
    conexao.executar()


if __name__ == "__main__":
    main()
