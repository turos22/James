"""Linux (Fedora, KDE, Wayland): voz sim, tela não.

Controle de tela no Wayland é a fatia B2, com spec própria. Até lá, a ponte
de tela responde com erro no mesmo formato de tela.executar_acao, e o jamesd
repassa isso pro modelo como faz quando o device cai.
"""

import logging
import subprocess

logger = logging.getLogger(__name__)

TELA_DISPONIVEL = False
MENSAGEM_TELA_INDISPONIVEL = (
    "tela indisponível neste sistema (Linux): controle de tela ainda não portado"
)


def tocar_som(caminho: str) -> None:
    """Bloqueia até o som acabar; quem chama já roda numa thread
    (main.tocar_em_paralelo). pw-play lê via libsndfile, que toca WAV e MP3."""
    try:
        subprocess.run(["pw-play", caminho], check=False)
    except FileNotFoundError:
        logger.warning("pw-play não encontrado, som %s não tocou (pipewire-utils instalado?)", caminho)


def ponte_de_tela():
    def recusar(dados: dict) -> dict:
        return {"erro": MENSAGEM_TELA_INDISPONIVEL}

    return recusar
