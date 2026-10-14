"""Abrir app, arquivo, pasta e URL (D4 da spec 2026-10-13 B2).

URL, arquivo e pasta vão ao app padrão do desktop (Gio, o mesmo que o
Dolphin usa). App pelo nome é procurado nos `.desktop` instalados, sem caixa
e sem acento; com mais de um candidato e nenhum nome exato, devolve a lista
em vez de chutar. Arquivo numa linha abre no VS Code com `code -g` (decisão
do Arthur: o VS Code não expõe a árvore de elementos).

SEGURANÇA: nada passa por shell. O `code -g` recebe uma lista de argumentos,
e a linha tem de ser um inteiro positivo.
"""

import logging
import os
import re
import subprocess
import unicodedata
from dataclasses import dataclass
from pathlib import Path

logger = logging.getLogger(__name__)

_ESQUEMA = re.compile(r"^[a-zA-Z][a-zA-Z0-9+.-]*://")
_CARA_DE_CAMINHO = ("/", "~", "./", "../")
_ESQUEMAS_PERMITIDOS = frozenset({"http", "https", "mailto"})


class ErroDeAbrir(ValueError):
    pass


@dataclass(frozen=True)
class App:
    id: str
    nome: str
    generico: str = ""


def _normalizar(texto: str) -> str:
    sem_acento = unicodedata.normalize("NFKD", texto)
    return "".join(c for c in sem_acento if not unicodedata.combining(c)).casefold().strip()


def _compactar(texto: str) -> str:
    """Sem espaço nem pontuação: "vs code" casa com com.microsoft.VSCode."""
    return re.sub(r"[^0-9a-z]", "", _normalizar(texto))


def classificar(alvo: str) -> tuple[str, str]:
    """("url" | "caminho" | "app", valor). Caminho que não existe é erro: se
    parece caminho, tentar abrir como app só confundiria."""
    texto = (alvo or "").strip()
    if not texto:
        raise ErroDeAbrir("alvo vazio para abrir")
    if _ESQUEMA.match(texto) or texto.lower().startswith("mailto:"):
        esquema = texto.split(":", 1)[0].lower()
        if esquema not in _ESQUEMAS_PERMITIDOS:
            # ssh://, smb://, vscode:// etc. acionam o programa registrado para
            # o esquema sem ninguém ter pedido: só web e e-mail.
            raise ErroDeAbrir(f"esquema {esquema}:// não permitido (só http, https e mailto)")
        return "url", texto
    if texto.lower().startswith("www."):
        return "url", "https://" + texto
    expandido = os.path.expanduser(texto)
    if texto.startswith(_CARA_DE_CAMINHO):
        if not os.path.exists(expandido):
            raise ErroDeAbrir(f"{texto} não existe")
        return "caminho", expandido
    if os.sep in texto and os.path.exists(expandido):
        return "caminho", expandido
    return "app", texto


def _linha_valida(linha) -> int:
    if isinstance(linha, bool) or not isinstance(linha, int) or linha < 1:
        raise ErroDeAbrir(f"linha inválida: {linha!r} (tem de ser um número a partir de 1)")
    return linha


class Abrir:
    def __init__(self, lancador) -> None:
        self._lancador = lancador

    def executar(self, alvo: str, linha=None) -> str:
        if linha is not None:
            linha = _linha_valida(linha)
            tipo, valor = classificar(alvo)
            if tipo != "caminho" or not os.path.isfile(valor):
                raise ErroDeAbrir("para abrir numa linha, o alvo tem de ser um arquivo que existe")
            self._lancador.rodar(["code", "-g", f"{valor}:{linha}"])
            return f"abri {valor} na linha {linha} no VS Code"

        tipo, valor = classificar(alvo)
        if tipo == "url":
            self._lancador.abrir_uri(valor)
            return f"abri {valor} no navegador padrão"
        if tipo == "caminho":
            self._lancador.abrir_uri(Path(valor).resolve().as_uri())
            return f"abri {valor}"
        app = self._achar_app(valor)
        self._lancador.lancar_app(app.id)
        return f"abri o {app.nome}"

    def _achar_app(self, nome: str) -> App:
        procurado = _normalizar(nome)
        apps = self._lancador.apps()
        exatos = [a for a in apps if _normalizar(a.nome) == procurado]
        if len(exatos) == 1:
            return exatos[0]
        compacto = _compactar(procurado)
        candidatos = [a for a in apps if procurado in _normalizar(a.nome) or procurado in _normalizar(a.generico)
                      or (compacto and (compacto in _compactar(a.nome)
                                        or compacto in _compactar(a.id.removesuffix(".desktop"))))]
        if len(candidatos) == 1:
            return candidatos[0]
        if not candidatos:
            raise ErroDeAbrir(f"não achei nenhum app chamado {nome!r}")
        lista = ", ".join(a.nome for a in candidatos[:6])
        raise ErroDeAbrir(f"achei {len(candidatos)} apps para {nome!r}: {lista}; diga qual")


class LancadorGio:
    """O lançador de verdade: Gio para URI e `.desktop`, Popen para o `code`."""

    def apps(self) -> list[App]:
        from gi.repository import Gio

        apps = []
        for info in Gio.AppInfo.get_all():
            if not info.should_show() or not info.get_id():
                continue
            generico = info.get_generic_name() if hasattr(info, "get_generic_name") else ""
            apps.append(App(info.get_id(), info.get_display_name() or info.get_name(), generico or ""))
        return apps

    def abrir_uri(self, uri: str) -> None:
        from gi.repository import Gio

        Gio.AppInfo.launch_default_for_uri(uri, None)

    def lancar_app(self, identificador: str) -> None:
        from gi.repository import Gio

        info = Gio.DesktopAppInfo.new(identificador)
        if info is None:
            raise ErroDeAbrir(f"o app {identificador} sumiu")
        info.launch([], None)

    def rodar(self, argumentos: list[str]) -> None:
        subprocess.Popen(argumentos, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                         stderr=subprocess.DEVNULL, start_new_session=True)
