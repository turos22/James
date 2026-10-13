"""A escolha do STT (JAMES_STT) e o transcritor do Whisper local.

O contrato não mudou: Callable[[bytes], str], PCM 16 kHz mono 16 bits dentro,
texto fora. Falha de rede ou do servidor devolve "" e loga: quem decide o que
fazer com um comando vazio é o escutar (que manda falar 'não entendi')."""

import io
import json
import logging
import threading
import wave
from http.server import BaseHTTPRequestHandler, HTTPServer

import pytest

from james_device.transcricao import FRASE_VAZIA, criar_transcritor, transcritor_whisper


class ServidorWhisperFalso:
    def __init__(self, status=200, corpo=b'{"text":" abre o navegador\\n"}'):
        self.status, self.corpo = status, corpo
        self.pedidos = []
        servidor = self

        class Manipulador(BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def do_POST(self):
                tamanho = int(self.headers["Content-Length"])
                servidor.pedidos.append({
                    "caminho": self.path,
                    "tipo": self.headers["Content-Type"],
                    "corpo": self.rfile.read(tamanho),
                })
                self.send_response(servidor.status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(servidor.corpo)))
                self.end_headers()
                self.wfile.write(servidor.corpo)

        self.http = HTTPServer(("127.0.0.1", 0), Manipulador)
        threading.Thread(target=self.http.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True).start()
        self.url = f"http://127.0.0.1:{self.http.server_address[1]}"

    def fechar(self):
        self.http.shutdown()
        self.http.server_close()


@pytest.fixture
def whisper():
    criados = []

    def _criar(**kw):
        s = ServidorWhisperFalso(**kw)
        criados.append(s)
        return s

    yield _criar
    for s in criados:
        s.fechar()


PCM = b"\x01\x00\x02\x00\x03\x00" * 1000  # 3000 amostras


def _wav_do_pedido(pedido) -> bytes:
    boundary = pedido["tipo"].split("boundary=")[1].encode()
    corpo = pedido["corpo"]
    inicio = corpo.index(b"RIFF")
    fim = corpo.index(b"\r\n--" + boundary, inicio)
    return corpo[inicio:fim]


def test_manda_o_audio_como_wav_16khz_mono_e_devolve_o_texto_sem_espacos(whisper):
    s = whisper()

    texto = transcritor_whisper(s.url)(PCM)

    assert texto == "abre o navegador"
    pedido = s.pedidos[0]
    assert pedido["caminho"] == "/inference"
    assert pedido["tipo"].startswith("multipart/form-data")
    with wave.open(io.BytesIO(_wav_do_pedido(pedido)), "rb") as w:
        assert (w.getnchannels(), w.getsampwidth(), w.getframerate()) == (1, 2, 16000)
        assert w.readframes(w.getnframes()) == PCM


def test_pede_json_e_temperatura_zero(whisper):
    s = whisper()
    transcritor_whisper(s.url)(PCM)
    corpo = s.pedidos[0]["corpo"].decode("latin-1")
    assert 'name="response_format"' in corpo and "json" in corpo
    assert 'name="temperature"' in corpo


def test_audio_vazio_nao_faz_requisicao(whisper):
    s = whisper()
    assert transcritor_whisper(s.url)(b"") == ""
    assert s.pedidos == []


def test_servidor_com_erro_devolve_vazio_e_loga(whisper, caplog):
    s = whisper(status=500, corpo=b"falhou")
    with caplog.at_level(logging.WARNING):
        assert transcritor_whisper(s.url)(PCM) == ""
    assert any("whisper" in r.message.lower() for r in caplog.records)


def test_servidor_fora_do_ar_devolve_vazio_e_loga(whisper, caplog):
    s = whisper()
    url = s.url
    s.fechar()
    with caplog.at_level(logging.WARNING):
        assert transcritor_whisper(url)(PCM) == ""
    assert any("whisper" in r.message.lower() for r in caplog.records)


@pytest.mark.parametrize("corpo", [b"isso nao e json", b'{"outra": 1}', b"[]", b'{"text": 5}'])
def test_resposta_ilegivel_devolve_vazio(whisper, corpo):
    s = whisper(corpo=corpo)
    assert transcritor_whisper(s.url)(PCM) == ""


def test_texto_de_silencio_do_whisper_vira_vazio(whisper):
    # O Whisper "alucina" frases-padrão em áudio sem fala; as mais comuns não
    # podem virar comando.
    for alucinacao in ("[BLANK_AUDIO]", "[Música]", "(silêncio)", "..."):
        s = whisper(corpo=json.dumps({"text": f" {alucinacao} "}).encode())
        assert transcritor_whisper(s.url)(PCM) == "", alucinacao
    assert FRASE_VAZIA  # a lista existe e é pública, pra poder crescer com os dados reais


# ---------- JAMES_STT ----------

def test_padrao_e_whisper_na_url_do_ambiente(whisper):
    s = whisper()
    transcrever = criar_transcritor({"JAMES_WHISPER_URL": s.url})

    assert transcrever(PCM) == "abre o navegador"
    assert len(s.pedidos) == 1


def test_whisper_explicito(whisper):
    s = whisper()
    assert criar_transcritor({"JAMES_STT": "whisper", "JAMES_WHISPER_URL": s.url})(PCM) == "abre o navegador"


def test_google_devolve_um_transcritor_sem_tocar_na_rede_com_audio_vazio():
    transcrever = criar_transcritor({"JAMES_STT": "google"})
    assert transcrever(b"") == ""


def test_valor_invalido_falha_no_boot_nomeando_a_variavel():
    with pytest.raises(ValueError, match="JAMES_STT"):
        criar_transcritor({"JAMES_STT": "deepgram"})


def test_whisper_sem_fallback_para_o_google(whisper, monkeypatch):
    # Decisão da Parte A: sem fallback para o Google. Se o Whisper cai, o
    # comando vem vazio, e o James pede pra repetir.
    import james_device.transcricao as t

    monkeypatch.setattr(t, "transcritor_google", lambda: (_ for _ in ()).throw(AssertionError("chamou o Google")))
    s = whisper(status=500, corpo=b"x")
    assert criar_transcritor({"JAMES_WHISPER_URL": s.url})(PCM) == ""
