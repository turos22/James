"""Controle de mídia pelo MPRIS (D3 da spec 2026-10-13 B2). Barramento falso."""

import pytest

from james_device.midia import ErroDeMidia, Midia

PLAYER = "org.mpris.MediaPlayer2.Player"
RAIZ = "org.mpris.MediaPlayer2"


class Barramento:
    def __init__(self, players):
        # players: {servico: {"Identity": ..., "PlaybackStatus": ..., "Metadata": {...}, "CanGoNext": ...}}
        self.players = players
        self.chamadas = []

    def nomes(self):
        return ["org.freedesktop.DBus", ":1.42", *self.players]

    def propriedade(self, servico, interface, nome):
        valores = self.players[servico]
        if interface == RAIZ:
            return valores["Identity"]
        return valores.get(nome)

    def chamar(self, servico, metodo):
        self.chamadas.append((servico, metodo))


def _spotify(status="Playing", proxima=True):
    return {"Identity": "Spotify", "PlaybackStatus": status, "CanGoNext": proxima, "CanGoPrevious": True,
            "Metadata": {"xesam:title": "Hopeful", "xesam:artist": ["Ajmw"]}}


def _firefox(status="Paused"):
    return {"Identity": "Mozilla Firefox", "PlaybackStatus": status, "CanGoNext": False, "CanGoPrevious": False,
            "Metadata": {"xesam:title": "Vídeo do YouTube"}}


SP = "org.mpris.MediaPlayer2.spotify"
FF = "org.mpris.MediaPlayer2.firefox.instance_1_234"


def test_lista_so_os_players_mpris():
    midia = Midia(Barramento({SP: _spotify(), FF: _firefox()}))
    assert sorted(p.servico for p in midia.players()) == [FF, SP]


@pytest.mark.parametrize("acao, metodo", [
    ("tocar", "Play"), ("pausar", "Pause"), ("alternar", "PlayPause"), ("proxima", "Next"), ("anterior", "Previous"),
])
def test_acoes_chamam_o_metodo_certo(acao, metodo):
    barramento = Barramento({SP: _spotify()})

    Midia(barramento).executar(acao)

    assert barramento.chamadas == [(SP, metodo)]


def test_sem_nome_escolhe_o_que_esta_tocando():
    barramento = Barramento({FF: _firefox("Paused"), SP: _spotify("Playing")})

    Midia(barramento).executar("pausar")

    assert barramento.chamadas == [(SP, "Pause")]


def test_pelo_nome_escolhe_o_pedido_sem_caixa():
    barramento = Barramento({FF: _firefox("Paused"), SP: _spotify("Playing")})

    Midia(barramento).executar("tocar", player="FIREFOX")

    assert barramento.chamadas == [(FF, "Play")]


def test_varios_parados_sem_nome_pede_para_escolher():
    barramento = Barramento({FF: _firefox("Paused"), SP: _spotify("Paused")})

    with pytest.raises(ErroDeMidia, match="Spotify.*Mozilla Firefox|Mozilla Firefox.*Spotify"):
        Midia(barramento).executar("tocar")
    assert barramento.chamadas == []


def test_um_so_player_parado_e_escolhido():
    barramento = Barramento({SP: _spotify("Paused")})
    Midia(barramento).executar("tocar")
    assert barramento.chamadas == [(SP, "Play")]


def test_nenhum_player():
    with pytest.raises(ErroDeMidia, match="nenhum player"):
        Midia(Barramento({})).executar("tocar")


def test_player_pedido_que_nao_existe_lista_os_abertos():
    with pytest.raises(ErroDeMidia, match="não achei.*vlc.*Spotify"):
        Midia(Barramento({SP: _spotify()})).executar("tocar", player="vlc")


def test_estado_descreve_a_musica():
    saida = Midia(Barramento({SP: _spotify("Playing")})).executar("estado")
    assert saida == "Spotify: tocando 'Hopeful' de Ajmw"


def test_estado_pausado_e_sem_artista():
    saida = Midia(Barramento({FF: _firefox("Paused")})).executar("estado")
    assert saida == "Mozilla Firefox: pausado em 'Vídeo do YouTube'"


def test_proxima_sem_suporte_avisa_em_vez_de_fingir():
    barramento = Barramento({SP: _spotify(proxima=False)})

    with pytest.raises(ErroDeMidia, match="não deixa"):
        Midia(barramento).executar("proxima")
    assert barramento.chamadas == []


def test_acao_desconhecida():
    with pytest.raises(ErroDeMidia, match="ação de mídia desconhecida"):
        Midia(Barramento({SP: _spotify()})).executar("rebobinar")


def _video(status="Playing"):
    # Medido no Firefox real: o curso da Udemy aparece como Playing, com artista "".
    return {"Identity": "Mozilla org.mozilla.firefox", "PlaybackStatus": status, "CanGoNext": False,
            "CanGoPrevious": False, "Metadata": {"xesam:title": "Course: Testing in Go", "xesam:artist": [""]}}


def test_dois_tocando_prefere_o_que_tem_artista():
    barramento = Barramento({FF: _video(), SP: _spotify("Playing")})

    Midia(barramento).executar("proxima")

    assert barramento.chamadas == [(SP, "Next")]


def test_artista_vazio_nao_conta_e_nao_aparece_no_estado():
    saida = Midia(Barramento({FF: _video()})).executar("estado")
    assert saida == "Mozilla org.mozilla.firefox: tocando 'Course: Testing in Go'"


def test_dois_tocando_com_artista_pede_para_escolher():
    outro = dict(_spotify("Playing"), Identity="VLC")
    with pytest.raises(ErroDeMidia, match="diga qual"):
        Midia(Barramento({SP: _spotify("Playing"), "org.mpris.MediaPlayer2.vlc": outro})).executar("pausar")
