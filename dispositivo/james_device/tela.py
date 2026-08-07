"""Executa ações de tela pedidas pelo jamesd: clique e tecla.

Coordenadas no protocolo são LOCAIS à superfície (D15 da spec da Fase 5) —
a conversão para o espaço global do desktop acontece aqui, uma vez, antes de
acionar o pyautogui.

Nunca levanta: erro vira valor de retorno e o jamesd decide o que fazer
(filosofia R2 do lado Go).
"""

import base64
import logging

# Import de superficies ANTES de pyautogui é proposital: superficies ativa
# DPI awareness per-monitor como efeito colateral do import, e isso só
# funciona se rodar antes do pyautogui setar seu próprio awareness (mais
# fraco) primeiro. Inverter esta ordem faz voltar o bug de coordenada
# deslocada em tela escalada que a Task 1 existe para prevenir.
from james_device import superficies
import pyautogui

from james_device import captura, elementos

logger = logging.getLogger(__name__)

pyautogui.PAUSE = 0.01

# Retrato da última varredura. _listar_elementos o preenche; _clicar_elemento
# o lê de volta pra resolver clique por id/nome, revalidando contra a árvore
# viva antes de agir (D9).
#
# Nunca invalidado explicitamente depois de um clique: valido_para cobre
# troca de superfície/janela, e o resto da defesa contra tela mudada é a
# revalidação contra a árvore viva em _revalidar. Zerar aqui daria falsa
# sensação de frescor sem checar o que importa de verdade.
_mapa_atual: elementos.Mapa | None = None


def _resolver_superficie(dados: dict) -> tuple[superficies.Superficie | None, str | None]:
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
            # tecla não resolve superfície: teclado vai pra janela em foco, não pra um monitor.
            texto = dados.get("texto", "")
            pyautogui.press(texto)
            return {"saida": f"tecla {texto} pressionada"}
        if acao == "ver_tela":
            return _ver_tela(dados)
        if acao == "clicar_elemento":
            return _clicar_elemento(dados)
        return {"erro": f"ação desconhecida: {acao}"}
    except Exception as erro:
        logger.exception("erro ao executar ação de tela")
        return {"erro": str(erro)}


def _clicar(dados: dict) -> dict:
    superficie, erro = _resolver_superficie(dados)
    if erro is not None:
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

    x_global, y_global = superficies.para_global(superficie, x, y)
    pyautogui.moveTo(x_global, y_global, duration=1.0)
    pyautogui.click()
    return {"saida": f"cliquei em ({x},{y}) de {superficie.id}"}


def _handle_janela_em_foco() -> int:
    """Handle bruto (HWND) da janela em foco, como número — é a chave de
    identidade que Mapa.valido_para usa pra saber se a janela trocou desde a
    última varredura.

    Duplica a chamada a GetForegroundWindow que superficies.py já faz dentro
    de retangulo_da_janela_em_foco(): aquela função devolve o retângulo da
    janela (o que _resolver_superficie precisa), não o handle (o que Mapa
    precisa pra identidade). superficies.py não expõe o handle cru hoje, e
    não está no escopo de arquivos desta task — duplicar duas linhas de
    ctypes aqui é o custo aceito em vez de mexer lá."""
    import ctypes

    return int(ctypes.windll.user32.GetForegroundWindow())


def _listar_elementos(superficie: superficies.Superficie) -> str:
    """Varre a árvore UIA da superfície e formata pro modelo. Guarda o
    retrato em _mapa_atual — _clicar_elemento o lê de volta pra resolver
    clique por elemento."""
    global _mapa_atual
    _mapa_atual = elementos.varrer(superficie, _handle_janela_em_foco())
    if not _mapa_atual.elementos:
        return "(nenhum elemento clicável legível nesta tela — use clique por coordenada)"
    return elementos.formatar(_mapa_atual.elementos, superficie)


