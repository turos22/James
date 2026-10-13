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

import concurrent.futures
import dataclasses
import itertools
import json
import logging
import re
import time
from collections.abc import Mapping
from dataclasses import dataclass

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


# ---------------------------------------------------------------------------
# Fim de fala por VAD (spec 2026-10-10 voz em tempo real, §6.1)
#
# Substitui a janela fixa de 8 s: o comando fecha quando a pessoa para de falar.
# Tudo em TEMPO DE ÁUDIO (bytes recebidos), não em relógio de parede: se a CPU
# atrasar, os blocos chegam mais rápido que o tempo real e a contagem continua
# certa. Em inteiros, pra 7 blocos de 0,1 s darem exatamente 0,7 s.
# ---------------------------------------------------------------------------

BYTES_POR_SEGUNDO = 32000  # PCM 16 kHz, mono, 16 bits

# Depois que o James cala, o microfone ainda pega o rabo da voz dele no ar da
# sala. Descartar esse tanto antes de abrir a continuação evita transcrever o
# próprio James.
GUARDA_ECO_PADRAO_S = 0.4

# Sinal que vai ao STT ANTES da primeira voz: o começo da fala costuma ser
# pego pelo VAD um pouco tarde, e cortar a consoante inicial piora o STT. Tem de
# ser curto: o áudio enviado inclui isto, e teto + pré-roll precisa caber no
# contexto do -ac do Whisper (ver validar_teto).
PRE_ROLL_S = 0.2

# Voz mais curta que isto, com transcrição vazia, é ruído (o bipe de acordar que
# vaza pro microfone, um clique, uma tosse) e não uma fala que o STT não
# entendeu: o James segue esperando em vez de dizer "não entendi".
VOZ_MINIMA_S = 1.0

# Palavras com que ninguém termina uma frase: se a transcrição acaba numa
# delas, a pessoa ainda está pensando e vale esperar mais um pouco. Já
# normalizadas (sem acento, minúsculas).
PALAVRAS_DE_LIGACAO = frozenset({
    "e", "ou", "mas", "entao", "pra", "para", "que", "de", "do", "da", "dos", "das",
    "no", "na", "nos", "nas", "em", "com", "por", "um", "uma", "o", "a", "os", "as",
    "se", "tipo", "assim", "tambem", "porque", "quando", "onde", "como", "depois",
    "antes", "sobre", "entre", "ate",
})


@dataclass(frozen=True)
class ParametrosDeFim:
    silencio_s: float = 0.7        # silêncio que fecha a frase
    silencio_longo_s: float = 1.5  # idem, quando a frase parece incompleta
    comando_max_s: float = 10.0    # teto da duração do comando
    espera_voz_s: float = 5.0      # quanto esperar a pessoa começar a falar


@dataclass(frozen=True)
class Coleta:
    texto: str
    desistiu: bool = False  # ninguém falou: volta a dormir, sem "não entendi"


def cobertura_do_audio_ctx(audio_ctx: int) -> float:
    """Segundos de áudio que o whisper-server cobre com `-ac audio_ctx`. O
    contexto cheio do Whisper são 1500 posições para 30 s."""
    return audio_ctx * 30.0 / 1500.0


def validar_teto(teto_s: float, audio_ctx: int) -> None:
    """Um comando mais longo que a cobertura do -ac seria cortado em silêncio
    pelo Whisper: o final da frase sumiria sem erro nenhum."""
    cobertura = cobertura_do_audio_ctx(audio_ctx)
    # O áudio enviado leva PRE_ROLL_S de sinal antes da primeira voz.
    if teto_s + PRE_ROLL_S > cobertura:
        raise ValueError(
            f"JAMES_COMANDO_MAX_S={teto_s:g}s (mais {PRE_ROLL_S:g}s de pré-roll) passa do que o whisper-server "
            f"cobre com -ac {audio_ctx} ({cobertura:.1f}s): o fim do comando seria cortado. "
            "Suba o -ac ou baixe o teto."
        )


def _numero(env: Mapping, nome: str, padrao: float, divisor: float = 1.0) -> float:
    bruto = env.get(nome)
    if bruto in (None, ""):
        return padrao
    try:
        valor = float(bruto) / divisor
    except ValueError:
        raise ValueError(f"{nome}={bruto!r} não é um número") from None
    if valor <= 0:
        raise ValueError(f"{nome}={bruto!r} precisa ser maior que zero")
    return valor


