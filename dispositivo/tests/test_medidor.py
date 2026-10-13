"""O medidor de latência por turno (spec 2026-10-10, §6.6).

Grava SÓ números: é a voz do Arthur, e o tempo basta pra saber se o James está
rápido. A garantia de que o texto do comando nunca chega ao arquivo é
estrutural: marca() só aceita um conjunto fixo de nomes de marco."""

import logging

import pytest

from james_device.medidor import CABECALHO, Medidor


def linhas(caminho):
    return caminho.read_text(encoding="utf-8").splitlines()


def turno_completo(m, base=10.0):
    m.marca("fim_fala", em=base)
    m.marca("stt_pronto", em=base + 0.6)
    m.marca("entrada_enviada", em=base + 0.8)
    m.marca("primeira_frase", em=base + 1.7)
    m.marca("primeiro_audio", em=base + 1.75)


def test_turno_completo_grava_os_intervalos_em_segundos(tmp_path):
    arquivo = tmp_path / "latencia.tsv"
    m = Medidor(arquivo)

    turno_completo(m)

    cabecalho, linha = linhas(arquivo)
    assert cabecalho == CABECALHO
    campos = dict(zip(CABECALHO.split("\t"), linha.split("\t")))
    assert campos["turno"] == "1"
    assert campos["total_s"] == "1.750", "fim da fala -> primeiro som"
    assert campos["stt_s"] == "0.600", "começo do silêncio -> texto pronto"
    assert campos["jamesd_s"] == "0.900", "comando enviado -> primeira frase recebida"
    assert campos["tts_s"] == "0.050", "frase recebida -> primeiro áudio tocando"


def test_cabecalho_uma_vez_so_e_turnos_numerados(tmp_path):
    arquivo = tmp_path / "latencia.tsv"
    m = Medidor(arquivo)

    turno_completo(m, 10.0)
    turno_completo(m, 20.0)

    todas = linhas(arquivo)
    assert todas.count(CABECALHO) == 1
    assert [l.split("\t")[1] for l in todas[1:]] == ["1", "2"]


def test_cria_a_pasta_que_nao_existe(tmp_path):
    arquivo = tmp_path / "ainda" / "nao" / "existe" / "latencia.tsv"
    turno_completo(Medidor(arquivo))
    assert arquivo.exists()


def test_turno_com_erro_grava_com_campos_vazios(tmp_path):
    arquivo = tmp_path / "latencia.tsv"
    m = Medidor(arquivo)

    m.marca("fim_fala", em=10.0)
    m.marca("stt_pronto", em=10.6)
    m.marca("entrada_enviada", em=10.8)
    m.marca("erro", em=11.0)

    campos = dict(zip(CABECALHO.split("\t"), linhas(arquivo)[1].split("\t")))
    assert campos["stt_s"] == "0.600"
    assert campos["total_s"] == "" and campos["jamesd_s"] == "" and campos["tts_s"] == ""


def test_pausa_no_meio_da_fala_so_vale_o_ultimo_fim_de_fala(tmp_path):
    arquivo = tmp_path / "latencia.tsv"
    m = Medidor(arquivo)

    m.marca("fim_fala", em=10.0)   # pausa que não era o fim
    m.marca("fim_fala", em=12.0)   # o fim de verdade
    m.marca("stt_pronto", em=12.5)
    m.marca("entrada_enviada", em=12.7)
    m.marca("primeira_frase", em=13.5)
    m.marca("primeiro_audio", em=13.6)

    assert len(linhas(arquivo)) == 2, "a pausa abriu um turno à parte"
    campos = dict(zip(CABECALHO.split("\t"), linhas(arquivo)[1].split("\t")))
    assert campos["total_s"] == "1.600"


def test_novo_fim_de_fala_depois_do_stt_fecha_o_turno_anterior(tmp_path):
    # Foi o caso do "não entendi": houve fala e STT, mas o comando nunca saiu.
    arquivo = tmp_path / "latencia.tsv"
    m = Medidor(arquivo)

    m.marca("fim_fala", em=10.0)
    m.marca("stt_pronto", em=10.6)
    m.marca("fim_fala", em=20.0)  # outro turno

    assert len(linhas(arquivo)) == 2, "o turno sem desfecho deveria ter sido gravado"


def test_marco_desconhecido_e_recusado_e_texto_nunca_chega_ao_arquivo(tmp_path):
    arquivo = tmp_path / "latencia.tsv"
    m = Medidor(arquivo)

    with pytest.raises(ValueError):
        m.marca("abre o terminal e apaga tudo")
    turno_completo(m)

    assert "terminal" not in arquivo.read_text(encoding="utf-8")


def test_falha_de_escrita_nao_derruba_o_fluxo_de_voz(tmp_path, caplog):
    m = Medidor(tmp_path)  # um DIRETÓRIO no lugar do arquivo: abrir pra escrever falha

    with caplog.at_level(logging.ERROR):
        turno_completo(m)  # não pode levantar

    assert any("latência" in r.message for r in caplog.records)


def test_loga_uma_linha_por_turno(tmp_path, caplog):
    m = Medidor(tmp_path / "latencia.tsv")

    with caplog.at_level(logging.INFO):
        turno_completo(m)

    msgs = [r.message for r in caplog.records if "latencia" in r.message]
    assert len(msgs) == 1
    assert "turno=1" in msgs[0] and "1.75" in msgs[0] and "stt=0.60" in msgs[0]


def test_usa_o_relogio_injetado_quando_nao_ha_instante(tmp_path):
    t = [100.0]
    m = Medidor(tmp_path / "latencia.tsv", agora=lambda: t[0])

    m.marca("fim_fala")
    t[0] = 101.5
    m.marca("primeiro_audio")

    campos = dict(zip(CABECALHO.split("\t"), linhas(tmp_path / "latencia.tsv")[1].split("\t")))
    assert campos["total_s"] == "1.500"
