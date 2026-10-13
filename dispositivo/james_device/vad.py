"""Detecção de voz com o Silero VAD (pysilero-vad, ONNX, sem PyTorch).

O microfone entrega blocos de 0,25 s e o Silero só aceita quadros de 512
amostras (32 ms, 1024 bytes). O modelo tem memória entre quadros, então este
wrapper garante o que ele exige: todos os quadros, na ordem, sem pular nem
repetir nenhum byte, com o resto de cada bloco guardado pro seguinte.
"""


def _criar_silero():
    from pysilero_vad import SileroVoiceActivityDetector

    return SileroVoiceActivityDetector()


class DetectorDeVoz:
    def __init__(self, limiar: float = 0.5, detector=None):
        self._detector = detector or _criar_silero()
        self._limiar = limiar
        self._quadro = self._detector.chunk_bytes()
        self._resto = b""

    def tem_voz(self, bloco: bytes) -> bool:
        """True se algum quadro completo do bloco passou do limiar. Processa
        TODOS os quadros mesmo depois de achar voz: parar no primeiro
        desalinharia o estado do modelo pro bloco seguinte."""
        dados = self._resto + bloco
        voz = False
        n = len(dados) // self._quadro
        for i in range(n):
            quadro = dados[i * self._quadro:(i + 1) * self._quadro]
            if self._detector(quadro) >= self._limiar:
                voz = True
        self._resto = dados[n * self._quadro:]
        return voz

    def reset(self) -> None:
        """Esquece tudo: o resto do bloco anterior e a memória do modelo. Chamado
        entre falas, pra uma não contaminar a decisão da outra."""
        self._resto = b""
        self._detector.reset()
