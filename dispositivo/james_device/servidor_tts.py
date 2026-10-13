"""Servidor de TTS local: o jamesd manda o texto da resposta DIGITADA e leva o
WAV de volta pro app tocar. É a mesma voz (Piper) que o device fala nas
respostas por voz, e carregada uma vez só.

Por que um processo à parte, em vez de o jamesd pedir ao device: o texto
digitado pode vir do celular, pela VPN, e o som tem de tocar onde a pessoa
digitou, não no alto-falante do PC. O jamesd gera o arquivo e o app toca.

Segurança: só escuta em loopback e não tem autenticação, porque recebe o texto
das respostas (que pode ter dado pessoal). Escutar em qualquer outro endereço
é recusado na construção.

    python -m dispositivo.james_device.servidor_tts
"""

import io
import json
import logging
import os
import threading
import wave
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

logger = logging.getLogger(__name__)

PORTA_PADRAO = 8180
VOZ_PADRAO = "vozes/pt_BR-faber-medium.onnx"

# O texto de uma resposta cabe folgado nisto; mais que isso é erro de quem
# chama ou abuso, e o Piper gastaria CPU à toa.
MAX_BYTES_DO_PEDIDO = 64 * 1024
MAX_CARACTERES = 20_000

_ENDERECOS_LOOPBACK = {"127.0.0.1", "localhost"}


def _wav_em_bytes(pcm: bytes, taxa: int) -> bytes:
    buffer = io.BytesIO()
    with wave.open(buffer, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(taxa)
        w.writeframes(pcm)
    return buffer.getvalue()


def criar_servidor(host: str, porta: int, sintetizador) -> ThreadingHTTPServer:
    if host not in _ENDERECOS_LOOPBACK:
        raise ValueError(
            f"o servidor de TTS só pode escutar em loopback (127.0.0.1 ou localhost), não em {host!r}: "
            "ele não tem autenticação e recebe o texto das respostas"
        )

    # O Piper não promete ser seguro entre threads; as sínteses entram uma por vez.
    trava = threading.Lock()

    class Manipulador(BaseHTTPRequestHandler):
        server_version = "james-tts"
        sys_version = ""

        def log_message(self, formato, *args):
            logger.debug("tts: " + formato, *args)

        def _responder(self, status, corpo=b"", tipo="text/plain; charset=utf-8"):
            self.send_response(status)
            self.send_header("Content-Type", tipo)
            self.send_header("Content-Length", str(len(corpo)))
            self.end_headers()
            self.wfile.write(corpo)

        def do_GET(self):
            if self.path == "/saude":
                self._responder(200, b"ok")
            else:
                self._responder(404, b"nao encontrado")

        def do_POST(self):
            if self.path != "/sintetizar":
                self._responder(404, b"nao encontrado")
                return
            try:
                tamanho = int(self.headers.get("Content-Length", ""))
            except ValueError:
                self._responder(400, b"Content-Length ausente")
                return
            if tamanho <= 0:
                self._responder(400, b"corpo vazio")
                return
            if tamanho > MAX_BYTES_DO_PEDIDO:
                self._responder(413, b"pedido grande demais")
                return

            try:
                dados = json.loads(self.rfile.read(tamanho))
                texto = dados["texto"]
            except (ValueError, KeyError, TypeError):
                self._responder(400, b"esperava JSON com o campo texto")
                return
            if not isinstance(texto, str) or not texto.strip():
                self._responder(400, b"texto vazio ou invalido")
                return
            if len(texto) > MAX_CARACTERES:
                self._responder(413, b"texto grande demais")
                return

            try:
                with trava:
                    pcm, taxa = sintetizador.sintetizar(texto)
                if not pcm:
                    self._responder(400, b"nada para falar")
                    return
                wav = _wav_em_bytes(pcm, taxa)
            except Exception:
                # O detalhe vai pro log; quem chamou recebe só o fato. A
                # mensagem de uma falha de síntese pode ter caminho de arquivo.
                logger.exception("falha ao sintetizar")
                self._responder(500, b"falha na sintese")
                return
            self._responder(200, wav, "audio/wav")

    return ThreadingHTTPServer((host, porta), Manipulador)


def main() -> None:
    from james_device.sintese import Sintetizador

    logging.basicConfig(level=logging.INFO, format="%(levelname)s:%(name)s:%(message)s")
    host = os.environ.get("JAMES_TTS_HOST", "127.0.0.1")
    porta = int(os.environ.get("JAMES_TTS_PORTA", PORTA_PADRAO))
    sintetizador = Sintetizador(os.environ.get("JAMES_PIPER_VOZ", VOZ_PADRAO))
    sintetizador.carregar_agora()  # a primeira resposta não paga a carga
    servidor = criar_servidor(host, porta, sintetizador)
    logger.info("servidor de TTS pronto em http://%s:%d", host, servidor.server_address[1])
    try:
        servidor.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        servidor.server_close()


if __name__ == "__main__":
    main()
