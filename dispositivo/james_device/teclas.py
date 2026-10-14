"""Nome de tecla ou combinação → keysyms para o portal (D6 da spec 2026-10-13 B2).

Só tecla nomeada e atalho. Texto NUNCA passa por keysym: o KWin converte o
keysym em tecla do layout ativo (aqui `us`), e no spike `ç` virou `<` e `ã`
sumiu. Texto vai por texto.py (AT-SPI ou colar).
"""

import unicodedata

_NOMES = {
    "enter": 0xFF0D, "return": 0xFF0D,
    "tab": 0xFF09,
    "esc": 0xFF1B, "escape": 0xFF1B,
    "space": 0x20, "espaco": 0x20,
    "backspace": 0xFF08,
    "delete": 0xFFFF, "del": 0xFFFF,
    "insert": 0xFF63,
    "up": 0xFF52, "cima": 0xFF52,
    "down": 0xFF54, "baixo": 0xFF54,
    "left": 0xFF51, "esquerda": 0xFF51,
    "right": 0xFF53, "direita": 0xFF53,
    "home": 0xFF50, "end": 0xFF57,
    "pageup": 0xFF55, "pagedown": 0xFF56,
    "printscreen": 0xFF61, "menu": 0xFF67,
    "ctrl": 0xFFE3, "control": 0xFFE3,
    "shift": 0xFFE1,
    "alt": 0xFFE9,
    "super": 0xFFEB, "meta": 0xFFEB, "win": 0xFFEB,
}
_NOMES.update({f"f{n}": 0xFFBE + n - 1 for n in range(1, 13)})

_DICA_TEXTO = "tecla aceita só uma tecla ou combinação (ex: enter, ctrl+l); para escrever texto use digitar"


class ErroDeTecla(ValueError):
    pass


def _sem_acento(texto: str) -> str:
    return "".join(c for c in unicodedata.normalize("NFKD", texto) if not unicodedata.combining(c))


def _uma(parte: str) -> int:
    crua = parte.strip().casefold()
    if not crua:
        raise ErroDeTecla(f"tecla vazia em uma combinação. {_DICA_TEXTO}")
    nome = _sem_acento(crua).replace(" ", "")
    if nome in _NOMES:
        return _NOMES[nome]
    if len(crua) == 1 and crua.isascii() and crua.isalnum():
        return ord(crua)
    if " " in crua or (len(crua) == 1 and not crua.isascii()):
        raise ErroDeTecla(_DICA_TEXTO)
    aceitos = ", ".join(sorted({"enter", "tab", "esc", "space", "backspace", "delete", "up", "down", "left",
                                "right", "home", "end", "pageup", "pagedown", "f1..f12", "ctrl", "shift",
                                "alt", "super"}))
    raise ErroDeTecla(f"tecla desconhecida: {parte.strip()!r}. Aceitas: {aceitos}, ou uma letra/dígito. {_DICA_TEXTO}")


def para_keysyms(texto: str) -> list[int]:
    """`ctrl+shift+t` → [Ctrl, Shift, t]. O portal aperta em ordem e solta ao
    contrário (portal.Portal.teclas)."""
    if not texto or not texto.strip():
        raise ErroDeTecla(f"tecla vazia. {_DICA_TEXTO}")
    return [_uma(parte) for parte in texto.split("+")]
