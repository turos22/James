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
import sys
import threading
from collections.abc import Mapping
from dataclasses import dataclass

import vosk

from james_device.conexao import ConexaoJames
from james_device import gramatica
from james_device.escuta import (
    GUARDA_ECO_PADRAO_S,
    JANELA_PADRAO_SEGUNDOS,
    PALAVRA_PARADA_PADRAO,
    ParametrosDeFim,
    escutar,
    parametros_do_ambiente,
)
from james_device.fala import FRASE_SEM_SERVIDOR, Fala, ReprodutorPyAudio
from james_device.medidor import Medidor
from james_device.sintese import Sintetizador
from james_device.transcricao import criar_transcritor
from james_device.plataforma import atual as plataforma
from james_device.wake import VARIANTES_PADRAO, quase_acerto, registrar_quase_acerto

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

# Tamanho do bloco do microfone. 0,1 s: o fim de fala é medido em blocos, e com
# 0,25 s a espera de 0,7 s só fecharia entre 0,75 s e 1,0 s. Menor custa mais
# CPU por segundo de áudio; o piso é o quadro do VAD (32 ms).
BLOCO_PADRAO_MS = 100
BLOCO_MINIMO_MS = 32

CAMINHO_VOZ_PIPER = os.environ.get("JAMES_PIPER_VOZ", "vozes/pt_BR-faber-medium.onnx")

JANELA_SEGUNDOS = float(os.environ.get("JAMES_JANELA_SEGUNDOS", JANELA_PADRAO_SEGUNDOS))

# Fecha a janela na hora em vez de esperar JANELA_SEGUNDOS. String vazia
# desliga e deixa só o tempo mandar.
PALAVRA_PARADA = os.environ.get("JAMES_PALAVRA_PARADA", PALAVRA_PARADA_PADRAO)


def blocos_do_microfone(quadros_por_bloco=None):
    """Gerador infinito de PCM cru do microfone padrão.

    Stream contínuo, e é o ponto: o desenho antigo alternava entre gravar e
    esperar a rede responder, e tudo que fosse dito durante a espera era
    perdido. Aqui o stream nunca para — quem consome é que decide o que fazer
    com cada bloco.
    """
    import pyaudio

    if quadros_por_bloco is None:
        quadros_por_bloco = bloco_do_ambiente(os.environ)
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


def _numero_do_ambiente(env: Mapping, nome: str, padrao: float, minimo: float, divisor: float = 1.0) -> float:
    bruto = env.get(nome)
    if bruto in (None, ""):
        return padrao
    try:
        valor = float(bruto) / divisor
    except ValueError:
        raise ValueError(f"{nome}={bruto!r} não é um número") from None
    if valor < minimo:
        raise ValueError(f"{nome}={bruto!r} é pequeno demais")
    return valor


def bloco_do_ambiente(env: Mapping) -> int:
    """Quadros (amostras) por bloco do microfone, de JAMES_BLOCO_MS."""
    ms = _numero_do_ambiente(env, "JAMES_BLOCO_MS", BLOCO_PADRAO_MS, BLOCO_MINIMO_MS)
    return round(ms * TAXA_AMOSTRAGEM / 1000)


def continuacao_do_ambiente(env: Mapping) -> float:
    """Segundos de janela, depois que o James cala, em que voz vira comando sem
    wake word. 0 desliga."""
    return _numero_do_ambiente(env, "JAMES_CONTINUACAO_S", 6.0, 0.0)


def guarda_eco_do_ambiente(env: Mapping) -> float:
    """Segundos descartados depois que o James cala, pro rabo da voz dele no ar
    da sala não virar comando."""
    return _numero_do_ambiente(env, "JAMES_GUARDA_ECO_MS", GUARDA_ECO_PADRAO_S, 0.0, 1000.0)


@dataclass(frozen=True)
class ConfiguracaoDeVoz:
    transcrever: object
    parametros_fim: ParametrosDeFim
    continuacao_s: float
    guarda_eco_s: float


