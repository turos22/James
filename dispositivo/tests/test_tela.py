import james_device.tela as tela


def test_executar_acao_click_chama_pyautogui(monkeypatch):
    chamadas = []
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append(("moveTo", x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: chamadas.append(("click",)))

    resultado = tela.executar_acao({"acao": "click", "x": 100, "y": 200})

    assert resultado == {"saida": "clique em (100,200)"}
    assert ("moveTo", 100, 200) in chamadas
    assert ("click",) in chamadas


def test_executar_acao_tecla_chama_pyautogui(monkeypatch):
    chamadas = []
    monkeypatch.setattr(tela.pyautogui, "press", lambda t: chamadas.append(t))

    resultado = tela.executar_acao({"acao": "tecla", "texto": "enter"})

    assert resultado == {"saida": "tecla enter pressionada"}
    assert chamadas == ["enter"]


def test_executar_acao_desconhecida_devolve_erro():
    resultado = tela.executar_acao({"acao": "voar"})
    assert "erro" in resultado


def test_executar_acao_propaga_excecao_como_erro(monkeypatch):
    def levanta(*a, **k):
        raise RuntimeError("tela travada")

    monkeypatch.setattr(tela.pyautogui, "moveTo", levanta)

    resultado = tela.executar_acao({"acao": "click", "x": 1, "y": 1})
    assert "erro" in resultado
    assert "tela travada" in resultado["erro"]
