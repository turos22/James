"""A separação entre enumerar (fala com o SO, não testável) e calcular
(aritmética pura, testável) é deliberada: o bug caro desta fase é o de
conversão de coordenada, e ele tem que estar coberto antes de qualquer
verificação ao vivo."""

from james_device.superficies import (
    Superficie,
    alvo_padrao,
    contendo,
    da_janela,
    para_global,
    para_local,
    por_id,
    primaria,
)

# Fixture sintética, escolhida para preservar a propriedade de área morta
# entre monitores desalinhados verticalmente — não é a geometria real desta
# máquina. A medição real (com DPI awareness ativa) é DISPLAY7 em
# 1920x1080 @ escala 1.5, na mesma origem (1920,255). Usar 1280x720 aqui
# não muda o que está sendo testado: é aritmética pura sobre origem e
# tamanho, e a área morta em (2000,100) continua fora dos dois monitores
# com qualquer um dos dois tamanhos.
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


def test_por_id_acha_pelo_identificador():
    assert por_id(TODAS, "DISPLAY7") is DISPLAY7


def test_por_id_nao_acha_devolve_none():
    assert por_id(TODAS, "DISPLAY99") is None


def test_alvo_padrao_sem_janela_em_foco_cai_na_primaria():
    assert alvo_padrao(TODAS, None) is DISPLAY1


def test_alvo_padrao_com_janela_em_foco_usa_da_janela():
    assert alvo_padrao(TODAS, (1950, 300, 3100, 900)) is DISPLAY7


def test_primaria_acha_quem_se_declara():
    assert primaria(TODAS) is DISPLAY1


def test_primaria_sem_ninguem_declarado_cai_no_primeiro():
    sem_declaracao = [
        Superficie(id="X", largura=800, altura=600, origem_x=0, origem_y=0, escala=1.0, primaria=False),
        Superficie(id="Y", largura=800, altura=600, origem_x=800, origem_y=0, escala=1.0, primaria=False),
    ]
    assert primaria(sem_declaracao) is sem_declaracao[0]


def test_primaria_com_lista_vazia_levanta_com_mensagem_util():
    import pytest

    with pytest.raises(RuntimeError, match="nenhuma superfície"):
        primaria([])


# --- fator: espaço da imagem → lógico (D9 da spec 2026-10-13 B2) -------------
#
# No Linux a coordenada do protocolo é o pixel da imagem que o modelo recebe
# (1280 de largura), e o desktop trabalha em lógico. Geometria real desta
# máquina no Fedora: HDMI-A-1 lógico 1920x1080 em (0,0); HDMI-A-2 lógico
# 1422x800 em (1920,588), escala 1,35.

from james_device.superficies import tamanho_da_imagem  # noqa: E402

HDMI1 = Superficie(id="HDMI-A-1", largura=1280, altura=720, origem_x=0, origem_y=0,
                   escala=1.0, primaria=True, fator=1920 / 1280)
HDMI2 = Superficie(id="HDMI-A-2", largura=1280, altura=720, origem_x=1920, origem_y=588,
                   escala=1.35, primaria=False, fator=1422 / 1280)


def test_fator_padrao_e_1_e_nao_muda_o_windows():
    assert DISPLAY1.fator == 1.0
    assert para_global(DISPLAY7, 100, 100) == (2020, 355)


def test_para_global_com_fator_escala_antes_de_somar_a_origem():
    assert para_global(HDMI1, 640, 360) == (960, 540)


def test_centro_da_imagem_do_monitor_escalado_vai_ao_centro_medido_no_spike():
    # Centro lógico de HDMI-A-2 medido pelo KWin no spike: (2631, 988).
    assert para_global(HDMI2, 640, 360) == (2631, 988)


def test_para_local_com_fator_desfaz_para_global():
    assert para_local(HDMI2, 2631, 988) == (640, 360)
    assert para_local(HDMI1, 960, 540) == (640, 360)


def test_para_global_arredonda_em_vez_de_truncar():
    # 1 px da imagem = 1,5 lógico: 1 → 1,5 → 2 (truncar daria 1).
    assert para_global(HDMI1, 1, 1) == (2, 2)


def test_ultimo_pixel_da_imagem_continua_dentro_do_monitor():
    x, y = para_global(HDMI2, 1279, 719)
    assert contendo([HDMI1, HDMI2], x, y) is HDMI2


def test_contendo_usa_a_extensao_logica_quando_ha_fator():
    # (1900, 1000) é lógico do HDMI-A-1; sem o fator, a largura 1280 da
    # imagem deixaria esse ponto de fora.
    assert contendo([HDMI1, HDMI2], 1900, 1000) is HDMI1
    assert contendo([HDMI1, HDMI2], 3300, 1380) is HDMI2
    assert contendo([HDMI1, HDMI2], 3342, 1000) is None


def test_tamanho_da_imagem_mantem_a_proporcao():
    assert tamanho_da_imagem(1920, 1080) == (1280, 720, 1.5)
    largura, altura, fator = tamanho_da_imagem(1422, 800)
    assert (largura, altura) == (1280, 720)
    assert abs(fator - 1422 / 1280) < 1e-9


def test_tamanho_da_imagem_nao_amplia_monitor_menor():
    assert tamanho_da_imagem(1024, 768) == (1024, 768, 1.0)
