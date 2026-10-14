"""Spike B2.2 + B2.3 + B2.4: teclado e mouse pelo portal RemoteDesktop, foco pelo KWin
e as coordenadas do AT-SPI e do portal no mesmo sistema, com escala fracionária.

O que o teste faz, nesta ordem:
  1. abre uma sessão do portal RemoteDesktop (teclado + mouse) com um ScreenCast de
     monitores, que é o que permite mouse em posição ABSOLUTA. O KDE mostra um
     diálogo de permissão uma vez; o restore_token devolvido evita o diálogo depois.
  2. abre um KWrite vazio (processo filho deste script) e o traz para frente pelo
     script do KWin.
  3. acha a área de texto pelo AT-SPI (retângulo relativo à janela), soma a origem
     do cliente que o KWin informa, move o mouse até o centro e clica.
  4. digita uma frase com acento e confere pelo AT-SPI o que ficou no documento.
  5. move o mouse para o centro de cada monitor e confere a posição pelo KWin.
  6. fecha o KWrite pelo PID, sem salvar.

Roda com o python3 do sistema (pyatspi e gi vêm do dnf):
    /usr/bin/python3 docs/superpowers/spikes/2026-10-13-tela-wayland/spike2_portal_entrada.py
"""
import os
import pathlib
import signal
import subprocess
import sys
import time
import uuid

import pyatspi
from gi.repository import Gio, GLib

BUS = Gio.bus_get_sync(Gio.BusType.SESSION, None)
REMETENTE = BUS.get_unique_name()[1:].replace(".", "_")
PORTAL = "org.freedesktop.portal.Desktop"
CAMINHO_PORTAL = "/org/freedesktop/portal/desktop"
TOKEN = pathlib.Path(os.environ.get("JAMES_SPIKE_TOKEN", "/tmp/james-spike-portal-token"))
BTN_ESQUERDO = 0x110  # BTN_LEFT do kernel (linux/input-event-codes.h)
FRASE = "James teste ação 123"


def esperar(condicao, segundos):
    fim = time.monotonic() + segundos
    contexto = GLib.MainContext.default()
    while not condicao() and time.monotonic() < fim:
        contexto.iteration(False)
        time.sleep(0.005)
    return condicao()


# --- KWin: um script curto que devolve linhas por D-Bus -----------------------

_linhas_kwin: list[str] = []
_XML = ("<node><interface name='org.james.Spike'><method name='receber'>"
        "<arg type='s' direction='in'/></method></interface></node>")


def _ao_receber(conn, sender, path, iface, metodo, params, invocacao):
    _linhas_kwin.append(params.unpack()[0])
    invocacao.return_value(None)


Gio.bus_own_name_on_connection(BUS, "org.james.Spike", 0, None, None)
BUS.register_object("/", Gio.DBusNodeInfo.new_for_xml(_XML).interfaces[0], _ao_receber, None, None)

_PREAMBULO = 'function devolver(s) { callDBus("org.james.Spike", "/", "org.james.Spike", "receber", String(s)); }\n'


def kwin(js: str) -> list[str]:
    """Roda js dentro do KWin e devolve o que ele mandou por devolver()."""
    _linhas_kwin.clear()
    nome = "james-spike-" + uuid.uuid4().hex[:8]
    arquivo = pathlib.Path("/tmp") / (nome + ".js")
    arquivo.write_text(_PREAMBULO + js + '\ndevolver("FIM");\n')
    try:
        ident = BUS.call_sync("org.kde.KWin", "/Scripting", "org.kde.kwin.Scripting", "loadScript",
                              GLib.Variant("(ss)", (str(arquivo), nome)), None, 0, 5000, None).unpack()[0]
        BUS.call_sync("org.kde.KWin", f"/Scripting/Script{ident}", "org.kde.kwin.Script", "run",
                      None, None, 0, 5000, None)
        if not esperar(lambda: "FIM" in _linhas_kwin, 3):
            raise RuntimeError("o script do KWin não respondeu")
        return [l for l in _linhas_kwin if l != "FIM"]
    finally:
        BUS.call_sync("org.kde.KWin", "/Scripting", "org.kde.kwin.Scripting", "unloadScript",
                      GLib.Variant("(s)", (nome,)), None, 0, 5000, None)
        arquivo.unlink(missing_ok=True)


def geometria_do_cliente(pid: int):
    linhas = kwin(f"""
for (const w of workspace.windowList()) if (w.pid == {pid} && w.normalWindow) {{
  const c = w.clientGeometry; devolver(c.x + " " + c.y + " " + c.width + " " + c.height);
}}""")
    if not linhas:
        return None
    return tuple(float(v) for v in linhas[0].split())


def ativar(pid: int) -> bool:
    linhas = kwin(f"""
for (const w of workspace.windowList()) if (w.pid == {pid} && w.normalWindow) workspace.activeWindow = w;
devolver(workspace.activeWindow ? workspace.activeWindow.pid : -1);""")
    return linhas == [str(pid)]


