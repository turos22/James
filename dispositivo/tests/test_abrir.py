"""Abrir app, arquivo, pasta e URL (D4 da spec 2026-10-13 B2). Lançador falso."""

import pytest

from james_device.abrir import Abrir, App, ErroDeAbrir, classificar


class Lancador:
    def __init__(self, apps=()):
        self._apps = list(apps)
        self.uris, self.lancados, self.comandos = [], [], []

    def apps(self):
        return list(self._apps)

    def abrir_uri(self, uri):
        self.uris.append(uri)

    def lancar_app(self, identificador):
        self.lancados.append(identificador)

    def rodar(self, argumentos):
        self.comandos.append(list(argumentos))


APPS = [
    App("org.kde.dolphin.desktop", "Dolphin", "Gerenciador de arquivos"),
    App("org.mozilla.firefox.desktop", "Firefox", "Navegador web"),
    App("code.desktop", "Visual Studio Code", "Editor de texto"),
    App("org.kde.kwrite.desktop", "KWrite", "Editor de texto"),
    App("org.kde.konsole.desktop", "Konsole", "Terminal"),
]


@pytest.fixture
def pasta(tmp_path):
    (tmp_path / "notas.txt").write_text("oi")
    return tmp_path


# --- classificar ----------------------------------------------------------------

def test_url_e_url():
    assert classificar("https://exemplo.com/a?b=1") == ("url", "https://exemplo.com/a?b=1")
    assert classificar("HTTP://exemplo.com") == ("url", "HTTP://exemplo.com")


def test_www_vira_https():
    assert classificar("www.exemplo.com") == ("url", "https://www.exemplo.com")


def test_caminho_existente(pasta):
    assert classificar(str(pasta / "notas.txt")) == ("caminho", str(pasta / "notas.txt"))


def test_til_e_expandido(pasta, monkeypatch):
    monkeypatch.setenv("HOME", str(pasta))
    assert classificar("~/notas.txt") == ("caminho", str(pasta / "notas.txt"))


def test_caminho_que_parece_caminho_mas_nao_existe_e_erro_e_nao_vira_app():
    with pytest.raises(ErroDeAbrir, match="não existe"):
        classificar("/nao/existe/nada.txt")
    with pytest.raises(ErroDeAbrir, match="não existe"):
        classificar("~/pasta-que-nao-existe-123")


def test_nome_solto_e_app():
    assert classificar("dolphin") == ("app", "dolphin")
    assert classificar("visual studio code") == ("app", "visual studio code")


# --- abrir ----------------------------------------------------------------------

def test_url_vai_ao_app_padrao():
    lancador = Lancador()
    Abrir(lancador).executar("https://exemplo.com")
    assert lancador.uris == ["https://exemplo.com"]


def test_arquivo_e_pasta_vao_ao_app_padrao_como_uri(pasta):
    lancador = Lancador()
    abrir = Abrir(lancador)

    abrir.executar(str(pasta))
    abrir.executar(str(pasta / "notas.txt"))

    assert lancador.uris == [pasta.as_uri(), (pasta / "notas.txt").as_uri()]


def test_app_pelo_nome_sem_caixa_e_sem_acento():
    lancador = Lancador(APPS)
    saida = Abrir(lancador).executar("DOLPHIN")
    assert lancador.lancados == ["org.kde.dolphin.desktop"]
    assert "Dolphin" in saida


def test_app_pelo_nome_generico():
    lancador = Lancador(APPS)
    Abrir(lancador).executar("navegador")
    assert lancador.lancados == ["org.mozilla.firefox.desktop"]


def test_nome_exato_vence_quando_ha_varios():
    lancador = Lancador(APPS + [App("org.kde.konsole-perfil.desktop", "Konsole (perfil zsh)", "Terminal")])
    Abrir(lancador).executar("konsole")
    assert lancador.lancados == ["org.kde.konsole.desktop"]


def test_varios_sem_exato_lista_e_nao_abre():
    lancador = Lancador(APPS)
    with pytest.raises(ErroDeAbrir, match="KWrite.*Visual Studio Code|Visual Studio Code.*KWrite"):
        Abrir(lancador).executar("editor")
    assert lancador.lancados == []


def test_app_que_nao_existe():
    with pytest.raises(ErroDeAbrir, match="não achei nenhum app"):
        Abrir(Lancador(APPS)).executar("photoshop")


def test_arquivo_numa_linha_abre_no_vs_code_sem_shell(pasta):
    lancador = Lancador(APPS)
    arquivo = pasta / "notas.txt"

    Abrir(lancador).executar(str(arquivo), linha=12)

    assert lancador.comandos == [["code", "-g", f"{arquivo}:12"]]


def test_linha_sem_arquivo_existente_e_erro(pasta):
    with pytest.raises(ErroDeAbrir):
        Abrir(Lancador()).executar("dolphin", linha=3)
    with pytest.raises(ErroDeAbrir, match="arquivo"):
        Abrir(Lancador()).executar(str(pasta), linha=3)


@pytest.mark.parametrize("linha", [0, -1, "12; rm -rf ~"])
def test_linha_invalida_e_erro(pasta, linha):
    with pytest.raises(ErroDeAbrir, match="linha"):
        Abrir(Lancador()).executar(str(pasta / "notas.txt"), linha=linha)


def test_alvo_vazio_e_erro():
    with pytest.raises(ErroDeAbrir, match="vazio"):
        Abrir(Lancador()).executar("   ")


def test_vs_code_falado_casa_com_o_id_real():
    # Id real no Fedora do Arthur: com.microsoft.VSCode.desktop.
    lancador = Lancador([App("com.microsoft.VSCode.desktop", "Visual Studio Code", "Editor de texto"), *APPS[:2]])
    Abrir(lancador).executar("vs code")
    assert lancador.lancados == ["com.microsoft.VSCode.desktop"]


def test_alvo_so_de_pontuacao_nao_casa_com_tudo():
    with pytest.raises(ErroDeAbrir, match="não achei nenhum app"):
        Abrir(Lancador(APPS)).executar("!!!")


@pytest.mark.parametrize("url", ["ssh://servidor/x", "smb://nas/pasta", "vscode://file/etc/passwd", "x-scheme://a"])
def test_so_http_https_e_mailto_viram_url(url):
    with pytest.raises(ErroDeAbrir, match="esquema"):
        classificar(url)


@pytest.mark.parametrize("url", ["https://a.com", "http://a.com", "mailto:fulano@exemplo.com", "www.a.com"])
def test_esquemas_permitidos(url):
    assert classificar(url)[0] == "url"
