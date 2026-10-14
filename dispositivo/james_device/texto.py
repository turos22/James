"""Escrever texto sem depender do layout do teclado (D6 da spec 2026-10-13 B2).

Ordem, medida no spike:
1. AT-SPI `insert_text` no campo em foco: exato, 5 ms, não toca na área de
   transferência. Sempre RELÊ depois: a barra de endereço do Firefox devolve
   True e não muda nada.
2. Colar: grava o texto na área de transferência, Ctrl+V pelo portal, e
   SEMPRE devolve o conteúdo anterior (finally).
3. Keysym nunca: com o layout `us`, `ç` virou `<` e `ã` sumiu.

Corrida do colar: depois do Ctrl+V o app busca o conteúdo de forma
assíncrona. Devolver o conteúdo antigo cedo demais faria o app colar o texto
antigo do Arthur. Com campo legível, a restauração espera o texto aparecer;
sem campo (app sem AT-SPI), espera um tempo fixo.

Área de transferência pelo Klipper (desvio da D6, ver o registro do plano):
síncrono, e o conteúdo sobrevive ao device. Conteúdo que não é texto (imagem,
arquivo) faz o James RECUSAR o colar, para não destruir o que o Arthur
copiou. Efeito colateral aceito: o texto colado fica no histórico do Klipper.
"""

import logging
import subprocess
import time

from james_device.acessibilidade import NoIlegivel

logger = logging.getLogger(__name__)

ESPERA_DEPOIS_DE_INSERIR = 0.3
PRAZO_DO_COLAR = 1.5
PRAZO_DA_AREA = 0.5
ESPERA_SEM_CONFIRMACAO = 0.5
_TIPOS_DE_TEXTO = frozenset({"STRING", "UTF8_STRING", "TEXT", "COMPOUND_TEXT"})
_TETO_NOS = 4000


class ErroDeTexto(ValueError):
    pass


def _esperar(condicao, segundos: float) -> bool:
    fim = time.monotonic() + segundos
    while True:
        if condicao():
            return True
        if time.monotonic() >= fim:
            return False
        time.sleep(0.03)


def _sem_espaco(texto: str) -> str:
    return "".join(texto.split())


def _entrou(campo, antes: str | None, texto: str) -> bool:
    """O texto apareceu no campo? Campo de uma linha descarta a quebra de
    linha, por isso a comparação ignora espaço em branco."""
    try:
        depois = campo.ler()
    except NoIlegivel:
        return False
    return depois is not None and depois != antes and _sem_espaco(texto) in _sem_espaco(depois)


_FORMATOS_RICOS = ("image/", "application/x-qt-image", "text/html", "text/uri-list", "text/x-moz-url")


def _e_texto(tipo: str) -> bool:
    return tipo.startswith("text/") or tipo in _TIPOS_DE_TEXTO


def _tem_formato_rico(tipos: list[str]) -> bool:
    """A restauração devolve só texto simples (o Klipper recebe uma string).
    Arquivos copiados, HTML e imagens se perderiam, e o James dizia 'voltou
    ao que era'."""
    return any(t.startswith(_FORMATOS_RICOS) for t in tipos)


