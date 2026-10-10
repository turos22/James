"""Geometria de monitor: o tipo Superficie, a conversão de coordenada
local↔global (D15 da spec da Fase 5) e a decisão de qual superfície contém
uma janela.

Único lugar do device que conhece origem e escala de monitor — captura.py,
elementos.py e tela.py perguntam aqui em vez de calcular por conta.

Este módulo é puro e importa em qualquer sistema. Quem fala com o SO
(enumerar monitores, achar a janela em foco) mora em plataforma/windows.py
e só é importado sob demanda, em listar() e retangulo_da_janela_em_foco(),
abaixo. Tudo o mais é aritmética sobre uma lista de Superficie recebida por
parâmetro, testável sem hardware.
"""

import logging
from dataclasses import dataclass

logger = logging.getLogger(__name__)


@dataclass(frozen=True)
class Superficie:
    """Um monitor. `origem_x`/`origem_y` são a posição do canto superior
    esquerdo no espaço global do desktop; `escala` é 1.0 para 100%."""

    id: str
    largura: int
    altura: int
    origem_x: int
    origem_y: int
    escala: float
    primaria: bool


def listar() -> list[Superficie]:
    """Enumera os monitores conectados. Fala com o SO, que hoje só é o
    Windows; o Win32 fica em plataforma/windows.py, importado aqui sob
    demanda pra este módulo importar em qualquer sistema."""
    from james_device.plataforma import windows

    return windows.listar_superficies()


def retangulo_da_janela_em_foco() -> tuple[int, int, int, int] | None:
    """Retângulo global da janela em foco, ou None se não houver."""
    from james_device.plataforma import windows

    return windows.retangulo_da_janela_em_foco()


def para_global(superficie: Superficie, x: int, y: int) -> tuple[int, int]:
    """Coordenada local da superfície para coordenada global do desktop.
    Esquecer esta soma faz o clique cair no monitor errado — e o feedback de
    tela.py reporta com honestidade o elemento errado, o que torna o bug
    plausível e difícil de rastrear."""
    return (superficie.origem_x + x, superficie.origem_y + y)


def para_local(superficie: Superficie, x: int, y: int) -> tuple[int, int]:
    """Inverso de para_global."""
    return (x - superficie.origem_x, y - superficie.origem_y)


def contendo(superficies: list[Superficie], x: int, y: int) -> Superficie | None:
    """Qual superfície contém o ponto global, ou None. O None é real e
    acontece: com monitores desalinhados, existem pontos dentro do retângulo
    que engloba tudo mas fora de qualquer monitor."""
    for superficie in superficies:
        dentro_x = superficie.origem_x <= x < superficie.origem_x + superficie.largura
        dentro_y = superficie.origem_y <= y < superficie.origem_y + superficie.altura
        if dentro_x and dentro_y:
            return superficie
    return None


def por_id(superficies: list[Superficie], identificador: str) -> Superficie | None:
    """Busca por id (ex: "DISPLAY1")."""
    for superficie in superficies:
        if superficie.id == identificador:
            return superficie
    return None


def primaria(superficies: list[Superficie]) -> Superficie:
    """A superfície primária, ou a primeira da lista se nenhuma se declarar.
    Levanta se a lista vier vazia — não devolve None, porque quem chama vai
    capturar alguma tela de qualquer jeito, e não ter resposta aqui só
    empurra o problema pra mais longe da causa real."""
    if not superficies:
        raise RuntimeError("nenhuma superfície disponível — listar() não encontrou monitor nenhum")
    for superficie in superficies:
        if superficie.primaria:
            return superficie
    return superficies[0]


def da_janela(superficies: list[Superficie], retangulo: tuple[int, int, int, int]) -> Superficie:
    """Superfície que contém o centro do retângulo da janela (regra de R6).
    Determinística e barata; conhecidamente incompleta para janela cortada
    quase ao meio entre dois monitores. Centro na área morta cai na
    primária."""
    esquerda, topo, direita, base = retangulo
    centro_x = (esquerda + direita) // 2
    centro_y = (topo + base) // 2
    return contendo(superficies, centro_x, centro_y) or primaria(superficies)


def alvo_padrao(superficies: list[Superficie], retangulo_em_foco: tuple[int, int, int, int] | None) -> Superficie:
    """A superfície que `ver_tela` sem argumento captura (D14): a que contém
    a janela em foco, ou a primária se não houver janela em foco."""
    if retangulo_em_foco is None:
        return primaria(superficies)
    return da_janela(superficies, retangulo_em_foco)
