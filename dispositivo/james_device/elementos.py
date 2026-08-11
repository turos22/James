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
#
# Os nomes levam o sufixo "Control" (ButtonControl, não Button) porque é
# assim que a biblioteca uiautomation devolve ControlTypeName de verdade —
# achado na verificação ao vivo da Task 15 (CA4): a lista original, sem o
# sufixo, não bate com nada, e varrer() volta sempre vazio em qualquer app
# real, silenciosamente (nenhum erro, só zero elementos encontrados).
TIPOS_INTERESSANTES = frozenset(
    {
        "ButtonControl",
        "CheckBoxControl",
        "ComboBoxControl",
        "EditControl",
        "HyperlinkControl",
        "ListItemControl",
        "MenuItemControl",
        "RadioButtonControl",
        "SliderControl",
        "TabControl",
        "TabItemControl",
        "TreeItemControl",
    }
)

# Profundidade 0 é a raiz (uiautomation.GetRootControl()); a guarda
# "profundidade > MAXIMA" deixa passar até o nível 13 de filhos, não 12 —
# o nome descreve a constante, não o último nível processado.
PROFUNDIDADE_MAXIMA = 12


@dataclass(frozen=True)
class Elemento:
    """Um controle clicável. `retangulo` é (esquerda, topo, direita, base) em
    coordenada GLOBAL, do jeito que a UIA entrega.

    `em_foco` diz se o controle pertence à janela em primeiro plano. É o que
    permite a `formatar` priorizar o que o Arthur está de fato usando na hora
    do corte — ver o comentário de MAXIMO_ELEMENTOS_FORMATADOS."""

    id: str
    nome: str
    tipo: str
    retangulo: tuple[int, int, int, int]
    em_foco: bool = False

    def centro_global(self) -> tuple[int, int]:
        esquerda, topo, direita, base = self.retangulo
        return ((esquerda + direita) // 2, (topo + base) // 2)

    def centro_local(self, superficie: superficies.Superficie) -> tuple[int, int]:
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


# Teto de linhas na lista formatada (não no Mapa — clicar_elemento continua
# resolvendo por id/nome contra a lista inteira). Achado ao vivo na Task 15
# (CA4): um desktop real com poucos apps abertos já produz 200+ elementos
# (a maior parte da barra de tarefas, não da janela em foco), porque D14
# varre todas as janelas visíveis da superfície, de propósito. Sem teto,
# ver_tela custaria bem mais em tokens de texto do que a imagem em si — o
# R2 que a Task 10 já tinha registrado como risco não fechado.
MAXIMO_ELEMENTOS_FORMATADOS = 80


def _nome_em_uma_linha(nome: str) -> str:
    """Colapsa qualquer quebra de linha embutida no nome do controle numa
    linha só. Achado ao vivo na Task 15 (CA4): controles de bandeja do
    sistema (relógio, indicador de rede, indicador de microfone) têm .Name
    com \\n/\\r\\n de verdade — texto de tooltip embutido no nome acessível.
    Sem isto, "uma linha por elemento" deixa de ser verdade justamente pros
    elementos mais exóticos, e o id/coordenada do elemento seguinte se
    confunde com a continuação do nome anterior."""
    return " ".join(nome.split())


def formatar(elementos: list[Elemento], superficie: superficies.Superficie | None = None) -> str:
    """Uma linha por elemento, até MAXIMO_ELEMENTOS_FORMATADOS, com a janela em
    foco na frente. Só o centro vai pro modelo — o retângulo inteiro custaria
    quatro vezes mais token e quem precisa dele é o device.

    A prioridade pra janela em foco existe porque o corte é real e a ordem de
    árvore é péssima pra ele: D14 manda varrer todas as janelas visíveis da
    superfície, e a barra de tarefas do Windows sozinha rende dezenas de
    controles que costumam vir primeiro. Num desktop com apps abertos (200+
    elementos, medido na Task 15), a janela que o Arthur está usando podia não
    sobrar NENHUM elemento na lista — e o modelo lia isso como "não há nada
    clicável aqui". Ordenação estável: dentro de cada grupo a ordem de árvore
    é preservada.

    Truncagem é honesta (avisa quantos ficaram de fora) em vez de silenciosa
    — cortar em silêncio faria o modelo tratar "não apareceu na lista" como
    "não existe", quando pode só estar depois do corte."""
    priorizados = sorted(elementos, key=lambda e: not e.em_foco)
    visiveis = priorizados[:MAXIMO_ELEMENTOS_FORMATADOS]
    linhas = []
    for elemento in visiveis:
        x, y = elemento.centro_local(superficie) if superficie else elemento.centro_global()
        nome = _nome_em_uma_linha(elemento.nome)
        linhas.append(f'{elemento.id}  {elemento.tipo}  "{nome}"  ({x},{y})')
    restantes = len(elementos) - len(visiveis)
    if restantes > 0:
        linhas.append(f"... e mais {restantes} elemento{'s' if restantes != 1 else ''} (peça por nome se não estiver na lista)")
    return "\n".join(linhas)


def _dentro(retangulo: tuple[int, int, int, int], superficie: superficies.Superficie) -> bool:
    """Delega em superficies.contendo em vez de reimplementar a regra de
    intervalo semiaberto — evitar duas cópias que podem divergir com o
    tempo."""
    esquerda, topo, direita, base = retangulo
    centro_x = (esquerda + direita) // 2
    centro_y = (topo + base) // 2
    return superficies.contendo([superficie], centro_x, centro_y) is not None


def varrer(superficie: superficies.Superficie, janela_em_foco: int) -> Mapa:
    """Percorre a árvore UIA e devolve o mapa da superfície. Fala com o SO —
    sem teste automatizado, coberto por CA4."""
    import uiautomation

    encontrados: list[Elemento] = []
    contador = 0
    ignorados = 0

    def descer(no, profundidade: int, em_foco: bool) -> None:
        nonlocal contador, ignorados
        if profundidade > PROFUNDIDADE_MAXIMA:
            return
        for filho in no.GetChildren():
            try:
                # Offscreen (janela minimizada, em background, noutro
                # monitor) poda a subárvore inteira aqui, não só o nó: um nó
                # offscreen não tem filho onscreen na prática, e sem esse
                # `continue` a recursão descia a árvore completa de toda
                # janela fora de vista — um roundtrip COM por nó, custo real
                # num desktop com vários apps abertos.
                if filho.IsOffscreen:
                    continue

                # A marca só SOBE, nunca desce de volta pra False: um controle
                # dentro da janela em foco pode ter HWND próprio (botão nativo,
                # painel hospedado), e comparar o handle dele com o da janela
                # daria diferente — o filho perderia a marca do pai por um
                # detalhe de implementação do app.
                filho_em_foco = em_foco
                if not filho_em_foco:
                    handle = getattr(filho, "NativeWindowHandle", 0) or 0
                    filho_em_foco = int(handle) == janela_em_foco

                if filho.ControlTypeName in TIPOS_INTERESSANTES:
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
                                em_foco=filho_em_foco,
                            )
                        )
                descer(filho, profundidade + 1, filho_em_foco)
            except Exception:
                # Um controle que some no meio da varredura (app fechando,
                # menu colapsando) não pode matar a varredura inteira.
                ignorados += 1
                logger.debug("elemento ilegível durante a varredura, pulando", exc_info=True)

    # A raiz (o desktop) nunca é a janela em foco: a marca nasce False e é
    # promovida quando a descida encontra o HWND certo.
    descer(uiautomation.GetRootControl(), 0, False)
    if ignorados > 0:
        # Sinal agregado: sem isso, uma falha sistemática (API do
        # uiautomation mudou, atributo faltando) se disfarça de "tela vazia"
        # — Mapa sai vazio e o usuário só vê a mensagem benigna de
        # tela.py, sem nenhum indício de que algo quebrou de verdade.
        logger.warning(
            "varredura pulou %d nós ilegíveis (%d elementos encontrados)", ignorados, len(encontrados)
        )
    return Mapa(superficie_id=superficie.id, janela_em_foco=janela_em_foco, elementos=encontrados)