def _ver_tela(dados: dict) -> dict:
    superficie, erro = _resolver_superficie(dados)
    if erro is not None:
        return {"erro": erro}

    imagem = captura.capturar(superficie)

    cabecalho = (
        f"superficie: {superficie.id}"
        f"{' (primaria)' if superficie.primaria else ''} "
        f"{superficie.largura}x{superficie.altura} "
        f"escala {int(superficie.escala * 100)}%"
    )

    # A varredura falhando não pode custar a imagem: são falhas
    # independentes, e a imagem sozinha já serve pro modelo se orientar.
    try:
        lista = _listar_elementos(superficie)
    except Exception as erro_varredura:
        logger.exception("falha ao varrer elementos, devolvendo só a imagem")
        lista = f"(não consegui ler os elementos desta tela: {erro_varredura})"

    return {
        "saida": cabecalho + ("\n" + lista if lista else ""),
        "imagem_b64": base64.b64encode(imagem).decode(),
    }


def _revalidar(elemento: elementos.Elemento) -> str | None:
    """Confere o elemento contra a árvore VIVA, não contra o retrato.
    Devolve o motivo da recusa, ou None se pode clicar.

    É esta função que produz o erro real que faltava: sem ela o device
    respondia sucesso incondicional e o modelo nunca sabia que errou.
    Fala com o SO — sem teste automatizado, coberto por CA6."""
    import uiautomation

    x, y = elemento.centro_global()
    try:
        vivo = uiautomation.ControlFromPoint(x, y)
    except Exception as erro:
        return f"não consegui ler a tela nesse ponto: {erro}"

    if vivo is None:
        return "o elemento não está mais na tela"

    r = vivo.BoundingRectangle
    if (r.left, r.top, r.right, r.bottom) != elemento.retangulo:
        return "outro controle está nesse ponto agora — a tela mudou desde o ver_tela"

    if vivo.IsOffscreen:
        return "o elemento está fora da área visível"
    try:
        if not vivo.IsEnabled:
            return "o elemento está desabilitado"
    except Exception:
        logger.debug("não consegui ler IsEnabled do elemento revalidado", exc_info=True)

    return None


def _clicar_elemento(dados: dict) -> dict:
    superficie, erro = _resolver_superficie(dados)
    if erro is not None:
        return {"erro": erro}

    if _mapa_atual is None:
        return {"erro": "não tenho varredura desta tela — chame ver_tela antes"}

    handle = _handle_janela_em_foco()
    if not _mapa_atual.valido_para(superficie.id, handle):
        return {"erro": "a tela mudou desde o ver_tela — chame ver_tela de novo"}

    identificador = dados.get("elemento_id", "")
    nome = dados.get("nome", "")
    tipo = dados.get("tipo", "")

    if identificador:
        elemento = _mapa_atual.por_id(identificador)
        if elemento is None:
            return {"erro": f"não achei o elemento {identificador} na última varredura"}
    elif nome:
        elemento = _mapa_atual.por_nome(nome, tipo)
        if elemento is None:
            quantos = sum(
                1 for e in _mapa_atual.elementos if e.nome == nome and (not tipo or e.tipo == tipo)
            )
            if quantos > 1:
                return {"erro": f'achei {quantos} elementos chamados "{nome}" — use o elemento_id da lista'}
            return {"erro": f'não achei nenhum elemento chamado "{nome}" na última varredura'}
    else:
        return {"erro": "informe elemento_id ou nome"}

    recusa = _revalidar(elemento)
    if recusa:
        return {"erro": f"{recusa} — chame ver_tela de novo"}

    x, y = elemento.centro_global()
    pyautogui.moveTo(x, y, duration=1.0)
    pyautogui.click()

    local_x, local_y = elemento.centro_local(superficie)
    return {"saida": f'cliquei em "{elemento.nome}" ({elemento.tipo}) em ({local_x},{local_y}) de {superficie.id}'}
