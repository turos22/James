import base64

import james_device.tela as tela
from james_device.elementos import Elemento, Mapa
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


def test_click_sem_monitor_devolve_erro(monkeypatch):
    _fixar_superficies(monkeypatch, [], DISPLAY1)
    resultado = tela.executar_acao({"acao": "click", "x": 1, "y": 1})
    assert "nenhum monitor" in resultado["erro"]


def test_click_consulta_janela_em_foco_para_a_superficie_padrao(monkeypatch):
    recebido = {}

    def alvo_padrao_espiao(todas, retangulo):
        recebido["todas"] = todas
        recebido["retangulo"] = retangulo
        return DISPLAY1

    monkeypatch.setattr(tela.superficies, "listar", lambda: [DISPLAY1, DISPLAY7])
    monkeypatch.setattr(tela.superficies, "retangulo_da_janela_em_foco", lambda: (10, 20, 30, 40))
    monkeypatch.setattr(tela.superficies, "alvo_padrao", alvo_padrao_espiao)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: None)
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)

    tela.executar_acao({"acao": "click", "x": 1, "y": 1})

    assert recebido["todas"] == [DISPLAY1, DISPLAY7]
    assert recebido["retangulo"] == (10, 20, 30, 40)


def test_ver_tela_devolve_imagem_em_base64(monkeypatch):
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY1)
    monkeypatch.setattr(tela.captura, "capturar", lambda s: b"\x89PNG-falso")
    monkeypatch.setattr(tela, "_listar_elementos", lambda s: "e1  Button  \"Salvar\"  (820,455)")

    resultado = tela.executar_acao({"acao": "ver_tela"})

    assert "erro" not in resultado
    assert resultado["imagem_b64"] == base64.b64encode(b"\x89PNG-falso").decode()
    assert "Salvar" in resultado["saida"]
    assert "(primaria)" in resultado["saida"]


def test_ver_tela_declara_a_superficie_no_cabecalho(monkeypatch):
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY7)
    monkeypatch.setattr(tela.captura, "capturar", lambda s: b"png")
    monkeypatch.setattr(tela, "_listar_elementos", lambda s: "")

    resultado = tela.executar_acao({"acao": "ver_tela"})

    primeira_linha = resultado["saida"].splitlines()[0]
    assert "DISPLAY7" in primeira_linha
    assert "1280x720" in primeira_linha
    assert "(primaria)" not in primeira_linha


def test_ver_tela_sobrevive_a_falha_da_varredura(monkeypatch):
    # Captura e varredura falham por motivos diferentes: uma tela sem árvore
    # de acessibilidade ainda produz imagem útil, e o modelo precisa receber
    # a imagem mesmo assim.
    def explode(_s):
        raise RuntimeError("sem arvore UIA")

    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela.captura, "capturar", lambda s: b"png")
    monkeypatch.setattr(tela, "_listar_elementos", explode)

    resultado = tela.executar_acao({"acao": "ver_tela"})

    assert "erro" not in resultado
    assert resultado["imagem_b64"] != ""
    assert "sem arvore UIA" in resultado["saida"]


def _preparar_mapa(monkeypatch, superficie, itens, handle=123):
    monkeypatch.setattr(tela, "_mapa_atual", Mapa(superficie_id=superficie.id, janela_em_foco=handle, elementos=itens))
    monkeypatch.setattr(tela, "_handle_janela_em_foco", lambda: handle)


def test_clicar_elemento_por_id_acerta_o_centro(monkeypatch):
    chamadas = []
    salvar = Elemento(id="e7", nome="Salvar", tipo="Button", retangulo=(800, 440, 840, 470))
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    _preparar_mapa(monkeypatch, DISPLAY1, [salvar])
    monkeypatch.setattr(tela, "_revalidar", lambda e: None)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)

    resultado = tela.executar_acao({"acao": "clicar_elemento", "elemento_id": "e7"})

    assert chamadas == [(820, 455)]
    assert "Salvar" in resultado["saida"]


def test_clicar_elemento_sem_varredura_pede_ver_tela(monkeypatch):
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela, "_mapa_atual", None)

    resultado = tela.executar_acao({"acao": "clicar_elemento", "elemento_id": "e7"})

    assert "erro" in resultado
    assert "ver_tela" in resultado["erro"]


def test_clicar_elemento_com_tela_mudada_devolve_erro_e_nao_clica(monkeypatch):
    # Núcleo do CA6: a janela em foco mudou desde o ver_tela, então o mapa
    # não vale mais. Clicar aqui seria clicar no lugar errado com certeza
    # absoluta, que é o bug que esta fase inteira existe pra matar.
    #
    # _revalidar é forçada a aprovar de propósito: isola o que está sendo
    # testado (o valido_para) do resto da cadeia de checagens. Sem isso, se
    # _revalidar real recusasse por outro motivo, o teste passaria mesmo com
    # o valido_para quebrado — foi exatamente esse buraco que a revisão de
    # qualidade achou por mutação, testando isso.
    chamadas = []
    salvar = Elemento(id="e7", nome="Salvar", tipo="Button", retangulo=(800, 440, 840, 470))
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    _preparar_mapa(monkeypatch, DISPLAY1, [salvar], handle=123)
    monkeypatch.setattr(tela, "_handle_janela_em_foco", lambda: 999)
    monkeypatch.setattr(tela, "_revalidar", lambda e: None)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: chamadas.append("click"))

    resultado = tela.executar_acao({"acao": "clicar_elemento", "elemento_id": "e7"})

    assert "erro" in resultado
    assert "chame ver_tela de novo" in resultado["erro"]
    assert chamadas == []


def test_clicar_elemento_que_sumiu_da_arvore_devolve_erro(monkeypatch):
    chamadas = []
    salvar = Elemento(id="e7", nome="Salvar", tipo="Button", retangulo=(800, 440, 840, 470))
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    _preparar_mapa(monkeypatch, DISPLAY1, [salvar])
    monkeypatch.setattr(tela, "_revalidar", lambda e: "o elemento não está mais na tela")
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))

    resultado = tela.executar_acao({"acao": "clicar_elemento", "elemento_id": "e7"})

    assert "erro" in resultado
    assert chamadas == []


def test_clicar_elemento_por_nome_ambiguo_devolve_erro_util(monkeypatch):
    a = Elemento(id="e1", nome="OK", tipo="Button", retangulo=(0, 0, 10, 10))
    b = Elemento(id="e2", nome="OK", tipo="Button", retangulo=(20, 20, 30, 30))
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    _preparar_mapa(monkeypatch, DISPLAY1, [a, b])

    resultado = tela.executar_acao({"acao": "clicar_elemento", "nome": "OK"})

    assert "erro" in resultado
    assert "OK" in resultado["erro"]


def test_clicar_elemento_no_segundo_monitor_converte_coordenada(monkeypatch):
    # CA8 em forma de teste unitário: elemento com retângulo global no
    # DISPLAY7. Se a conversão sumir, o clique vai pro monitor errado.
    chamadas = []
    botao = Elemento(id="e1", nome="Play", tipo="Button", retangulo=(2000, 400, 2040, 430))
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY7)
    _preparar_mapa(monkeypatch, DISPLAY7, [botao])
    monkeypatch.setattr(tela, "_revalidar", lambda e: None)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)

    tela.executar_acao({"acao": "clicar_elemento", "elemento_id": "e1"})

    # O retângulo já é global, então o centro global é direto (2020,415).
    assert chamadas == [(2020, 415)]
