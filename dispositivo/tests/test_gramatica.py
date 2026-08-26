"""Gramática do Vosk: restringe o decoder ao vocabulário da wake word.

O modelo pt-BR tem vocabulário FECHADO. Sem gramática, o decoder escolhe
entre o português inteiro e "james" compete com "gêmeos", "times" e todo o
resto — medido: 2 acertos em 5 frases. Com a gramática ele só pode escolher
entre a wake word, a palavra de parada e "[unk]" — medido: 5 em 5.

Medição em dispositivo/tests/test_gramatica.py::test_wake_word_no_lexico_do_modelo
e no experimento que originou esta mudança (Piper sintetizando, Vosk
transcrevendo).
"""

import json
import os
import pathlib

import pytest

from james_device import gramatica
from james_device.wake import VARIANTES_PADRAO

# Relativo à raiz do repo, resolvido a partir deste arquivo: o pytest roda
# com cwd em dispositivo/, e um caminho relativo simples fazia o teste do
# léxico ser PULADO em silêncio — justo o guarda que impede a lista de
# variantes voltar a encher de palavra que o modelo não emite.
CAMINHO_MODELO = str(
    pathlib.Path(__file__).resolve().parents[2] / "vosk" / "vosk-model-small-pt-0.3"
)


def test_gramatica_junta_variantes_palavra_de_parada_e_unk():
    montada = json.loads(gramatica.montar(["james", "jaime"], "enviar"))
    assert set(montada) == {"james", "jaime", "enviar", "[unk]"}


def test_unk_e_obrigatorio():
    """Sem "[unk]" o decoder é forçado a encaixar TUDO que ouve numa das
    palavras da lista — qualquer ruído vira wake word. É o token que dá ao
    decoder a opção de dizer "não foi nada disso"."""
    montada = json.loads(gramatica.montar(["james"], "enviar"))
    assert "[unk]" in montada


def test_sem_palavra_de_parada_a_gramatica_ainda_e_valida():
    montada = json.loads(gramatica.montar(["james"], ""))
    assert set(montada) == {"james", "[unk]"}


def test_nao_repete_palavra():
    montada = json.loads(gramatica.montar(["james", "james"], "james"))
    assert sorted(montada) == sorted(set(montada))


@pytest.mark.skipif(not os.path.isdir(CAMINHO_MODELO), reason="modelo Vosk ausente")
def test_wake_word_no_lexico_do_modelo():
    """Guarda contra o erro que originou esta mudança: das 20 variantes que
    existiam antes, 17 NÃO estavam no léxico do modelo. Palavra fora do
    léxico é descartada da gramática em silêncio — nunca casa, e infla a
    lista dando falsa sensação de cobertura.

    Este teste é a forma de revalidar a lista se o modelo for trocado."""
    ausentes = gramatica.palavras_fora_do_lexico(
        CAMINHO_MODELO, list(VARIANTES_PADRAO) + ["enviar"]
    )
    assert ausentes == [], (
        f"estas palavras não existem no léxico do modelo e nunca vão casar: {ausentes}. "
        "Tire-as de VARIANTES_PADRAO ou troque de modelo."
    )
