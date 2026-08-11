"""Detecção da palavra de ativação do James.

O STT em português transcreve "James" de várias formas. Este módulo
normaliza o que foi ouvido e compara contra a lista de variantes, e também
mantém o log passivo de quase-acertos usado na calibração da Fase 5.
"""

import difflib
import logging
import re
import unicodedata
from datetime import datetime
from pathlib import Path

logger = logging.getLogger(__name__)

# Variantes com que o STT em português costuma transcrever "James".
#
# A lista foi ampliada quando a escuta passou a ser offline e contínua
# (escuta.py). A régua de risco mudou junto: antes, a wake word ainda era
# seguida da palavra "enviar" pra confirmar o comando; hoje ela abre uma
# janela de tempo, e falso negativo custa a pessoa repetir a frase inteira
# enquanto falso positivo custa uma janela de 8s que fecha sozinha sem
# comando entendido. Perder ativação é o erro caro, então a lista pende pro
# lado de reconhecer demais.
#
# O que continua FORA, de propósito: "games" e "gente" — palavras comuns na
# fala do dia a dia, e o Arthur é desenvolvedor (fala "games" em contexto
# normal). "gemeos" e "genese", que aparecem no log de calibração, não batem
# em nada aqui porque a comparação é por palavra inteira.
VARIANTES_PADRAO = [
    # A grafia certa e os truncamentos
    "james",
    "jame",
    "jaims",
    "jeims",
    "jeim",
    # Com a vogal aberta que o STT em português costuma escolher
    "jeimes",
    "jeime",
    "jaimes",
    "jaime",
    "jemes",
    "jems",
    # Com o "j" ouvido como "g" — o erro mais comum no log de calibração
    "geimes",
    "geime",
    "gemes",
    "geims",
    # Com africada, que o português brasileiro produz no "dj" inicial
    "djeimes",
    "djames",
    "tchaimes",
    "chaimes",
    "xeimes",
]

# Arquivo de calibração (D10 da spec da Fase 5). Fica fora do Vault de
# propósito: o Vault é lido pelo modelo a cada turno e não deve receber esse
# ruído. Está no .gitignore — é transcrição do que o Arthur falou.
CAMINHO_LOG_QUASE_ACERTO = Path("dispositivo/wake_candidatos.log")

# 0.65 pega "games" (0.8 contra "james") e deixa passar palavra distante.
# Subiu de 0.6 quando "jame" entrou na lista: variante de 4 letras infla o
# ratio de qualquer palavra que comece com "ja", e "janela" passou a bater
# 0.6 cravado — o log de calibração ia encher de "abre a janela". Ativação
# nunca dependeu deste limiar, só o log.
LIMIAR_QUASE_ACERTO = 0.65


def normalizar(texto: str) -> str:
    """Devolve o texto em minúsculas e sem acentos."""
    decomposto = unicodedata.normalize("NFD", texto.lower())
    return "".join(c for c in decomposto if unicodedata.category(c) != "Mn")


def contem_wake_word(texto: str, variantes: list[str] | None = None) -> bool:
    """Informa se o texto ouvido contém alguma variante da wake word como palavra isolada."""
    if variantes is None:
        variantes = VARIANTES_PADRAO
    normalizado = normalizar(texto)
    return any(
        re.search(rf"\b{re.escape(normalizar(v))}\b", normalizado)
        for v in variantes
    )


def quase_acerto(texto: str, variantes: list[str] | None = None) -> str | None:
    """Palavra que chegou mais perto de uma variante sem bater, ou None se a
    wake word já bateu (não é candidato a calibração, é ativação real) ou se
    nenhuma palavra passou do limiar.

    Existe porque a alternativa — sessão de calibração guiada — tem um furo
    lógico: o Arthur falando de propósito no microfone é exatamente a
    condição em que a detecção já funciona. Os erros acontecem longe do mic,
    com ruído, na fala rápida. Só o log passivo coleta esses.
    """
    variantes = variantes if variantes is not None else VARIANTES_PADRAO
    if contem_wake_word(texto, variantes):
        return None

    melhor_palavra = None
    melhor_ratio = 0.0
    for palavra in normalizar(texto).split():
        ratio = max(
            (difflib.SequenceMatcher(None, palavra, variante).ratio() for variante in variantes),
            default=0.0,
        )
        if ratio > melhor_ratio:
            melhor_ratio = ratio
            melhor_palavra = palavra

    return melhor_palavra if melhor_ratio >= LIMIAR_QUASE_ACERTO else None


def registrar_quase_acerto(palavra: str, caminho: Path | None = None) -> None:
    """Anexa a palavra ao log de calibração. Nunca levanta: log é
    conveniência, e falhar aqui não pode derrubar a escuta."""
    try:
        destino = Path(caminho) if caminho else CAMINHO_LOG_QUASE_ACERTO
        linha = f"{datetime.now().isoformat(timespec='seconds')}\t{palavra}\n"
        with destino.open("a", encoding="utf-8") as arquivo:
            arquivo.write(linha)
    except OSError:
        logger.debug("não consegui gravar o log de wake word", exc_info=True)
