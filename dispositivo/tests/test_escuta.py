"""Testes da escuta contínua: wake word offline + janela de comando.

Tudo injetado — fonte de áudio, reconhecedor e relógio. Nenhum teste aqui
toca microfone, rede ou Vosk de verdade: o que está sob teste é a máquina de
estados (dormindo, acordado, transcrevendo), não a acústica.
"""

import json

import pytest

from james_device import escuta


class ReconhecedorFalso:
    """Duble do KaldiRecognizer do Vosk. Cada chunk de áudio consome uma
    entrada de `roteiro`: ("parcial", texto) ou ("final", texto)."""

    def __init__(self, roteiro):
        self.roteiro = list(roteiro)
        self.resets = 0
        self._ultimo = ("parcial", "")

    def AcceptWaveform(self, chunk):  # noqa: N802 — nome vem da API do Vosk
        self._ultimo = self.roteiro.pop(0) if self.roteiro else ("parcial", "")
        return self._ultimo[0] == "final"

    def Result(self):  # noqa: N802
        return json.dumps({"text": self._ultimo[1]})

    def PartialResult(self):  # noqa: N802
        return json.dumps({"partial": self._ultimo[1]})

    def Reset(self):  # noqa: N802
        self.resets += 1


def relogio_de(valores):
    """Relógio determinístico: devolve os valores na ordem e depois repete o
    último — a janela nunca deve depender de quantas vezes o código consulta
    o relógio, só de o tempo ter passado."""
    restantes = list(valores)
    ultimo = [restantes[-1] if restantes else 0.0]

    def relogio():
        if restantes:
            ultimo[0] = restantes.pop(0)
        return ultimo[0]

    return relogio


def test_sem_wake_word_nao_manda_nada():
    comandos = []
    escuta.escutar(
        fonte_audio=[b"a", b"b", b"c"],
        reconhecedor=ReconhecedorFalso([("parcial", "abre o navegador"), ("final", "que horas sao"), ("parcial", "")]),
        transcrever_comando=lambda audio: "nao deveria ser chamado",
        ao_acordar=lambda: None,
        ao_comando=comandos.append,
    )
    assert comandos == []


def test_wake_word_em_resultado_parcial_acorda_na_hora():
    """Esperar o resultado final significa esperar o silêncio no fim da frase.
    O parcial chega enquanto a pessoa ainda fala — é o que torna a ativação
    instantânea em vez de depender de uma pausa."""
    acordou = []
    escuta.escutar(
        fonte_audio=[b"a", b"b"],
        reconhecedor=ReconhecedorFalso([("parcial", "james"), ("parcial", "")]),
        transcrever_comando=lambda audio: "",
        ao_acordar=lambda: acordou.append(True),
        ao_comando=lambda texto: None,
        janela_segundos=1.0,
        relogio=relogio_de([0.0, 5.0]),
    )
    assert acordou == [True]


def test_junta_audio_da_janela_e_manda_o_comando():
    recebidos = {}

    def transcrever(audio):
        recebidos["audio"] = audio
        return "abre o navegador"

    comandos = []
    escuta.escutar(
        fonte_audio=[b"wake", b"um", b"dois", b"tres", b"tarde demais"],
        reconhecedor=ReconhecedorFalso([("parcial", "james")]),
        transcrever_comando=transcrever,
        ao_acordar=lambda: None,
        ao_comando=comandos.append,
        janela_segundos=3.0,
        # 0.0 abre a janela; 1.0/2.0 cabem; 3.0 fecha.
        relogio=relogio_de([0.0, 1.0, 2.0, 3.0]),
    )

    assert comandos == ["abre o navegador"]
    # O chunk que carregava a wake word não entra: é a ativação, não o pedido.
    assert recebidos["audio"] == b"umdois"


def test_janela_fecha_por_tempo_sem_palavra_de_parada():
    """O modelo antigo esperava a palavra "enviar" num while True: sem ela, a
    escuta travava para sempre e nunca voltava a ouvir a wake word."""
    comandos = []
    escuta.escutar(
        fonte_audio=[b"wake", b"um", b"dois", b"tres"],
        reconhecedor=ReconhecedorFalso([("parcial", "james")]),
        transcrever_comando=lambda audio: "sem palavra de parada nenhuma",
        ao_acordar=lambda: None,
        ao_comando=comandos.append,
        janela_segundos=2.0,
        relogio=relogio_de([0.0, 1.0, 2.0]),
    )
    assert comandos == ["sem palavra de parada nenhuma"]


