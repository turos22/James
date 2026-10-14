"""Spike B2.2 (parte 3): texto por área de transferência + Ctrl+V, para campo que
ignora o EditableText do AT-SPI (barra de endereço do Firefox).

Salva a área de transferência do Arthur, cola no FIM do campo, confere, apaga o que
colou com Backspace e devolve a área de transferência original. Não aperta Enter.
    /usr/bin/python3 docs/superpowers/spikes/2026-10-13-tela-wayland/spike6_colar.py
"""
import subprocess
import sys
import time

import pyatspi

sys.path.insert(0, __file__.rsplit("/", 1)[0])
import spike2_portal_entrada as s2  # noqa: E402

FRASE = " teste do James: ação e coração"
XK = {"Control_L": 0xFFE3, "End": 0xFF57, "BackSpace": 0xFF08}


def tecla(sessao, keysym, estado):
    s2.notificar("NotifyKeyboardKeysym", "(oa{sv}iu)", (sessao, {}, keysym, estado))


def main():
    salvo = subprocess.run(["wl-paste", "--no-newline", "--type", "text/plain;charset=utf-8"],
                           capture_output=True, timeout=5).stdout
    sessao, fluxos = s2.abrir_sessao()
    d = pyatspi.Registry.getDesktop(0)
    ff = next(d[i] for i in range(d.childCount) if d[i] is not None and d[i].name == "Firefox")
    pid = ff.get_process_id()
    janela = next(ff[i] for i in range(ff.childCount) if "Course" not in ff[i].name)
    try:
        geo = s2.kwin(f"""
for (const w of workspace.windowList()) if (w.pid == {pid} && w.caption.indexOf("Course") < 0 && w.normalWindow) {{
  workspace.activeWindow = w; const c = w.clientGeometry; devolver(c.x + " " + c.y);
}}""")
        cx, cy = map(float, geo[0].split())
        time.sleep(0.4)
        campo, r = s2.achar(janela, lambda n: n.getState().contains(pyatspi.STATE_EDITABLE)
                            and n.getRoleName() == "combo box")
        antes = campo.queryText().getText(0, -1)

        subprocess.run(["wl-copy", "--type", "text/plain;charset=utf-8"], input=FRASE.encode(), timeout=5)
        inicio = time.monotonic()
        s2.mover(sessao, fluxos, cx + r.x + r.width / 2, cy + r.y + r.height / 2)
        s2.clicar(sessao)
        time.sleep(0.15)
        for k in ("End",):
            tecla(sessao, XK[k], 1); tecla(sessao, XK[k], 0)
        tecla(sessao, XK["Control_L"], 1)
        tecla(sessao, ord("v"), 1); tecla(sessao, ord("v"), 0)
        tecla(sessao, XK["Control_L"], 0)
        s2.esperar(lambda: campo.queryText().getText(0, -1) != antes, 2)
        dur = time.monotonic() - inicio
        depois = campo.queryText().getText(0, -1)
        print(f"colar: antes={antes!r} depois={depois!r} em {dur:.3f}s")
        ok = depois == antes + FRASE

        if depois.startswith(antes) and depois != antes:
            for _ in range(len(depois) - len(antes)):
                tecla(sessao, XK["BackSpace"], 1); tecla(sessao, XK["BackSpace"], 0)
            time.sleep(0.3)
        limpo = campo.queryText().getText(0, -1)
        print(f"limpeza: campo = {limpo!r}")
        print("RESULTADO:", f"colar={'ok' if ok else 'FALHOU'}", f"campo_restaurado={'ok' if limpo == antes else 'FALHOU'}")
    finally:
        subprocess.run(["wl-copy", "--type", "text/plain;charset=utf-8"], input=salvo, timeout=5)
        volta = subprocess.run(["wl-paste", "--no-newline"], capture_output=True, timeout=5).stdout
        print("área de transferência devolvida:", "ok" if volta == salvo else "DIFERENTE")


if __name__ == "__main__":
    main()
