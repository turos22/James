"""Escuta contínua: wake word offline e janela de comando por tempo.

Substitui o laço antigo de main.py, que tinha três problemas de desenho, não
de ajuste fino:

1. A wake word era detectada mandando blocos de 3s pro Google Speech. Entre o
   fim de um `listen` e o começo do próximo, o microfone não era lido — o
   round-trip de rede acontecia nesse buraco. Wake word que caísse ali sumia,
   e nenhuma variante nova conserta buraco de captura.
2. O fim do comando dependia da palavra "enviar" num `while True`. Sem ela, a
   escuta travava para sempre e nunca voltava a ouvir a wake word.
3. Erro de rede na transcrição subia até matar o processo do device — e com
   ele a ponte de tela, que não tem nada a ver com áudio.

Aqui a wake word roda local (Vosk) sobre o stream contínuo, sem buraco e sem
rede; o fim do comando é uma janela de tempo; e nenhuma falha de callback
derruba o laço.

Fronteira deliberada: este módulo não abre microfone, não carrega modelo e não
fala com a rede. Fonte de áudio, reconhecedor, transcritor e relógio entram
por parâmetro — é o que deixa a máquina de estados inteira testável sem
acústica (tests/test_escuta.py). Quem faz a fiação com o mundo real é main.py.
"""

import json
import logging
import time

from james_device.wake import contem_wake_word, normalizar

logger = logging.getLogger(__name__)

# 8s: meio da faixa de 5 a 10 pedida. Curto o bastante pra não ficar gravando
# a sala depois que a pessoa terminou, longo o bastante pra caber uma frase
# com pausa no meio.
JANELA_PADRAO_SEGUNDOS = 8.0

# Fecha a janela na hora, sem esperar o tempo acabar. Voltou a pedido do
# Arthur (2026-08-11) com um objetivo concreto: dar retorno imediato de que o
# James ouviu, em vez de deixá-lo falando pro vazio por 8s.
#
# A diferença pro laço antigo — que é o que não pode se perder — é que ele
# deixou de ser a ÚNICA forma de fechar a janela. Antes era `while True`
# esperando a palavra, e sem ela a escuta travava para sempre e nunca voltava
# a ouvir a wake word. Agora o tempo é o piso e a palavra só antecipa.
PALAVRA_PARADA_PADRAO = "enviar"


def _texto_reconhecido(reconhecedor, final: bool) -> str:
    """Texto do Vosk, seja resultado final ou parcial. JSON ilegível vira
    string vazia: um frame estranho do reconhecedor não pode derrubar a
    escuta."""
    bruto = reconhecedor.Result() if final else reconhecedor.PartialResult()
    try:
        dados = json.loads(bruto)
    except (TypeError, ValueError):
        logger.debug("resultado ilegível do reconhecedor, ignorando", exc_info=True)
        return ""
    return dados.get("text") or dados.get("partial") or ""


def _sem_derrubar(acao, o_que: str):
    """Roda o callback engolindo qualquer exceção. A escuta é o laço que não
    pode morrer: se ele para, o James fica surdo até alguém reiniciar o
    processo na mão, e nada avisa."""
    try:
        return acao()
    except Exception:
        logger.exception("falha ao %s, seguindo na escuta", o_que)
        return None


def _tem_palavra(texto: str, palavra: str) -> bool:
    """Palavra isolada, não substring: `enviar` não pode bater dentro de
    `enviaram`."""
    return palavra in normalizar(texto).replace(",", " ").replace(".", " ").split()


def _coletar_janela(fonte, janela_segundos: float, relogio, reconhecedor, palavra_parada: str) -> bytes:
    """Junta áudio até a janela fechar — pelo tempo, ou pela palavra de parada,
    o que vier primeiro.

    Consome da MESMA fonte do laço principal de propósito: o que foi falado
    dentro da janela é o comando, e não pode voltar a ser avaliado como wake
    word depois.

    O áudio vai pro reconhecedor local ENQUANTO é coletado. É o que permite
    ouvir a palavra de parada em tempo real: a transcrição de verdade (Google)
    só acontece depois que a janela fecha, então esperar por ela pra saber se
    a pessoa disse "enviar" seria tarde demais — a janela já teria fechado por
    tempo, que é exatamente o que a palavra existe pra evitar."""
    inicio = relogio()
    pedacos = []
    for chunk in fonte:
        if relogio() - inicio >= janela_segundos:
            break
        pedacos.append(chunk)

        if not palavra_parada:
            continue
        final = reconhecedor.AcceptWaveform(chunk)
        if _tem_palavra(_texto_reconhecido(reconhecedor, final), palavra_parada):
            logger.info("ouvi %r, fechando a janela", palavra_parada)
            break
    return b"".join(pedacos)


