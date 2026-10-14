"""Portal RemoteDesktop + ScreenCast (D5, D5a, D8 da spec 2026-10-13 B2).

A lógica é testada com um canal falso. O caminho real (o portal do KDE) só
roda com JAMES_TESTE_DESKTOP=1, e mesmo assim só move o mouse: não clica.
"""

import os
import stat
import sys

import pytest

from james_device.kwin import Tela
from james_device.portal import (
    BTN_ESQUERDO,
    MENSAGEM_SEM_PERMISSAO,
    Fluxo,
    Portal,
    RecusaDoPortal,
    SemPermissao,
    Token,
    casar_fluxos,
    para_fluxo,
)

SESSAO = "/org/freedesktop/portal/desktop/session/1_234/james"
RD = "org.freedesktop.portal.RemoteDesktop"
SC = "org.freedesktop.portal.ScreenCast"


class CanalFalso:
    def __init__(self, recusar_start=False, falhas_de_create=0):
        self.pedidos: list[tuple] = []
        self.chamadas: list[tuple] = []
        self.fechamento = None
        self.recusar_start = recusar_start
        self.falhas_de_create = falhas_de_create
        self.token_devolvido = "token-novo"

    def pedir(self, interface, metodo, assinatura, valores, timeout):
        self.pedidos.append((interface, metodo, assinatura, valores, timeout))
        if metodo == "CreateSession":
            if self.falhas_de_create:
                self.falhas_de_create -= 1
                raise RecusaDoPortal(2)
            return {"session_handle": SESSAO}
        if metodo == "Start":
            if self.recusar_start:
                raise RecusaDoPortal(1)
            return {
                "devices": 3,
                "restore_token": self.token_devolvido,
                # node_id trocado de propósito: casar é por posição.
                "streams": [(142, {"position": (0, 0), "size": (1920, 1080)}),
                            (138, {"position": (1920, 588), "size": (1422, 800)})],
            }
        return {}

    def chamar(self, interface, metodo, assinatura, valores):
        self.chamadas.append((metodo, valores))

    def assinar_fechamento(self, sessao, callback):
        self.fechamento = callback

    def metodos_pedidos(self):
        return [p[1] for p in self.pedidos]

    def opcoes_de(self, metodo):
        return next(p[3] for p in reversed(self.pedidos) if p[1] == metodo)[-1]  # o mais recente


@pytest.fixture
def token(tmp_path):
    return Token(tmp_path / "james" / "portal-token")


class Relogio:
    def __init__(self):
        self.agora = 0.0

    def __call__(self):
        return self.agora


def _portal(canal, token, relogio=None, disparos=None):
    disparos = disparos if disparos is not None else []
    return Portal(canal, token, relogio=relogio or Relogio(), em_segundo_plano=lambda f: (disparos.append(f), f()))


# --- token -----------------------------------------------------------------------

def test_token_ausente_le_none(token):
    assert token.ler() is None


@pytest.mark.skipif(sys.platform == "win32", reason="permissão POSIX")
def test_token_grava_0600_em_pasta_0700_sem_deixar_temporario(token):
    token.gravar("abc")
    token.gravar("def\n")

    assert token.ler() == "def"
    assert stat.S_IMODE(os.stat(token.caminho).st_mode) == 0o600
    assert stat.S_IMODE(os.stat(token.caminho.parent).st_mode) == 0o700
    assert sorted(p.name for p in token.caminho.parent.iterdir()) == ["portal-token"]


def test_token_vazio_conta_como_ausente(token):
    token.gravar("   ")
    assert token.ler() is None


# --- abrir a sessão --------------------------------------------------------------

def test_abrir_sem_token_pede_teclado_mouse_e_monitores_persistentes(token):
    canal = CanalFalso()
    portal = _portal(canal, token)

    portal.abrir()

    assert canal.metodos_pedidos() == ["CreateSession", "SelectDevices", "SelectSources", "Start"]
    opcoes = canal.opcoes_de("SelectDevices")
    assert opcoes["types"] == ("u", 3)
    assert opcoes["persist_mode"] == ("u", 2)
    assert "restore_token" not in opcoes
    fontes = canal.opcoes_de("SelectSources")
    assert fontes["types"] == ("u", 1)
    assert fontes["multiple"] == ("b", True)
    assert portal.estado == "aberto"
    assert token.ler() == "token-novo"


