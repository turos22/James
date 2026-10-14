"""Laço GLib numa thread própria (D12 da spec 2026-10-13 B2).

O portal, o KWin, o MPRIS e o AT-SPI respondem por D-Bus, e parte das
respostas chega como SINAL (o portal só devolve o resultado de um pedido pelo
sinal Request.Response). Sinal só é entregue se algum laço GLib estiver
girando no contexto em que a assinatura foi feita. A ponte de tela roda na
thread principal, presa no WebSocket; por isso o D-Bus ganha uma thread com
laço próprio, e TODA chamada D-Bus do device passa por `executar` (R9 da
spec): duas threads mexendo no mesmo contexto GLib é corrida certa.

Uma função que trava dentro do laço trava o laço junto (é uma thread só). O
timeout de `executar` protege QUEM CHAMOU, que recebe TimeoutError na hora;
o laço volta a atender quando a função lenta termina.
"""

import logging
import threading

from gi.repository import GLib

logger = logging.getLogger(__name__)

TIMEOUT_PADRAO = 5.0


class LacoGlib:
    def __init__(self) -> None:
        self._contexto = GLib.MainContext.new()
        self._laco = GLib.MainLoop.new(self._contexto, False)
        self._thread: threading.Thread | None = None
        self._pronto = threading.Event()
        self._encerrado = False

    @property
    def contexto(self):
        return self._contexto

    @property
    def ident(self) -> int | None:
        return self._thread.ident if self._thread else None

    def iniciar(self) -> None:
        if self._thread is not None:
            return
        self._thread = threading.Thread(target=self._rodar, name="laco-glib", daemon=True)
        self._thread.start()
        if not self._pronto.wait(5):
            raise RuntimeError("o laço GLib não subiu em 5s")

    def _rodar(self) -> None:
        # Contexto como padrão DESTA thread: é nele que o Gio entrega os
        # sinais das assinaturas feitas aqui dentro.
        self._contexto.push_thread_default()
        try:
            self._contexto.invoke_full(GLib.PRIORITY_DEFAULT, self._avisar_pronto)
            self._laco.run()
        finally:
            self._contexto.pop_thread_default()

    def _avisar_pronto(self) -> bool:
        self._pronto.set()
        return GLib.SOURCE_REMOVE

    def vivo(self) -> bool:
        return self._thread is not None and self._thread.is_alive() and not self._encerrado

    def executar(self, funcao, timeout: float = TIMEOUT_PADRAO):
        """Roda `funcao()` na thread do laço e devolve o que ela devolver.
        Exceção dentro dela chega aqui com o tipo original."""
        if self._thread is None or self._encerrado:
            raise RuntimeError("o laço GLib não foi iniciado (ou já foi encerrado)")
        if threading.get_ident() == self._thread.ident:
            return funcao()  # já estamos no laço: esperar a si mesmo travaria

        feito = threading.Event()
        caixa: dict = {}

        def rodar() -> bool:
            try:
                caixa["valor"] = funcao()
            except BaseException as erro:  # noqa: BLE001 — repassada intacta a quem chamou
                caixa["erro"] = erro
            finally:
                feito.set()
            return GLib.SOURCE_REMOVE

        self._contexto.invoke_full(GLib.PRIORITY_DEFAULT, rodar)
        if not feito.wait(timeout):
            raise TimeoutError(f"o laço GLib não respondeu em {timeout}s")
        if "erro" in caixa:
            raise caixa["erro"]
        return caixa.get("valor")

    def assinar_sinal(self, conexao, remetente, interface, membro, caminho) -> "Espera":
        """Assina um sinal DENTRO do laço e devolve uma Espera, que quem
        chamou aguarda da própria thread. Assinar antes de disparar o pedido
        que gera o sinal fecha a corrida de o sinal chegar antes da escuta."""
        espera = Espera(self, conexao)

        def assinar():
            espera.assinatura = conexao.signal_subscribe(
                remetente, interface, membro, caminho, None, 0, espera._receber
            )

        self.executar(assinar)
        return espera

    def encerrar(self) -> None:
        if self._thread is None or self._encerrado:
            return
        self._encerrado = True
        self._contexto.invoke_full(GLib.PRIORITY_DEFAULT, self._parar)
        self._thread.join(2)
        if self._thread.is_alive():
            logger.warning("a thread do laço GLib não terminou em 2s")

    def _parar(self) -> bool:
        self._laco.quit()
        return GLib.SOURCE_REMOVE


class Espera:
    """Um sinal D-Bus que alguém está esperando. O primeiro que chega vale."""

    def __init__(self, laco: LacoGlib, conexao) -> None:
        self._laco = laco
        self._conexao = conexao
        self._chegou = threading.Event()
        self._parametros = None
        self.assinatura: int | None = None

    def _receber(self, conexao, remetente, caminho, interface, membro, parametros) -> None:
        if self._chegou.is_set():
            return
        self._parametros = parametros.unpack()
        self._chegou.set()

    def aguardar(self, timeout: float):
        """Devolve os parâmetros do sinal, já desempacotados. Cancela a
        assinatura em qualquer caso: assinatura esquecida é vazamento que só
        aparece depois de horas de uso."""
        try:
            if not self._chegou.wait(timeout):
                raise TimeoutError(f"o sinal não chegou em {timeout}s")
            return self._parametros
        finally:
            self.cancelar()

    def cancelar(self) -> None:
        if self.assinatura is None:
            return
        assinatura, self.assinatura = self.assinatura, None
        try:
            self._laco.executar(lambda: self._conexao.signal_unsubscribe(assinatura))
        except (RuntimeError, TimeoutError):
            logger.debug("não consegui cancelar a assinatura %s", assinatura, exc_info=True)
