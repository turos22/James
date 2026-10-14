"""Ponte de tela do Linux (spec 2026-10-13 B2, seção 6). Todas as peças falsas."""

import base64

import pytest

from james_device.kwin import Janela, Tela
from james_device.portal import Fluxo, SemPermissao
from james_device.tela_linux import TelaLinux
from tests.test_acessibilidade import NoFalso

ID_KW = "{11111111-1111-4111-8111-111111111111}"
ID_FF = "{22222222-2222-4222-8222-222222222222}"
PNG = b"\x89PNG\r\n\x1a\nfalso"


def _janela(id_, titulo, pid, cliente, ativa=False, app="app", monitor="HDMI-A-1"):
    return Janela(id=id_, app=app, titulo=titulo, pid=pid, ativa=ativa, minimizada=False, normal=True,
                  monitor=monitor, frame=cliente, cliente=cliente)


class Kwin:
    def __init__(self, eventos, ativa=ID_KW):
        self.eventos = eventos
        self.ativa_id = ativa
        self.fechadas = []

    def listar(self):
        return [
            _janela(ID_KW, "Bem-vindo — KWrite", 500, (253.0, 200.0, 1414.0, 734.0), self.ativa_id == ID_KW,
                    "org.kde.kwrite"),
            _janela(ID_FF, "Google — Mozilla Firefox", 600, (1920.0, 588.0, 1422.0, 800.0), self.ativa_id == ID_FF,
                    "org.mozilla.firefox", "HDMI-A-2"),
        ]

    def ativa(self):
        return next((j for j in self.listar() if j.ativa), None)

    def telas(self):
        return [Tela("HDMI-A-1", 0, 0, 1920, 1080, 1.0), Tela("HDMI-A-2", 1920, 588, 1422, 800, 1.35)]

    def focar(self, identificador):
        self.eventos.append(("focar", identificador))
        self.ativa_id = identificador
        return True

    def fechar(self, identificador):
        self.fechadas.append(identificador)
        return True


class Portal:
    def __init__(self, eventos, sem_permissao=False):
        self.eventos = eventos
        self.sem_permissao = sem_permissao
        self.fluxos = [Fluxo(142, 0, 0, 1920, 1080), Fluxo(138, 1920, 588, 1422, 800)]

    def _checar(self):
        if self.sem_permissao:
            raise SemPermissao()

    def mover(self, gx, gy):
        self._checar()
        self.eventos.append(("mover", round(gx), round(gy)))

    def clicar(self):
        self._checar()
        self.eventos.append(("clicar",))

    def teclas(self, keysyms):
        self._checar()
        self.eventos.append(("teclas", tuple(keysyms)))

    def abrir_pipewire(self):
        self._checar()
        return 99


class Laco:
    def __init__(self):
        self.chamadas = 0

    def executar(self, funcao, timeout=5.0):
        self.chamadas += 1
        return funcao()


def _apps():
    novo = NoFalso("button", "Novo", (6, 6, 74, 34))
    excluir = NoFalso("button", "Excluir", (200, 6, 60, 34))
    campo = NoFalso("text", "Sem título", (39, 46, 1315, 665), editavel=True)
    campo._estados.add("focused")
    kwrite = NoFalso("application", "kwrite", filhos=[
        NoFalso("frame", "Bem-vindo — KWrite", filhos=[novo, excluir, campo])], pid=500)
    busca = NoFalso("entry", "Pesquisar", (325, 49, 1332, 32), editavel=True)
    firefox = NoFalso("application", "Firefox", filhos=[
        NoFalso("frame", "Google — Mozilla Firefox", filhos=[busca])], pid=600)
    return [kwrite, firefox]


class Midia:
    def __init__(self):
        self.pedidos = []

    def executar(self, acao, player=None):
        self.pedidos.append((acao, player))
        return f"{acao} no Spotify"


class Abrir:
    def __init__(self):
        self.pedidos = []

    def executar(self, alvo, linha=None):
        self.pedidos.append((alvo, linha))
        return f"abri {alvo}"


class Area:
    def tipos(self):
        return ["text/plain"]

    def ler(self):
        return "copiado"

    def gravar(self, texto):
        pass

    def limpar(self):
        pass


