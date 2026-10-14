"""Controle de mídia pelo MPRIS (D3 da spec 2026-10-13 B2).

"Troca a música" não precisa de tela: todo player do Linux (Spotify,
Firefox, VLC...) publica `org.mpris.MediaPlayer2.<nome>` no barramento, com
Play/Pause/Next/Previous e o que está tocando. Sem clique errado, sem
depender de onde o botão está.

A lógica é testada com um barramento falso; o BarramentoMpris fala D-Bus
pelo laço GLib.
"""

import logging
from dataclasses import dataclass

logger = logging.getLogger(__name__)

PREFIXO = "org.mpris.MediaPlayer2."
_RAIZ = "org.mpris.MediaPlayer2"
_PLAYER = "org.mpris.MediaPlayer2.Player"
_METODOS = {"tocar": "Play", "pausar": "Pause", "alternar": "PlayPause", "proxima": "Next", "anterior": "Previous"}
_PERMISSOES = {"proxima": ("CanGoNext", "pular para a próxima"), "anterior": ("CanGoPrevious", "voltar para a anterior")}
_STATUS = {"Playing": "tocando", "Paused": "pausado em", "Stopped": "parado"}


class ErroDeMidia(ValueError):
    pass


@dataclass(frozen=True)
class Player:
    servico: str
    nome: str
    status: str
    titulo: str
    artistas: tuple[str, ...]


class Midia:
    def __init__(self, barramento) -> None:
        self._barramento = barramento

    def players(self) -> list[Player]:
        players = []
        for servico in self._barramento.nomes():
            if not servico.startswith(PREFIXO):
                continue
            try:
                nome = self._barramento.propriedade(servico, _RAIZ, "Identity") or servico[len(PREFIXO):]
                status = self._barramento.propriedade(servico, _PLAYER, "PlaybackStatus") or "Stopped"
                metadados = self._barramento.propriedade(servico, _PLAYER, "Metadata") or {}
            except Exception:  # noqa: BLE001 — player que fechou no meio da leitura só sai da lista
                logger.debug("player %s ilegível", servico, exc_info=True)
                continue
            artistas = metadados.get("xesam:artist") or []
            if not isinstance(artistas, (list, tuple)):
                artistas = [artistas]
            # O Firefox manda artista "" para vídeo: não conta como artista.
            artistas = tuple(str(a) for a in artistas if str(a).strip())
            players.append(Player(servico, nome, status, metadados.get("xesam:title") or "", artistas))
        return players

    def escolher(self, pedido: str | None = None) -> Player:
        players = self.players()
        if not players:
            raise ErroDeMidia("nenhum player de mídia aberto")
        nomes = ", ".join(p.nome for p in players)
        if pedido:
            alvo = pedido.casefold()
            achados = [p for p in players if alvo in p.nome.casefold() or alvo in p.servico.casefold()]
            if not achados:
                raise ErroDeMidia(f"não achei o player {pedido!r}; abertos: {nomes}")
            return achados[0]
        tocando = [p for p in players if p.status == "Playing"]
        if len(tocando) == 1:
            return tocando[0]
        # Dois tocando (medido: Spotify e um curso em vídeo no Firefox): o que
        # tem artista é música, que é o que "troca a música" quer dizer.
        musica = [p for p in tocando if p.artistas]
        if len(musica) == 1:
            return musica[0]
        if len(players) == 1:
            return players[0]
        raise ErroDeMidia(f"há mais de um player aberto ({nomes}); diga qual")

    def executar(self, acao: str, player: str | None = None) -> str:
        if acao != "estado" and acao not in _METODOS:
            raise ErroDeMidia(f"ação de mídia desconhecida: {acao!r} (aceitas: estado, {', '.join(_METODOS)})")
        escolhido = self.escolher(player)
        if acao == "estado":
            return _descrever(escolhido)
        if acao in _PERMISSOES:
            propriedade, frase = _PERMISSOES[acao]
            if not self._barramento.propriedade(escolhido.servico, _PLAYER, propriedade):
                raise ErroDeMidia(f"o {escolhido.nome} não deixa {frase} agora")
        self._barramento.chamar(escolhido.servico, _METODOS[acao])
        return f"{acao} no {escolhido.nome}"


def _descrever(player: Player) -> str:
    status = _STATUS.get(player.status, player.status.lower())
    if not player.titulo:
        return f"{player.nome}: {status.removesuffix(' em')}"
    texto = f"{player.nome}: {status} '{player.titulo}'"
    if player.artistas:
        texto += " de " + ", ".join(player.artistas)
    return texto


class BarramentoMpris:
    """O barramento de verdade: D-Bus pelo laço GLib."""

    _CAMINHO = "/org/mpris/MediaPlayer2"

    def __init__(self, laco, conexao) -> None:
        self._laco = laco
        self._conexao = conexao

    def _chamar(self, destino, caminho, interface, metodo, variante):
        return self._laco.executar(lambda: self._conexao.call_sync(
            destino, caminho, interface, metodo, variante, None, 0, 3000, None))

    def nomes(self) -> list[str]:
        resposta = self._chamar("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
                                "ListNames", None)
        return list(resposta.unpack()[0])

    def propriedade(self, servico, interface, nome):
        from gi.repository import GLib

        resposta = self._chamar(servico, self._CAMINHO, "org.freedesktop.DBus.Properties", "Get",
                                GLib.Variant("(ss)", (interface, nome)))
        return resposta.unpack()[0]

    def chamar(self, servico, metodo) -> None:
        self._chamar(servico, self._CAMINHO, _PLAYER, metodo, None)
