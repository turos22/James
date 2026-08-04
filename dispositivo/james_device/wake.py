"""Detecção da palavra de ativação do James.

O STT em português transcreve "James" de várias formas. Esta função
normaliza o que foi ouvido e compara contra a lista de variantes.
"""

import unicodedata

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


def normalizar(texto: str) -> str:
    """Devolve o texto em minúsculas e sem acentos."""
    decomposto = unicodedata.normalize("NFD", texto.lower())
    return "".join(c for c in decomposto if unicodedata.category(c) != "Mn")


def contem_wake_word(texto: str, variantes: list[str] | None = None) -> bool:
    """Informa se o texto ouvido contém alguma variante da wake word."""
    if variantes is None:
        variantes = VARIANTES_PADRAO
    normalizado = normalizar(texto)
    return any(normalizar(v) in normalizado for v in variantes)
