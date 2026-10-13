"""O detector de voz. O microfone entrega blocos de 0,25 s (8000 bytes) e o
Silero só aceita quadros de 512 amostras (1024 bytes); o modelo tem memória
entre quadros, então nenhum pode ser pulado nem reordenado."""

from pathlib import Path

import pytest

from james_device.vad import DetectorDeVoz

RAIZ = Path(__file__).resolve().parents[2]
VOZ_REAL = RAIZ / "vozes" / "pt_BR-faber-medium.onnx"
QUADRO = 1024


class SileroFalso:
    """Devolve as probabilidades roteirizadas, uma por quadro, e anota o que recebeu."""

    def __init__(self, probabilidades=None, padrao=0.0):
        self.probabilidades = list(probabilidades or [])
        self.padrao = padrao
        self.quadros = []
        self.resets = 0

    def chunk_bytes(self):
        return QUADRO

    def __call__(self, quadro):
        self.quadros.append(bytes(quadro))
        return self.probabilidades.pop(0) if self.probabilidades else self.padrao

    def reset(self):
        self.resets += 1


def test_bloco_de_025s_vira_sete_quadros_e_guarda_o_resto():
    silero = SileroFalso()
    vad = DetectorDeVoz(detector=silero)

    vad.tem_voz(b"\x00" * 8000)

    assert len(silero.quadros) == 7
    assert all(len(q) == QUADRO for q in silero.quadros)


def test_o_resto_do_bloco_anterior_entra_no_comeco_do_proximo():
    silero = SileroFalso()
    vad = DetectorDeVoz(detector=silero)
    primeiro = bytes(range(256)) * 32          # 8192 bytes: 8 quadros certos, sem resto
    vad.tem_voz(b"\xaa" * 100)                  # 100 bytes: nenhum quadro, tudo vira resto
    vad.tem_voz(primeiro)

    # 100 + 8192 = 8292 bytes -> 8 quadros; o primeiro começa com os 100 bytes antigos.
    assert len(silero.quadros) == 8
    assert silero.quadros[0][:100] == b"\xaa" * 100
    assert silero.quadros[0][100:] == primeiro[:QUADRO - 100]


def test_nenhum_byte_se_perde_nem_se_repete_entre_blocos():
    silero = SileroFalso()
    vad = DetectorDeVoz(detector=silero)
    tudo = bytes((i * 7) % 256 for i in range(QUADRO * 6 + 500))

    for i in range(0, len(tudo), 777):          # blocos de tamanho irregular
        vad.tem_voz(tudo[i:i + 777])

    assert b"".join(silero.quadros) == tudo[: QUADRO * 6]


def test_tem_voz_se_algum_quadro_passa_do_limiar():
    vad = DetectorDeVoz(detector=SileroFalso(probabilidades=[0.1, 0.2, 0.9, 0.1]))
    assert vad.tem_voz(b"\x00" * (QUADRO * 4)) is True


def test_nao_tem_voz_se_todos_ficam_abaixo():
    vad = DetectorDeVoz(detector=SileroFalso(probabilidades=[0.1, 0.4, 0.49]))
    assert vad.tem_voz(b"\x00" * (QUADRO * 3)) is False


def test_o_limiar_vale_igual_e_e_configuravel():
    assert DetectorDeVoz(detector=SileroFalso(probabilidades=[0.5])).tem_voz(b"\x00" * QUADRO) is True
    assert DetectorDeVoz(detector=SileroFalso(probabilidades=[0.7]), limiar=0.8).tem_voz(b"\x00" * QUADRO) is False


def test_processa_todos_os_quadros_mesmo_depois_de_achar_voz():
    # O modelo tem memória: parar no primeiro quadro com voz desalinharia o
    # estado dele pro bloco seguinte.
    silero = SileroFalso(probabilidades=[0.9, 0.0, 0.0])
    DetectorDeVoz(detector=silero).tem_voz(b"\x00" * (QUADRO * 3))
    assert len(silero.quadros) == 3


def test_bloco_menor_que_um_quadro_nao_acusa_voz():
    silero = SileroFalso(padrao=1.0)
    assert DetectorDeVoz(detector=silero).tem_voz(b"\x00" * 100) is False
    assert silero.quadros == []


def test_reset_limpa_o_resto_e_zera_o_modelo():
    silero = SileroFalso()
    vad = DetectorDeVoz(detector=silero)
    vad.tem_voz(b"\x01" * 100)

    vad.reset()
    vad.tem_voz(b"\x02" * QUADRO)

    assert silero.resets == 1
    assert silero.quadros == [b"\x02" * QUADRO], "o resto de antes do reset vazou pro quadro novo"


def test_silero_real_silencio_nao_e_voz():
    vad = DetectorDeVoz()
    for _ in range(8):
        assert vad.tem_voz(b"\x00" * 8000) is False


@pytest.mark.skipif(not VOZ_REAL.exists(), reason="voz do Piper ausente em vozes/ (ver Task 0 do plano)")
def test_silero_real_acha_voz_numa_frase_falada():
    import numpy as np
    from james_device.sintese import Sintetizador

    pcm, taxa = Sintetizador(str(VOZ_REAL)).sintetizar("Qual é a capital da Austrália?")
    amostras = np.frombuffer(pcm, dtype=np.int16).astype(np.float32)
    n_alvo = int(len(amostras) * 16000 / taxa)
    reamostrado = np.interp(np.linspace(0, len(amostras) - 1, n_alvo), np.arange(len(amostras)), amostras)
    pcm16k = reamostrado.astype(np.int16).tobytes()

    vad = DetectorDeVoz()
    achou = False
    for i in range(0, len(pcm16k), 8000):
        achou = vad.tem_voz(pcm16k[i:i + 8000]) or achou

    assert achou, "o Silero não achou voz numa frase sintetizada clara"
