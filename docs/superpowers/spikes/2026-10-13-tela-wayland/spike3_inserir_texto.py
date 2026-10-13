"""Spike B2.2 (parte 2): texto em português sem passar pelo teclado.

A emulação de tecla do portal depende do layout ativo (aqui `us`): `ç` e `ã` saem
errados. Este teste escreve pelo AT-SPI EditableText, no campo que tem o foco.
Reaproveita a sessão do portal e as funções do spike 2.
    /usr/bin/python3 docs/superpowers/spikes/2026-10-13-tela-wayland/spike3_inserir_texto.py
"""
import signal
import subprocess
import sys
import time

import pyatspi

sys.path.insert(0, __file__.rsplit("/", 1)[0])
import spike2_portal_entrada as s2  # noqa: E402

FRASE = "Olá, James! Ação, coração, pão e maçã — 100% ok?"


def main():
    sessao, fluxos = s2.abrir_sessao()
    editor = subprocess.Popen(["kwrite"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        s2.esperar(lambda: s2.geometria_do_cliente(editor.pid) is not None, 20)
        time.sleep(1.0)
        s2.ativar(editor.pid)
        s2.esperar(lambda: s2.app_do_pid(editor.pid) is not None, 5)
        app = s2.app_do_pid(editor.pid)
        cx, cy, _, _ = s2.geometria_do_cliente(editor.pid)
        botao, rb = s2.achar(app, lambda n: n.name == "Novo" and n.getRoleName() in ("button", "push button"))
        s2.mover(sessao, fluxos, cx + rb.x + rb.width / 2, cy + rb.y + rb.height / 2)
        s2.clicar(sessao)
        s2.esperar(lambda: s2.area_de_texto(app)[0] is not None, 5)
        no, _ = s2.area_de_texto(app)

        inicio = time.monotonic()
        editavel = no.queryEditableText()
        cursor = no.queryText().caretOffset
        ok_insercao = editavel.insertText(cursor, FRASE, len(FRASE.encode("utf-8")))
        dur = time.monotonic() - inicio
        time.sleep(0.3)
        lido = no.queryText().getText(0, -1)
        print(f"atspi: insertText devolveu {ok_insercao} em {dur:.3f}s; leu {lido!r}")
        print("RESULTADO:", "inserir_texto=ok" if lido.strip() == FRASE else "inserir_texto=FALHOU")
    finally:
        editor.send_signal(signal.SIGTERM)
        try:
            editor.wait(5)
        except subprocess.TimeoutExpired:
            editor.kill()


if __name__ == "__main__":
    main()
