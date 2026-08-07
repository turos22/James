"""A separação entre enumerar (fala com o SO, não testável) e calcular
(aritmética pura, testável) é deliberada: o bug caro desta fase é o de
conversão de coordenada, e ele tem que estar coberto antes de qualquer
verificação ao vivo."""

from james_device.superficies import (
    Superficie,
    contendo,
    da_janela,
    para_global,
    para_local,
)

# Geometria real da máquina do Arthur, medida em 2026-08-07. O segundo
# monitor está desalinhado verticalmente em 255px, e é exatamente isso que
# faz a conversão errada passar despercebida numa tela e não na outra.
DISPLAY1 = Superficie(id="DISPLAY1", largura=1920, altura=1080, origem_x=0, origem_y=0, escala=1.0, primaria=True)
DISPLAY7 = Superficie(id="DISPLAY7", largura=1280, altura=720, origem_x=1920, origem_y=255, escala=1.0, primaria=False)
TODAS = [DISPLAY1, DISPLAY7]


def test_para_global_no_primario_e_identidade():
    assert para_global(DISPLAY1, 100, 200) == (100, 200)


def test_para_global_no_secundario_soma_a_origem():
    assert para_global(DISPLAY7, 100, 100) == (2020, 355)


def test_para_local_desfaz_para_global():
    assert para_local(DISPLAY7, 2020, 355) == (100, 100)


def test_contendo_acha_o_primario():
    assert contendo(TODAS, 100, 100) is DISPLAY1


def test_contendo_acha_o_secundario():
    assert contendo(TODAS, 2020, 355) is DISPLAY7


def test_contendo_devolve_none_na_area_morta():
    # (2000, 100) está dentro do retângulo que engloba os dois monitores,
    # mas acima do topo do DISPLAY7 (origem_y=255) e à direita do DISPLAY1.
    # É a área preta que fez D14 descartar capturar o desktop virtual.
    assert contendo(TODAS, 2000, 100) is None


def test_da_janela_usa_o_centro_do_retangulo():
    # Janela inteiramente no segundo monitor.
    assert da_janela(TODAS, (1950, 300, 3100, 900)) is DISPLAY7


def test_da_janela_atravessando_dois_monitores_escolhe_pelo_centro():
    # Janela de (1800,300) a (2200,700): centro em (2000,500), que cai no
    # DISPLAY7. Regra de R6 da spec — determinística, e conhecidamente
    # incompleta pra janela cortada ao meio.
    assert da_janela(TODAS, (1800, 300, 2200, 700)) is DISPLAY7


def test_da_janela_sem_superficie_no_centro_cai_no_primario():
    # Centro na área morta: precisa devolver algo utilizável, não None —
    # quem chama vai capturar uma tela de qualquer jeito.
    assert da_janela(TODAS, (1900, 50, 2100, 150)) is DISPLAY1
