"""A árvore UIA de verdade depende de tela; aqui ela é injetada. O que está
sendo testado é a lógica em cima da árvore — filtro, formatação, ids — não a
biblioteca."""

from james_device.elementos import (
    MAXIMO_ELEMENTOS_FORMATADOS,
    Elemento,
    Mapa,
    formatar,
)
from james_device.superficies import Superficie

DISPLAY1 = Superficie(id="DISPLAY1", largura=1920, altura=1080, origem_x=0, origem_y=0, escala=1.0, primaria=True)


def elemento(identificador, nome, tipo, esquerda, topo, direita, base):
    return Elemento(id=identificador, nome=nome, tipo=tipo, retangulo=(esquerda, topo, direita, base))


def test_formatar_uma_linha_por_elemento():
    elementos = [
        elemento("e1", "Salvar", "Button", 800, 440, 840, 470),
        elemento("e2", "Nome do arquivo", "Edit", 600, 400, 680, 420),
    ]

    linhas = formatar(elementos).splitlines()

    assert len(linhas) == 2
    assert "e1" in linhas[0]
    assert "Button" in linhas[0]
    assert "Salvar" in linhas[0]
    # Centro do retângulo, não o canto: (800+840)/2, (440+470)/2
    assert "(820,455)" in linhas[0]


def test_formatar_mantem_elemento_sem_nome():
    # Botão só de ícone. É o caso que torna o id necessário: sem ele, esse
    # elemento seria inendereçável.
    linhas = formatar([elemento("e3", "", "Button", 1170, 50, 1190, 70)]).splitlines()
    assert "e3" in linhas[0]
    assert '""' in linhas[0]


def test_formatar_com_superficie_usa_coordenada_local():
    # Produção sempre passa a superfície (tela.py); este teste garante que
    # esse caminho — não só o default sem superfície — está coberto.
    display7 = Superficie(id="DISPLAY7", largura=1280, altura=720, origem_x=1920, origem_y=255, escala=1.0, primaria=False)
    linhas = formatar([elemento("e1", "Salvar", "Button", 2000, 400, 2040, 430)], display7).splitlines()
    assert "(100,160)" in linhas[0]


def test_mapa_resolve_por_id():
    e = elemento("e7", "Salvar", "Button", 800, 440, 840, 470)
    mapa = Mapa(superficie_id="DISPLAY1", janela_em_foco=123, elementos=[e])
    assert mapa.por_id("e7") is e
    assert mapa.por_id("e99") is None


def test_mapa_resolve_por_nome_e_tipo():
    salvar = elemento("e1", "Salvar", "Button", 0, 0, 10, 10)
    outro = elemento("e2", "Salvar", "MenuItem", 20, 20, 30, 30)
    mapa = Mapa(superficie_id="DISPLAY1", janela_em_foco=123, elementos=[salvar, outro])

    assert mapa.por_nome("Salvar", "Button") is salvar
    assert mapa.por_nome("Salvar", "MenuItem") is outro


def test_mapa_por_nome_ambiguo_devolve_none():
    # Dois botões "OK" na mesma tela: preferir errar a chutar um deles.
    a = elemento("e1", "OK", "Button", 0, 0, 10, 10)
    b = elemento("e2", "OK", "Button", 20, 20, 30, 30)
    mapa = Mapa(superficie_id="DISPLAY1", janela_em_foco=123, elementos=[a, b])

    assert mapa.por_nome("OK", "Button") is None


def test_mapa_valido_so_para_a_mesma_superficie_e_janela():
    mapa = Mapa(superficie_id="DISPLAY1", janela_em_foco=123, elementos=[])

    assert mapa.valido_para("DISPLAY1", 123)
    assert not mapa.valido_para("DISPLAY7", 123)
    assert not mapa.valido_para("DISPLAY1", 999)


def test_centro_local_converte_do_global():
    # A árvore UIA dá retângulo em coordenada global; a lista mostra local.
    e = elemento("e1", "Salvar", "Button", 2000, 400, 2040, 430)
    display7 = Superficie(id="DISPLAY7", largura=1280, altura=720, origem_x=1920, origem_y=255, escala=1.0, primaria=False)
    assert e.centro_local(display7) == (100, 160)


