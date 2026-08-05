"""Executa ações de tela pedidas pelo jamesd: clique de mouse e tecla.
Porta processador_voz.py:manipular_tela como está — coordenadas continuam
adivinhadas pelo modelo (Anexo A #20), consertar é Fase 5.
"""

import logging

import pyautogui

logger = logging.getLogger(__name__)

pyautogui.PAUSE = 0.01


def executar_acao(dados: dict) -> dict:
    """Executa uma ação de tela e devolve {"saida": ...} ou {"erro": ...}.
    Nunca levanta — erro vira valor de retorno, o jamesd decide o que fazer
    (mesma filosofia de R2 do lado Go)."""
    acao = dados.get("acao")
    try:
        if acao == "click":
            x = int(dados.get("x", 0))
            y = int(dados.get("y", 0))
            pyautogui.moveTo(x, y, duration=1.0)
            pyautogui.click()
            return {"saida": f"clique em ({x},{y})"}
        if acao == "tecla":
            texto = dados.get("texto", "")
            pyautogui.press(texto)
            return {"saida": f"tecla {texto} pressionada"}
        return {"erro": f"ação desconhecida: {acao}"}
    except Exception as erro:
        logger.exception("erro ao executar ação de tela")
        return {"erro": str(erro)}
