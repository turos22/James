"""Screenshot de uma superfície, em PNG na resolução nativa.

PNG e não JPEG (D13 da spec da Fase 5): o custo de token no Gemini é por
área e não por byte, então comprimir mais não economiza token nenhum — e
JPEG borra texto pequeno, que é exatamente o que precisa ficar legível numa
interface. O MIME em internal/agente/gemini/mapeamento.go é a constante
"image/png", e esta escolha é o que a mantém verdadeira.

Separado de elementos.py de propósito: as duas coisas falham por motivos
diferentes, e uma tela sem árvore de acessibilidade ainda produz imagem útil.
"""

import io

import pyautogui


def capturar(superficie) -> bytes:
    """PNG da superfície inteira. A região usa coordenada GLOBAL, que é o
    espaço em que o pyautogui trabalha."""
    imagem = pyautogui.screenshot(
        region=(superficie.origem_x, superficie.origem_y, superficie.largura, superficie.altura)
    )
    buffer = io.BytesIO()
    imagem.save(buffer, format="PNG")
    return buffer.getvalue()
