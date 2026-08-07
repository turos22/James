"""Leitura da árvore de acessibilidade do Windows (UI Automation).

É o que faz o clique parar de adivinhar coordenada (D2 da spec da Fase 5): a
posição sai do sistema operacional, nunca de um palpite do modelo.

Fronteira deliberada: `uiautomation` só é tocada em `varrer`. Todo o resto é
lógica sobre a dataclass Elemento, testável sem tela. Se a biblioteca
precisar ser trocada por pywinauto, só `varrer` muda.
"""

import logging
from dataclasses import dataclass, field

from james_device import superficies

logger = logging.getLogger(__name__)

# Tipos de controle que valem a pena oferecer ao modelo. A lista existe pra
# conter R2 (tela densa gerando lista enorme): painel, texto estático e
# separador ocupariam a maior parte da saída sem serem clicáveis.
TIPOS_INTERESSANTES = frozenset(
    {
        "Button",
        "CheckBox",
        "ComboBox",
        "Edit",
        "Hyperlink",
        "ListItem",
        "MenuItem",
        "RadioButton",
        "Slider",
        "Tab",
        "TabItem",
        "TreeItem",
    }
)

PROFUNDIDADE_MAXIMA = 12


@dataclass(frozen=True)
class Elemento:
    """Um controle clicável. `retangulo` é (esquerda, topo, direita, base) em
    coordenada GLOBAL, do jeito que a UIA entrega."""

    id: str
    nome: str
    tipo: str
    retangulo: tuple[int, int, int, int]

    def centro_global(self) -> tuple[int, int]:
        esquerda, topo, direita, base = self.retangulo
        return ((esquerda + direita) // 2, (topo + base) // 2)

    def centro_local(self, superficie) -> tuple[int, int]:
        x, y = self.centro_global()
        return superficies.para_local(superficie, x, y)


@dataclass
class Mapa:
    """Resultado de uma varredura. Descartável de propósito: é um retrato,
    não uma fonte da verdade paralela. Quem clica revalida contra a árvore
    viva antes de agir."""

    superficie_id: str
    janela_em_foco: int
    elementos: list[Elemento] = field(default_factory=list)

    def valido_para(self, superficie_id: str, janela_em_foco: int) -> bool:
        """Invalidação de D9: mudou a superfície ou a janela em foco, o
        retrato morreu. Trocar de janela reordena o que está visível no
        monitor, então o mapa não vale mais nem no mesmo monitor."""
        return self.superficie_id == superficie_id and self.janela_em_foco == janela_em_foco

    def por_id(self, identificador: str) -> Elemento | None:
        for elemento in self.elementos:
            if elemento.id == identificador:
                return elemento
        return None

    def por_nome(self, nome: str, tipo: str = "") -> Elemento | None:
        """None quando não acha E quando acha mais de um: com dois botões
        "OK" na tela, chutar um seria reintroduzir o palpite que esta fase
        existe pra eliminar."""
        candidatos = [
            e for e in self.elementos if e.nome == nome and (not tipo or e.tipo == tipo)
        ]
        if len(candidatos) == 1:
            return candidatos[0]
        return None


def formatar(elementos: list[Elemento], superficie=None) -> str:
    """Uma linha por elemento. Só o centro vai pro modelo — o retângulo
    inteiro custaria quatro vezes mais token e quem precisa dele é o device."""
    linhas = []
    for elemento in elementos:
        x, y = elemento.centro_local(superficie) if superficie else elemento.centro_global()
        linhas.append(f'{elemento.id}  {elemento.tipo}  "{elemento.nome}"  ({x},{y})')
    return "\n".join(linhas)


def _dentro(retangulo: tuple[int, int, int, int], superficie) -> bool:
    esquerda, topo, direita, base = retangulo
    centro_x = (esquerda + direita) // 2
    centro_y = (topo + base) // 2
    return (
        superficie.origem_x <= centro_x < superficie.origem_x + superficie.largura
        and superficie.origem_y <= centro_y < superficie.origem_y + superficie.altura
    )


def varrer(superficie, janela_em_foco: int) -> Mapa:
    """Percorre a árvore UIA e devolve o mapa da superfície. Fala com o SO —
    sem teste automatizado, coberto por CA4."""
    import uiautomation

    encontrados: list[Elemento] = []
    contador = 0

    def descer(no, profundidade: int) -> None:
        nonlocal contador
        if profundidade > PROFUNDIDADE_MAXIMA:
            return
        for filho in no.GetChildren():
            try:
                if not filho.IsOffscreen and filho.ControlTypeName in TIPOS_INTERESSANTES:
                    r = filho.BoundingRectangle
                    retangulo = (r.left, r.top, r.right, r.bottom)
                    if r.right > r.left and r.bottom > r.top and _dentro(retangulo, superficie):
                        contador += 1
                        encontrados.append(
                            Elemento(
                                id=f"e{contador}",
                                nome=filho.Name or "",
                                tipo=filho.ControlTypeName,
                                retangulo=retangulo,
                            )
                        )
                descer(filho, profundidade + 1)
            except Exception:
                # Um controle que some no meio da varredura (app fechando,
                # menu colapsando) não pode matar a varredura inteira.
                logger.debug("elemento ilegível durante a varredura, pulando", exc_info=True)

    descer(uiautomation.GetRootControl(), 0)
    return Mapa(superficie_id=superficie.id, janela_em_foco=janela_em_foco, elementos=encontrados)
