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


# Ações da spec 2026-10-13 B2 que só existem no Linux por enquanto (D13).
ACOES_SO_DO_LINUX = frozenset({"janelas_listar", "janela_focar", "janela_fechar", "midia", "abrir"})


def recusar_acoes_novas(executar_acao, sistema: str):
    """Envolve uma ponte que não conhece as ações novas: elas respondem
    "indisponível" e todo o resto passa intacto. Mora aqui (portável) para
    ser testável sem carregar o Win32."""

    def ponte(dados: dict) -> dict:
        if dados.get("acao") in ACOES_SO_DO_LINUX:
            return {"erro": f"indisponível neste sistema ({sistema})"}
        return executar_acao(dados)

    return ponte


atual = importlib.import_module(nome_do_modulo(sys.platform))