def parametros_do_ambiente(env: Mapping, audio_ctx: int = 512) -> ParametrosDeFim:
    """Lê os parâmetros do ambiente e falha no boot se forem incoerentes, em vez
    de falhar no meio de uma fala."""
    padrao = ParametrosDeFim()
    p = ParametrosDeFim(
        silencio_s=_numero(env, "JAMES_FIM_SILENCIO_MS", padrao.silencio_s, 1000.0),
        silencio_longo_s=_numero(env, "JAMES_FIM_SILENCIO_LONGO_MS", padrao.silencio_longo_s, 1000.0),
        comando_max_s=_numero(env, "JAMES_COMANDO_MAX_S", padrao.comando_max_s),
        espera_voz_s=_numero(env, "JAMES_ESPERA_VOZ_S", padrao.espera_voz_s),
    )
    if p.silencio_longo_s < p.silencio_s:
        raise ValueError(
            f"o silêncio longo ({p.silencio_longo_s:g}s) não pode ser menor que o curto ({p.silencio_s:g}s)"
        )
    validar_teto(p.comando_max_s, audio_ctx)
    return p


def parece_incompleta(texto: str) -> bool:
    """A transcrição soa como frase pela metade? Reticências ou terminar numa
    palavra de ligação dizem que sim; ponto, exclamação ou interrogação dizem
    que não. Dica barata e de graça: a transcrição já ia acontecer."""
    t = texto.strip()
    if not t:
        return False
    if t.endswith("...") or t.endswith("…"):
        return True
    if t[-1] in ".!?":
        return False
    palavras = re.sub(r"[^\w]+$", "", normalizar(t)).split()
    return bool(palavras) and palavras[-1] in PALAVRAS_DE_LIGACAO


class _MedidorMudo:
    def marca(self, nome, em=None):
        pass


def _aparar_ate_a_pre_roll(audio: list, limite_bytes: int) -> list:
    """`audio` termina no bloco com a primeira voz. Mantém esse bloco e, antes
    dele, só o silêncio que cabe em `limite_bytes`."""
    atual, antes = audio[-1], audio[:-1]
    mantidos, soma = [], 0
    for bloco in reversed(antes):
        if soma + len(bloco) > limite_bytes:
            break
        mantidos.append(bloco)
        soma += len(bloco)
    return list(reversed(mantidos)) + [atual]


