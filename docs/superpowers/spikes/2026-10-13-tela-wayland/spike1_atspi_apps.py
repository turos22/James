"""Spike B2.1 (parte 1): que apps o AT-SPI enxerga e quantos elementos úteis cada um expõe.

Só leitura. Roda com o python3 do sistema (pyatspi vem do dnf):
    python3 docs/superpowers/spikes/2026-10-13-tela-wayland/spike1_atspi_apps.py
"""
import time

import pyatspi

UTEIS = {
    pyatspi.ROLE_PUSH_BUTTON, pyatspi.ROLE_TOGGLE_BUTTON, pyatspi.ROLE_CHECK_BOX,
    pyatspi.ROLE_RADIO_BUTTON, pyatspi.ROLE_ENTRY, pyatspi.ROLE_PASSWORD_TEXT,
    pyatspi.ROLE_LINK, pyatspi.ROLE_MENU_ITEM, pyatspi.ROLE_PAGE_TAB,
    pyatspi.ROLE_COMBO_BOX, pyatspi.ROLE_LIST_ITEM, pyatspi.ROLE_TABLE_CELL,
    pyatspi.ROLE_TREE_ITEM, pyatspi.ROLE_TEXT, pyatspi.ROLE_SLIDER,
}
LIMITE = 4000  # nós por app, para não travar em árvore gigante


def varrer(raiz):
    total = uteis = com_retangulo = 0
    exemplos = []
    pilha = [raiz]
    while pilha and total < LIMITE:
        no = pilha.pop()
        total += 1
        try:
            papel = no.getRole()
            estados = no.getState()
            if papel in UTEIS and estados.contains(pyatspi.STATE_SHOWING):
                uteis += 1
                try:
                    r = no.queryComponent().getExtents(pyatspi.DESKTOP_COORDS)
                    if r.width > 0 and r.height > 0:
                        com_retangulo += 1
                        if len(exemplos) < 3 and no.name:
                            exemplos.append(f"{no.getRoleName()}:{no.name[:30]!r}@{r.x},{r.y},{r.width}x{r.height}")
                except NotImplementedError:
                    pass
            pilha.extend(no[i] for i in range(no.childCount))
        except Exception:
            continue
    return total, uteis, com_retangulo, exemplos


def main():
    desktop = pyatspi.Registry.getDesktop(0)
    print(f"apps no barramento AT-SPI: {desktop.childCount}")
    for i in range(desktop.childCount):
        app = desktop[i]
        if app is None:
            continue
        inicio = time.monotonic()
        total, uteis, com_ret, exemplos = varrer(app)
        dur = time.monotonic() - inicio
        print(f"- {app.name or '?':<28} janelas={app.childCount:<3} nós={total:<5} úteis={uteis:<4} "
              f"com_retângulo={com_ret:<4} {dur:.2f}s")
        for e in exemplos:
            print(f"    {e}")


if __name__ == "__main__":
    main()
