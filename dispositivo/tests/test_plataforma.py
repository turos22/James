import subprocess
import sys
from pathlib import Path

RAIZ_DISPOSITIVO = Path(__file__).resolve().parents[1]


def _importa_limpo(modulo: str) -> subprocess.CompletedProcess:
    """Importa num processo novo: sys.modules deste processo já pode ter o
    módulo (ou o Win32) carregado por outro teste."""
    return subprocess.run(
        [sys.executable, "-c", f"import {modulo}"],
        cwd=RAIZ_DISPOSITIVO, capture_output=True, text=True,
    )


def test_superficies_importa_em_qualquer_sistema():
    r = _importa_limpo("james_device.superficies")
    assert r.returncode == 0, r.stderr


from james_device import plataforma
from james_device.plataforma import linux


def test_escolhe_o_modulo_pelo_sistema():
    assert plataforma.nome_do_modulo("win32") == "james_device.plataforma.windows"
    assert plataforma.nome_do_modulo("linux") == "james_device.plataforma.linux"


def test_sistema_desconhecido_cai_no_linux():
    # Mac e afins: sem Win32, o que sobra de portável é o caminho Linux
    # (tela desligada). Falhar no import seria pior que voz sem tela.
    assert plataforma.nome_do_modulo("darwin") == "james_device.plataforma.linux"


def test_linux_toca_com_pw_play(monkeypatch):
    chamadas = []
    monkeypatch.setattr(linux.subprocess, "run", lambda args, **kw: chamadas.append(args))
    linux.tocar_som("dispositivo/assets/entrada.mp3")
    assert chamadas == [["pw-play", "dispositivo/assets/entrada.mp3"]]


def test_linux_sem_pw_play_nao_levanta(monkeypatch, caplog):
    def sem_binario(args, **kw):
        raise FileNotFoundError("pw-play")

    monkeypatch.setattr(linux.subprocess, "run", sem_binario)
    linux.tocar_som("x.wav")  # não pode levantar: som é enfeite, não pode derrubar a escuta
    assert "pw-play" in caplog.text


def test_linux_ponte_de_tela_recusa_sem_levantar():
    ponte = linux.ponte_de_tela()
    resposta = ponte({"acao": "click", "x": 1, "y": 1})
    assert set(resposta) == {"erro"}
    assert "indisponível" in resposta["erro"]


def test_linux_declara_tela_indisponivel():
    assert linux.TELA_DISPONIVEL is False
