"""Ponto único de escolha de plataforma do device.

Cada sistema tem um módulo com as mesmas funções (tocar_som, ponte_de_tela,
TELA_DISPONIVEL). Só o módulo do sistema atual é importado: windows.py roda
Win32 no import e derrubaria o device no Linux.
"""

import importlib
import sys


def nome_do_modulo(sistema: str) -> str:
    if sistema == "win32":
        return "james_device.plataforma.windows"
    return "james_device.plataforma.linux"


atual = importlib.import_module(nome_do_modulo(sys.platform))
