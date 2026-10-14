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
        self.fechado = False

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def __iter__(self):
        yield from self._mensagens

    def send(self, mensagem):
        self.enviados.append(json.loads(mensagem))

    def close(self):
        self.fechado = True


class FakeWSBloqueante:
    """Simula um recv() bloqueado esperando mensagem — só o `for ws` de
    _sessao retorna quando close() é chamado, como o socket real faria ao
    ser fechado por fora enquanto está ocioso esperando dado."""

    def __init__(self):
        self._liberar = threading.Event()
        self.fechado = False

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def __iter__(self):
        self._liberar.wait()
        return
        yield  # nunca executado — só faz __iter__ ser um gerador

    def send(self, mensagem):
        pass

    def close(self):
        self.fechado = True
        self._liberar.set()


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


def test_mensagem_malformada_nao_derruba_a_sessao():
    chamadas = []

    def manipulador(dados):
        chamadas.append(dados)
        return {"saida": "ok"}

    envelope_bom = json.dumps({"tipo": "acao_tela", "id": "1", "dados": {"acao": "click"}})
    ws = FakeWS(mensagens=["isso não é json válido", envelope_bom])

    conexao = ConexaoJames("ws://localhost/ws/dispositivo", manipulador)
    conexao._sessao(ws)  # não deve levantar exceção por causa da 1ª mensagem

    assert chamadas == [{"acao": "click"}]
    assert ws.enviados[-1]["tipo"] == "resultado_acao"


def test_manipulador_com_bug_nao_derruba_a_sessao():
    def manipulador_com_bug(dados):
        raise ValueError("bug no manipulador")

    envelope_com_bug = json.dumps({"tipo": "acao_tela", "id": "1", "dados": {}})
    envelope_depois = json.dumps({"tipo": "ping", "id": "2", "dados": {}})
    ws = FakeWS(mensagens=[envelope_com_bug, envelope_depois])

    conexao = ConexaoJames("ws://localhost/ws/dispositivo", manipulador_com_bug)
    conexao._sessao(ws)  # não deve propagar o ValueError do manipulador

    # chegou até a 2ª mensagem (tipo desconhecido, ignorado) — prova que a
    # sessão continuou depois do erro na 1ª, em vez de morrer ali
    assert ws.enviados == []


def test_executar_nao_para_pra_sempre_se_sessao_levantar_erro_inesperado():
    chamadas = {"n": 0}

    def conectar_com_sessao_quebrada(url):
        chamadas["n"] += 1
        if chamadas["n"] == 1:
            # 1ª sessão: mensagem que quebra o parsing dentro do próprio
            # __iter__ do ws (não um erro tratável por _sessao) — simula um
            # erro inesperado que escapa da rede de segurança por mensagem.
            class WSQuebrado(FakeWS):
                def __iter__(self):
                    raise RuntimeError("erro inesperado de baixo nível")

            return WSQuebrado()
        return FakeWS(mensagens=[])

    conexao = ConexaoJames(
        "ws://localhost/ws/dispositivo",
        lambda dados: {},
        intervalo_reconexao=0.01,
        conectar=conectar_com_sessao_quebrada,
    )
    thread = threading.Thread(target=conexao.executar, daemon=True)
    thread.start()
    time.sleep(0.1)
    conexao.parar()
    thread.join(timeout=1)

    assert chamadas["n"] >= 2, "erro inesperado na 1ª sessão deveria ter permitido reconectar, não matar o loop"


def test_enviar_entrada_voz_manda_envelope_pela_conexao_ativa():
    ws = FakeWS(mensagens=[])
    conexao = ConexaoJames("ws://localhost/ws/dispositivo", lambda dados: {})
    conexao._ws_atual = ws

    enviado = conexao.enviar_entrada_voz("liga o spotify")

    assert enviado is True
    assert ws.enviados[-1]["tipo"] == "entrada_voz"
    assert ws.enviados[-1]["dados"] == {"texto": "liga o spotify"}


def test_enviar_entrada_voz_sem_conexao_devolve_false():
    conexao = ConexaoJames("ws://localhost/ws/dispositivo", lambda dados: {})
    assert conexao.enviar_entrada_voz("oi") is False


def test_parar_fecha_a_conexao_ativa_e_libera_sessao_ociosa_bloqueada():
    ws = FakeWSBloqueante()
    conexao = ConexaoJames("ws://localhost/ws/dispositivo", lambda dados: {})

    thread = threading.Thread(target=conexao._sessao, args=(ws,), daemon=True)
    thread.start()
    time.sleep(0.05)  # dá tempo da thread entrar no wait() bloqueado

    conexao.parar()
    thread.join(timeout=1)

    assert not thread.is_alive(), "parar() deveria ter liberado a sessão bloqueada em recv()"
    assert ws.fechado is True


