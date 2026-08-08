import pytest

from james_device.wake import (
    contem_wake_word,
    normalizar,
    quase_acerto,
    registrar_quase_acerto,
)


def test_normalizar_remove_acento_e_caixa():
    assert normalizar("Jaímes") == "jaimes"


@pytest.mark.parametrize(
    "ouvido",
    ["james", "James", "Jeimes, tudo bem?", "ei jaimes", "JEIMS me ajuda"],
)
def test_reconhece_variantes_da_wake_word(ouvido):
    assert contem_wake_word(ouvido) is True


@pytest.mark.parametrize(
    "ouvido",
    ["athena", "bom dia", "", "jazz", "gosto de games", "hyperjames", "ohjamesboy", "jaimesabe"],
)
def test_ignora_texto_sem_wake_word(ouvido):
    assert contem_wake_word(ouvido) is False


def test_aceita_lista_de_variantes_customizada():
    assert contem_wake_word("chapolin", ["chapolin"]) is True
    assert contem_wake_word("james", ["chapolin"]) is False


def test_quase_acerto_pega_palavra_parecida():
    # "games" é o falso negativo clássico: o STT em pt-BR devolve isso com
    # frequência e a lista conservadora de VARIANTES_PADRAO não cobre.
    # SequenceMatcher("games", "james") dá 0.8, acima do limiar de 0.6.
    assert quase_acerto("games abre o spotify") == "games"


def test_quase_acerto_ignora_palavra_distante():
    assert quase_acerto("abre a janela da casa") is None


def test_quase_acerto_nao_dispara_quando_a_wake_word_bateu():
    # Se bateu, não é candidato a calibração — é ativação normal.
    assert quase_acerto("james abre o spotify") is None


def test_quase_acerto_pega_o_melhor_match_nao_o_primeiro():
    # "jeito" bate 0.6 contra "jeims" (limiar exato), mas "games" bate 0.8
    # contra "james" — sem essa correção, "jeito" vinha primeiro na frase e
    # "games" era descartado em silêncio.
    assert quase_acerto("jeito games abre o spotify") == "games"


def test_quase_acerto_confere_todas_as_variantes_passadas():
    # "chapolin" (primeiro elemento) tem ratio baixo contra "games" — só
    # comparando também contra "james" (segundo elemento) o quase-acerto
    # aparece. Uma implementação que olhasse só variantes[0] devolveria None.
    assert quase_acerto("games abre o spotify", ["chapolin", "james"]) == "games"


def test_registrar_quase_acerto_grava_linha(tmp_path):
    caminho = tmp_path / "wake_candidatos.log"

    registrar_quase_acerto("games", caminho=caminho)
    registrar_quase_acerto("jeimis", caminho=caminho)

    linhas = caminho.read_text(encoding="utf-8").strip().splitlines()
    assert len(linhas) == 2
    assert linhas[0].endswith("\tgames")
    assert linhas[1].endswith("\tjeimis")


def test_registrar_quase_acerto_nunca_levanta(tmp_path):
    # Log de calibração é conveniência: falhar ao gravar não pode derrubar a
    # escuta, que é a função principal do device.
    caminho = tmp_path / "nao" / "existe" / "wake.log"
    registrar_quase_acerto("games", caminho=caminho)
