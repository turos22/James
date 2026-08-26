"""Gramática do Vosk — restringe o decoder ao vocabulário da wake word.

POR QUE ISTO EXISTE. O modelo pt-BR do Vosk tem vocabulário fechado e um
modelo de linguagem geral. Sem gramática, cada bloco de áudio é decodificado
contra o português inteiro, e "james" compete com "gêmeos", "times", "temos"
e milhares de outras palavras — que, para o modelo, são hipóteses melhores,
porque aparecem muito mais no texto com que ele foi treinado.

Medido com fala sintetizada (Piper pt-BR) passando pelo Vosk, cinco frases
por candidato:

    james, decoder livre .......... 2/5
    james, decoder com gramática .. 5/5

A gramática entrega ao decoder uma lista fechada — as variantes, a palavra de
parada e "[unk]" — e ele passa a escolher só entre elas. É o modo pensado
para wake word, e a implementação anterior simplesmente não o usava: a causa
real de "o James não atende quando eu chamo" era esta, não a lista de
variantes.

CUSTO ACEITO: com gramática o reconhecedor deixa de transcrever fala livre.
Aqui isso não custa nada — quem transcreve o comando é o Google, depois que a
janela fecha (ver escuta.py). O Vosk só decide "chamaram o James?" e "ouvi a
palavra de parada?", que são exatamente perguntas de vocabulário fechado.
"""

import json
import logging
import subprocess
import sys

logger = logging.getLogger(__name__)

# Sem este token o decoder é obrigado a encaixar tudo que ouve numa das
# palavras da lista, e qualquer ruído vira wake word. É ele que dá ao decoder
# a opção de dizer "não foi nada disso".
DESCONHECIDO = "[unk]"


def montar(variantes, palavra_parada: str) -> str:
    """JSON da gramática, no formato que o KaldiRecognizer aceita."""
    palavras = list(variantes)
    if palavra_parada:
        palavras.append(palavra_parada)
    palavras.append(DESCONHECIDO)

    # dict.fromkeys em vez de set: preserva a ordem, que deixa o JSON estável
    # entre execuções e o diff legível quando a lista muda.
    return json.dumps(list(dict.fromkeys(palavras)))


def palavras_fora_do_lexico(caminho_modelo: str, palavras) -> list:
    """As palavras que o modelo NÃO consegue emitir, na ordem em que vieram.

    Palavra fora do léxico é descartada da gramática em silêncio pelo Vosk —
    ela nunca casa, e ficar na lista só dá falsa sensação de cobertura. Foi
    exatamente o que aconteceu: das 20 variantes que existiram por um dia, 17
    eram peso morto.

    O aviso do Kaldi sai no stderr do PROCESSO, não no stderr do Python — por
    isso a checagem roda num subprocesso e lê a saída dele. Tentar capturar
    com os.dup2 dentro do processo atual não funciona (verificado: o controle
    negativo passava, o que provava que a medição não media nada).
    """
    programa = (
        "import json, sys, vosk\n"
        "vosk.SetLogLevel(-1)\n"
        "m = vosk.Model(sys.argv[1])\n"
        "for p in sys.argv[2:]:\n"
        "    print('@@' + p, flush=True)\n"
        "    vosk.KaldiRecognizer(m, 16000, json.dumps([p, '[unk]']))\n"
    )
    # stderr=STDOUT, e não capture_output: os dois precisam chegar no MESMO
    # stream, intercalados na ordem real. Concatenar stdout + stderr põe todos
    # os marcadores antes de todos os avisos, e a correlação entre "qual
    # palavra" e "qual aviso" se perde — o efeito é toda ausência ser
    # atribuída à última palavra da lista.
    saida = subprocess.run(
        [sys.executable, "-u", "-c", programa, caminho_modelo, *palavras],
        stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        text=True, encoding="utf-8", errors="replace",
    )

    ausentes, atual = [], None
    for linha in saida.stdout.splitlines():
        linha = linha.strip()
        if linha.startswith("@@"):
            atual = linha[2:]
        elif "missing in vocabulary" in linha and atual and atual not in ausentes:
            ausentes.append(atual)
    return ausentes