# --- fala_frase / fala_fim / fala_erro (spec 2026-10-10 voz em tempo real) ---

def _fala_ws(*envelopes):
    return FakeWS(mensagens=[json.dumps(e) for e in envelopes])


def test_fala_frase_entrega_indice_e_texto_sem_responder_nada():
    ws = _fala_ws({"tipo": "fala_frase", "id": "v1", "dados": {"texto": "Olá, Arthur.", "indice": 3}})
    recebidas = []
    conexao = ConexaoJames("ws://x", lambda d: {}, ao_fala_frase=lambda i, t: recebidas.append((i, t)))

    conexao._sessao(ws)

    assert recebidas == [(3, "Olá, Arthur.")]
    assert ws.enviados == [], "fala não tem resposta pro jamesd"


def test_fala_fim_e_fala_erro_chegam_aos_callbacks():
    ws = _fala_ws(
        {"tipo": "fala_fim", "id": "v1", "dados": {}},
        {"tipo": "fala_erro", "id": "v2", "dados": {"mensagem": "não consegui completar esse turno"}},
    )
    eventos = []
    conexao = ConexaoJames(
        "ws://x", lambda d: {},
        ao_fala_fim=lambda: eventos.append("fim"),
        ao_fala_erro=lambda m: eventos.append(("erro", m)),
    )

    conexao._sessao(ws)

    assert eventos == ["fim", ("erro", "não consegui completar esse turno")]


def test_silencio_chega_ao_callback_com_os_segundos():
    ws = _fala_ws({"tipo": "silencio", "id": "", "dados": {"segundos": 6.5}})
    pedidos = []
    conexao = ConexaoJames("ws://x", lambda d: {}, ao_silencio=pedidos.append)

    conexao._sessao(ws)

    assert pedidos == [6.5]
    assert ws.enviados == []


def test_silencio_sem_callback_ou_malformado_nao_derruba():
    conexao = ConexaoJames("ws://x", lambda d: {})
    conexao._sessao(_fala_ws({"tipo": "silencio", "id": "", "dados": {"segundos": 3}}))

    pedidos = []
    conexao = ConexaoJames("ws://x", lambda d: {}, ao_silencio=pedidos.append)
    conexao._sessao(_fala_ws({"tipo": "silencio", "id": "", "dados": {"segundos": "muito"}},
                             {"tipo": "silencio", "id": "", "dados": None}))
    assert pedidos == []


def test_acao_de_tela_continua_funcionando_junto_com_a_fala():
    ws = _fala_ws(
        {"tipo": "fala_frase", "id": "v1", "dados": {"texto": "Vou ver.", "indice": 0}},
        {"tipo": "acao_tela", "id": "7", "dados": {"acao": "click"}},
    )
    conexao = ConexaoJames("ws://x", lambda d: {"saida": "ok"}, ao_fala_frase=lambda i, t: None)

    conexao._sessao(ws)

    assert [e["tipo"] for e in ws.enviados] == ["resultado_acao"]


def test_sem_callbacks_de_fala_o_device_ignora_a_fala():
    ws = _fala_ws({"tipo": "fala_frase", "id": "v1", "dados": {"texto": "oi", "indice": 0}},
                  {"tipo": "fala_fim", "id": "v1", "dados": {}})
    conexao = ConexaoJames("ws://x", lambda d: {})

    conexao._sessao(ws)  # não pode levantar

    assert ws.enviados == []


def test_fala_frase_malformada_e_ignorada_sem_chamar_o_callback(caplog):
    ws = _fala_ws({"tipo": "fala_frase", "id": "v1", "dados": {"indice": 1}},          # sem texto
                  {"tipo": "fala_frase", "id": "v1", "dados": {"texto": "x"}},          # sem índice
                  {"tipo": "fala_frase", "id": "v1", "dados": {"texto": 5, "indice": 0}})  # texto não é string
    recebidas = []
    conexao = ConexaoJames("ws://x", lambda d: {}, ao_fala_frase=lambda i, t: recebidas.append((i, t)))

    conexao._sessao(ws)

    assert recebidas == []


def test_callback_de_fala_com_bug_nao_derruba_a_sessao():
    ws = _fala_ws({"tipo": "fala_frase", "id": "v1", "dados": {"texto": "a", "indice": 0}},
                  {"tipo": "acao_tela", "id": "9", "dados": {}})

    def estoura(i, t):
        raise RuntimeError("bug no callback")

    conexao = ConexaoJames("ws://x", lambda d: {"saida": "ok"}, ao_fala_frase=estoura)

    conexao._sessao(ws)

    assert [e["tipo"] for e in ws.enviados] == ["resultado_acao"], "a mensagem seguinte tem de ser tratada"
