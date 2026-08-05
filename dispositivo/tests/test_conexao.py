import json
import threading
import time

from james_device.conexao import ConexaoJames


class FakeWS:
    """Simula o objeto que `websockets.sync.client.connect` devolve: context
    manager, iterável de mensagens recebidas, e `.send()` que grava o que
    foi mandado."""

    def __init__(self, mensagens=None):
        self._mensagens = list(mensagens or [])
        self.enviados = []

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def __iter__(self):
        yield from self._mensagens

    def send(self, mensagem):
        self.enviados.append(json.loads(mensagem))


def test_tratar_acao_tela_chama_manipulador_e_envia_resultado():
    envelope_entrada = json.dumps({"tipo": "acao_tela", "id": "42", "dados": {"acao": "click", "x": 10, "y": 20}})
    ws = FakeWS(mensagens=[envelope_entrada])

    chamadas = []

    def manipulador(dados):
        chamadas.append(dados)
        return {"saida": "clique feito"}

    conexao = ConexaoJames("ws://localhost/ws/dispositivo", manipulador)
    conexao._sessao(ws)

    assert chamadas == [{"acao": "click", "x": 10, "y": 20}]
    assert len(ws.enviados) == 1
    assert ws.enviados[0]["tipo"] == "resultado_acao"
    assert ws.enviados[0]["id"] == "42"
    assert ws.enviados[0]["dados"] == {"saida": "clique feito"}


def test_ignora_envelope_de_tipo_desconhecido():
    envelope_entrada = json.dumps({"tipo": "ping", "id": "1", "dados": {}})
    ws = FakeWS(mensagens=[envelope_entrada])

    conexao = ConexaoJames("ws://localhost/ws/dispositivo", lambda dados: {"saida": "não devia rodar"})
    conexao._sessao(ws)

    assert ws.enviados == []


def test_executar_reconecta_apos_queda_de_conexao():
    chamadas = {"n": 0}

    def conectar_falso(url):
        chamadas["n"] += 1
        if chamadas["n"] == 1:
            raise OSError("recusado")
        return FakeWS(mensagens=[])

    conexao = ConexaoJames(
        "ws://localhost/ws/dispositivo", lambda dados: {}, intervalo_reconexao=0.01, conectar=conectar_falso
    )
    thread = threading.Thread(target=conexao.executar, daemon=True)
    thread.start()
    time.sleep(0.1)
    conexao.parar()
    thread.join(timeout=1)

    assert chamadas["n"] >= 2, "esperava pelo menos uma reconexão após a primeira falha"


def test_heartbeat_enviado_periodicamente():
    ws = FakeWS(mensagens=[])
    conexao = ConexaoJames("ws://localhost/ws/dispositivo", lambda dados: {}, intervalo_heartbeat=0.01)

    thread = threading.Thread(target=conexao._heartbeat_loop, args=(ws,), daemon=True)
    thread.start()
    time.sleep(0.05)
    conexao.parar()
    thread.join(timeout=1)

    assert len(ws.enviados) >= 1
    assert ws.enviados[0]["tipo"] == "heartbeat"
