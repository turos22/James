"""Verificação ao vivo do B2 (Task 17 do plano 2026-10-13), pelo caminho de produção.

Monta a ponte com as mesmas peças de plataforma/linux.py e chama
tela_linux.executar_acao como o jamesd chamaria. Regras do script:
- só fecha janela que ELE abriu (comparando ids antes e depois);
- o texto escrito no Firefox é apagado, e a área de transferência é conferida;
- no Dolphin, "Dividir" é clicado duas vezes (volta ao que era);
- devolve o foco à janela que estava ativa;
- NÃO mexe na música (CA3 é com o Arthur).

    cd dispositivo && .venv/bin/python ../docs/superpowers/spikes/2026-10-13-tela-wayland/verificar_b2.py
"""

import base64
import os
import re
import struct
import subprocess
import sys
import time

from gi.repository import Gio

from james_device import acessibilidade
from james_device.abrir import Abrir, LancadorGio
from james_device.kwin import ExecutorKWin, Kwin
from james_device.laco_glib import LacoGlib
from james_device.midia import BarramentoMpris, Midia
from james_device.portal import CanalPortal, Portal, Token, caminho_do_token
from james_device.tela_linux import TelaLinux
from james_device.teclas import para_keysyms
from james_device.texto import AreaKlipper, CampoAtspi, campo_em_foco

FRASE = "Olá, ação, coração e maçã — 100%"
resultados: list[tuple[str, bool, str]] = []
duracoes: dict[str, list[float]] = {}


def registrar(nome: str, ok: bool, detalhe: str = "") -> None:
    resultados.append((nome, ok, detalhe))
    print(f"{'ok   ' if ok else 'FALHA'} {nome}: {detalhe}")


def montar():
    laco = LacoGlib()
    laco.iniciar()
    acessibilidade.vincular_ao_laco(laco)
    conexao = laco.executar(lambda: Gio.bus_get_sync(Gio.BusType.SESSION, None))
    acessibilidade.ligar(acessibilidade.propriedade_da_acessibilidade(laco, conexao))
    kwin = Kwin(ExecutorKWin(laco, conexao))
    portal = Portal(CanalPortal(laco, conexao), Token(caminho_do_token()))
    portal.abrir()
    area = AreaKlipper(laco, conexao)
    tela = TelaLinux(laco=laco, kwin=kwin, portal=portal, area=area,
                     midia=Midia(BarramentoMpris(laco, conexao)), abrir=Abrir(LancadorGio()))
    return laco, kwin, portal, area, tela


def acao(tela, dados: dict) -> dict:
    inicio = time.monotonic()
    resposta = tela.executar_acao(dados)
    duracoes.setdefault(dados["acao"], []).append(time.monotonic() - inicio)
    return resposta


def esperar_janela_nova(kwin, antes: set[str], criterio, segundos=15):
    fim = time.monotonic() + segundos
    while time.monotonic() < fim:
        novas = [j for j in kwin.listar() if j.id not in antes and j.normal and criterio(j)]
        if novas:
            return novas[0]
        time.sleep(0.3)
    return None


def coordenada_na_lista(saida: str, nome: str):
    m = re.search(rf'^(e\d+)\s+\S+\s+"{re.escape(nome)}"\s+\((\d+),(\d+)\)', saida, re.M)
    return (m.group(1), int(m.group(2)), int(m.group(3))) if m else None


