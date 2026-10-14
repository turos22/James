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


def test_linux_ponte_de_tela_recusa_sem_levantar_quando_nao_ha_tela(monkeypatch):
    monkeypatch.setattr(linux, "TELA_DISPONIVEL", False)
    monkeypatch.setattr(linux, "MOTIVO_SEM_TELA", "a sessão não é Wayland (XDG_SESSION_TYPE=x11)")
    ponte = linux.ponte_de_tela()
    resposta = ponte({"acao": "click", "x": 1, "y": 1})
    assert set(resposta) == {"erro"}
    assert "indisponível" in resposta["erro"]
    assert "Wayland" in resposta["erro"]


KDE_WAYLAND = {"XDG_SESSION_TYPE": "wayland", "XDG_CURRENT_DESKTOP": "KDE"}


def _gi_ok():
    return None


def test_tela_so_com_wayland_kde_e_gi():
    assert linux.motivo_sem_tela(KDE_WAYLAND, _gi_ok) is None


def test_sem_wayland_nao_ha_tela():
    assert "Wayland" in linux.motivo_sem_tela({**KDE_WAYLAND, "XDG_SESSION_TYPE": "x11"}, _gi_ok)
    assert "Wayland" in linux.motivo_sem_tela({"XDG_CURRENT_DESKTOP": "KDE"}, _gi_ok)


def test_sem_kde_nao_ha_tela():
    assert "KDE" in linux.motivo_sem_tela({**KDE_WAYLAND, "XDG_CURRENT_DESKTOP": "GNOME"}, _gi_ok)


def test_kde_em_lista_de_desktops_conta():
    assert linux.motivo_sem_tela({**KDE_WAYLAND, "XDG_CURRENT_DESKTOP": "KDE:plasma"}, _gi_ok) is None


def test_sem_gi_nao_ha_tela():
    def sem_gi():
        raise ImportError("No module named 'gi'")

    assert "gi" in linux.motivo_sem_tela(KDE_WAYLAND, sem_gi)


def test_ponte_que_nao_monta_vira_recusa_e_nao_derruba(monkeypatch, caplog):
    monkeypatch.setattr(linux, "TELA_DISPONIVEL", True)

    def quebrar():
        raise RuntimeError("D-Bus fora")

    monkeypatch.setattr(linux, "_montar_ponte", quebrar)
    resposta = linux.ponte_de_tela()({"acao": "ver_tela"})
    assert "indisponível" in resposta["erro"]
    assert "D-Bus fora" in caplog.text


# --- Windows: ações novas respondem indisponível, o resto passa intacto -----------

def test_windows_recusa_as_acoes_so_do_linux_e_delega_o_resto():
    recebidas = []

    def executar_acao(dados):
        recebidas.append(dados)
        return {"saida": "ok"}

    ponte = plataforma.recusar_acoes_novas(executar_acao, "Windows")

    for acao in ("janelas_listar", "janela_focar", "janela_fechar", "midia", "abrir"):
        resposta = ponte({"acao": acao})
        assert resposta == {"erro": "indisponível neste sistema (Windows)"}
    for acao in ("ver_tela", "click", "clicar_elemento", "digitar", "tecla"):
        assert ponte({"acao": acao, "x": 1}) == {"saida": "ok"}
    assert recebidas == [{"acao": a, "x": 1} for a in ("ver_tela", "click", "clicar_elemento", "digitar", "tecla")]


def test_main_importa_sem_puxar_tela_nem_playsound():
    """main não pode depender de tela/pyautogui/playsound no import: no Linux
    eles não existem, e o device inteiro (voz inclusa) morria antes do laço."""
    r = subprocess.run(
        [sys.executable, "-c",
         "import sys, james_device.main; "
         "print(sorted(m for m in ('james_device.tela', 'playsound', 'pyautogui') if m in sys.modules))"],
        cwd=RAIZ_DISPOSITIVO, capture_output=True, text=True,
    )
    assert r.returncode == 0, r.stderr
    assert r.stdout.strip() == "[]"
