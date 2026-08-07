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

# Import de superficies ANTES de pyautogui é proposital: superficies ativa
# DPI awareness per-monitor como efeito colateral do import, e isso só
# funciona se rodar antes do pyautogui setar seu próprio awareness (mais
# fraco) primeiro. Sem isto, um consumidor que importe captura.py antes de
# tela.py (ou sozinho) queima o awareness em definitivo — importar
# superficies depois não recupera (ver tela.py).
from james_device import superficies
import pyautogui


def capturar(superficie: "superficies.Superficie") -> bytes:
    """PNG da superfície inteira. A região usa coordenada GLOBAL, que é o
    espaço em que o pyautogui trabalha.

    Fala com o SO — sem teste automatizado, coberto por verificação ao vivo
    (mesma categoria de superficies.listar())."""
    imagem = pyautogui.screenshot(
        region=(superficie.origem_x, superficie.origem_y, superficie.largura, superficie.altura)
    )
    buffer = io.BytesIO()
    imagem.save(buffer, format="PNG")
    return buffer.getvalue()
