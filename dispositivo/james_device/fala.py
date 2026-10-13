"""A fila de fala do device.

Recebe do jamesd, frase a frase, o que o James vai dizer (fala_frase), toca
na ordem do índice e diz à escuta quando ela pode voltar a ouvir (ocupado).

Meia-duplex: enquanto o James fala, o microfone ouve o próprio James. A escuta
consulta ocupado() e descarta o áudio enquanto for verdade; sem isso a fala dele
dispararia a wake word ou o fim de fala.

Sintetizador e reprodutor entram por parâmetro: a lógica da fila (ordem, fim de
turno, falha, prazo) é testável sem Piper e sem alto-falante.
"""

import logging
import threading
import time
from collections import deque

logger = logging.getLogger(__name__)

FRASE_NAO_ENTENDI = "Não entendi, pode repetir?"
FRASE_SEM_SERVIDOR = "Não consegui falar com o servidor."
TIMEOUT_RESPOSTA_PADRAO = 30.0


class _MedidorMudo:
    def marca(self, nome):
        pass


class Fala:
    def __init__(self, sintetizador, reprodutor, medidor=None, relogio=time.monotonic,
                 timeout_resposta=TIMEOUT_RESPOSTA_PADRAO):
        self._sintetizador = sintetizador
        self._reprodutor = reprodutor
        self._medidor = medidor or _MedidorMudo()
        self._relogio = relogio
        self._timeout = timeout_resposta

        self._cond = threading.Condition()
        self._pendentes: dict[int, str] = {}
        self._locais: deque[str] = deque()  # erro e "não entendi": não passam pelo jamesd
        self._proximo = 0
        self._fim = False
        self._ocupado = False
        self._falando = False
        self._marcou_primeiro = False
        self._fechar_reprodutor = False
        self._ultima_atividade = 0.0
        self._geracao = 0  # sobe a cada turno; deixa um áudio atrasado do turno anterior sem efeito no novo
        self._parar = False

        self._thread = threading.Thread(target=self._laco, name="fala", daemon=True)
        self._thread.start()

    # ---- o que o resto do device chama ----

    def comando_enviado(self) -> None:
        """O comando foi pro jamesd: dali até a resposta acabar de tocar, a
        escuta não pode voltar."""
        with self._cond:
            self._novo_turno_locked()
            self._cond.notify_all()

    def frase(self, indice: int, texto: str) -> None:
        with self._cond:
            self._pendentes[indice] = texto
            self._ocupado = True
            self._ultima_atividade = self._relogio()
            self._cond.notify_all()

    def fim(self) -> None:
        with self._cond:
            self._fim = True
            self._ultima_atividade = self._relogio()
            self._cond.notify_all()

    def erro(self, mensagem: str) -> None:
        """O turno falhou no jamesd: fala a mensagem segura que ele mandou."""
        with self._cond:
            self._locais.append(mensagem)
            self._fim = True
            self._ocupado = True
            self._ultima_atividade = self._relogio()
            self._cond.notify_all()

    def nao_entendi(self) -> None:
        """A transcrição falhou ou veio vazia. Fala sem passar pelo jamesd: ele
        pode estar fora do ar, e é justamente aí que o Arthur precisa saber."""
        with self._cond:
            self._novo_turno_locked()
            self._locais.append(FRASE_NAO_ENTENDI)
            self._fim = True
            self._cond.notify_all()

    def ocupado(self) -> bool:
        with self._cond:
            if not self._ocupado:
                return False
            if self._falando or self._fim or self._ha_o_que_tocar_locked():
                return True
            if self._relogio() - self._ultima_atividade > self._timeout:
                logger.warning(
                    "sem resposta do jamesd há mais de %.0fs; liberando a escuta pra o James não ficar surdo",
                    self._timeout,
                )
                self._ocupado = False
                self._fechar_reprodutor = True
                self._cond.notify_all()
                return False
            return True

    def aguardar_fim_do_turno(self, timeout: float | None = None) -> bool:
        with self._cond:
            return self._cond.wait_for(lambda: not self._ocupado, timeout)

    def aguardar_ocioso(self, timeout: float | None = None) -> bool:
        with self._cond:
            return self._cond.wait_for(lambda: not self._falando and not self._ha_o_que_tocar_locked(), timeout)

    def parar(self) -> None:
        with self._cond:
            self._parar = True
            self._cond.notify_all()
        self._thread.join(timeout=2)

    # ---- internos ----

    def _novo_turno_locked(self) -> None:
        self._geracao += 1
        self._pendentes.clear()
        self._locais.clear()
        self._proximo = 0
        self._fim = False
        self._marcou_primeiro = False
        self._ocupado = True
        self._ultima_atividade = self._relogio()

    def _ha_o_que_tocar_locked(self) -> bool:
        if self._locais or self._proximo in self._pendentes:
            return True
        # Com o fim recebido e um buraco na sequência, toca o que há.
        return self._fim and bool(self._pendentes)

    def _tirar_proxima_locked(self) -> str | None:
        if self._locais:
            return self._locais.popleft()
        if self._proximo in self._pendentes:
            texto = self._pendentes.pop(self._proximo)
            self._proximo += 1
            return texto
        if self._fim and self._pendentes:
            menor = min(self._pendentes)
            self._proximo = menor + 1
            return self._pendentes.pop(menor)
        return None

    def _laco(self) -> None:
        while True:
            with self._cond:
                acao = None
                while acao is None:
                    if self._parar:
                        return
                    texto = self._tirar_proxima_locked()
                    if texto is not None:
                        self._falando = True
                        acao = ("tocar", texto, self._geracao)
                    elif self._ocupado and self._fim:
                        acao = ("encerrar", None, self._geracao)
                    elif self._fechar_reprodutor:
                        self._fechar_reprodutor = False
                        acao = ("fechar", None, self._geracao)
                    else:
                        self._cond.wait(0.2)

            tipo, texto, geracao = acao
            if tipo == "tocar":
                self._tocar(texto)
                with self._cond:
                    self._falando = False
                    self._ultima_atividade = self._relogio()
                    self._cond.notify_all()
            else:
                self._fechar()
                if tipo == "encerrar":
                    with self._cond:
                        # Um comando novo no meio do caminho já começou outro turno.
                        if geracao == self._geracao:
                            self._ocupado = False
                            self._fim = False
                        self._cond.notify_all()

    def _tocar(self, texto: str) -> None:
        try:
            pcm, taxa = self._sintetizador.sintetizar(texto)
        except Exception:
            logger.exception("falha ao sintetizar %r, seguindo pra próxima frase", texto)
            return
        if not pcm:
            return
        self._marcar_primeiro_audio()
        try:
            self._reprodutor.tocar(pcm, taxa)
        except Exception:
            logger.exception("falha ao tocar %r, seguindo pra próxima frase", texto)

    def _marcar_primeiro_audio(self) -> None:
        with self._cond:
            if self._marcou_primeiro:
                return
            self._marcou_primeiro = True
        try:
            self._medidor.marca("primeiro_audio")
        except Exception:
            logger.exception("falha ao registrar o primeiro áudio")

    def _fechar(self) -> None:
        try:
            self._reprodutor.fim_do_turno()
        except Exception:
            logger.exception("falha ao fechar o reprodutor")


class ReprodutorPyAudio:
    """Toca PCM int16 mono por um stream de saída do PyAudio, aberto no
    primeiro som do turno e fechado no fim dele (D12 da spec): as frases de
    uma resposta saem em sequência, sem buraco de abrir e fechar entre elas."""

    def __init__(self):
        self._pa = None
        self._stream = None
        self._taxa = None

    def tocar(self, pcm: bytes, taxa: int) -> None:
        import pyaudio

        if self._stream is None or self._taxa != taxa:
            self._fechar_stream()
            if self._pa is None:
                self._pa = pyaudio.PyAudio()
            self._stream = self._pa.open(format=pyaudio.paInt16, channels=1, rate=taxa, output=True)
            self._taxa = taxa
        self._stream.write(pcm)

    def fim_do_turno(self) -> None:
        self._fechar_stream()

    def _fechar_stream(self) -> None:
        if self._stream is not None:
            try:
                self._stream.stop_stream()  # espera o buffer acabar de tocar
                self._stream.close()
            finally:
                self._stream = None
                self._taxa = None
