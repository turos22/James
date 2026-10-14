"""Ponte de tela do Linux: KDE Plasma em Wayland (spec 2026-10-13 B2, seção 6).

Mesmo contrato do tela.py do Windows: `executar_acao(dados)` recebe a ação do
jamesd e devolve {"saida": ...} (mais "imagem_b64" e "elementos" no
ver_tela) ou {"erro": ...}. NUNCA levanta: erro vira valor, e o jamesd
repassa ao modelo.

Coordenadas no protocolo são pixels da IMAGEM que o modelo recebeu (1280 de
largura), locais ao monitor (D9). superficies.para_global/para_local fazem a
conta com o `fator` de cada monitor.

Regras que vieram dos achados da execução (ver o registro do plano):
- Toda chamada ao AT-SPI roda dentro do laço GLib (cache da libatspi); quem
  espera a tela mudar espera fora dele.
- clicar_elemento foca a janela dona antes de clicar: no Wayland o AT-SPI
  não sabe se uma janela está coberta.
- click por coordenada em cima de um elemento listado é recusado (D10 d):
  o clique por pixel fica para onde não há árvore (Spotify, VS Code), e não
  serve de atalho para fugir da aprovação do clique destrutivo.
"""

import base64
import logging
import os
import time

from james_device import acessibilidade, captura_linux, elementos, superficies
from james_device.abrir import ErroDeAbrir
from james_device.kwin import por_alvo
from james_device.midia import ErroDeMidia
from james_device.portal import SemPermissao, casar_fluxos
from james_device.teclas import ErroDeTecla, para_keysyms
from james_device.texto import CampoAtspi, ErroDeTexto, campo_em_foco, digitar

logger = logging.getLogger(__name__)

_AVISO_SEM_ELEMENTOS = (
    "(este app não expõe elementos — talvez tenha sido aberto antes do James ligar a acessibilidade: "
    "reabra-o. Por enquanto, use a imagem e clique por coordenada)"
)
_ERROS_ESPERADOS = (ErroDeTecla, ErroDeTexto, ErroDeMidia, ErroDeAbrir, ValueError)


