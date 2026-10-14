"""Spike B2.1 (Firefox): achar o campo de busca pelo AT-SPI, escrever com acento e apagar.

Não aperta Enter: só escreve, lê de volta e apaga o que escreveu.
    /usr/bin/python3 docs/superpowers/spikes/2026-10-13-tela-wayland/spike5_firefox_campo.py
"""
import sys
import time

import pyatspi

sys.path.insert(0, __file__.rsplit("/", 1)[0])
import spike2_portal_entrada as s2  # noqa: E402

FRASE = "teste do James: ação e coração"


def main():
    d = pyatspi.Registry.getDesktop(0)
    ff = next(d[i] for i in range(d.childCount) if d[i] is not None and d[i].name == "Firefox")
    pid = ff.get_process_id()
    # Ativa a janela do Firefox que está na Nova aba (a outra é o curso).
    janela = next(ff[i] for i in range(ff.childCount) if "Course" not in ff[i].name)
    inicio = time.monotonic()
    linhas = s2.kwin(f"""
for (const w of workspace.windowList()) if (w.pid == {pid} && w.caption.indexOf("Course") < 0 && w.normalWindow) workspace.activeWindow = w;
devolver(workspace.activeWindow ? workspace.activeWindow.caption : "-");""")
    print(f"kwin: ativa agora = {linhas} em {time.monotonic() - inicio:.3f}s")
    time.sleep(0.5)

    campo, r = s2.achar(janela, lambda n: n.getState().contains(pyatspi.STATE_EDITABLE)
                        and n.getState().contains(pyatspi.STATE_SHOWING)
                        and n.getRoleName() in ("combo box", "entry"))
    if campo is None:
        sys.exit("AT-SPI: nenhum campo editável visível")
    alvo = campo
    if "EditableText" not in pyatspi.utils.listInterfaces(alvo):
        # combo box do Firefox: o texto mora no filho entry
        alvo, _ = s2.achar(campo, lambda n: "EditableText" in pyatspi.utils.listInterfaces(n))
    print(f"atspi: campo {campo.getRoleName()} {campo.name[:45]!r} em janela ({r.x},{r.y},{r.width}x{r.height}); "
          f"escreve em {alvo.getRoleName()}")

    texto = alvo.queryText()
    antes = texto.getText(0, -1)
    inicio = time.monotonic()
    ok = alvo.queryEditableText().insertText(len(antes), FRASE, len(FRASE.encode("utf-8")))
    time.sleep(0.3)
    depois = alvo.queryText().getText(0, -1)
    print(f"atspi: insertText={ok} em {time.monotonic() - inicio:.3f}s; antes={antes!r} depois={depois!r}")
    resultado = depois.endswith(FRASE)

    alvo.queryEditableText().deleteText(len(antes), len(antes) + len(FRASE))
    time.sleep(0.2)
    limpo = alvo.queryText().getText(0, -1)
    print(f"atspi: depois de apagar = {limpo!r}")
    print("RESULTADO:", f"firefox_escrever={'ok' if resultado else 'FALHOU'}",
          f"firefox_limpar={'ok' if limpo == antes else 'FALHOU'}")


if __name__ == "__main__":
    main()
