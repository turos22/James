import james_device.tela as tela
from james_device.superficies import Superficie

DISPLAY1 = Superficie(id="DISPLAY1", largura=1920, altura=1080, origem_x=0, origem_y=0, escala=1.0, primaria=True)
DISPLAY7 = Superficie(id="DISPLAY7", largura=1280, altura=720, origem_x=1920, origem_y=255, escala=1.0, primaria=False)


def _fixar_superficies(monkeypatch, superficies, alvo):
    """Tira o SO do caminho: tela.py passa a enxergar a geometria que o
    teste mandar, em vez dos monitores de verdade."""
    monkeypatch.setattr(tela.superficies, "listar", lambda: superficies)
    monkeypatch.setattr(tela.superficies, "retangulo_da_janela_em_foco", lambda: None)
    monkeypatch.setattr(tela.superficies, "alvo_padrao", lambda _s, _r: alvo)


def test_executar_acao_click_chama_pyautogui(monkeypatch):
    chamadas = []
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append(("moveTo", x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: chamadas.append(("click",)))

    resultado = tela.executar_acao({"acao": "click", "x": 100, "y": 200})

    assert resultado["saida"].startswith("cliquei em (100,200)")
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

    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela.pyautogui, "moveTo", levanta)

    resultado = tela.executar_acao({"acao": "click", "x": 1, "y": 1})
    assert "erro" in resultado
    assert "tela travada" in resultado["erro"]


def test_click_converte_para_coordenada_global(monkeypatch):
    chamadas = []
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY7)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)

    resultado = tela.executar_acao({"acao": "click", "x": 100, "y": 100})

    # (100,100) local no DISPLAY7 é (2020,355) global. Se esta soma sumir, o
    # clique cai no monitor primário e o teste falha aqui.
    assert chamadas == [(2020, 355)]
    assert "erro" not in resultado


def test_click_fora_dos_limites_devolve_erro_e_nao_clica(monkeypatch):
    chamadas = []
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY7)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: chamadas.append("click"))

    resultado = tela.executar_acao({"acao": "click", "x": 5000, "y": 100})

    assert "erro" in resultado
    assert "limites" in resultado["erro"]
    assert chamadas == []


def test_click_usa_a_superficie_pedida(monkeypatch):
    chamadas = []
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY1)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)

    tela.executar_acao({"acao": "click", "x": 10, "y": 20, "superficie": "DISPLAY7"})

    assert chamadas == [(1930, 275)]


def test_click_com_superficie_inexistente_devolve_erro(monkeypatch):
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY1)
    resultado = tela.executar_acao({"acao": "click", "x": 10, "y": 20, "superficie": "DISPLAY99"})
    assert "erro" in resultado
    assert "DISPLAY99" in resultado["erro"]