def configurar(env: Mapping) -> ConfiguracaoDeVoz:
    """Lê e valida tudo de uma vez, no boot. Configuração errada tem de matar o
    device na partida com uma mensagem clara, não no meio de uma fala."""
    audio_ctx = int(_numero_do_ambiente(env, "JAMES_WHISPER_AC", 512, 1))
    return ConfiguracaoDeVoz(
        transcrever=criar_transcritor(env),
        parametros_fim=parametros_do_ambiente(env, audio_ctx=audio_ctx),
        continuacao_s=continuacao_do_ambiente(env),
        guarda_eco_s=guarda_eco_do_ambiente(env),
    )


def configurar_ou_degradar(env: Mapping, tela_disponivel: bool) -> "ConfiguracaoDeVoz | None":
    """configurar() com a política de falha certa pra cada sistema.

    Com ponte de tela (Windows, e Linux com KDE Wayland desde a fatia B2),
    configuração de voz inválida desliga SÓ a voz, em voz alta: o controle de
    tela não tem nada a ver com áudio e não pode morrer por um JAMES_STT
    digitado errado. Sem ponte de tela (outro desktop Linux) o device é só voz,
    então sem voz não há o que manter de pé: falha na partida."""
    try:
        return configurar(env)
    except ValueError as erro:
        if tela_disponivel:
            logger.error("configuração de voz inválida, a VOZ fica desligada e a ponte de tela segue: %s", erro)
            return None
        logger.error("configuração de voz inválida, o device não sobe: %s", erro)
        raise SystemExit(2) from erro


def criar_ao_comando(fala, conexao, medidor):
    """O que acontece quando um comando está pronto pra sair.

    Marca o device como ocupado ANTES de enviar: se a resposta chegasse antes do
    ocupado, a primeira frase tocaria com a escuta ainda aberta. Se o jamesd não
    está lá, o James diz isso, localmente."""
    if fala is None:
        return conexao.enviar_entrada_voz

    def ao_comando(texto: str):
        fala.comando_enviado()
        if conexao.enviar_entrada_voz(texto):
            if medidor is not None:
                medidor.marca("entrada_enviada")
        else:
            fala.erro(FRASE_SEM_SERVIDOR)

    return ao_comando


def tocar_em_paralelo(caminho: str):
    """Toca o som numa thread. Bloquear aqui comeria o começo da janela de
    escuta, que abre imediatamente depois — o preço é o bipe vazar um pouco
    pro áudio gravado, e a transcrição lida bem com isso."""
    threading.Thread(target=plataforma.tocar_som, args=(caminho,), daemon=True).start()


def _criar_vad():
    """O VAD é o que faz o comando fechar quando a pessoa para de falar. Se não
    carregar, o device cai na janela fixa de antes em vez de ficar surdo."""
    try:
        from james_device.vad import DetectorDeVoz

        return DetectorDeVoz()
    except Exception:
        logger.exception("VAD não carregou: o comando volta a fechar por janela fixa de %.0fs", JANELA_SEGUNDOS)
        return None


