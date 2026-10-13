"""O servidor de TTS local: o jamesd manda o texto da resposta digitada e leva
o WAV de volta pro app tocar. Roda em loopback, sem autenticação, e recebe o
texto das respostas (que pode ter dado pessoal): por isso a recusa de
qualquer endereço que não seja loopback é um teste, não só um costume."""

import json
import threading
import urllib.error
import urllib.request
import wave
import io

import pytest

from james_device.servidor_tts import criar_servidor


class SintetizadorFalso:
    def __init__(self):
        self.textos = []

    def sintetizar(self, texto):
        self.textos.append(texto)
        return b"\x01\x00" * 441, 22050  # 441 amostras


@pytest.fixture
def servidor():
    sintetizador = SintetizadorFalso()
    srv = criar_servidor("127.0.0.1", 0, sintetizador)
    fio = threading.Thread(target=srv.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
    fio.start()
    yield srv, sintetizador
    srv.shutdown()
    srv.server_close()


def _url(srv, caminho):
    return f"http://127.0.0.1:{srv.server_address[1]}{caminho}"


def _post(srv, corpo, tipo="application/json"):
    dados = corpo if isinstance(corpo, bytes) else json.dumps(corpo).encode()
    req = urllib.request.Request(_url(srv, "/sintetizar"), data=dados, method="POST",
                                 headers={"Content-Type": tipo})
    return urllib.request.urlopen(req, timeout=5)


def test_sintetizar_devolve_wav_valido(servidor):
    srv, sint = servidor

    with _post(srv, {"texto": "oi."}) as resp:
        assert resp.status == 200
        assert resp.headers["Content-Type"] == "audio/wav"
        corpo = resp.read()

    assert sint.textos == ["oi."]
    with wave.open(io.BytesIO(corpo), "rb") as w:
        assert (w.getnchannels(), w.getsampwidth(), w.getframerate()) == (1, 2, 22050)
        assert w.getnframes() == 441


@pytest.mark.parametrize("corpo", [
    {"texto": ""},
    {"texto": "   "},
    {"outra": "coisa"},
    {"texto": 5},
    b"isto nao e json",
    b"",
])
def test_pedido_invalido_devolve_400(servidor, corpo):
    srv, sint = servidor

    with pytest.raises(urllib.error.HTTPError) as erro:
        _post(srv, corpo)

    assert erro.value.code == 400
    assert sint.textos == [], "texto inválido não pode chegar ao Piper"


def test_saude_responde_200(servidor):
    srv, _ = servidor
    with urllib.request.urlopen(_url(srv, "/saude"), timeout=5) as resp:
        assert resp.status == 200


def test_caminho_desconhecido_devolve_404(servidor):
    srv, _ = servidor
    with pytest.raises(urllib.error.HTTPError) as erro:
        urllib.request.urlopen(_url(srv, "/qualquer"), timeout=5)
    assert erro.value.code == 404


def test_texto_gigante_e_recusado(servidor):
    srv, sint = servidor
    with pytest.raises(urllib.error.HTTPError) as erro:
        _post(srv, {"texto": "a" * 200_000})
    assert erro.value.code in (400, 413)
    assert sint.textos == []


def test_sintese_que_quebra_devolve_500_sem_vazar_o_erro():
    class Quebrado:
        def sintetizar(self, texto):
            raise RuntimeError("caminho secreto /home/arthur/voz.onnx corrompido")

    srv = criar_servidor("127.0.0.1", 0, Quebrado())
    threading.Thread(target=srv.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True).start()
    try:
        with pytest.raises(urllib.error.HTTPError) as erro:
            _post(srv, {"texto": "oi."})
        corpo = erro.value.read().decode()
    finally:
        srv.shutdown()
        srv.server_close()

    assert erro.value.code == 500
    assert "secreto" not in corpo and "/home/" not in corpo


@pytest.mark.parametrize("endereco", ["0.0.0.0", "192.168.0.10", "::", "exemplo.invalid", ""])
def test_recusa_escutar_fora_do_loopback(endereco):
    with pytest.raises(ValueError, match="loopback"):
        criar_servidor(endereco, 0, SintetizadorFalso())


@pytest.mark.parametrize("endereco", ["127.0.0.1", "localhost"])
def test_aceita_loopback(endereco):
    srv = criar_servidor(endereco, 0, SintetizadorFalso())
    srv.server_close()