def cursor():
    x, y = kwin('devolver(workspace.cursorPos.x + " " + workspace.cursorPos.y);')[0].split()
    return float(x), float(y)


def telas():
    saida = []
    for l in kwin('for (const o of workspace.screens) { const g = o.geometry; '
                  'devolver(o.name + " " + g.x + " " + g.y + " " + g.width + " " + g.height + " " + o.devicePixelRatio); }'):
        nome, *nums = l.split()
        saida.append((nome, *map(float, nums)))
    return saida


# --- Portal RemoteDesktop ------------------------------------------------------

def pedir(interface: str, metodo: str, assinatura: str, args: tuple, segundos=120):
    """Chama um método do portal que responde pelo sinal Request.Response."""
    token = "james" + uuid.uuid4().hex[:8]
    opcoes = args[-1]
    opcoes["handle_token"] = GLib.Variant("s", token)
    caminho = f"/org/freedesktop/portal/desktop/request/{REMETENTE}/{token}"
    resposta = {}

    def ao_responder(conn, sender, path, iface, sinal, params):
        resposta["r"] = params.unpack()

    sub = BUS.signal_subscribe(PORTAL, "org.freedesktop.portal.Request", "Response", caminho,
                               None, 0, ao_responder)
    try:
        BUS.call_sync(PORTAL, CAMINHO_PORTAL, interface, metodo, GLib.Variant(assinatura, args),
                      None, 0, 10000, None)
        if not esperar(lambda: "r" in resposta, segundos):
            raise RuntimeError(f"{metodo}: o portal não respondeu em {segundos}s")
    finally:
        BUS.signal_unsubscribe(sub)
    codigo, resultados = resposta["r"]
    if codigo != 0:
        raise RuntimeError(f"{metodo}: recusado ou cancelado (código {codigo})")
    return resultados


def notificar(metodo: str, assinatura: str, args: tuple):
    BUS.call_sync(PORTAL, CAMINHO_PORTAL, "org.freedesktop.portal.RemoteDesktop", metodo,
                  GLib.Variant(assinatura, args), None, 0, 5000, None)


def abrir_sessao():
    rd = "org.freedesktop.portal.RemoteDesktop"
    sessao = pedir(rd, "CreateSession", "(a{sv})",
                   ({"session_handle_token": GLib.Variant("s", "jamesspike")},))["session_handle"]
    opcoes = {"types": GLib.Variant("u", 3), "persist_mode": GLib.Variant("u", 2)}
    if TOKEN.exists():
        opcoes["restore_token"] = GLib.Variant("s", TOKEN.read_text().strip())
    pedir(rd, "SelectDevices", "(oa{sv})", (sessao, opcoes))
    pedir("org.freedesktop.portal.ScreenCast", "SelectSources", "(oa{sv})",
          (sessao, {"types": GLib.Variant("u", 1), "multiple": GLib.Variant("b", True)}))
    inicio = time.monotonic()
    resultado = pedir(rd, "Start", "(osa{sv})", (sessao, "", {}))
    print(f"portal: Start levou {time.monotonic() - inicio:.2f}s "
          f"({'com' if 'restore_token' in opcoes else 'sem'} restore_token salvo)")
    if "restore_token" in resultado:
        TOKEN.write_text(resultado["restore_token"])
        TOKEN.chmod(0o600)
    fluxos = []
    for no, props in resultado.get("streams", []):
        (px, py), (lx, ly) = props["position"], props["size"]
        fluxos.append((no, px, py, lx, ly))
    print(f"portal: dispositivos={resultado.get('devices')} fluxos={fluxos}")
    return sessao, fluxos


def mover(sessao, fluxos, gx: float, gy: float):
    for no, px, py, lx, ly in fluxos:
        if px <= gx < px + lx and py <= gy < py + ly:
            notificar("NotifyPointerMotionAbsolute", "(oa{sv}udd)", (sessao, {}, no, gx - px, gy - py))
            return
    raise ValueError(f"({gx}, {gy}) fora de todos os monitores")


def clicar(sessao):
    notificar("NotifyPointerButton", "(oa{sv}iu)", (sessao, {}, BTN_ESQUERDO, 1))
    notificar("NotifyPointerButton", "(oa{sv}iu)", (sessao, {}, BTN_ESQUERDO, 0))


def digitar(sessao, texto: str):
    for ch in texto:
        # Latin-1 tem keysym igual ao código; fora dele, 0x01000000 + código (X11).
        keysym = ord(ch) if ord(ch) < 0x100 else 0x01000000 + ord(ch)
        notificar("NotifyKeyboardKeysym", "(oa{sv}iu)", (sessao, {}, keysym, 1))
        notificar("NotifyKeyboardKeysym", "(oa{sv}iu)", (sessao, {}, keysym, 0))


# --- AT-SPI --------------------------------------------------------------------