def test_formatar_colapsa_quebra_de_linha_no_nome():
    # Achado ao vivo (Task 15, CA4): controles reais de bandeja (relógio,
    # rede, microfone) têm .Name com \n/\r\n de tooltip embutido.
    el = elemento("e1", "Relógio 02:14\n‎08/‎08/2026", "ButtonControl", 0, 0, 10, 10)
    linhas = formatar([el]).splitlines()
    assert len(linhas) == 1
    assert "\n" not in linhas[0]


def test_formatar_nao_trunca_abaixo_do_teto():
    elementos = [elemento(f"e{i}", f"item{i}", "Button", 0, 0, 10, 10) for i in range(MAXIMO_ELEMENTOS_FORMATADOS)]
    linhas = formatar(elementos).splitlines()
    assert len(linhas) == MAXIMO_ELEMENTOS_FORMATADOS
    assert "..." not in linhas[-1]


def test_formatar_trunca_e_avisa_quantos_ficaram_de_fora():
    # Achado ao vivo na Task 15 (CA4): um desktop real produz 200+
    # elementos, dominado pela barra de tarefas, não pela janela em foco.
    total = MAXIMO_ELEMENTOS_FORMATADOS + 15
    elementos = [elemento(f"e{i}", f"item{i}", "Button", 0, 0, 10, 10) for i in range(total)]
    linhas = formatar(elementos).splitlines()

    assert len(linhas) == MAXIMO_ELEMENTOS_FORMATADOS + 1
    assert "15 elementos" in linhas[-1]
    # O elemento truncado ainda existe no Mapa — só não aparece formatado.
    # Quem cortou foi formatar(), não varrer()/Mapa.
    mapa = Mapa(superficie_id="DISPLAY1", janela_em_foco=1, elementos=elementos)
    assert mapa.por_id(f"e{total - 1}") is not None


def elemento_em_foco(identificador, nome, tipo, esquerda, topo, direita, base):
    return Elemento(
        id=identificador, nome=nome, tipo=tipo,
        retangulo=(esquerda, topo, direita, base), em_foco=True,
    )


def test_janela_em_foco_vem_antes_na_lista():
    """D14 manda varrer TODAS as janelas visíveis da superfície, e a barra de
    tarefas do Windows sozinha produz dezenas de controles. Em ordem de árvore
    ela costuma vir primeiro, então a janela que o Arthur está de fato usando
    aparecia depois — e, num desktop cheio, depois do corte."""
    elementos = [
        elemento("e1", "Iniciar", "ButtonControl", 0, 1040, 40, 1080),
        elemento("e2", "Pesquisar", "EditControl", 50, 1040, 300, 1080),
        elemento_em_foco("e3", "Salvar", "ButtonControl", 800, 440, 840, 470),
    ]

    linhas = formatar(elementos, DISPLAY1).split("\n")

    assert '"Salvar"' in linhas[0], f"a janela em foco não veio primeiro: {linhas[0]!r}"


def test_ordem_relativa_preservada_dentro_de_cada_grupo():
    elementos = [
        elemento("e1", "fundo A", "ButtonControl", 0, 0, 10, 10),
        elemento_em_foco("e2", "foco A", "ButtonControl", 20, 20, 30, 30),
        elemento("e3", "fundo B", "ButtonControl", 40, 40, 50, 50),
        elemento_em_foco("e4", "foco B", "ButtonControl", 60, 60, 70, 70),
    ]

    linhas = formatar(elementos, DISPLAY1).split("\n")
    nomes = [l.split('"')[1] for l in linhas]

    assert nomes == ["foco A", "foco B", "fundo A", "fundo B"]


def test_corte_nao_sacrifica_a_janela_em_foco():
    """O caso real: desktop com apps abertos passa de 200 elementos, quase
    todos de barra de tarefas e janelas de fundo. Com o corte em ordem de
    árvore, a janela em foco podia não sobrar nenhum elemento na lista — e o
    modelo concluía que não havia nada clicável na tela."""
    fundo = [
        elemento(f"e{i}", f"fundo {i}", "ButtonControl", 0, 0, 10, 10)
        for i in range(MAXIMO_ELEMENTOS_FORMATADOS + 50)
    ]
    alvo = elemento_em_foco("alvo", "Salvar", "ButtonControl", 800, 440, 840, 470)

    saida = formatar(fundo + [alvo], DISPLAY1)

    assert '"Salvar"' in saida, "o elemento da janela em foco caiu no corte"