def test_transcricao_vazia_nao_vira_comando():
    """Acordar por engano acontece. Mandar string vazia pro jamesd viraria um
    turno do agente sobre nada."""
    comandos = []
    escuta.escutar(
        fonte_audio=[b"wake", b"um", b"dois"],
        reconhecedor=ReconhecedorFalso([("parcial", "james")]),
        transcrever_comando=lambda audio: "   ",
        ao_acordar=lambda: None,
        ao_comando=comandos.append,
        janela_segundos=1.0,
        relogio=relogio_de([0.0, 1.0]),
    )
    assert comandos == []


def test_falha_ao_transcrever_nao_derruba_a_escuta():
    """A transcrição do comando é a única parte que fala com a rede. Uma
    oscilação ali matava o processo inteiro do device — e com ele a ponte de
    tela, que não tem nada a ver com áudio."""
    tentativas = []

    def transcrever(audio):
        tentativas.append(audio)
        if len(tentativas) == 1:
            raise OSError("rede caiu no meio da transcrição")
        return "segundo comando"

    comandos = []
    escuta.escutar(
        fonte_audio=[b"wake", b"um", b"wake", b"dois"],
        reconhecedor=ReconhecedorFalso([("parcial", "james"), ("parcial", "james")]),
        transcrever_comando=transcrever,
        ao_acordar=lambda: None,
        ao_comando=comandos.append,
        janela_segundos=1.0,
        relogio=relogio_de([0.0, 1.0, 0.0, 1.0]),
    )

    assert len(tentativas) == 2, "a escuta parou no primeiro erro em vez de seguir"
    assert comandos == ["segundo comando"]


def test_reconhecedor_e_zerado_depois_de_acordar():
    """Sem reset, o texto que continha a wake word segue no buffer do Vosk e
    dispara de novo assim que a janela fecha — ativação em loop."""
    reconhecedor = ReconhecedorFalso([("parcial", "james")])
    escuta.escutar(
        fonte_audio=[b"wake", b"um"],
        reconhecedor=reconhecedor,
        transcrever_comando=lambda audio: "",
        ao_acordar=lambda: None,
        ao_comando=lambda texto: None,
        janela_segundos=1.0,
        relogio=relogio_de([0.0, 1.0]),
    )
    assert reconhecedor.resets >= 1


def test_falha_ao_avisar_que_acordou_nao_derruba_a_escuta():
    """ao_acordar toca um som. playsound levanta com arquivo ausente, codec
    faltando ou cwd errado — e isso ficava fora do try, matando o processo."""
    comandos = []
    escuta.escutar(
        fonte_audio=[b"wake", b"um"],
        reconhecedor=ReconhecedorFalso([("parcial", "james")]),
        transcrever_comando=lambda audio: "o comando mesmo assim",
        ao_acordar=lambda: (_ for _ in ()).throw(OSError("sem placa de som")),
        ao_comando=comandos.append,
        janela_segundos=1.0,
        relogio=relogio_de([0.0, 1.0]),
    )
    assert comandos == ["o comando mesmo assim"]


@pytest.mark.parametrize(
    "ouvido",
    ["james", "james abre o navegador", "ei james", "jeimes", "gemes", "jaimes", "jeims"],
)
def test_variantes_reais_acordam(ouvido):
    acordou = []
    escuta.escutar(
        fonte_audio=[b"a"],
        reconhecedor=ReconhecedorFalso([("parcial", ouvido)]),
        transcrever_comando=lambda audio: "",
        ao_acordar=lambda: acordou.append(True),
        ao_comando=lambda texto: None,
        janela_segundos=0.0,
        relogio=relogio_de([0.0]),
    )
    assert acordou == [True], f"{ouvido!r} não acordou o James"


# --- palavra de parada: fechar a janela na hora, em vez de esperar o tempo ---
#
# O "enviar" voltou a pedido do Arthur (2026-08-11) com um objetivo concreto:
# saber se o James está ouvindo. Sem ele, a única forma de descobrir era falar
# e esperar 8s no escuro.
#
# Volta com uma diferença que NÃO pode ser perdida: ele fecha a janela mais
# cedo, mas não é mais a única forma de fechá-la. O laço antigo era
# `while True` esperando a palavra, e sem ela a escuta travava para sempre e
# nunca voltava a ouvir a wake word. O tempo continua sendo o piso.