def test_abrir_com_token_manda_o_token_e_guarda_o_novo(token):
    token.gravar("token-velho")
    canal = CanalFalso()

    _portal(canal, token).abrir()

    assert canal.opcoes_de("SelectDevices")["restore_token"] == ("s", "token-velho")
    assert token.ler() == "token-novo"


def test_fluxos_ficam_disponiveis_depois_de_abrir(token):
    portal = _portal(CanalFalso(), token)
    portal.abrir()

    assert sorted(f.node for f in portal.fluxos) == [138, 142]


def test_start_recusado_deixa_sem_permissao_e_acao_explica(token):
    portal = _portal(CanalFalso(recusar_start=True), token)

    portal.abrir()

    assert portal.estado == "sem_permissao"
    with pytest.raises(SemPermissao, match="clique em Permitir"):
        portal.mover(10, 10)
    assert MENSAGEM_SEM_PERMISSAO.startswith("preciso de permissão")


def test_abrir_assina_o_fechamento_da_sessao(token):
    canal = CanalFalso()
    _portal(canal, token).abrir()

    assert canal.fechamento is not None


# --- fluxos e coordenadas ----------------------------------------------------------

FLUXOS = [Fluxo(142, 0, 0, 1920, 1080), Fluxo(138, 1920, 588, 1422, 800)]
TELAS = [Tela("HDMI-A-1", 0, 0, 1920, 1080, 1.0), Tela("HDMI-A-2", 1920, 588, 1422, 800, 1.35)]


def test_casar_fluxos_pela_posicao_mesmo_com_node_trocado():
    casados = casar_fluxos(FLUXOS, TELAS)

    assert casados["HDMI-A-1"].node == 142
    assert casados["HDMI-A-2"].node == 138


def test_casar_fluxos_sem_fluxo_para_uma_tela_e_erro():
    with pytest.raises(ValueError, match="HDMI-A-2"):
        casar_fluxos(FLUXOS[:1], TELAS)


def test_para_fluxo_da_coordenada_relativa_ao_monitor():
    assert para_fluxo(FLUXOS, 2631, 988) == (138, 711, 400)
    assert para_fluxo(FLUXOS, 960, 540) == (142, 960, 540)


def test_para_fluxo_fora_de_todo_monitor_e_erro():
    with pytest.raises(ValueError, match="fora"):
        para_fluxo(FLUXOS, 2000, 100)  # área morta acima do HDMI-A-2


# --- mouse e teclado ---------------------------------------------------------------

def _aberto(token):
    canal = CanalFalso()
    portal = _portal(canal, token)
    portal.abrir()
    return canal, portal


def test_mover_manda_posicao_absoluta_no_fluxo_certo(token):
    canal, portal = _aberto(token)

    portal.mover(2631, 988)

    metodo, valores = canal.chamadas[-1]
    assert metodo == "NotifyPointerMotionAbsolute"
    assert valores == (SESSAO, {}, 138, 711.0, 400.0)


def test_clicar_aperta_e_solta_o_botao_esquerdo(token):
    canal, portal = _aberto(token)

    portal.clicar()

    assert canal.chamadas == [
        ("NotifyPointerButton", (SESSAO, {}, BTN_ESQUERDO, 1)),
        ("NotifyPointerButton", (SESSAO, {}, BTN_ESQUERDO, 0)),
    ]
    assert BTN_ESQUERDO == 0x110


def test_teclas_apertam_em_ordem_e_soltam_ao_contrario(token):
    canal, portal = _aberto(token)

    portal.teclas([0xFFE3, ord("v")])

    assert canal.chamadas == [
        ("NotifyKeyboardKeysym", (SESSAO, {}, 0xFFE3, 1)),
        ("NotifyKeyboardKeysym", (SESSAO, {}, ord("v"), 1)),
        ("NotifyKeyboardKeysym", (SESSAO, {}, ord("v"), 0)),
        ("NotifyKeyboardKeysym", (SESSAO, {}, 0xFFE3, 0)),
    ]