def escutar_e_enviar(conexao: ConexaoJames, fala: Fala | None, config: ConfiguracaoDeVoz, medidor: Medidor):
    """Carrega o modelo, abre o microfone e entrega o laço pro escuta.py."""
    # SetLogLevel(-1) antes de carregar: o Vosk despeja o grafo inteiro em
    # stderr no nível padrão e enterra qualquer log útil do device.
    vosk.SetLogLevel(-1)
    modelo = vosk.Model(CAMINHO_MODELO_VOSK)

    # Gramática, e não decoder livre. Sem ela o Vosk decodifica contra o
    # português inteiro e "james" perde para "gêmeos"/"times", que são
    # hipóteses muito mais prováveis no modelo de linguagem — medido: 2
    # acertos em 5 frases. Com a gramática o decoder só escolhe entre as
    # variantes, a palavra de parada e "[unk]": 5 em 5. Ver gramatica.py.
    reconhecedor = vosk.KaldiRecognizer(
        modelo, TAXA_AMOSTRAGEM, gramatica.montar(VARIANTES_PADRAO, PALAVRA_PARADA)
    )
    vad = _criar_vad()

    logger.info(
        "escuta ativa: wake word offline, fim de fala por %s, teto de %.0fs, continuação de %.0fs, palavra de parada %r",
        "VAD" if vad is not None else f"janela fixa de {JANELA_SEGUNDOS:.0f}s",
        config.parametros_fim.comando_max_s,
        config.continuacao_s if vad is not None else 0,
        PALAVRA_PARADA or "(desligada)",
    )

    escutar(
        fonte_audio=blocos_do_microfone(),
        reconhecedor=reconhecedor,
        transcrever_comando=config.transcrever,
        # Dois sons, dois significados, iguais aos do laço antigo: entrada.mp3
        # é "acordei, pode falar", faaah.wav é "ouvi, fechei, estou pensando".
        ao_acordar=lambda: tocar_em_paralelo("dispositivo/assets/entrada.mp3"),
        ao_fechar_janela=lambda: tocar_em_paralelo("dispositivo/assets/faaah.wav"),
        ao_comando=criar_ao_comando(fala, conexao, medidor),
        janela_segundos=JANELA_SEGUNDOS,
        palavra_parada=PALAVRA_PARADA,
        ao_quase_acerto=lambda texto: _registrar_se_quase(texto),
        vad=vad,
        medidor=medidor,
        parametros_fim=config.parametros_fim,
        ocupado=fala.ocupado if fala is not None else None,
        continuacao_s=config.continuacao_s,
        guarda_eco_s=config.guarda_eco_s,
        ao_nao_entendeu=fala.nao_entendi if fala is not None else None,
    )


def _registrar_se_quase(texto: str) -> None:
    candidato = quase_acerto(texto)
    if candidato:
        registrar_quase_acerto(candidato)


def _thread_de_escuta(conexao: ConexaoJames, fala: Fala | None, config: ConfiguracaoDeVoz, medidor: Medidor):
    try:
        escutar_e_enviar(conexao, fala, config, medidor)
    except Exception:
        # Alto e explícito: sem isso, uma thread que morre no arranque some em
        # silêncio e o sintoma vira "o James não responde quando eu falo", sem
        # nenhuma pista de por quê.
        logger.exception(
            "a escuta caiu — o James ficou surdo, mas a ponte de tela continua de pé. "
            "Microfone conectado? PyAudio instalado? O modelo existe em %s?",
            CAMINHO_MODELO_VOSK,
        )


def _montar_fala(medidor: Medidor) -> Fala | None:
    """Carrega a voz do Piper (0,7 s, uma vez, no boot: a primeira frase falada
    não pode pagar a carga). Sem voz o device segue ouvindo e enviando, mas as
    respostas ficam mudas, e isso é dito alto aqui."""
    try:
        sintetizador = Sintetizador(CAMINHO_VOZ_PIPER)
        sintetizador.carregar_agora()
        return Fala(sintetizador, ReprodutorPyAudio(), medidor=medidor)
    except Exception:
        logger.exception(
            "a voz não carregou: o James vai ouvir mas NÃO vai falar. A voz do Piper existe em %s? (ver vozes/)",
            CAMINHO_VOZ_PIPER,
        )
        return None


def main():
    # Configuração inválida é descoberta AQUI, na partida, e não no meio de uma
    # fala (ver configurar_ou_degradar pra política de cada sistema).
    config = configurar_ou_degradar(os.environ, plataforma.TELA_DISPONIVEL)
    if not plataforma.TELA_DISPONIVEL:
        motivo = getattr(plataforma, "MOTIVO_SEM_TELA", None) or sys.platform
        logger.warning("ponte de tela desligada: %s; voz segue normal", motivo)
    medidor = Medidor(assincrono=True)
    fala = _montar_fala(medidor) if config is not None else None
    conexao = ConexaoJames(
        URL_JAMESD,
        manipulador_acao=plataforma.ponte_de_tela(),
        ao_fala_frase=fala.frase if fala else None,
        ao_fala_fim=fala.fim if fala else None,
        ao_silencio=fala.silencio if fala else None,
        ao_fala_erro=fala.erro if fala else None,
    )
    if config is not None:
        threading.Thread(target=_thread_de_escuta, args=(conexao, fala, config, medidor), daemon=True).start()
    # Na thread principal: é a parte que não pode morrer por causa de áudio.
    conexao.executar()


if __name__ == "__main__":
    main()