def coletar_comando(fonte, vad, reconhecedor, transcrever, params: ParametrosDeFim, medidor=None,
                    executor=None, palavra_parada: str = "", agora=time.monotonic) -> Coleta:
    """Junta áudio da MESMA fonte do laço principal até o comando acabar, e
    devolve a transcrição.

    A transcrição é ESPECULATIVA: no primeiro bloco de silêncio depois da voz,
    o áudio até ali já vai pro STT. Se a voz volta antes de `silencio_s`, a
    especulação é descartada; senão, quando o silêncio fecha, o texto já está
    pronto e o STT não soma latência (ele roda dentro da espera).

    Fecha por: silêncio, teto de duração, palavra de parada (Vosk) ou fim da
    fonte. Desiste (Coleta.desistiu) se ninguém falar em `espera_voz_s`.
    """
    medidor = medidor or _MedidorMudo()
    proprio = executor is None
    if proprio:
        executor = concurrent.futures.ThreadPoolExecutor(max_workers=1)

    def tarefa(audio: bytes):
        # Devolve também QUANDO terminou: o medidor precisa do instante da
        # transcrição que foi usada, não de quando alguém foi olhar o resultado.
        return transcrever(audio), agora()

    falhou = False

    def resultado(futuro):
        nonlocal falhou
        try:
            falhou = False
            return futuro.result()
        except Exception:
            # Falha do STT não é ruído: o Arthur falou, e o James tem de pedir
            # pra repetir, não ficar esperando em silêncio.
            logger.exception("falha ao transcrever o comando, seguindo sem texto")
            falhou = True
            return "", agora()

    def transcrever_agora(pedacos):
        medidor.marca("fim_fala", em=agora())
        futuro = concurrent.futures.Future()
        try:
            futuro.set_result(tarefa(b"".join(pedacos)))
        except Exception as erro:
            futuro.set_exception(erro)
        return resultado(futuro)

    def fechar(texto_e_instante):
        texto, instante = texto_e_instante
        medidor.marca("stt_pronto", em=instante)
        return Coleta(texto=texto or "")

    silencio = round(params.silencio_s * BYTES_POR_SEGUNDO)
    silencio_longo = round(params.silencio_longo_s * BYTES_POR_SEGUNDO)
    teto = round(params.comando_max_s * BYTES_POR_SEGUNDO)
    espera = round(params.espera_voz_s * BYTES_POR_SEGUNDO)

    pre_roll = round(PRE_ROLL_S * BYTES_POR_SEGUNDO)
    voz_minima = round(VOZ_MINIMA_S * BYTES_POR_SEGUNDO)
    audio = []
    houve_voz = False
    bytes_espera = bytes_comando = bytes_silencio = bytes_voz = 0
    especulacao = None

    try:
        for chunk in fonte:
            audio.append(chunk)
            tem_voz = vad.tem_voz(chunk)

            if palavra_parada and reconhecedor is not None:
                final = reconhecedor.AcceptWaveform(chunk)
                if _tem_palavra(_texto_reconhecido(reconhecedor, final), palavra_parada):
                    logger.info("ouvi %r, fechando o comando", palavra_parada)
                    return fechar(transcrever_agora(audio))

            if tem_voz:
                if not houve_voz:
                    # O silêncio de antes da fala não vai ao STT (só a pré-roll):
                    # senão ele consome o contexto do -ac e corta o fim do comando.
                    audio = _aparar_ate_a_pre_roll(audio, pre_roll)
                houve_voz = True
                bytes_voz += len(chunk)
                bytes_silencio = 0
                if especulacao is not None:
                    especulacao.cancel()
                    especulacao = None
                bytes_comando += len(chunk)
            elif houve_voz:
                bytes_comando += len(chunk)
                if bytes_silencio == 0:
                    # Começo de uma pausa: pode ser o fim da fala. Se a voz
                    # voltar, a marca é refeita no próximo começo de pausa.
                    medidor.marca("fim_fala", em=agora())
                    especulacao = executor.submit(tarefa, b"".join(audio))
                bytes_silencio += len(chunk)
            else:
                bytes_espera += len(chunk)
                if bytes_espera >= espera:
                    logger.info("ninguém falou em %.0fs depois de acordar, desisti", params.espera_voz_s)
                    return Coleta(texto="", desistiu=True)
                continue

            if especulacao is not None and bytes_silencio >= silencio:
                texto, instante = resultado(especulacao)
                if not texto.strip() and not falhou and bytes_voz < voz_minima:
                    logger.info("%.1fs de som sem fala entendida, tratado como ruído; sigo esperando",
                                bytes_voz / BYTES_POR_SEGUNDO)
                    # O episódio de ruído conta como espera: se ninguém falar de
                    # verdade, o prazo de espera_voz_s continua valendo.
                    bytes_espera += bytes_comando
                    audio = []
                    houve_voz = False
                    bytes_comando = bytes_voz = bytes_silencio = 0
                    especulacao = None
                    if bytes_espera >= espera:
                        logger.info("ninguém falou em %.0fs depois de acordar, desisti", params.espera_voz_s)
                        return Coleta(texto="", desistiu=True)
                    continue
                if not (parece_incompleta(texto) and bytes_silencio < silencio_longo):
                    return fechar((texto, instante))
                # Frase pela metade: espera até silencio_longo antes de fechar.

            if bytes_comando >= teto:
                logger.info("comando chegou ao teto de %.0fs, fechando", params.comando_max_s)
                if especulacao is not None:
                    return fechar(resultado(especulacao))
                return fechar(transcrever_agora(audio))

        # A fonte acabou (o microfone nunca acaba; isto é teste e desligamento).
        if not houve_voz:
            return Coleta(texto="", desistiu=True)
        if especulacao is not None:
            return fechar(resultado(especulacao))
        return fechar(transcrever_agora(audio))
    finally:
        if proprio:
            executor.shutdown(wait=False)


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
    vad=None,
    parametros_fim: ParametrosDeFim | None = None,
    executor=None,
    medidor=None,
    ocupado=None,
    continuacao_s: float = 0.0,
    guarda_eco_s: float = GUARDA_ECO_PADRAO_S,
    ao_nao_entendeu=None,
):
    """Laço da escuta. Só retorna quando `fonte_audio` se esgota — com um
    microfone de verdade, isso é nunca.

    `reconhecedor` é um KaldiRecognizer do Vosk (ou qualquer coisa com
    AcceptWaveform/Result/PartialResult/Reset). `transcrever_comando` recebe os
    bytes da janela e devolve o texto; é o único ponto que fala com a rede.
    `ao_quase_acerto`, quando passado, recebe a palavra que quase bateu — é o
    log passivo de calibração (D10 da Fase 5).

    Com `vad`, o comando fecha quando a pessoa para de falar (coletar_comando) e
    a transcrição sai especulativa. Sem `vad`, vale a janela fixa de
    `janela_segundos`, o comportamento de antes: é o que sobra se o VAD não
    carregar.

    `ocupado` é um callable (Fala.ocupado): enquanto devolver True o James está
    falando, e o áudio do microfone é DESCARTADO, sem passar pelo Vosk nem pelo
    VAD, senão a voz dele dispara a wake word. Quando ele cala, e se
    `continuacao_s` > 0 (e há `vad`), abre uma janela em que voz vira comando
    SEM wake word, depois de `guarda_eco_s` descartados pra não pegar o rabo da
    voz dele no ar da sala. `ao_nao_entendeu` é chamado quando alguém falou e a
    transcrição veio vazia.
    """
    fonte = iter(fonte_audio)
    if vad is not None and executor is None:
        executor = concurrent.futures.ThreadPoolExecutor(max_workers=1)
    params = parametros_fim or ParametrosDeFim()

    def entregar(comando_bruto):
        comando = _tirar_palavra_parada(comando_bruto or "", palavra_parada).strip()
        if not comando:
            logger.info("janela fechou sem comando entendido, voltando a dormir")
            if ao_nao_entendeu is not None:
                _sem_derrubar(ao_nao_entendeu, "avisar que não entendeu")
            return
        logger.info("comando entendido: %r", comando)
        _sem_derrubar(lambda: ao_comando(comando), "entregar o comando")

    def coletar(origem, parametros):
        return _sem_derrubar(
            lambda: coletar_comando(origem, vad, reconhecedor, transcrever_comando, parametros,
                                    medidor=medidor, executor=executor, palavra_parada=palavra_parada),
            "coletar o comando",
        )

    descartando = False  # estava ocupado no chunk anterior
    guarda = 0           # bytes de eco ainda a descartar
    retomando = False    # o James acabou de calar: a próxima voz pode ser continuação

    for chunk in fonte:
        if ocupado is not None:
            if ocupado():
                descartando = True
                continue
            if descartando:
                descartando = False
                guarda = round(guarda_eco_s * BYTES_POR_SEGUNDO)
                retomando = True
                # O que o Vosk e o VAD guardaram é de antes (ou é a voz do
                # próprio James): começa limpo.
                _sem_derrubar(reconhecedor.Reset, "zerar o reconhecedor depois da fala")
                if vad is not None:
                    vad.reset()
            if guarda > 0:
                guarda -= len(chunk)
                continue
            if retomando:
                retomando = False
                if vad is not None and continuacao_s > 0:
                    # O chunk que acabou de chegar é o primeiro da janela.
                    janela = dataclasses.replace(params, espera_voz_s=continuacao_s)
                    coleta = coletar(itertools.chain([chunk], fonte), janela)
                    _sem_derrubar(reconhecedor.Reset, "zerar o reconhecedor depois da continuação")
                    if coleta is None or coleta.desistiu:
                        logger.info("sem continuação em %.0fs, volto a exigir a wake word", continuacao_s)
                        continue
                    entregar(coleta.texto)
                    continue

        final = reconhecedor.AcceptWaveform(chunk)
        texto = _texto_reconhecido(reconhecedor, final)
        if not texto:
            continue

        if not contem_wake_word(texto):
            # Pedido do Arthur (2026-08-15): sem isto, a única prova de que o
            # microfone está vivo era acertar a wake word — falar qualquer
            # outra coisa não deixava rastro nenhum, e "não apareceu nada no
            # log" virava indistinguível de "o microfone está surdo". Só o
            # resultado FINAL (não o parcial, que muda a cada bloco enquanto
            # a frase ainda está sendo dita) evita encher o console de linhas
            # repetidas do mesmo trecho de fala.
            if final:
                logger.info("ouvindo: %r (sem a wake word)", texto)
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

        if vad is not None:
            vad.reset()
            coleta = coletar(fonte, params)
            audio = None
        else:
            coleta = None
            audio = _coletar_janela(fonte, janela_segundos, relogio, reconhecedor, palavra_parada)

        # Zerar DEPOIS da janela também: ela alimentou o reconhecedor pra ouvir
        # a palavra de parada, então o texto do comando ficou no buffer. Sem
        # isto, o laço de wake word passa a avaliar o que foi comandado — e um
        # comando que por acaso contenha uma variante reativa a escuta sozinha.
        _sem_derrubar(reconhecedor.Reset, "zerar o reconhecedor depois da janela")

        if ao_fechar_janela is not None:
            _sem_derrubar(ao_fechar_janela, "avisar que a janela fechou")

        if vad is not None:
            if coleta is None or coleta.desistiu:
                logger.info("janela fechou sem ninguém falar, voltando a dormir")
                continue
            entregar(coleta.texto)
        else:
            entregar(_sem_derrubar(lambda: transcrever_comando(audio), "transcrever o comando"))
