"""Captura de um monitor pelo ScreenCast do portal (D9 da spec 2026-10-13 B2).

Um quadro do fluxo PipeWire que a sessão do portal já tem: 21–39 ms no
spike, sem diálogo a mais. O `spectacle` levou 0,6 s e devolveu os dois
monitores juntos em 6684x2776.

A imagem sai reduzida para o tamanho que o modelo recebe (1280 de largura),
porque a API reduziria sozinha e o espaço de coordenada deixaria de bater
(superficies.tamanho_da_imagem). PNG, como no Windows (captura.py: JPEG borra
texto pequeno).

A imagem vai direto para a memória (appsink): a tela do Arthur não toca o
disco.
"""

import os
import threading

import gi

gi.require_version("Gst", "1.0")
from gi.repository import Gst  # noqa: E402

TIMEOUT_CAPTURA = 5.0
_iniciado = False
_trava_das_travas = threading.Lock()
_travas_por_fluxo: dict[int, threading.Lock] = {}


def _iniciar() -> None:
    global _iniciado
    if not _iniciado:
        Gst.init(None)
        _iniciado = True


def montar_pipeline(fd: int, node: int, largura: int, altura: int) -> str:
    # num-buffers=2: o primeiro quadro de um fluxo recém-conectado pode vir
    # antes do primeiro dano da tela; o pngenc em snapshot fica com um e
    # manda EOS.
    return (
        f"pipewiresrc fd={fd} path={node} num-buffers=2 always-copy=true ! "
        f"videoconvert ! videoscale ! video/x-raw,format=RGB,width={largura},height={altura} ! "
        f"pngenc snapshot=true ! appsink name=saida sync=false max-buffers=1"
    )


def capturar(fd: int, node: int, largura: int, altura: int, timeout: float = TIMEOUT_CAPTURA) -> bytes:
    """PNG de um fluxo, ou RuntimeError dentro do prazo.

    O prazo é obrigatório: com um descritor que não responde (sessão do
    portal fechada, PipeWire reiniciado), o set_state(PLAYING) do pipewiresrc
    TRAVA esperando conectar, e travaria a ponte de tela inteira. A captura
    roda numa thread; estourado o prazo, quem chamou recebe o erro na hora e
    a thread presa fica para trás. Uma captura presa trava só o fluxo dela:
    o próximo pedido no mesmo fluxo é recusado na hora (sem empilhar threads),
    e depois que o portal reabre os fluxos são outros.

    O descritor continua de quem chamou; o pipewiresrc recebe uma cópia."""
    with _trava_das_travas:
        trava = _travas_por_fluxo.setdefault(node, threading.Lock())
    if not trava.acquire(blocking=False):
        raise RuntimeError(f"captura falhou: uma captura anterior ainda está presa no fluxo {node}")

    copia = os.dup(fd)
    caixa: dict = {}
    feito = threading.Event()

    def trabalhar() -> None:
        try:
            caixa["png"] = _capturar(copia, node, largura, altura, timeout)
        except BaseException as erro:  # noqa: BLE001 — repassada a quem chamou
            caixa["erro"] = erro
        finally:
            try:
                os.close(copia)
            except OSError:
                pass  # o pipewiresrc pode já ter fechado a cópia
            feito.set()
            trava.release()

    threading.Thread(target=trabalhar, name=f"captura-{node}", daemon=True).start()
    if not feito.wait(timeout):
        raise RuntimeError(f"captura falhou: o PipeWire não respondeu em {timeout}s (fluxo {node})")
    if "erro" in caixa:
        raise caixa["erro"]
    return caixa["png"]


def _capturar(fd: int, node: int, largura: int, altura: int, timeout: float) -> bytes:
    _iniciar()
    pipeline = Gst.parse_launch(montar_pipeline(fd, node, largura, altura))
    try:
        pipeline.set_state(Gst.State.PLAYING)
        amostra = pipeline.get_by_name("saida").emit("try-pull-sample", int(timeout * Gst.SECOND))
        if amostra is None:
            mensagem = pipeline.get_bus().pop_filtered(Gst.MessageType.ERROR)
            if mensagem is not None:
                erro, detalhe = mensagem.parse_error()
                raise RuntimeError(f"captura falhou: {erro.message} ({detalhe})")
            raise RuntimeError(f"captura falhou: nenhum quadro em {timeout}s")
        buffer = amostra.get_buffer()
        return buffer.extract_dup(0, buffer.get_size())
    finally:
        pipeline.set_state(Gst.State.NULL)
