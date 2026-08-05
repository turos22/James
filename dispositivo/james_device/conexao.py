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
        self._ws_atual = None
        self._lock_ws_atual = threading.Lock()

    def parar(self):
        self._parar.set()
        # Sem isso, uma sessão conectada e ociosa (esperando recv() sem
        # nenhuma mensagem chegando) nunca percebe que devia parar — o
        # for/iter em _sessao só retorna quando o socket real gera alguma
        # atividade. Fechar a conexão ativa força esse retorno.
        with self._lock_ws_atual:
            if self._ws_atual is not None:
                try:
                    self._ws_atual.close()
                except Exception:
                    pass

    def executar(self):
        """Loop principal: conecta, processa mensagens, reconecta se cair.
        Só retorna depois de parar() ser chamado."""
        while not self._parar.is_set():
            try:
                with self._conectar(self.url) as ws:
                    logger.info("conectado ao jamesd em %s", self.url)
                    self._sessao(ws)
            except Exception as erro:
                # Exception genérica de propósito, não só (OSError,
                # ConnectionClosed): _sessao já isola erro por mensagem (ver
                # abaixo), mas isso é a rede de segurança final — nenhum erro
                # inesperado pode matar o loop de reconexão pra sempre, já
                # que "reconecta sozinho se cair" é a promessa central deste
                # módulo (achado do revisor de qualidade da Task 21).
                logger.info(
                    "conexão com jamesd caiu (%s), tentando de novo em %ss", erro, self.intervalo_reconexao
                )
            if not self._parar.is_set():
                time.sleep(self.intervalo_reconexao)

    def enviar_entrada_voz(self, texto: str) -> bool:
        """Manda entrada_voz pela conexão ativa. Devolve False sem levantar
        se não há conexão no momento — quem fala não trava esperando o
        jamesd voltar."""
        with self._lock_ws_atual:
            if self._ws_atual is None:
                logger.warning("sem conexão com jamesd, entrada de voz descartada: %r", texto)
                return False
            try:
                self._ws_atual.send(json.dumps({"tipo": "entrada_voz", "id": "", "dados": {"texto": texto}}))
                return True
            except (OSError, ConnectionClosed):
                logger.warning("falha ao enviar entrada de voz, conexão deve ter caído")
                return False

    def _sessao(self, ws):
        with self._lock_ws_atual:
            self._ws_atual = ws
        tarefa = threading.Thread(target=self._heartbeat_loop, args=(ws,), daemon=True)
        tarefa.start()
        try:
            for mensagem_bruta in ws:
                try:
                    envelope = json.loads(mensagem_bruta)
                    self._tratar(ws, envelope)
                except Exception:
                    # JSON malformado ou erro dentro do manipulador não pode
                    # derrubar a sessão inteira — só essa mensagem é
                    # descartada, a conexão continua de pé pra próxima.
                    logger.exception("erro ao tratar mensagem do jamesd, ignorando e seguindo na mesma sessão")
        finally:
            with self._lock_ws_atual:
                self._ws_atual = None

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