def _tirar_palavra_parada(texto: str, palavra_parada: str) -> str:
    """Tira a palavra de parada do comando transcrito. O áudio dela ficou no
    buffer que foi pro Google (a janela fecha DEPOIS de coletar o chunk em que
    ela apareceu), então sem isto o James receberia "abre o navegador enviar"
    e trataria o "enviar" como parte do pedido."""
    if not palavra_parada:
        return texto
    return " ".join(p for p in texto.split() if not _tem_palavra(p, palavra_parada))


def escutar(
    fonte_audio,
    reconhecedor,
    transcrever_comando,
    ao_acordar,
    ao_comando,
    janela_segundos: float = JANELA_PADRAO_SEGUNDOS,
    relogio=time.monotonic,
    ao_quase_acerto=None,
    ao_fechar_janela=None,
    palavra_parada: str = PALAVRA_PARADA_PADRAO,
):
    """Laço da escuta. Só retorna quando `fonte_audio` se esgota — com um
    microfone de verdade, isso é nunca.

    `reconhecedor` é um KaldiRecognizer do Vosk (ou qualquer coisa com
    AcceptWaveform/Result/PartialResult/Reset). `transcrever_comando` recebe os
    bytes da janela e devolve o texto; é o único ponto que fala com a rede.
    `ao_quase_acerto`, quando passado, recebe a palavra que quase bateu — é o
    log passivo de calibração (D10 da Fase 5).
    """
    fonte = iter(fonte_audio)

    for chunk in fonte:
        final = reconhecedor.AcceptWaveform(chunk)
        texto = _texto_reconhecido(reconhecedor, final)
        if not texto:
            continue

        if not contem_wake_word(texto):
            if ao_quase_acerto is not None:
                _sem_derrubar(lambda: ao_quase_acerto(texto), "registrar quase-acerto")
            continue

        # INFO e não DEBUG: enquanto a wake word estiver sendo calibrada com a
        # voz do Arthur, esta linha é a evidência mais direta de que o James
        # está de fato ouvindo — e do que ele achou que ouviu.
        logger.info("acordei — ouvi %r", texto)

        # Zerar antes de coletar, não depois: o texto que carregava a wake
        # word continua no buffer do Vosk, e sem isso o parcial seguinte ainda
        # a contém — a escuta acordaria em loop sobre a mesma palavra.
        _sem_derrubar(reconhecedor.Reset, "zerar o reconhecedor")
        _sem_derrubar(ao_acordar, "avisar que acordou")

        audio = _coletar_janela(fonte, janela_segundos, relogio, reconhecedor, palavra_parada)

        # Zerar DEPOIS da janela também: ela alimentou o reconhecedor pra ouvir
        # a palavra de parada, então o texto do comando ficou no buffer. Sem
        # isto, o laço de wake word passa a avaliar o que foi comandado — e um
        # comando que por acaso contenha uma variante reativa a escuta sozinha.
        _sem_derrubar(reconhecedor.Reset, "zerar o reconhecedor depois da janela")

        if ao_fechar_janela is not None:
            _sem_derrubar(ao_fechar_janela, "avisar que a janela fechou")

        comando = _sem_derrubar(lambda: transcrever_comando(audio), "transcrever o comando")
        comando = _tirar_palavra_parada(comando or "", palavra_parada).strip()
        if not comando:
            logger.info("janela fechou sem comando entendido, voltando a dormir")
            continue

        logger.info("comando entendido: %r", comando)
        _sem_derrubar(lambda: ao_comando(comando), "entregar o comando")