def app_do_pid(pid: int):
    desktop = pyatspi.Registry.getDesktop(0)
    for i in range(desktop.childCount):
        app = desktop[i]
        try:
            if app is not None and app.get_process_id() == pid:
                return app
        except Exception:
            continue
    return None


def achar(app, criterio):
    pilha = [app]
    while pilha:
        no = pilha.pop()
        try:
            if criterio(no):
                return no, no.queryComponent().getExtents(pyatspi.WINDOW_COORDS)
            pilha.extend(no[i] for i in range(no.childCount))
        except Exception:
            continue
    return None, None


def area_de_texto(app):
    pilha = [app]
    while pilha:
        no = pilha.pop()
        try:
            estados = no.getState()
            if estados.contains(pyatspi.STATE_EDITABLE) and estados.contains(pyatspi.STATE_SHOWING):
                r = no.queryComponent().getExtents(pyatspi.WINDOW_COORDS)
                if r.width > 100 and r.height > 50:
                    return no, r
            pilha.extend(no[i] for i in range(no.childCount))
        except Exception:
            continue
    return None, None


# --- O teste -------------------------------------------------------------------

def main():
    resultados = {}
    sessao, fluxos = abrir_sessao()

    editor = subprocess.Popen(["kwrite"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    try:
        if not esperar(lambda: geometria_do_cliente(editor.pid) is not None, 20):
            sys.exit("o KWrite não abriu janela em 20s")
        time.sleep(1.0)  # deixa a árvore de acessibilidade montar

        inicio = time.monotonic()
        resultados["foco_kwin"] = ativar(editor.pid)
        print(f"kwin: ativar janela={resultados['foco_kwin']} em {time.monotonic() - inicio:.3f}s")

        esperar(lambda: app_do_pid(editor.pid) is not None, 5)
        app = app_do_pid(editor.pid)
        if app is None:
            sys.exit("AT-SPI: o KWrite não apareceu no barramento")
        cx, cy, cw, ch = geometria_do_cliente(editor.pid)

        # clicar_elemento: o botão "Novo" da tela de boas-vindas, achado por nome e papel.
        botao, rb = achar(app, lambda n: n.name == "Novo" and n.getRoleName() in ("button", "push button")
                          and n.getState().contains(pyatspi.STATE_SHOWING))
        if botao is None:
            sys.exit("AT-SPI: não achei o botão Novo")
        inicio = time.monotonic()
        mover(sessao, fluxos, cx + rb.x + rb.width / 2, cy + rb.y + rb.height / 2)
        clicar(sessao)
        resultados["clicou_no_botao"] = esperar(lambda: area_de_texto(app)[0] is not None, 5)
        print(f"portal: clique em 'Novo' ({rb.x},{rb.y}) → editor apareceu={resultados['clicou_no_botao']} "
              f"em {time.monotonic() - inicio:.3f}s")
        no, r = area_de_texto(app)
        if no is None:
            sys.exit("AT-SPI: o clique não abriu o editor")
        cx, cy, cw, ch = geometria_do_cliente(editor.pid)
        alvo = (cx + r.x + r.width / 2, cy + r.y + r.height / 2)
        print(f"atspi: área de texto em janela ({r.x},{r.y},{r.width}x{r.height}); cliente kwin em ({cx},{cy}) "
              f"→ alvo global ({alvo[0]:.1f}, {alvo[1]:.1f})")

        inicio = time.monotonic()
        mover(sessao, fluxos, *alvo)
        time.sleep(0.1)
        real = cursor()
        erro = max(abs(real[0] - alvo[0]), abs(real[1] - alvo[1]))
        resultados["mouse_no_alvo"] = erro <= 1.5
        print(f"portal: mouse em {real} (erro {erro:.2f} px lógico) em {time.monotonic() - inicio:.3f}s")

        clicar(sessao)
        time.sleep(0.2)
        inicio = time.monotonic()
        digitar(sessao, FRASE)
        dur = time.monotonic() - inicio
        time.sleep(0.5)
        texto = no.queryText().getText(0, -1)
        resultados["digitou_certo"] = texto.strip() == FRASE
        print(f"portal: digitou {len(FRASE)} caracteres em {dur:.3f}s; AT-SPI leu {texto!r}")

        for nome, x, y, w, h, escala in telas():
            centro = (x + w / 2, y + h / 2)
            mover(sessao, fluxos, *centro)
            time.sleep(0.1)
            real = cursor()
            erro = max(abs(real[0] - centro[0]), abs(real[1] - centro[1]))
            resultados[f"centro_{nome}"] = erro <= 1.5
            print(f"tela {nome} escala {escala}: alvo {centro} real {real} erro {erro:.2f}")
    finally:
        editor.send_signal(signal.SIGTERM)
        try:
            editor.wait(5)
        except subprocess.TimeoutExpired:
            editor.kill()

    print("\nRESULTADO:", " ".join(f"{k}={'ok' if v else 'FALHOU'}" for k, v in resultados.items()))


if __name__ == "__main__":
    main()
