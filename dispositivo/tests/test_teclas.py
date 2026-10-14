"""Nome de tecla ou combinação → keysyms para o portal (D6 da spec 2026-10-13 B2).

Keysym serve só para tecla nomeada e atalho. Texto nunca passa por aqui:
com o layout `us`, `ç` virou `<` e `ã` sumiu no spike.
"""

import pytest

from james_device.teclas import ErroDeTecla, para_keysyms

CTRL, SHIFT, ALT, SUPER = 0xFFE3, 0xFFE1, 0xFFE9, 0xFFEB


@pytest.mark.parametrize("nome, esperado", [
    ("enter", [0xFF0D]),
    ("return", [0xFF0D]),
    ("tab", [0xFF09]),
    ("esc", [0xFF1B]),
    ("escape", [0xFF1B]),
    ("space", [0x20]),
    ("espaço", [0x20]),
    ("backspace", [0xFF08]),
    ("delete", [0xFFFF]),
    ("del", [0xFFFF]),
    ("up", [0xFF52]),
    ("down", [0xFF54]),
    ("left", [0xFF51]),
    ("right", [0xFF53]),
    ("cima", [0xFF52]),
    ("home", [0xFF50]),
    ("end", [0xFF57]),
    ("pageup", [0xFF55]),
    ("page down", [0xFF56]),
    ("f1", [0xFFBE]),
    ("f12", [0xFFC9]),
    ("super", [SUPER]),
    ("a", [ord("a")]),
    ("7", [ord("7")]),
])
def test_tecla_sozinha(nome, esperado):
    assert para_keysyms(nome) == esperado


@pytest.mark.parametrize("combinacao, esperado", [
    ("ctrl+l", [CTRL, ord("l")]),
    ("Ctrl + L", [CTRL, ord("l")]),
    ("ctrl+shift+t", [CTRL, SHIFT, ord("t")]),
    ("alt+tab", [ALT, 0xFF09]),
    ("ctrl+v", [CTRL, ord("v")]),
    ("shift+delete", [SHIFT, 0xFFFF]),
    ("control+alt+f2", [CTRL, ALT, 0xFFBF]),
])
def test_combinacao(combinacao, esperado):
    assert para_keysyms(combinacao) == esperado


def test_nome_desconhecido_lista_os_aceitos():
    with pytest.raises(ErroDeTecla) as erro:
        para_keysyms("ctrl+banana")
    assert "banana" in str(erro.value)
    assert "enter" in str(erro.value)


def test_frase_e_recusada_e_aponta_para_digitar():
    with pytest.raises(ErroDeTecla, match="use digitar"):
        para_keysyms("olá mundo")


def test_vazio_e_recusado():
    with pytest.raises(ErroDeTecla):
        para_keysyms("  ")
    with pytest.raises(ErroDeTecla):
        para_keysyms("ctrl+")


def test_letra_acentuada_sozinha_e_recusada():
    # Keysym de letra fora do layout sai errado (achado do spike).
    with pytest.raises(ErroDeTecla, match="use digitar"):
        para_keysyms("ç")