def digitar(texto: str, campo, area, colar, esperar=_esperar, dormir=time.sleep) -> str:
    """Escreve `texto` no campo em foco. `campo` pode ser None (app sem
    AT-SPI); `colar()` manda o Ctrl+V. Devolve a frase para o modelo."""
    if not texto:
        raise ErroDeTexto("texto vazio para digitar")

    if campo is not None:
        try:
            antes = campo.ler()
            # False = inserir_conferindo já provou que não entrou (barra do
            # Firefox): esperar o prazo só atrasaria o colar em 300 ms.
            if campo.inserir(texto) is not False and esperar(
                    lambda: _entrou(campo, antes, texto), ESPERA_DEPOIS_DE_INSERIR):
                return f"digitei {len(texto)} caracteres no campo em foco (AT-SPI)"
        except NoIlegivel:
            campo = None  # o campo sumiu: tenta colar, sem como conferir

    tipos = area.tipos()
    if tipos and (not any(_e_texto(t) for t in tipos) or _tem_formato_rico(tipos)):
        raise ErroDeTexto(
            "não consegui escrever direto no campo, e a área de transferência tem algo que não é texto "
            "simples (imagem, arquivo ou texto formatado) e se perderia: não vou sobrescrever o que o Arthur "
            "copiou; o formato precisa ser texto puro")
    original = area.ler() if tipos else None
    antes = campo.ler() if campo is not None else None
    try:
        area.gravar(texto)
        # O Klipper leva um instante para assumir a área de transferência; o
        # Ctrl+V antes disso cola o conteúdo antigo (medido ao vivo).
        if not esperar(lambda: area.ler() == texto, PRAZO_DA_AREA):
            raise ErroDeTexto("a área de transferência não aceitou o texto; não colei nada")
        colar()
        if campo is not None:
            if not esperar(lambda: _entrou(campo, antes, texto), PRAZO_DO_COLAR):
                raise ErroDeTexto("colei, mas o texto não apareceu no campo")
            return f"colei {len(texto)} caracteres no campo em foco (a área de transferência voltou ao que era)"
        dormir(ESPERA_SEM_CONFIRMACAO)
        return (f"colei {len(texto)} caracteres sem confirmação (este app não expõe o campo); "
                "a área de transferência voltou ao que era")
    finally:
        if original is None:
            area.limpar()
        else:
            area.gravar(original)


def inserir_conferindo(documento, conteudo: str) -> bool:
    """Insere `conteudo` no cursor e CONFERE; se não conferir, desfaz.

    Medido ao vivo (Task 17): o Qt conta o comprimento do insert_text em
    CARACTERES, e com o comprimento em bytes (o que a libatspi documenta)
    insere o texto e mais caracteres NULOS no documento; o GTK conta em
    bytes. Por isso tenta em caracteres, confere que o documento cresceu
    exatamente o texto e que o trecho é o texto, desfaz o que entrou errado,
    e tenta em bytes. Nada conferiu: o documento volta ao que era e quem
    chamou cai para colar.

    `documento` dá contar(), caret(), ler(ini, fim), inserir(pos, texto,
    comprimento) e apagar(ini, fim)."""
    tentativas = [len(conteudo)]
    em_bytes = len(conteudo.encode("utf-8"))
    if em_bytes != len(conteudo):
        tentativas.append(em_bytes)
    for comprimento in tentativas:
        antes = documento.contar()
        posicao = documento.caret()
        if posicao < 0 or posicao > antes:
            posicao = antes
        documento.inserir(posicao, conteudo, comprimento)
        cresceu = documento.contar() - antes
        if cresceu == len(conteudo) and documento.ler(posicao, posicao + cresceu) == conteudo:
            return True
        if cresceu > 0:
            documento.apagar(posicao, posicao + cresceu)
    return False


def campo_em_foco(raiz, envolver=None):
    """O campo editável com foco dentro de `raiz` (a janela ativa), ou None."""
    envolver = envolver or CampoAtspi
    pilha, visitados = [raiz], 0
    while pilha and visitados < _TETO_NOS:
        no = pilha.pop()
        visitados += 1
        try:
            estados = no.estados()
            if "showing" not in estados:
                continue
            if "focused" in estados and "editable" in estados:
                return envolver(no)
            pilha.extend(no.filhos())
        except NoIlegivel:
            continue
    return None


