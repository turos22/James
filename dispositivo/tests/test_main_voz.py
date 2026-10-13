"""A fiação da voz no main: o que acontece quando um comando sai, e a leitura
da configuração, que tem de falhar no BOOT e não no meio de uma fala."""

import pytest

from james_device.fala import FRASE_SEM_SERVIDOR
from james_device.main import (
    bloco_do_ambiente,
    configurar,
    continuacao_do_ambiente,
    criar_ao_comando,
    guarda_eco_do_ambiente,
)


class FalaFalsa:
    def __init__(self, eventos):
        self.eventos = eventos

    def comando_enviado(self):
        self.eventos.append("comando_enviado")

    def erro(self, mensagem):
        self.eventos.append(("erro", mensagem))


class ConexaoFalsa:
    def __init__(self, eventos, conectada=True):
        self.eventos, self.conectada = eventos, conectada

    def enviar_entrada_voz(self, texto):
        self.eventos.append(("enviar", texto))
        return self.conectada


class MedidorFalso:
    def __init__(self, eventos):
        self.eventos = eventos

    def marca(self, nome, em=None):
        self.eventos.append(("marca", nome))


def test_o_device_fica_ocupado_ANTES_de_o_comando_sair():
    # Se a resposta chegasse antes do ocupado, a primeira frase tocaria com a
    # escuta ainda aberta.
    eventos = []
    ao_comando = criar_ao_comando(FalaFalsa(eventos), ConexaoFalsa(eventos), MedidorFalso(eventos))

    ao_comando("abre o navegador")

    assert eventos == ["comando_enviado", ("enviar", "abre o navegador"), ("marca", "entrada_enviada")]


def test_jamesd_fora_do_ar_faz_o_james_dizer_isso_localmente():
    eventos = []
    ao_comando = criar_ao_comando(FalaFalsa(eventos), ConexaoFalsa(eventos, conectada=False), MedidorFalso(eventos))

    ao_comando("abre o navegador")

    assert ("erro", FRASE_SEM_SERVIDOR) in eventos
    assert FRASE_SEM_SERVIDOR == "Não consegui falar com o servidor."
    assert ("marca", "entrada_enviada") not in eventos, "nada saiu, não há instante de envio pra medir"


def test_sem_fala_o_comando_so_e_enviado():
    eventos = []
    ao_comando = criar_ao_comando(None, ConexaoFalsa(eventos), None)

    ao_comando("oi")

    assert eventos == [("enviar", "oi")]


# ---------- ambiente ----------

def test_continuacao_padrao_e_6_segundos_e_zero_desliga():
    assert continuacao_do_ambiente({}) == 6.0
    assert continuacao_do_ambiente({"JAMES_CONTINUACAO_S": "0"}) == 0.0
    assert continuacao_do_ambiente({"JAMES_CONTINUACAO_S": "3.5"}) == 3.5


@pytest.mark.parametrize("valor", ["-1", "seis", ""])
def test_continuacao_invalida_falha_nomeando_a_variavel(valor):
    if valor == "":
        assert continuacao_do_ambiente({"JAMES_CONTINUACAO_S": valor}) == 6.0  # vazio = padrão
        return
    with pytest.raises(ValueError, match="JAMES_CONTINUACAO_S"):
        continuacao_do_ambiente({"JAMES_CONTINUACAO_S": valor})


def test_guarda_de_eco_em_milissegundos():
    assert guarda_eco_do_ambiente({}) == pytest.approx(0.4)
    assert guarda_eco_do_ambiente({"JAMES_GUARDA_ECO_MS": "250"}) == pytest.approx(0.25)
    assert guarda_eco_do_ambiente({"JAMES_GUARDA_ECO_MS": "0"}) == 0.0
    with pytest.raises(ValueError, match="JAMES_GUARDA_ECO_MS"):
        guarda_eco_do_ambiente({"JAMES_GUARDA_ECO_MS": "-5"})


def test_bloco_do_microfone_padrao_e_100ms():
    # 0,1 s: blocos menores deixam o fim de fala fechar no tempo certo
    # (com 0,25 s a espera de 0,7 s só fecharia em 0,75 s a 1,0 s).
    assert bloco_do_ambiente({}) == 1600
    assert bloco_do_ambiente({"JAMES_BLOCO_MS": "250"}) == 4000
    with pytest.raises(ValueError, match="JAMES_BLOCO_MS"):
        bloco_do_ambiente({"JAMES_BLOCO_MS": "10"})  # menor que um quadro do VAD (32 ms) não faz sentido


def test_configurar_junta_tudo_e_falha_no_boot_se_algo_estiver_errado():
    cfg = configurar({})
    assert callable(cfg.transcrever)
    assert cfg.parametros_fim.comando_max_s == 10.0
    assert cfg.continuacao_s == 6.0

    with pytest.raises(ValueError, match="JAMES_STT"):
        configurar({"JAMES_STT": "deepgram"})
    with pytest.raises(ValueError, match="-ac"):
        configurar({"JAMES_COMANDO_MAX_S": "15"})
    # com o ac certo, 15 s cabe
    assert configurar({"JAMES_COMANDO_MAX_S": "15", "JAMES_WHISPER_AC": "768"}).parametros_fim.comando_max_s == 15.0