@pytest.fixture
def mundo():
    eventos = []
    capturas = []

    def capturar(fd, node, largura, altura, timeout=5.0):
        capturas.append((fd, node, largura, altura))
        return PNG

    tela = TelaLinux(laco=Laco(), kwin=Kwin(eventos), portal=Portal(eventos), area=Area(), midia=Midia(),
                     abrir=Abrir(), listar_apps=_apps, capturar=capturar, fechar_fd=lambda fd: None)
    tela.eventos = eventos
    tela.capturas = capturas
    return tela


# --- nunca levanta ------------------------------------------------------------------

def test_acao_desconhecida_vira_erro(mundo):
    assert "desconhecida" in mundo.executar_acao({"acao": "dançar"})["erro"]


def test_excecao_qualquer_vira_erro(mundo):
    mundo._kwin.telas = lambda: (_ for _ in ()).throw(RuntimeError("KWin caiu"))
    assert "KWin caiu" in mundo.executar_acao({"acao": "ver_tela"})["erro"]


def test_sem_permissao_explica(mundo):
    mundo._portal.sem_permissao = True
    assert "clique em Permitir" in mundo.executar_acao({"acao": "ver_tela"})["erro"]


# --- ver_tela -------------------------------------------------------------------------

def test_ver_tela_do_monitor_da_janela_ativa(mundo):
    r = mundo.executar_acao({"acao": "ver_tela"})

    assert r["saida"].startswith("superficie: HDMI-A-1 (primaria) 1280x720 escala 100%")
    assert base64.b64decode(r["imagem_b64"]) == PNG
    assert mundo.capturas == [(99, 142, 1280, 720)]


def test_ver_tela_de_monitor_pedido(mundo):
    r = mundo.executar_acao({"acao": "ver_tela", "superficie": "HDMI-A-2"})

    assert "HDMI-A-2 1280x720 escala 135%" in r["saida"]
    assert mundo.capturas[0][1] == 138


def test_ver_tela_monitor_inexistente(mundo):
    assert "não existe" in mundo.executar_acao({"acao": "ver_tela", "superficie": "DP-9"})["erro"]


def test_ver_tela_lista_elementos_no_espaco_da_imagem_e_estruturados(mundo):
    r = mundo.executar_acao({"acao": "ver_tela"})

    # Novo: global (259..333, 206..240) → centro (296, 223) → imagem / 1,5 = (197, 149)
    assert 'e1  Button  "Novo"  (197,149)' in r["saida"]
    assert {"id": "e1", "nome": "Novo", "tipo": "Button"} in r["elementos"]
    assert {"id": "e2", "nome": "Excluir", "tipo": "Button"} in r["elementos"]


def test_ver_tela_roda_o_atspi_dentro_do_laco(mundo):
    antes = mundo._laco.chamadas
    mundo.executar_acao({"acao": "ver_tela"})
    assert mundo._laco.chamadas > antes


def test_ver_tela_avisa_app_sem_elementos(mundo):
    mundo._listar_apps = lambda: [NoFalso("application", "code", filhos=[NoFalso("frame", "Bem-vindo — KWrite")],
                                          pid=500)]
    r = mundo.executar_acao({"acao": "ver_tela"})

    assert "não expõe elementos" in r["saida"]
    assert r["imagem_b64"]


def test_ver_tela_com_varredura_quebrada_ainda_manda_a_imagem(mundo):
    mundo._listar_apps = lambda: (_ for _ in ()).throw(RuntimeError("AT-SPI fora"))
    r = mundo.executar_acao({"acao": "ver_tela"})

    assert r["imagem_b64"]
    assert "não consegui ler os elementos" in r["saida"]
    assert r["elementos"] == []


def test_ver_tela_com_captura_quebrada_ainda_manda_a_lista(mundo):
    mundo._capturar = lambda *a, **k: (_ for _ in ()).throw(RuntimeError("captura falhou: PipeWire"))
    r = mundo.executar_acao({"acao": "ver_tela"})

    assert "imagem_b64" not in r
    assert "não consegui capturar a imagem" in r["saida"]
    assert '"Novo"' in r["saida"]