class CampoAtspi:
    """Texto de um nó do AT-SPI (acessibilidade.NoAtspi).

    `executar` é o laco.executar: cada leitura entra no laço e sai, porque
    `digitar` espera o texto aparecer FORA do laço (ver acessibilidade.py,
    cache da libatspi)."""

    def __init__(self, no, executar=None) -> None:
        self._no = no
        self._executar = executar or (lambda funcao: funcao())

    def _iface(self, obter):
        from gi.repository import GLib

        try:
            iface = obter()
        except GLib.Error as erro:
            raise NoIlegivel(erro.message) from erro
        if iface is None:
            raise NoIlegivel("o campo não tem interface de texto")
        return iface

    def ler(self) -> str:
        return self._executar(self._ler)

    # As funções vêm da INTERFACE (Atspi.Text, Atspi.EditableText): o objeto
    # que get_text_iface devolve é o próprio Accessible, e `.get_text` nele é
    # outro método, antigo, de outra assinatura (achado ao vivo na Task 8).

    def _ler(self) -> str:
        from gi.repository import Atspi, GLib

        texto = self._iface(self._no.acessivel.get_text_iface)
        try:
            return Atspi.Text.get_text(texto, 0, -1)
        except GLib.Error as erro:
            raise NoIlegivel(erro.message) from erro

    def inserir(self, conteudo: str) -> bool:
        return self._executar(lambda: self._inserir(conteudo))

    def _inserir(self, conteudo: str) -> bool:
        from gi.repository import GLib

        try:
            return inserir_conferindo(_DocumentoAtspi(self), conteudo)
        except GLib.Error as erro:
            raise NoIlegivel(erro.message) from erro


class _DocumentoAtspi:
    """O documento de um CampoAtspi para inserir_conferindo. Roda dentro do
    laço (CampoAtspi.inserir já entrou nele)."""

    def __init__(self, campo: CampoAtspi) -> None:
        self._texto = campo._iface(campo._no.acessivel.get_text_iface)
        self._editavel = campo._iface(campo._no.acessivel.get_editable_text_iface)

    def contar(self) -> int:
        from gi.repository import Atspi

        return Atspi.Text.get_character_count(self._texto)

    def caret(self) -> int:
        from gi.repository import Atspi

        return Atspi.Text.get_caret_offset(self._texto)

    def ler(self, inicio: int, fim: int) -> str:
        from gi.repository import Atspi

        return Atspi.Text.get_text(self._texto, inicio, fim)

    def inserir(self, posicao: int, conteudo: str, comprimento: int) -> bool:
        from gi.repository import Atspi

        return Atspi.EditableText.insert_text(self._editavel, posicao, conteudo, comprimento)

    def apagar(self, inicio: int, fim: int) -> None:
        from gi.repository import Atspi

        Atspi.EditableText.delete_text(self._editavel, inicio, fim)


class AreaKlipper:
    """Área de transferência: leitura pelo `wl-paste` (exata, e não vira
    daemon, ao contrário do `wl-copy`) e escrita pelo Klipper por D-Bus."""

    _TIPO = "text/plain;charset=utf-8"

    def __init__(self, laco, conexao) -> None:
        self._laco = laco
        self._conexao = conexao

    def tipos(self) -> list[str]:
        resultado = subprocess.run(["wl-paste", "--list-types"], stdin=subprocess.DEVNULL,
                                   capture_output=True, timeout=2, check=False)
        if resultado.returncode != 0:
            return []  # "Nothing is copied"
        return [t for t in resultado.stdout.decode("utf-8", "replace").splitlines() if t.strip()]

    def ler(self) -> str:
        resultado = subprocess.run(["wl-paste", "--no-newline", "--type", self._TIPO], stdin=subprocess.DEVNULL,
                                   capture_output=True, timeout=2, check=False)
        return resultado.stdout.decode("utf-8", "replace")

    def _klipper(self, metodo: str, *argumentos: str) -> None:
        from gi.repository import GLib

        variante = GLib.Variant(f"({'s' * len(argumentos)})", argumentos) if argumentos else None
        self._laco.executar(lambda: self._conexao.call_sync(
            "org.kde.klipper", "/klipper", "org.kde.klipper.klipper", metodo, variante, None, 0, 3000, None))

    def gravar(self, texto: str) -> None:
        self._klipper("setClipboardContents", texto)

    def limpar(self) -> None:
        self._klipper("clearClipboardContents")