class TelaLinux:
    def __init__(self, laco, kwin, portal, area, midia, abrir, listar_apps=acessibilidade.listar_apps,
                 capturar=captura_linux.capturar, fechar_fd=os.close) -> None:
        self._laco = laco
        self._kwin = kwin
        self._portal = portal
        self._area = area
        self._midia = midia
        self._abrir = abrir
        self._listar_apps = listar_apps
        self._capturar = capturar
        self._fechar_fd = fechar_fd
        self._envolver_campo = lambda no: CampoAtspi(no, laco.executar)
        # Retrato da última varredura, como o _mapa_atual do tela.py.
        self._mapa: elementos.Mapa | None = None
        self._donos: dict[str, acessibilidade.Dono] = {}

    # --- entrada ------------------------------------------------------------------

    def executar_acao(self, dados: dict) -> dict:
        acao = dados.get("acao")
        inicio = time.monotonic()
        try:
            manipulador = self._acoes().get(acao)
            if manipulador is None:
                return {"erro": f"ação desconhecida: {acao}"}
            return manipulador(dados)
        except SemPermissao as erro:
            return {"erro": str(erro)}
        except _ERROS_ESPERADOS as erro:
            return {"erro": str(erro)}
        except Exception as erro:  # noqa: BLE001 — a ponte nunca derruba o device
            logger.exception("erro ao executar a ação de tela %s", acao)
            return {"erro": str(erro)}
        finally:
            logger.info("acao=%s dur_ms=%d", acao, (time.monotonic() - inicio) * 1000)

    def _acoes(self) -> dict:
        return {
            "ver_tela": self._ver_tela,
            "click": self._clicar,
            "clicar_elemento": self._clicar_elemento,
            "digitar": self._digitar,
            "tecla": self._tecla,
            "janelas_listar": self._janelas_listar,
            "janela_focar": self._janela_focar,
            "janela_fechar": self._janela_fechar,
            "midia": lambda d: {"saida": self._midia.executar(d.get("texto", ""), d.get("alvo") or None)},
            "abrir": lambda d: {"saida": self._abrir.executar(d.get("alvo", ""), linha=d.get("linha") or None)},
        }

    # --- monitores ----------------------------------------------------------------

    def _superficies(self, telas):
        todas = []
        for tela in telas:
            largura, altura, fator = superficies.tamanho_da_imagem(round(tela.largura), round(tela.altura))
            todas.append(superficies.Superficie(
                id=tela.nome, largura=largura, altura=altura, origem_x=round(tela.x), origem_y=round(tela.y),
                escala=tela.escala, primaria=(tela.x == 0 and tela.y == 0), fator=fator))
        return todas

    def _resolver(self, dados: dict):
        telas = self._kwin.telas()
        todas = self._superficies(telas)
        if not todas:
            raise ValueError("nenhum monitor encontrado")
        pedida = dados.get("superficie")
        if pedida:
            achada = superficies.por_id(todas, pedida)
            if achada is None:
                raise ValueError(f"superfície {pedida} não existe (disponíveis: {', '.join(s.id for s in todas)})")
            return achada, telas
        ativa = self._kwin.ativa()
        retangulo = None
        if ativa is not None:
            x, y, largura, altura = ativa.frame
            retangulo = (round(x), round(y), round(x + largura), round(y + altura))
        return superficies.alvo_padrao(todas, retangulo), telas

    # --- ver_tela -----------------------------------------------------------------

    def _ver_tela(self, dados: dict) -> dict:
        superficie, telas = self._resolver(dados)
        partes = [f"superficie: {superficie.id}{' (primaria)' if superficie.primaria else ''} "
                  f"{superficie.largura}x{superficie.altura} escala {round(superficie.escala * 100)}%"]

        imagem = None
        descritor = self._portal.abrir_pipewire()  # SemPermissao sobe daqui
        try:
            fluxo = casar_fluxos(self._portal.fluxos, telas)[superficie.id]
            imagem = self._capturar(descritor, fluxo.node, superficie.largura, superficie.altura)
        except Exception as erro:  # noqa: BLE001 — a lista de elementos ainda serve sozinha
            logger.warning("captura falhou: %s", erro)
            partes.append(f"(não consegui capturar a imagem: {erro})")
        finally:
            self._fechar_fd(descritor)

        estruturados = []
        try:
            janelas = self._kwin.listar()
            ativa = next((j for j in janelas if j.ativa), None)
            varredura = self._laco.executar(lambda: acessibilidade.varrer(
                self._listar_apps(), janelas, superficie, ativa.id if ativa else None))
            self._mapa, self._donos = varredura.mapa, varredura.donos
            if varredura.mapa.elementos:
                partes.append(elementos.formatar(varredura.mapa.elementos, superficie))
            else:
                partes.append("(nenhum elemento clicável legível nesta tela — use clique por coordenada)")
            partes.extend(f"({aviso})" for aviso in varredura.avisos)
            if varredura.em_foco_sem_elementos:
                partes.append(_AVISO_SEM_ELEMENTOS)
            estruturados = [{"id": e.id, "nome": e.nome, "tipo": e.tipo} for e in varredura.mapa.elementos]
        except Exception as erro:  # noqa: BLE001 — a varredura falhando não pode custar a imagem
            logger.exception("falha ao varrer elementos")
            self._mapa, self._donos = None, {}
            partes.append(f"(não consegui ler os elementos desta tela: {erro})")

        resultado = {"saida": "\n".join(partes), "elementos": estruturados}
        if imagem is not None:
            resultado["imagem_b64"] = base64.b64encode(imagem).decode()
        return resultado

    # --- cliques ------------------------------------------------------------------

    def _mapa_valido(self, superficie) -> bool:
        if self._mapa is None:
            return False
        ativa = self._kwin.ativa()
        return self._mapa.valido_para(superficie.id, ativa.id if ativa else None)

    def _elemento_no_ponto(self, gx: int, gy: int):
        for elemento in self._mapa.elementos:
            esquerda, topo, direita, base = elemento.retangulo
            if esquerda <= gx < direita and topo <= gy < base:
                return elemento
        return None

    def _clicar(self, dados: dict) -> dict:
        superficie, _ = self._resolver(dados)
        x, y = int(dados.get("x", 0)), int(dados.get("y", 0))
        if not (0 <= x < superficie.largura and 0 <= y < superficie.altura):
            raise ValueError(f"({x},{y}) fora dos limites de {superficie.id} ({superficie.largura}x{superficie.altura})")
        gx, gy = superficies.para_global(superficie, x, y)
        if self._mapa_valido(superficie):
            debaixo = self._elemento_no_ponto(gx, gy)
            if debaixo is not None:
                raise ValueError(f'o ponto ({x},{y}) cai em "{debaixo.nome}" ({debaixo.tipo}, {debaixo.id}): '
                                 f"use clicar_elemento com o id {debaixo.id}")
        self._portal.mover(gx, gy)
        self._portal.clicar()
        return {"saida": f"cliquei em ({x},{y}) de {superficie.id}"}

    def _resolver_elemento(self, dados: dict):
        identificador, nome, tipo = dados.get("elemento_id", ""), dados.get("nome", ""), dados.get("tipo", "")
        if identificador:
            elemento = self._mapa.por_id(identificador)
            if elemento is None:
                raise ValueError(f"não achei o elemento {identificador} na última varredura")
            return elemento
        if nome:
            elemento = self._mapa.por_nome(nome, tipo)
            if elemento is None:
                quantos = sum(1 for e in self._mapa.elementos if e.nome == nome and (not tipo or e.tipo == tipo))
                if quantos > 1:
                    raise ValueError(f'achei {quantos} elementos chamados "{nome}" — use o elemento_id da lista')
                raise ValueError(f'não achei nenhum elemento chamado "{nome}" na última varredura')
            return elemento
        raise ValueError("informe elemento_id ou nome")

    def _clicar_elemento(self, dados: dict) -> dict:
        if self._mapa is None:
            raise ValueError("não tenho varredura desta tela — chame ver_tela antes")
        # O id (e7) sempre se refere à ÚLTIMA varredura: sem monitor pedido,
        # é o monitor dela, não o da janela ativa.
        if not dados.get("superficie"):
            dados = {**dados, "superficie": self._mapa.superficie_id}
        superficie, _ = self._resolver(dados)
        if not self._mapa_valido(superficie):
            raise ValueError("a tela mudou desde o ver_tela — chame ver_tela de novo")
        elemento = self._resolver_elemento(dados)
        dono = self._donos[elemento.id]

        if dono.janela_id is not None:
            ativa = self._kwin.ativa()
            if ativa is None or ativa.id != dono.janela_id:
                if not self._kwin.focar(dono.janela_id):
                    raise ValueError("a janela do elemento não existe mais — chame ver_tela de novo")
                # Focar outra janela não move nada na tela; o retrato segue válido.
                self._mapa.janela_em_foco = dono.janela_id
        origem = (0.0, 0.0)
        if dono.janela_id is not None:
            janela = next((j for j in self._kwin.listar() if j.id == dono.janela_id), None)
            if janela is None:
                raise ValueError("a janela do elemento não existe mais — chame ver_tela de novo")
            origem = janela.cliente[:2]

        recusa = self._laco.executar(lambda: acessibilidade.revalidar(elemento, dono, origem))
        if recusa:
            raise ValueError(f"{recusa} — chame ver_tela de novo")

        gx, gy = elemento.centro_global()
        self._portal.mover(gx, gy)
        self._portal.clicar()
        x, y = elemento.centro_local(superficie)
        return {"saida": f'cliquei em "{elemento.nome}" ({elemento.tipo}) em ({x},{y}) de {superficie.id}'}

    # --- teclado ------------------------------------------------------------------

    def _digitar(self, dados: dict) -> dict:
        texto = dados.get("texto", "")
        if not texto:
            raise ErroDeTexto("parâmetro texto ausente ou vazio para a ação digitar")
        ativa = self._kwin.ativa()
        campo = self._laco.executar(lambda: self._campo_da_janela(ativa)) if ativa else None
        saida = digitar(texto, campo, self._area, lambda: self._portal.teclas(para_keysyms("ctrl+v")))
        return {"saida": saida}

    def _campo_da_janela(self, janela):
        for app in self._listar_apps():
            try:
                pid, raizes = app.pid(), app.filhos()
                for raiz in raizes:
                    if acessibilidade.casar_janela(raiz.nome(), pid, [janela]) is janela:
                        return campo_em_foco(raiz, envolver=self._envolver_campo)
            except acessibilidade.NoIlegivel:
                continue
        return None

    def _tecla(self, dados: dict) -> dict:
        texto = dados.get("texto", "")
        self._portal.teclas(para_keysyms(texto))
        return {"saida": f"tecla {texto} pressionada"}

    # --- janelas ------------------------------------------------------------------

    def _janelas_listar(self, dados: dict) -> dict:
        linhas = [f"{j.titulo} — {j.app}{' (ativa)' if j.ativa else ''}{' (minimizada)' if j.minimizada else ''}"
                  for j in self._kwin.listar() if j.normal]
        return {"saida": "\n".join(linhas) or "(nenhuma janela aberta)"}

    def _uma_janela(self, dados: dict):
        alvo = (dados.get("alvo") or "").strip()
        if not alvo:
            raise ValueError("informe o alvo: parte do título ou o nome do app")
        achadas = por_alvo(self._kwin.listar(), alvo)
        if not achadas:
            raise ValueError(f"não achei janela com {alvo!r}")
        if len(achadas) > 1:
            titulos = "; ".join(j.titulo for j in achadas[:6])
            raise ValueError(f"achei {len(achadas)} janelas com {alvo!r}: {titulos}; diga qual")
        return achadas[0]

    def _janela_focar(self, dados: dict) -> dict:
        janela = self._uma_janela(dados)
        if not self._kwin.focar(janela.id):
            raise ValueError(f"não consegui trazer {janela.titulo!r} para frente")
        return {"saida": f"trouxe para frente: {janela.titulo}"}

    def _janela_fechar(self, dados: dict) -> dict:
        janela = self._uma_janela(dados)
        if not self._kwin.fechar(janela.id):
            raise ValueError(f"a janela {janela.titulo!r} não existe mais")
        return {"saida": f"pedi para fechar: {janela.titulo} (o app pode perguntar se quer salvar)"}
