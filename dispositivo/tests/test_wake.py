import pytest

from james_device.wake import contem_wake_word, normalizar


def test_normalizar_remove_acento_e_caixa():
    assert normalizar("Jaímes") == "jaimes"


@pytest.mark.parametrize(
    "ouvido",
    ["james", "James", "Jeimes, tudo bem?", "ei jaimes", "JEIMS me ajuda"],
)
def test_reconhece_variantes_da_wake_word(ouvido):
    assert contem_wake_word(ouvido) is True


@pytest.mark.parametrize("ouvido", ["athena", "bom dia", "", "jazz", "gosto de games"])
def test_ignora_texto_sem_wake_word(ouvido):
    assert contem_wake_word(ouvido) is False


def test_aceita_lista_de_variantes_customizada():
    assert contem_wake_word("chapolin", ["chapolin"]) is True
    assert contem_wake_word("james", ["chapolin"]) is False
