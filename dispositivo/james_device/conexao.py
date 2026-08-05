"""Cliente WS do james-device: conecta no jamesd, reconecta sozinho se cair
(nunca escuta — invariante I1 da spec), manda heartbeat, despacha acao_tela
pro manipulador injetado e devolve resultado_acao.
"""

import json
import logging
import threading
import time

from websockets.exceptions import ConnectionClosed
from websockets.sync.client import connect as conectar_padrao

logger = logging.getLogger(__name__)


class ConexaoJames:
    def __init__(
        self,
        url,
        manipulador_acao,
        intervalo_heartbeat=15.0,
        intervalo_reconexao=2.0,
        conectar=conectar_padrao,
    ):
        self.url = url
        self.manipulador_acao = manipulador_acao
        self.intervalo_heartbeat = intervalo_heartbeat
        self.intervalo_reconexao = intervalo_reconexao
        self._conectar = conectar
        self._parar = threading.Event()

    def parar(self):
        self._parar.set()

    def executar(self):
        """Loop principal: conecta, processa mensagens, reconecta se cair.
        Só retorna depois de parar() ser chamado."""
        while not self._parar.is_set():
            try:
                with self._conectar(self.url) as ws:
                    logger.info("conectado ao jamesd em %s", self.url)
                    self._sessao(ws)
            except (OSError, ConnectionClosed) as erro:
                logger.info(
                    "conexão com jamesd caiu (%s), tentando de novo em %ss", erro, self.intervalo_reconexao
                )
            if not self._parar.is_set():
                time.sleep(self.intervalo_reconexao)

    def _sessao(self, ws):
        tarefa = threading.Thread(target=self._heartbeat_loop, args=(ws,), daemon=True)
        tarefa.start()
        for mensagem_bruta in ws:
            envelope = json.loads(mensagem_bruta)
            self._tratar(ws, envelope)

    def _heartbeat_loop(self, ws):
        while not self._parar.is_set():
            time.sleep(self.intervalo_heartbeat)
            try:
                ws.send(json.dumps({"tipo": "heartbeat", "id": "", "dados": {}}))
            except (OSError, ConnectionClosed):
                return

    def _tratar(self, ws, envelope):
        if envelope.get("tipo") != "acao_tela":
            return
        resultado = self.manipulador_acao(envelope.get("dados", {}))
        resposta = json.dumps({"tipo": "resultado_acao", "id": envelope.get("id", ""), "dados": resultado})
        ws.send(resposta)