def main() -> int:
    laco, kwin, portal, area, tela = montar()
    if portal.estado != "aberto":
        print("o portal não abriu: sem permissão; nada a verificar")
        return 1
    ativa_no_inicio = kwin.ativa()
    tipos = area.tipos()
    area_antes = area.ler() if tipos else None
    editor = None
    abertas_por_mim: list[str] = []

    try:
        # --- CA2: janelas ------------------------------------------------------
        r = acao(tela, {"acao": "janelas_listar"})
        linhas = r.get("saida", "").splitlines()
        registrar("CA2 listar", len(linhas) >= 2 and "(ativa)" in r.get("saida", ""), f"{len(linhas)} janelas")
        firefoxes = [j for j in kwin.listar() if j.normal and "firefox" in j.app.lower()]
        r = acao(tela, {"acao": "janela_focar", "alvo": "firefox"})
        if len(firefoxes) > 1:
            registrar("CA2 alvo ambíguo devolve lista", "diga qual" in r.get("erro", ""), r.get("erro", "")[:80])
        r = acao(tela, {"acao": "janela_focar", "alvo": firefoxes[0].titulo})
        registrar("CA2 focar Firefox", "trouxe para frente" in r.get("saida", "") and kwin.ativa().id == firefoxes[0].id,
                  r.get("saida", r.get("erro", ""))[:80])

        # --- KWrite próprio -------------------------------------------------------
        antes = {j.id for j in kwin.listar()}
        editor = subprocess.Popen(["kwrite"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        meu = esperar_janela_nova(kwin, antes, lambda j: j.pid == editor.pid)
        kwin.focar(meu.id)
        time.sleep(1.0)

        # --- CA5: ver_tela -----------------------------------------------------------
        # App recém-aberto: a árvore pode levar um instante para montar.
        fim, novo = time.monotonic() + 5, None
        while novo is None and time.monotonic() < fim:
            r = acao(tela, {"acao": "ver_tela"})
            saida = r.get("saida", "")
            novo = coordenada_na_lista(saida, "Novo") or coordenada_na_lista(saida, "Novo arquivo")
            if novo is None:
                time.sleep(0.5)
        png = base64.b64decode(r.get("imagem_b64", ""))
        largura_altura = struct.unpack(">II", png[16:24]) if png[:8] == b"\x89PNG\r\n\x1a\n" else None
        registrar("CA5 imagem 1280x720", largura_altura == (1280, 720), str(largura_altura))
        # Nomes de elemento não vão para a saída (podem ter nome de arquivo do Arthur).
        primeira_eh_do_kwrite = novo is not None and saida.splitlines()[1].startswith("e1 ")
        registrar("CA5 botão Novo listado", novo is not None, f"{novo[0] if novo else '-'}")
        registrar("CA5 elementos estruturados", len(r.get("elementos", [])) > 0, f"{len(r.get('elementos', []))} elementos")

        # --- CA8 (device): click por coordenada em cima do Novo é recusado ----------
        eid, x, y = novo
        r = acao(tela, {"acao": "click", "x": x, "y": y})
        registrar("CA8 click em cima de elemento é recusado", "use clicar_elemento" in r.get("erro", ""),
                  r.get("erro", "")[:90])

        # --- CA6: clicar_elemento, com a dona focada antes ----------------------------
        outra = next(j for j in kwin.listar() if j.normal and j.id != meu.id and not j.minimizada)
        kwin.focar(outra.id)  # tira o foco do KWrite: a ponte tem de focar a dona antes
        time.sleep(0.4)
        r = acao(tela, {"acao": "clicar_elemento", "elemento_id": eid})
        time.sleep(0.8)
        app = laco.executar(lambda: next(a for a in acessibilidade.listar_apps() if a.pid() == editor.pid))
        campo = laco.executar(lambda: campo_em_foco(app, envolver=lambda no: CampoAtspi(no, laco.executar)))
        registrar("CA6 clicar_elemento no KWrite (com outra janela na frente)",
                  "cliquei" in r.get("saida", "") and kwin.ativa().id == meu.id and campo is not None,
                  r.get("saida", r.get("erro", ""))[:80])

        # --- CA7: digitar com acento no app Qt ------------------------------------------
        r = acao(tela, {"acao": "digitar", "texto": FRASE})
        lido = campo.ler().strip() if campo else ""
        registrar("CA7 digitar no KWrite", lido == FRASE, r.get("saida", r.get("erro", ""))[:70])

        # --- CA7: digitar no Firefox (barra de endereço: vai colando) e limpar ---------
        kwin.focar(firefoxes[0].id)
        time.sleep(0.4)
        r = acao(tela, {"acao": "ver_tela"})
        barra = next((e for e in r.get("elementos", []) if e["tipo"] == "ComboBox"), None)
        if barra is None:
            registrar("CA7 digitar no Firefox", False, "não achei a barra de endereço na lista")
        else:
            r = acao(tela, {"acao": "clicar_elemento", "elemento_id": barra["id"]})
            time.sleep(0.3)
            portal.teclas(para_keysyms("end"))
            r = acao(tela, {"acao": "digitar", "texto": " " + FRASE})
            ok = "colei" in r.get("saida", "") or "digitei" in r.get("saida", "")
            for _ in range(len(FRASE) + 1):
                portal.teclas(para_keysyms("backspace"))
            portal.teclas(para_keysyms("esc"))
            registrar("CA7 digitar no Firefox (e apagar)", ok, r.get("saida", r.get("erro", ""))[:80])
            # A barra cresce ao ganhar foco ("breakout"): a revalidação recusa o
            # id velho, e o certo é olhar de novo, como o modelo faria.
            r = acao(tela, {"acao": "ver_tela"})
            barra = next((e for e in r.get("elementos", []) if e["tipo"] == "ComboBox"), barra)
            r = acao(tela, {"acao": "clicar_elemento", "elemento_id": barra["id"]})
            registrar("CA6 clicar_elemento no Firefox", "cliquei" in r.get("saida", ""), r.get("saida", r.get("erro", ""))[:70])
            portal.teclas(para_keysyms("esc"))
        depois = area.ler() if area.tipos() else None
        registrar("CA7 área de transferência igual antes e depois", depois == area_antes,
                  "igual" if depois == area_antes else "DIFERENTE")

        # --- CA6: Dolphin, "Dividir" duas vezes ---------------------------------------
        dolphins = [j for j in kwin.listar() if j.normal and "dolphin" in j.app.lower() and not j.minimizada]
        if dolphins:
            kwin.focar(dolphins[0].id)
            time.sleep(0.5)
            r = acao(tela, {"acao": "ver_tela"})
            dividir = next((e for e in r.get("elementos", []) if e["nome"] in ("Dividir", "Fechar", "Fechar divisão")), None)
            if dividir:
                r1 = acao(tela, {"acao": "clicar_elemento", "elemento_id": dividir["id"]})
                time.sleep(0.6)
                r = acao(tela, {"acao": "ver_tela"})
                volta = next((e for e in r.get("elementos", []) if e["nome"] in ("Dividir", "Fechar", "Fechar divisão")), None)
                r2 = acao(tela, {"acao": "clicar_elemento", "elemento_id": volta["id"]}) if volta else {"erro": "sumiu"}
                registrar("CA6 clicar_elemento no Dolphin (e desfazer)", "cliquei" in r1.get("saida", "") and "cliquei" in r2.get("saida", ""),
                          f"{r1.get('saida', r1.get('erro', ''))[:40]} / {r2.get('saida', r2.get('erro', ''))[:40]}")
            else:
                registrar("CA6 clicar_elemento no Dolphin", False, "botão Dividir não listado")

        # --- CA10: Spotify não expõe elementos -----------------------------------------
        spotify = next((j for j in kwin.listar() if j.normal and "spotify" in j.app.lower()), None)
        if spotify:
            kwin.focar(spotify.id)
            time.sleep(0.5)
            r = acao(tela, {"acao": "ver_tela"})
            registrar("CA10 app sem árvore avisa e manda a imagem",
                      "não expõe elementos" in r.get("saida", "") and bool(r.get("imagem_b64")), "")

        # --- CA4: abrir (VERIFICAR_SEM_ABRIR=1 pula: abre aba e arquivo no VS Code) ---
        if os.environ.get("VERIFICAR_SEM_ABRIR") == "1":
            raise StopIteration
        antes = {j.id for j in kwin.listar()}
        r = acao(tela, {"acao": "abrir", "alvo": "kwrite"})
        nova = esperar_janela_nova(kwin, antes, lambda j: "kwrite" in j.app.lower())
        registrar("CA4 abrir app pelo nome", nova is not None, r.get("saida", r.get("erro", "")))
        if nova:
            abertas_por_mim.append(nova.id)
        antes = {j.id for j in kwin.listar()}
        r = acao(tela, {"acao": "abrir", "alvo": "~/Downloads"})
        nova = esperar_janela_nova(kwin, antes, lambda j: "dolphin" in j.app.lower())
        registrar("CA4 abrir pasta", nova is not None, r.get("saida", r.get("erro", "")))
        if nova:
            abertas_por_mim.append(nova.id)
        r = acao(tela, {"acao": "abrir", "alvo": "https://example.com"})
        registrar("CA4 abrir URL", "abri" in r.get("saida", ""), r.get("saida", r.get("erro", "")))
        arquivo = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "2026-10-13-tela-wayland.md"))
        r = acao(tela, {"acao": "abrir", "alvo": arquivo, "linha": 3})
        registrar("CA4 abrir arquivo na linha no VS Code", "linha 3" in r.get("saida", ""), r.get("saida", r.get("erro", ""))[-60:])
    except StopIteration:
        pass
    finally:
        for identificador in abertas_por_mim:
            kwin.fechar(identificador)
        if editor is not None:
            editor.terminate()
            try:
                editor.wait(15)  # com documento alterado, o KWrite demora a sair
            except subprocess.TimeoutExpired:
                editor.kill()
                editor.wait(5)
        if ativa_no_inicio is not None:
            try:
                kwin.focar(ativa_no_inicio.id)
            except Exception:  # noqa: BLE001
                pass
        laco.encerrar()

    # --- CA12: rapidez -------------------------------------------------------------------
    tetos = {"janela_focar": 0.05, "clicar_elemento": 0.3, "digitar": 0.3, "ver_tela": 0.6}
    for nome, teto in tetos.items():
        medidas = sorted(duracoes.get(nome, []))
        if medidas:
            mediana = medidas[len(medidas) // 2]
            registrar(f"CA12 {nome} ≤ {teto * 1000:.0f} ms (mediana)", mediana <= teto,
                      f"mediana {mediana * 1000:.0f} ms, máx {medidas[-1] * 1000:.0f} ms, n={len(medidas)}")

    falhas = [r for r in resultados if not r[1]]
    print(f"\nRESULTADO: {len(resultados) - len(falhas)} de {len(resultados)} ok")
    return 1 if falhas else 0


if __name__ == "__main__":
    sys.exit(main())