def test_acao_antes_de_abrir_tenta_abrir(token):
    canal = CanalFalso()
    portal = _portal(canal, token)

    portal.clicar()

    assert canal.metodos_pedidos()[0] == "CreateSession"
    assert canal.chamadas[0][0] == "NotifyPointerButton"


# --- sessão fechada por fora e permissão negada ------------------------------------------

def test_sessao_fechada_reabre_uma_vez_com_o_token(token):
    canal, portal = _aberto(token)
    canal.fechamento()
    assert portal.estado == "fechado"

    portal.clicar()

    assert canal.metodos_pedidos().count("CreateSession") == 2
    assert canal.opcoes_de("SelectDevices")["restore_token"] == ("s", "token-novo")
    assert portal.estado == "aberto"


def test_sessao_fechada_que_nao_reabre_vira_sem_permissao(token):
    canal, portal = _aberto(token)
    canal.fechamento()
    canal.recusar_start = True

    with pytest.raises(SemPermissao):
        portal.clicar()
    assert portal.estado == "sem_permissao"


def test_sem_permissao_tenta_de_novo_em_segundo_plano_no_maximo_por_minuto(token):
    relogio = Relogio()
    disparos = []
    canal = CanalFalso(recusar_start=True)
    portal = _portal(canal, token, relogio, disparos)
    portal.abrir()
    tentativas = canal.metodos_pedidos().count("Start")

    relogio.agora = 61
    with pytest.raises(SemPermissao):
        portal.clicar()
    relogio.agora = 70
    with pytest.raises(SemPermissao):
        portal.clicar()

    assert len(disparos) == 1
    assert canal.metodos_pedidos().count("Start") == tentativas + 1


def test_permissao_concedida_na_nova_tentativa_volta_a_funcionar(token):
    relogio = Relogio()
    canal = CanalFalso(recusar_start=True)
    portal = _portal(canal, token, relogio)
    portal.abrir()

    canal.recusar_start = False
    relogio.agora = 61
    with pytest.raises(SemPermissao):
        portal.clicar()  # esta ainda explica; a tentativa em segundo plano reabre

    portal.clicar()
    assert portal.estado == "aberto"


# --- desktop de verdade -----------------------------------------------------------

@pytest.mark.skipif(
    os.environ.get("JAMES_TESTE_DESKTOP") != "1" or sys.platform != "linux",
    reason="defina JAMES_TESTE_DESKTOP=1 para falar com o portal de verdade",
)
def test_ao_vivo_move_o_mouse_ao_centro_de_cada_monitor():
    from gi.repository import Gio

    from james_device.kwin import ExecutorKWin, Kwin
    from james_device.laco_glib import LacoGlib
    from james_device.portal import CanalPortal, caminho_do_token

    laco = LacoGlib()
    laco.iniciar()
    try:
        conexao = laco.executar(lambda: Gio.bus_get_sync(Gio.BusType.SESSION, None))
        kwin = Kwin(ExecutorKWin(laco, conexao))
        portal = Portal(CanalPortal(laco, conexao), Token(caminho_do_token()))
        portal.abrir()
        assert portal.estado == "aberto", "o portal não abriu (permissão negada ou diálogo sem resposta)"
        casar_fluxos(portal.fluxos, kwin.telas())
        for tela in kwin.telas():
            alvo = (tela.x + tela.largura / 2, tela.y + tela.altura / 2)
            portal.mover(*alvo)
            x, y = (float(v) for v in kwin._executor.rodar(
                'devolver(workspace.cursorPos.x + "\\u001f" + workspace.cursorPos.y);')[0].split("\x1f"))
            print(f"{tela.nome}: alvo {alvo} real {(x, y)}")
            assert abs(x - alvo[0]) <= 1 and abs(y - alvo[1]) <= 1
    finally:
        laco.encerrar()


def test_acao_enquanto_a_sessao_abre_nao_diz_que_falta_permissao(token):
    portal = _portal(CanalPortal_falso := CanalFalso(), token)
    portal.estado = "abrindo"

    with pytest.raises(SemPermissao) as erro:
        portal.clicar()

    assert "abrindo" in str(erro.value)
    assert "clique em Permitir" not in str(erro.value)
