"""Medidor de latência por turno: do fim da fala ao primeiro som do James.

Grava SÓ números (um TSV), nunca o texto do comando: é a voz do Arthur, e o
tempo basta pra saber se o James está rápido. A garantia é estrutural: marca()
só aceita um conjunto fixo de nomes de marco, então não há por onde um texto
chegar ao arquivo.

Marcos de um turno, na ordem em que acontecem:

    fim_fala         começo do silêncio que fechou o comando
    stt_pronto       o texto do comando ficou pronto
    entrada_enviada  o comando saiu pro jamesd
    primeira_frase   a primeira fala_frase chegou do jamesd
    primeiro_audio   o primeiro som da resposta começou a tocar
    erro             o turno falhou (fecha a linha sem primeiro_audio)
"""

import logging
import threading
import time
from datetime import datetime
from pathlib import Path

logger = logging.getLogger(__name__)

NOMES = frozenset({"fim_fala", "stt_pronto", "entrada_enviada", "primeira_frase", "primeiro_audio", "erro"})

CABECALHO = "quando\tturno\ttotal_s\tstt_s\tjamesd_s\ttts_s"

CAMINHO_PADRAO = ".executar/latencia.tsv"


class Medidor:
    def __init__(self, caminho=CAMINHO_PADRAO, agora=time.monotonic):
        self._caminho = Path(caminho)
        self._agora = agora
        self._trava = threading.Lock()
        self._marcas: dict[str, float] = {}
        self._turno = 0

    def marca(self, nome: str, em: float | None = None) -> None:
        if nome not in NOMES:
            raise ValueError(f"marco {nome!r} desconhecido (use um de {sorted(NOMES)})")
        instante = em if em is not None else self._agora()
        with self._trava:
            # Um fim_fala depois do stt_pronto é OUTRA fala: o turno anterior
            # nunca teve desfecho (foi o "não entendi") e precisa ser gravado
            # antes. Um fim_fala ANTES do stt_pronto é só outra pausa da mesma
            # fala, e o último vale.
            if nome == "fim_fala" and "stt_pronto" in self._marcas:
                self._gravar()
            self._marcas[nome] = instante
            if nome in ("primeiro_audio", "erro"):
                self._gravar()

    def _intervalo(self, de: str, ate: str):
        if de in self._marcas and ate in self._marcas:
            return self._marcas[ate] - self._marcas[de]
        return None

    def _gravar(self) -> None:
        """Grava o turno atual e zera. Com a trava já tomada."""
        if not self._marcas:
            return
        self._turno += 1
        total = self._intervalo("fim_fala", "primeiro_audio")
        stt = self._intervalo("fim_fala", "stt_pronto")
        jamesd = self._intervalo("entrada_enviada", "primeira_frase")
        tts = self._intervalo("primeira_frase", "primeiro_audio")
        self._marcas = {}

        def campo(v):
            return "" if v is None else f"{v:.3f}"

        linha = "\t".join([datetime.now().isoformat(timespec="seconds"), str(self._turno),
                           campo(total), campo(stt), campo(jamesd), campo(tts)])
        logger.info(
            "latencia turno=%d fim_fala→1º_som=%s stt=%s jamesd=%s tts=%s",
            self._turno,
            "?" if total is None else f"{total:.2f}s",
            "?" if stt is None else f"{stt:.2f}",
            "?" if jamesd is None else f"{jamesd:.2f}",
            "?" if tts is None else f"{tts:.2f}",
        )
        try:
            self._caminho.parent.mkdir(parents=True, exist_ok=True)
            novo = not self._caminho.exists() or self._caminho.stat().st_size == 0
            with open(self._caminho, "a", encoding="utf-8") as f:
                if novo:
                    f.write(CABECALHO + "\n")
                f.write(linha + "\n")
        except OSError:
            # Medir não pode derrubar a voz: o pior caso é perder a linha.
            logger.exception("falha ao gravar a latência do turno %d em %s", self._turno, self._caminho)
