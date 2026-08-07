"""Executa ações de tela pedidas pelo jamesd: clique, tecla e visão.

Coordenadas no protocolo são LOCAIS à superfície (D15 da spec da Fase 5) —
a conversão para o espaço global do desktop acontece aqui, uma vez, antes de
acionar o pyautogui.

Nunca levanta: erro vira valor de retorno e o jamesd decide o que fazer
(filosofia R2 do lado Go).
"""

import logging

# Import de superficies ANTES de pyautogui é proposital: superficies ativa
# DPI awareness per-monitor como efeito colateral do import, e isso só
# funciona se rodar antes do pyautogui setar seu próprio awareness (mais
# fraco) primeiro. Inverter esta ordem faz voltar o bug de coordenada
# deslocada em tela escalada que a Task 1 existe para prevenir.
from james_device import superficies
import pyautogui

logger = logging.getLogger(__name__)

pyautogui.PAUSE = 0.01


def _resolver_superficie(dados: dict):
    """Devolve (superficie, erro). A superfície pedida por id, ou a padrão
    de D14 — a que contém a janela em foco."""
    todas = superficies.listar()
    if not todas:
        return None, "nenhum monitor encontrado"

    pedida = dados.get("superficie")
    if pedida:
        achada = superficies.por_id(todas, pedida)
        if achada is None:
            disponiveis = ", ".join(s.id for s in todas)
            return None, f"superfície {pedida} não existe (disponíveis: {disponiveis})"
        return achada, None

    return superficies.alvo_padrao(todas, superficies.retangulo_da_janela_em_foco()), None


def executar_acao(dados: dict) -> dict:
    """Executa uma ação de tela e devolve {"saida": ...} ou {"erro": ...}."""
    acao = dados.get("acao")
    try:
        if acao == "click":
            return _clicar(dados)
        if acao == "tecla":
            texto = dados.get("texto", "")
            pyautogui.press(texto)
            return {"saida": f"tecla {texto} pressionada"}
        return {"erro": f"ação desconhecida: {acao}"}
    except Exception as erro:
        logger.exception("erro ao executar ação de tela")
        return {"erro": str(erro)}


def _clicar(dados: dict) -> dict:
    superficie, erro = _resolver_superficie(dados)
    if erro:
        return {"erro": erro}

    x = int(dados.get("x", 0))
    y = int(dados.get("y", 0))

    # Validar ANTES de mover: clique fora da tela não é "quase certo", é
    # pedido inválido, e o modelo precisa saber disso pra corrigir. Deixar
    # passar era metade do bug do Anexo A #20.
    if not (0 <= x < superficie.largura and 0 <= y < superficie.altura):
        return {
            "erro": f"({x},{y}) fora dos limites de {superficie.id} "
            f"({superficie.largura}x{superficie.altura})"
        }

    global_x, global_y = superficies.para_global(superficie, x, y)
    pyautogui.moveTo(global_x, global_y, duration=1.0)
    pyautogui.click()
    return {"saida": f"cliquei em ({x},{y}) de {superficie.id}"}
