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
# Deliberadamente conservadora: palavras comuns do português que soam
# parecido (games, jaime) ficam de fora para não disparar ativação à toa.
# A calibração com gravações reais do Arthur é tarefa da Fase 5.
VARIANTES_PADRAO = [
    "james",
    "jeimes",
    "jaimes",
    "jeims",
    "geimes",
]

# Arquivo de calibração (D10 da spec da Fase 5). Fica fora do Vault de
# propósito: o Vault é lido pelo modelo a cada turno e não deve receber esse
# ruído. Está no .gitignore — é transcrição do que o Arthur falou.
CAMINHO_LOG_QUASE_ACERTO = Path("dispositivo/wake_candidatos.log")

# 0.6 pega "games" (0.8 contra "james") e deixa passar palavra distante.
# Calibrável depois, com o próprio log em mãos.
LIMIAR_QUASE_ACERTO = 0.6


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
        ratio = max(difflib.SequenceMatcher(None, palavra, variante).ratio() for variante in variantes)
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