# --- click por coordenada ---------------------------------------------------------------

def test_click_converte_imagem_para_global_e_clica(mundo):
    r = mundo.executar_acao({"acao": "click", "x": 640, "y": 100})

    assert ("mover", 960, 150) in mundo.eventos
    assert mundo.eventos[-1] == ("clicar",)
    assert "cliquei em (640,100) de HDMI-A-1" in r["saida"]


def test_click_fora_da_imagem_e_erro(mundo):
    assert "fora dos limites" in mundo.executar_acao({"acao": "click", "x": 1280, "y": 10})["erro"]
    assert mundo.eventos == []


def test_click_em_cima_de_elemento_listado_e_recusado(mundo):
    mundo.executar_acao({"acao": "ver_tela"})

    r = mundo.executar_acao({"acao": "click", "x": 197, "y": 149})

    assert "use clicar_elemento" in r["erro"] and "Novo" in r["erro"]
    assert ("clicar",) not in mundo.eventos


# --- clicar_elemento ---------------------------------------------------------------------

def test_clicar_elemento_sem_ver_tela(mundo):
    assert "chame ver_tela antes" in mundo.executar_acao({"acao": "clicar_elemento", "elemento_id": "e1"})["erro"]


def test_clicar_elemento_por_id_clica_no_centro(mundo):
    mundo.executar_acao({"acao": "ver_tela"})

    r = mundo.executar_acao({"acao": "clicar_elemento", "elemento_id": "e1"})

    assert ("mover", 296, 223) in mundo.eventos
    assert mundo.eventos[-1] == ("clicar",)
    assert 'cliquei em "Novo" (Button)' in r["saida"]


def test_clicar_elemento_por_nome(mundo):
    mundo.executar_acao({"acao": "ver_tela"})
    assert "Excluir" in mundo.executar_acao({"acao": "clicar_elemento", "nome": "Excluir"})["saida"]


def test_clicar_elemento_que_nao_existe(mundo):
    mundo.executar_acao({"acao": "ver_tela"})
    assert "não achei" in mundo.executar_acao({"acao": "clicar_elemento", "elemento_id": "e99"})["erro"]


def test_clicar_elemento_com_a_tela_trocada(mundo):
    mundo.executar_acao({"acao": "ver_tela"})
    mundo._kwin.ativa_id = ID_FF  # o Arthur trocou de janela

    assert "a tela mudou" in mundo.executar_acao({"acao": "clicar_elemento", "elemento_id": "e1"})["erro"]


def test_clicar_elemento_revalida_contra_a_arvore_viva(mundo):
    mundo.executar_acao({"acao": "ver_tela"})
    mundo._donos["e1"].no._estados.discard("enabled")

    r = mundo.executar_acao({"acao": "clicar_elemento", "elemento_id": "e1"})

    assert "desabilitado" in r["erro"]
    assert ("clicar",) not in mundo.eventos


def test_clicar_elemento_de_outra_janela_foca_a_dona_antes(mundo):
    mundo.executar_acao({"acao": "ver_tela", "superficie": "HDMI-A-2"})

    mundo.executar_acao({"acao": "clicar_elemento", "nome": "Pesquisar"})

    ordem = [e[0] for e in mundo.eventos]
    assert ordem.index("focar") < ordem.index("mover") < ordem.index("clicar")
    assert ("focar", ID_FF) in mundo.eventos


# --- digitar e tecla -----------------------------------------------------------------------

def test_digitar_escreve_no_campo_em_foco(mundo):
    campo = mundo._listar_apps()[0]
    apps = [campo]
    mundo._listar_apps = lambda: apps
    alvo = apps[0]._filhos[0]._filhos[2]
    escritos = []
    mundo._envolver_campo = lambda no: type("C", (), {
        "ler": lambda self: "".join(escritos), "inserir": lambda self, t: escritos.append(t) or True})()

    r = mundo.executar_acao({"acao": "digitar", "texto": "ação"})

    assert escritos == ["ação"]
    assert "AT-SPI" in r["saida"]
    assert alvo is not None