def test_palavra_de_parada_fecha_a_janela_antes_do_tempo():
    recebidos = {}

    def transcrever(audio):
        recebidos["audio"] = audio
        return "abre o navegador enviar"

    comandos = []
    escuta.escutar(
        fonte_audio=[b"wake", b"um", b"dois", b"tarde demais"],
        # 1 entrada pro chunk da wake word, depois uma por chunk da janela.
        reconhecedor=ReconhecedorFalso([
            ("parcial", "james"),
            ("parcial", "abre o navegador"),
            ("parcial", "abre o navegador enviar"),
        ]),
        transcrever_comando=transcrever,
        ao_acordar=lambda: None,
        ao_comando=comandos.append,
        janela_segundos=60.0,  # tempo de sobra: quem fecha aqui é a palavra
        relogio=relogio_de([0.0, 1.0, 2.0, 3.0]),
    )

    assert comandos == ["abre o navegador"], "a palavra de parada não devia sobrar no comando"
    assert recebidos["audio"] == b"umdois", "a janela não fechou no chunk em que a palavra foi ouvida"


def test_sem_palavra_de_parada_o_tempo_ainda_fecha():
    """A regressão que não pode voltar: janela que só fecha pela palavra trava
    a escuta para sempre se ela nunca vier."""
    comandos = []
    escuta.escutar(
        fonte_audio=[b"wake", b"um", b"dois", b"tres"],
        reconhecedor=ReconhecedorFalso([("parcial", "james")]),
        transcrever_comando=lambda audio: "nunca falei a palavra",
        ao_acordar=lambda: None,
        ao_comando=comandos.append,
        janela_segundos=2.0,
        relogio=relogio_de([0.0, 1.0, 2.0]),
    )
    assert comandos == ["nunca falei a palavra"]


def test_palavra_de_parada_sozinha_nao_vira_comando():
    comandos = []
    escuta.escutar(
        fonte_audio=[b"wake", b"um", b"dois"],
        reconhecedor=ReconhecedorFalso([("parcial", "james"), ("parcial", "enviar")]),
        transcrever_comando=lambda audio: "enviar",
        ao_acordar=lambda: None,
        ao_comando=comandos.append,
        janela_segundos=60.0,
        relogio=relogio_de([0.0, 1.0]),
    )
    assert comandos == []


def test_avisa_quando_a_janela_fecha():
    """O sinal sonoro que o Arthur precisa pra saber que foi ouvido — é o
    faaah.wav que o laço antigo tocava ao captar o comando."""
    avisos = []
    escuta.escutar(
        fonte_audio=[b"wake", b"um", b"dois"],
        reconhecedor=ReconhecedorFalso([("parcial", "james"), ("parcial", "enviar")]),
        transcrever_comando=lambda audio: "alguma coisa enviar",
        ao_acordar=lambda: None,
        ao_comando=lambda texto: None,
        ao_fechar_janela=lambda: avisos.append(True),
        janela_segundos=60.0,
        relogio=relogio_de([0.0, 1.0]),
    )
    assert avisos == [True]


def test_falha_no_aviso_de_fechamento_nao_derruba_a_escuta():
    comandos = []
    escuta.escutar(
        fonte_audio=[b"wake", b"um"],
        reconhecedor=ReconhecedorFalso([("parcial", "james")]),
        transcrever_comando=lambda audio: "o comando mesmo assim",
        ao_acordar=lambda: None,
        ao_comando=comandos.append,
        ao_fechar_janela=lambda: (_ for _ in ()).throw(OSError("sem placa de som")),
        janela_segundos=1.0,
        relogio=relogio_de([0.0, 1.0]),
    )
    assert comandos == ["o comando mesmo assim"]


def test_reconhecedor_zerado_depois_da_janela():
    """A janela alimenta o reconhecedor pra ouvir a palavra de parada. Sem
    zerar depois, o texto do COMANDO fica no buffer e o laço de wake word
    passa a avaliá-lo — um comando que por acaso contenha uma variante
    reativaria a escuta sozinha."""
    reconhecedor = ReconhecedorFalso([("parcial", "james"), ("parcial", "enviar")])
    escuta.escutar(
        fonte_audio=[b"wake", b"um"],
        reconhecedor=reconhecedor,
        transcrever_comando=lambda audio: "",
        ao_acordar=lambda: None,
        ao_comando=lambda texto: None,
        janela_segundos=60.0,
        relogio=relogio_de([0.0, 1.0]),
    )
    assert reconhecedor.resets >= 2, "faltou zerar depois da janela (só zerou ao acordar)"