def test_digitar_sem_texto(mundo):
    assert "vazio" in mundo.executar_acao({"acao": "digitar", "texto": ""})["erro"]


def test_tecla_vira_keysyms(mundo):
    r = mundo.executar_acao({"acao": "tecla", "texto": "ctrl+l"})

    assert mundo.eventos == [("teclas", (0xFFE3, ord("l")))]
    assert "ctrl+l" in r["saida"]


def test_tecla_com_frase_aponta_para_digitar(mundo):
    assert "use digitar" in mundo.executar_acao({"acao": "tecla", "texto": "olá mundo"})["erro"]


# --- janelas, mídia, abrir -------------------------------------------------------------------

def test_janelas_listar(mundo):
    saida = mundo.executar_acao({"acao": "janelas_listar"})["saida"]
    assert "Bem-vindo — KWrite" in saida and "(ativa)" in saida
    assert "Google — Mozilla Firefox" in saida


def test_janela_focar_por_alvo(mundo):
    r = mundo.executar_acao({"acao": "janela_focar", "alvo": "firefox"})

    assert ("focar", ID_FF) in mundo.eventos
    assert "Google — Mozilla Firefox" in r["saida"]


def test_janela_focar_ambiguo_lista_e_nao_foca(mundo):
    r = mundo.executar_acao({"acao": "janela_focar", "alvo": "—"})

    assert "diga qual" in r["erro"]
    assert not any(e[0] == "focar" for e in mundo.eventos)


def test_janela_focar_sem_alvo_ou_sem_casamento(mundo):
    assert "não achei" in mundo.executar_acao({"acao": "janela_focar", "alvo": "photoshop"})["erro"]
    assert "alvo" in mundo.executar_acao({"acao": "janela_focar"})["erro"]


def test_janela_fechar(mundo):
    r = mundo.executar_acao({"acao": "janela_fechar", "alvo": "kwrite"})

    assert mundo._kwin.fechadas == [ID_KW]
    assert "salvar" in r["saida"]


def test_midia_usa_texto_como_subacao_e_alvo_como_player(mundo):
    mundo.executar_acao({"acao": "midia", "texto": "proxima", "alvo": "spotify"})
    mundo.executar_acao({"acao": "midia", "texto": "estado"})

    assert mundo._midia.pedidos == [("proxima", "spotify"), ("estado", None)]


def test_abrir_com_e_sem_linha(mundo):
    mundo.executar_acao({"acao": "abrir", "alvo": "dolphin"})
    mundo.executar_acao({"acao": "abrir", "alvo": "/tmp/a.go", "linha": 10})

    assert mundo._abrir.pedidos == [("dolphin", None), ("/tmp/a.go", 10)]


# --- achados do code-review -----------------------------------------------------

def test_click_em_cima_de_elemento_e_recusado_mesmo_com_o_mapa_invalido(mundo):
    # ver_tela, depois o foco muda (o mapa deixa de valer): o clique por
    # coordenada no "Novo" não pode passar só porque o retrato velho morreu.
    mundo.executar_acao({"acao": "ver_tela"})
    mundo._kwin.ativa_id = ID_FF

    r = mundo.executar_acao({"acao": "click", "x": 197, "y": 149, "superficie": "HDMI-A-1"})

    assert "use clicar_elemento" in r["erro"]
    assert ("clicar",) not in mundo.eventos


def test_click_sem_nenhum_ver_tela_antes_tambem_confere_a_tela_agora(mundo):
    r = mundo.executar_acao({"acao": "click", "x": 197, "y": 149})

    assert "use clicar_elemento" in r["erro"]


def test_click_onde_nao_ha_elemento_passa(mundo):
    r = mundo.executar_acao({"acao": "click", "x": 640, "y": 100})

    assert "cliquei" in r["saida"]


def test_click_com_a_varredura_quebrada_ainda_clica(mundo):
    # Spotify e jogo: sem árvore, o clique por coordenada é o único caminho.
    mundo._listar_apps = lambda: (_ for _ in ()).throw(RuntimeError("AT-SPI fora"))

    r = mundo.executar_acao({"acao": "click", "x": 640, "y": 100})

    assert "cliquei" in r["saida"]
