# James — Fase 5: Máquina física (visão e controle de tela) — Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Dar visão ao James e fazer o clique parar de adivinhar coordenada — o modelo passa a ver a tela, endereçar elementos por nome ou id, e receber erro real quando a ação não pôde ser feita.

**Architecture:** O device Python ganha três módulos novos (`superficies`, `captura`, `elementos`) que enumeram monitores, capturam PNG por monitor e leem a árvore de acessibilidade do Windows. Do lado Go, uma ferramenta `ver_tela` devolve imagem mais lista de elementos, trafegando por uma interface opcional (`FerramentaComImagem`) que segue o padrão aditivo já usado por `ExigeConfirmacao`. Coordenadas são locais ao monitor no protocolo e convertidas para globais só no device.

**Tech Stack:** Go 1.x (`gorilla/websocket`, `google.golang.org/genai`), Python 3.11+ (`pyautogui`, `uiautomation`, `ctypes` para enumerar monitores), pytest, Postgres.

**Spec:** [`2026-08-07-james-fase-5-maquina-fisica-design.md`](../specs/2026-08-07-james-fase-5-maquina-fisica-design.md)

---

## Estrutura de arquivos

**Criados:**

| Arquivo | Responsabilidade |
|---|---|
| `dispositivo/james_device/superficies.py` | Enumerar monitores, escala DPI, conversão local↔global, qual superfície contém a janela em foco. Único lugar do device que conhece geometria de monitor |
| `dispositivo/james_device/captura.py` | Screenshot de uma superfície em PNG na resolução nativa |
| `dispositivo/james_device/elementos.py` | Varredura da árvore UIA por superfície, formatação da lista, revalidação antes do clique |
| `dispositivo/tests/test_superficies.py` | Aritmética de coordenada e escolha de superfície |
| `dispositivo/tests/test_elementos.py` | Formatação e revalidação com árvore falsa |
| `internal/ferramentas/vertela.go` | Ferramenta `ver_tela` |
| `internal/ferramentas/vertela_test.go` | Teste da ferramenta, reusando a `ponteFalsa` de `dispositivo_test.go` |
| `internal/api/vertela_prompt_test.go` | Trava o system prompt nomeando a ferramenta que passou a existir |
| `scripts/verificar_tokens_imagem/main.go` | Confere a tabela de tiles contra o `CountTokens` real (CA3) |

**Modificados:**

| Arquivo | Mudança |
|---|---|
| `dispositivo/james_device/tela.py` | DPI awareness, validação de limites, ações novas, retorno honesto |
| `dispositivo/james_device/wake.py` | Detecção e log de quase-acertos |
| `internal/memoria/janela.go` | Custo de imagem por tile |
| `internal/memoria/janela_test.go` | Reescrita do teste de custo de imagem |
| `internal/agente/tipos.go` | `ResultadoFerramenta.Imagem` |
| `internal/agente/loop.go` | Type assertion para `FerramentaComImagem` |
| `internal/agente/gemini/mapeamento.go` | `InlineData` no ramo `PapelFerramenta` |
| `internal/ferramentas/tipos.go` | Interface `FerramentaComImagem` |
| `internal/dispositivo/hub.go` | Campos de superfície e elemento, imagem no resultado, timeout por ação |
| `internal/api/dispositivo.go` | Decodificar `imagem_b64` do `resultado_acao` |
| `cmd/jamesd/main.go` | Registrar `ver_tela` |
| `prompts/sistema.md` | Item 2 passa a instruir a sequência real |
| `.gitignore` | Log de wake word |
| `dispositivo/pyproject.toml` | Dependência `uiautomation` |

---

## Task 1: `superficies.py` — geometria de monitor

Primeira porque tudo depende dela para saber o que "a tela" significa, e porque é a peça mais perigosa da fase (erro de aritmética manda o clique pro monitor errado com feedback plausível) sendo também a mais barata de testar.

**Files:**
- Create: `dispositivo/james_device/superficies.py`
- Test: `dispositivo/tests/test_superficies.py`

- [ ] **Step 1: Escrever os testes que falham**

Criar `dispositivo/tests/test_superficies.py`:

```python
"""A separação entre enumerar (fala com o SO, não testável) e calcular
(aritmética pura, testável) é deliberada: o bug caro desta fase é o de
conversão de coordenada, e ele tem que estar coberto antes de qualquer
verificação ao vivo."""

from james_device.superficies import (
    Superficie,
    contendo,
    da_janela,
    para_global,
    para_local,
)

# Geometria real da máquina do Arthur, medida em 2026-08-07. O segundo
# monitor está desalinhado verticalmente em 255px, e é exatamente isso que
# faz a conversão errada passar despercebida numa tela e não na outra.
DISPLAY1 = Superficie(id="DISPLAY1", largura=1920, altura=1080, origem_x=0, origem_y=0, escala=1.0, primaria=True)
DISPLAY7 = Superficie(id="DISPLAY7", largura=1280, altura=720, origem_x=1920, origem_y=255, escala=1.0, primaria=False)
TODAS = [DISPLAY1, DISPLAY7]


def test_para_global_no_primario_e_identidade():
    assert para_global(DISPLAY1, 100, 200) == (100, 200)


def test_para_global_no_secundario_soma_a_origem():
    assert para_global(DISPLAY7, 100, 100) == (2020, 355)


def test_para_local_desfaz_para_global():
    assert para_local(DISPLAY7, 2020, 355) == (100, 100)


def test_contendo_acha_o_primario():
    assert contendo(TODAS, 100, 100) is DISPLAY1


def test_contendo_acha_o_secundario():
    assert contendo(TODAS, 2020, 355) is DISPLAY7


def test_contendo_devolve_none_na_area_morta():
    # (2000, 100) está dentro do retângulo que engloba os dois monitores,
    # mas acima do topo do DISPLAY7 (origem_y=255) e à direita do DISPLAY1.
    # É a área preta que fez D14 descartar capturar o desktop virtual.
    assert contendo(TODAS, 2000, 100) is None


def test_da_janela_usa_o_centro_do_retangulo():
    # Janela inteiramente no segundo monitor.
    assert da_janela(TODAS, (1950, 300, 3100, 900)) is DISPLAY7


def test_da_janela_atravessando_dois_monitores_escolhe_pelo_centro():
    # Janela de (1800,300) a (2200,700): centro em (2000,500), que cai no
    # DISPLAY7. Regra de R6 da spec — determinística, e conhecidamente
    # incompleta pra janela cortada ao meio.
    assert da_janela(TODAS, (1800, 300, 2200, 700)) is DISPLAY7


def test_da_janela_sem_superficie_no_centro_cai_no_primario():
    # Centro na área morta: precisa devolver algo utilizável, não None —
    # quem chama vai capturar uma tela de qualquer jeito.
    assert da_janela(TODAS, (1900, 50, 2100, 150)) is DISPLAY1
```

- [ ] **Step 2: Rodar os testes e confirmar que falham**

```bash
cd dispositivo && python -m pytest tests/test_superficies.py -v
```

Esperado: FAIL com `ModuleNotFoundError: No module named 'james_device.superficies'`.

- [ ] **Step 3: Escrever `superficies.py`**

Criar `dispositivo/james_device/superficies.py`:

```python
"""Geometria de monitor: enumera as superfícies (D14 da spec da Fase 5),
converte coordenada local↔global (D15) e decide qual superfície contém uma
janela.

Único lugar do device que conhece origem e escala de monitor — captura.py,
elementos.py e tela.py perguntam aqui em vez de calcular por conta.

Divisão importante: listar() fala com o SO e não tem teste automatizado;
tudo abaixo dela é aritmética pura sobre uma lista de Superficie recebida
por parâmetro, e é testável sem hardware.
"""

import ctypes
import logging
from ctypes import wintypes
from dataclasses import dataclass

logger = logging.getLogger(__name__)

_MONITORINFOF_PRIMARY = 1
_MDT_EFFECTIVE_DPI = 0
_DPI_PADRAO = 96.0


@dataclass(frozen=True)
class Superficie:
    """Um monitor. `origem_x`/`origem_y` são a posição do canto superior
    esquerdo no espaço global do desktop; `escala` é 1.0 para 100%."""

    id: str
    largura: int
    altura: int
    origem_x: int
    origem_y: int
    escala: float
    primaria: bool


def ativar_dpi_awareness() -> None:
    """Declara o processo como per-monitor DPI aware. Precisa rodar antes de
    qualquer uso do pyautogui: sem isso, numa tela a 150% o Windows mente a
    resolução (reporta 1280x720 numa tela de 1920x1080) e todo clique cai
    deslocado. Idempotente na prática — a segunda chamada devolve erro, que
    é ignorado de propósito."""
    try:
        ctypes.windll.shcore.SetProcessDpiAwareness(2)
    except Exception:
        try:
            ctypes.windll.user32.SetProcessDPIAware()
        except Exception:
            logger.warning("não consegui ativar DPI awareness — coordenadas podem sair deslocadas em tela escalada")


# Efeito colateral de import, deliberado: DPI awareness precisa valer antes
# do primeiro uso do pyautogui, e qualquer módulo que mexa com tela importa
# este aqui.
ativar_dpi_awareness()


class _RECT(ctypes.Structure):
    _fields_ = [
        ("left", ctypes.c_long),
        ("top", ctypes.c_long),
        ("right", ctypes.c_long),
        ("bottom", ctypes.c_long),
    ]


class _MONITORINFOEXW(ctypes.Structure):
    _fields_ = [
        ("cbSize", ctypes.c_ulong),
        ("rcMonitor", _RECT),
        ("rcWork", _RECT),
        ("dwFlags", ctypes.c_ulong),
        ("szDevice", ctypes.c_wchar * 32),
    ]


_MONITORENUMPROC = ctypes.WINFUNCTYPE(
    ctypes.c_int,
    wintypes.HANDLE,
    wintypes.HANDLE,
    ctypes.POINTER(_RECT),
    wintypes.LPARAM,
)


def _escala_de(handle_monitor) -> float:
    """Escala do monitor (1.25 para 125%). Cai no padrão de 1.0 se o shcore
    não responder — monitor com escala desconhecida é melhor tratado como
    100% do que fazer a enumeração inteira falhar."""
    dpi_x = ctypes.c_uint()
    dpi_y = ctypes.c_uint()
    try:
        ctypes.windll.shcore.GetDpiForMonitor(
            handle_monitor, _MDT_EFFECTIVE_DPI, ctypes.byref(dpi_x), ctypes.byref(dpi_y)
        )
    except Exception:
        return 1.0
    if dpi_x.value == 0:
        return 1.0
    return dpi_x.value / _DPI_PADRAO


def listar() -> list[Superficie]:
    """Enumera os monitores conectados. Fala com o SO — sem teste
    automatizado, coberto por verificação ao vivo (CA4/CA8)."""
    encontradas: list[Superficie] = []

    def callback(handle_monitor, _hdc, _lprect, _lparam):
        info = _MONITORINFOEXW()
        info.cbSize = ctypes.sizeof(_MONITORINFOEXW)
        if not ctypes.windll.user32.GetMonitorInfoW(handle_monitor, ctypes.byref(info)):
            return 1
        retangulo = info.rcMonitor
        encontradas.append(
            Superficie(
                id=info.szDevice.replace("\\\\.\\", ""),
                largura=retangulo.right - retangulo.left,
                altura=retangulo.bottom - retangulo.top,
                origem_x=retangulo.left,
                origem_y=retangulo.top,
                escala=_escala_de(handle_monitor),
                primaria=bool(info.dwFlags & _MONITORINFOF_PRIMARY),
            )
        )
        return 1

    ctypes.windll.user32.EnumDisplayMonitors(None, None, _MONITORENUMPROC(callback), 0)
    return encontradas


def retangulo_da_janela_em_foco() -> tuple[int, int, int, int] | None:
    """Retângulo global da janela em foco, ou None se não houver.
    Fala com o SO — sem teste automatizado."""
    handle = ctypes.windll.user32.GetForegroundWindow()
    if not handle:
        return None
    retangulo = _RECT()
    if not ctypes.windll.user32.GetWindowRect(handle, ctypes.byref(retangulo)):
        return None
    return (retangulo.left, retangulo.top, retangulo.right, retangulo.bottom)


def para_global(superficie: Superficie, x: int, y: int) -> tuple[int, int]:
    """Coordenada local da superfície para coordenada global do desktop.
    Esquecer esta soma faz o clique cair no monitor errado — e o feedback de
    tela.py reporta com honestidade o elemento errado, o que torna o bug
    plausível e difícil de rastrear."""
    return (superficie.origem_x + x, superficie.origem_y + y)


def para_local(superficie: Superficie, x: int, y: int) -> tuple[int, int]:
    """Inverso de para_global."""
    return (x - superficie.origem_x, y - superficie.origem_y)


def contendo(superficies: list[Superficie], x: int, y: int) -> Superficie | None:
    """Qual superfície contém o ponto global, ou None. O None é real e
    acontece: com monitores desalinhados, existem pontos dentro do retângulo
    que engloba tudo mas fora de qualquer monitor."""
    for superficie in superficies:
        dentro_x = superficie.origem_x <= x < superficie.origem_x + superficie.largura
        dentro_y = superficie.origem_y <= y < superficie.origem_y + superficie.altura
        if dentro_x and dentro_y:
            return superficie
    return None


def por_id(superficies: list[Superficie], identificador: str) -> Superficie | None:
    """Busca por id (ex: "DISPLAY1")."""
    for superficie in superficies:
        if superficie.id == identificador:
            return superficie
    return None


def primaria(superficies: list[Superficie]) -> Superficie:
    """A superfície primária, ou a primeira da lista se nenhuma se declarar.
    Nunca devolve None: quem chama vai capturar alguma tela de qualquer
    jeito, e não ter resposta aqui só empurra o problema."""
    for superficie in superficies:
        if superficie.primaria:
            return superficie
    return superficies[0]


def da_janela(superficies: list[Superficie], retangulo: tuple[int, int, int, int]) -> Superficie:
    """Superfície que contém o centro do retângulo da janela (regra de R6).
    Determinística e barata; conhecidamente incompleta para janela cortada
    quase ao meio entre dois monitores. Centro na área morta cai na
    primária."""
    esquerda, topo, direita, base = retangulo
    centro_x = (esquerda + direita) // 2
    centro_y = (topo + base) // 2
    return contendo(superficies, centro_x, centro_y) or primaria(superficies)


def alvo_padrao(superficies: list[Superficie], retangulo_em_foco: tuple[int, int, int, int] | None) -> Superficie:
    """A superfície que `ver_tela` sem argumento captura (D14): a que contém
    a janela em foco, ou a primária se não houver janela em foco."""
    if retangulo_em_foco is None:
        return primaria(superficies)
    return da_janela(superficies, retangulo_em_foco)
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

```bash
cd dispositivo && python -m pytest tests/test_superficies.py -v
```

Esperado: 9 passed.

- [ ] **Step 5: Confirmar que a enumeração real funciona nesta máquina**

`listar()` não tem teste automatizado. Rodar uma vez à mão e conferir contra as origens conhecidas (`DISPLAY1` em `(0,0)`, `DISPLAY7` em `(1920,255)`):

```bash
cd dispositivo && python -c "from james_device.superficies import listar; [print(s) for s in listar()]"
```

Esperado: duas linhas, com ids e origens batendo. A resolução em si **não é conhecida de antemão com confiança**: uma medição anterior via `.NET Screen.AllScreens`, sem DPI awareness, apontou `DISPLAY7` como `1280x720 @ 100%`, mas esta função ativa DPI awareness antes de enumerar (ver `ativar_dpi_awareness()`), então o valor certo é a resolução física real do painel, que pode divergir do que uma ferramenta não-DPI-aware relataria — se vier diferente do que outra medição sugeriu, **confie nesta**, e registre o valor real observado no relatório desta task. Se os ids vierem diferentes (`\\.\DISPLAY1` em vez de `DISPLAY1`), ajustar o `replace` em `listar()` e rodar de novo.

- [ ] **Step 6: Commit**

```bash
git add dispositivo/james_device/superficies.py dispositivo/tests/test_superficies.py
git commit -m "feat(device): superficies.py com geometria de monitor e conversao de coordenada"
```

---

## Task 2: DPI awareness e limites em `tela.py`

**Files:**
- Modify: `dispositivo/james_device/tela.py`
- Test: `dispositivo/tests/test_tela.py`

- [ ] **Step 1: Escrever os testes que falham**

Adicionar ao fim de `dispositivo/tests/test_tela.py`:

```python
from james_device.superficies import Superficie

DISPLAY1 = Superficie(id="DISPLAY1", largura=1920, altura=1080, origem_x=0, origem_y=0, escala=1.0, primaria=True)
DISPLAY7 = Superficie(id="DISPLAY7", largura=1280, altura=720, origem_x=1920, origem_y=255, escala=1.0, primaria=False)


def _fixar_superficies(monkeypatch, superficies, alvo):
    """Tira o SO do caminho: tela.py passa a enxergar a geometria que o
    teste mandar, em vez dos monitores de verdade."""
    monkeypatch.setattr(tela.superficies, "listar", lambda: superficies)
    monkeypatch.setattr(tela.superficies, "retangulo_da_janela_em_foco", lambda: None)
    monkeypatch.setattr(tela.superficies, "alvo_padrao", lambda _s, _r: alvo)


def test_click_converte_para_coordenada_global(monkeypatch):
    chamadas = []
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY7)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)

    resultado = tela.executar_acao({"acao": "click", "x": 100, "y": 100})

    # (100,100) local no DISPLAY7 é (2020,355) global. Se esta soma sumir, o
    # clique cai no monitor primário e o teste falha aqui.
    assert chamadas == [(2020, 355)]
    assert "erro" not in resultado


def test_click_fora_dos_limites_devolve_erro_e_nao_clica(monkeypatch):
    chamadas = []
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY7)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: chamadas.append("click"))

    resultado = tela.executar_acao({"acao": "click", "x": 5000, "y": 100})

    assert "erro" in resultado
    assert "limites" in resultado["erro"]
    assert chamadas == []


def test_click_usa_a_superficie_pedida(monkeypatch):
    chamadas = []
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY1)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)

    tela.executar_acao({"acao": "click", "x": 10, "y": 20, "superficie": "DISPLAY7"})

    assert chamadas == [(1930, 275)]


def test_click_com_superficie_inexistente_devolve_erro(monkeypatch):
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY1)
    resultado = tela.executar_acao({"acao": "click", "x": 10, "y": 20, "superficie": "DISPLAY99"})
    assert "erro" in resultado
    assert "DISPLAY99" in resultado["erro"]
```

O teste `test_executar_acao_click_chama_pyautogui` que já existe no arquivo passa a precisar de superfícies fixadas. Substituí-lo por:

```python
def test_executar_acao_click_chama_pyautogui(monkeypatch):
    chamadas = []
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append(("moveTo", x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: chamadas.append(("click",)))

    resultado = tela.executar_acao({"acao": "click", "x": 100, "y": 200})

    assert resultado["saida"].startswith("cliquei em (100,200)")
    assert ("moveTo", 100, 200) in chamadas
    assert ("click",) in chamadas
```

E `test_executar_acao_propaga_excecao_como_erro` idem:

```python
def test_executar_acao_propaga_excecao_como_erro(monkeypatch):
    def levanta(*a, **k):
        raise RuntimeError("tela travada")

    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela.pyautogui, "moveTo", levanta)

    resultado = tela.executar_acao({"acao": "click", "x": 1, "y": 1})
    assert "erro" in resultado
    assert "tela travada" in resultado["erro"]
```

- [ ] **Step 2: Rodar os testes e confirmar que falham**

```bash
cd dispositivo && python -m pytest tests/test_tela.py -v
```

Esperado: FAIL — `AttributeError: module 'james_device.tela' has no attribute 'superficies'`.

- [ ] **Step 3: Reescrever `tela.py`**

Substituir o conteúdo de `dispositivo/james_device/tela.py` por:

```python
"""Executa ações de tela pedidas pelo jamesd: clique, tecla e visão.

Coordenadas no protocolo são LOCAIS à superfície (D15 da spec da Fase 5) —
a conversão para o espaço global do desktop acontece aqui, uma vez, antes de
acionar o pyautogui.

Nunca levanta: erro vira valor de retorno e o jamesd decide o que fazer
(filosofia R2 do lado Go).
"""

import logging

import pyautogui

from james_device import superficies

logger = logging.getLogger(__name__)

pyautogui.PAUSE = 0.01


def _resolver_superficie(dados: dict):
    """Devolve (superficie, erro). A superfície pedida por id, ou a padrão
    de D14 — a que contém a janela em foco."""
    todas = superficies.listar()
    if not todas:
        return None, "nenhum monitor encontrado"

    pedida = dados.get("superficie")
    if pedida:
        achada = superficies.por_id(todas, pedida)
        if achada is None:
            disponiveis = ", ".join(s.id for s in todas)
            return None, f"superfície {pedida} não existe (disponíveis: {disponiveis})"
        return achada, None

    return superficies.alvo_padrao(todas, superficies.retangulo_da_janela_em_foco()), None


def executar_acao(dados: dict) -> dict:
    """Executa uma ação de tela e devolve {"saida": ...} ou {"erro": ...}."""
    acao = dados.get("acao")
    try:
        if acao == "click":
            return _clicar(dados)
        if acao == "tecla":
            texto = dados.get("texto", "")
            pyautogui.press(texto)
            return {"saida": f"tecla {texto} pressionada"}
        return {"erro": f"ação desconhecida: {acao}"}
    except Exception as erro:
        logger.exception("erro ao executar ação de tela")
        return {"erro": str(erro)}


def _clicar(dados: dict) -> dict:
    superficie, erro = _resolver_superficie(dados)
    if erro:
        return {"erro": erro}

    x = int(dados.get("x", 0))
    y = int(dados.get("y", 0))

    # Validar ANTES de mover: clique fora da tela não é "quase certo", é
    # pedido inválido, e o modelo precisa saber disso pra corrigir. Deixar
    # passar era metade do bug do Anexo A #20.
    if not (0 <= x < superficie.largura and 0 <= y < superficie.altura):
        return {
            "erro": f"({x},{y}) fora dos limites de {superficie.id} "
            f"({superficie.largura}x{superficie.altura})"
        }

    global_x, global_y = superficies.para_global(superficie, x, y)
    pyautogui.moveTo(global_x, global_y, duration=1.0)
    pyautogui.click()
    return {"saida": f"cliquei em ({x},{y}) de {superficie.id}"}
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

```bash
cd dispositivo && python -m pytest tests/test_tela.py -v
```

Esperado: 8 passed.

- [ ] **Step 5: Commit**

```bash
git add dispositivo/james_device/tela.py dispositivo/tests/test_tela.py
git commit -m "fix(device): clique valida limites e converte coordenada local para global"
```

---

## Task 3: Custo de imagem por tile

**Files:**
- Modify: `internal/memoria/janela.go`
- Test: `internal/memoria/janela_test.go:74-90`

- [ ] **Step 1: Escrever os testes que falham**

Adicionar em `internal/memoria/janela_test.go`, e trocar o `TestMontarContaImagemComoCusto` existente:

```go
// pngDeTeste gera um PNG real do tamanho pedido. Bytes zerados não servem
// mais: depois que o custo passou a sair da dimensão da imagem, o header
// virou o que importa.
func pngDeTeste(t *testing.T, largura, altura int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, largura, altura))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("erro ao gerar PNG de teste: %v", err)
	}
	return buf.Bytes()
}

func TestEstimarTokensImagemSegueTabelaDoGemini(t *testing.T) {
	casos := []struct {
		nome     string
		largura  int
		altura   int
		esperado int
	}{
		{"ambos os lados até 384 custam um tile", 300, 200, 258},
		{"exatamente 384 ainda é um tile", 384, 384, 258},
		{"exemplo da doc oficial: 960x540 dá 6 tiles", 960, 540, 1548},
		{"1920x1080 tem a mesma proporção, mesmos 6 tiles", 1920, 1080, 1548},
		{"dimensão inválida cai no custo de um tile", 0, 0, 258},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := EstimarTokensImagem(c.largura, c.altura); got != c.esperado {
				t.Errorf("EstimarTokensImagem(%d, %d) = %d, queria %d", c.largura, c.altura, got, c.esperado)
			}
		})
	}
}

func TestCustoDeImagemNaoUsaMaisTamanhoEmBytes(t *testing.T) {
	// Um PNG 1920x1080 de área uniforme comprime pra poucos KB. Pela régua
	// antiga (len/4) isso custaria quase nada; pela tabela do Gemini custa
	// 1548 tokens, porque o que pesa é a área, não o arquivo.
	imagem := pngDeTeste(t, 1920, 1080)
	if len(imagem)/4 >= 1548 {
		t.Fatalf("o PNG de teste ficou grande demais (%d bytes) para provar a diferença entre as duas réguas", len(imagem))
	}

	custo := custoMensagem(agente.Mensagem{Papel: agente.PapelUsuario, Imagem: imagem})
	if custo < 1548 {
		t.Errorf("custo = %d, queria pelo menos 1548 (a régua antiga por bytes daria ~%d)", custo, len(imagem)/4)
	}
}

func TestCustoDeImagemQuebradaCaiNoFallbackConservador(t *testing.T) {
	// Header ilegível: cobrar caro continua sendo o lado seguro pra um
	// orçamento, que é o argumento original deste arquivo.
	lixo := make([]byte, 10000)
	custo := custoMensagem(agente.Mensagem{Papel: agente.PapelUsuario, Imagem: lixo})
	if custo != 2500 {
		t.Errorf("custo = %d, queria 2500 (fallback len/4)", custo)
	}
}

func TestMontarContaImagemComoCusto(t *testing.T) {
	j := Janela{OrcamentoTokens: 120}
	// 800x600: menor lado 600, crop 400, 2x2 = 4 tiles = 1032 tokens.
	// Sozinha já estoura o orçamento de 120, que é o ponto.
	imagemGrande := pngDeTeste(t, 800, 600)
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "pergunta antiga"},
		{Papel: agente.PapelUsuario, Texto: "com screenshot", Imagem: imagemGrande},
		{Papel: agente.PapelUsuario, Texto: "última, curta"},
	}

	resultado := j.Montar(mensagens)
	for _, m := range resultado {
		if len(m.Imagem) > 0 {
			t.Errorf("mensagem com screenshot de %d bytes deveria ter sido cortada (custo zero pra imagem reabriria o bug #4), mas sobreviveu: %+v", len(m.Imagem), m)
		}
	}
}
```

Acrescentar aos imports do arquivo de teste: `"bytes"`, `"image"`, `"image/png"`.

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
go test ./internal/memoria/ -run 'Imagem|Custo' -v
```

Esperado: FAIL com `undefined: EstimarTokensImagem`.

- [ ] **Step 3: Implementar em `janela.go`**

Substituir o corpo de `custoMensagem` e acrescentar a função nova:

```go
package memoria

import (
	"bytes"
	"image"
	_ "image/png" // registra o decoder — DecodeConfig só lê o header

	"james/internal/agente"
)

const (
	tokensPorTile = 258
	ladoTileUnico = 384
)

// EstimarTokensImagem devolve o custo em tokens de uma imagem pela tabela do
// Gemini: 258 tokens quando ambos os lados cabem em 384 px, e 258 por tile
// acima disso, com o tile derivado de floor(menor lado / 1.5).
//
// Confirmado contra a doc oficial (ai.google.dev/gemini-api/docs/image-
// understanding) e ancorado no exemplo de lá: 960x540 tem crop 360 e dá 6
// tiles (3x2). A régua antiga deste arquivo cobrava len(bytes)/4, o que
// errava por cerca de 80x e criava um teto que tornaria a visão inviável.
func EstimarTokensImagem(largura, altura int) int {
	if largura <= 0 || altura <= 0 {
		return tokensPorTile
	}
	if largura <= ladoTileUnico && altura <= ladoTileUnico {
		return tokensPorTile
	}

	menor := largura
	if altura < menor {
		menor = altura
	}
	crop := menor * 2 / 3 // floor(menor / 1.5)
	if crop <= 0 {
		crop = 1
	}
	return dividirArredondandoPraCima(largura, crop) * dividirArredondandoPraCima(altura, crop) * tokensPorTile
}

func dividirArredondandoPraCima(a, b int) int {
	return (a + b - 1) / b
}

func custoMensagem(m agente.Mensagem) int {
	custo := EstimarTokens(m.Texto)
	for _, r := range m.Resultados {
		custo += EstimarTokens(r.Saida) + EstimarTokens(r.Erro)
	}
	custo += custoImagem(m.Imagem)
	return custo
}

// custoImagem usa a dimensão da imagem, não o tamanho do arquivo. Quando o
// header não decodifica, cai na régua antiga (len/4): o argumento original
// deste arquivo — superestimar é o lado seguro pra um orçamento — continua
// valendo exatamente aí, para imagem quebrada.
func custoImagem(imagem []byte) int {
	if len(imagem) == 0 {
		return 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(imagem))
	if err != nil {
		return (len(imagem) + 3) / 4
	}
	return EstimarTokensImagem(cfg.Width, cfg.Height)
}
```

- [ ] **Step 4: Rodar e confirmar que passa**

```bash
go test ./internal/memoria/ -v
```

Esperado: todos os testes do pacote passam, incluindo os cinco subtestes de `TestEstimarTokensImagemSegueTabelaDoGemini`.

- [ ] **Step 5: Commit**

```bash
git add internal/memoria/janela.go internal/memoria/janela_test.go
git commit -m "fix(memoria): custo de imagem por tile, nao por bytes"
```

---

## Task 4: Imagem sobrevive no mapeamento do Gemini

Hoje o ramo `PapelFerramenta` monta só `FunctionResponse` e nunca lê imagem — o resultado de `ver_tela` chegaria ao modelo sem a tela, em silêncio.

**Files:**
- Modify: `internal/agente/tipos.go:26-31`
- Modify: `internal/agente/gemini/mapeamento.go:29-51`
- Test: `internal/agente/gemini/mapeamento_test.go`

- [ ] **Step 1: Escrever o teste que falha**

Adicionar em `internal/agente/gemini/mapeamento_test.go`:

```go
func TestParaGenaiContentsLevaImagemDeResultadoDeFerramenta(t *testing.T) {
	imagem := []byte{0x89, 'P', 'N', 'G', 1, 2, 3}
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
			{Nome: "ver_tela", Saida: "e1 Button \"Salvar\" (820,455)", Imagem: imagem},
		}},
	}

	contents := paraGenaiContents(mensagens)
	if len(contents) != 1 {
		t.Fatalf("esperava 1 Content, vieram %d", len(contents))
	}

	var achouResposta, achouImagem bool
	for _, part := range contents[0].Parts {
		if part.FunctionResponse != nil && part.FunctionResponse.Name == "ver_tela" {
			achouResposta = true
		}
		if part.InlineData != nil {
			achouImagem = true
			if part.InlineData.MIMEType != "image/png" {
				t.Errorf("MIMEType = %q, queria image/png", part.InlineData.MIMEType)
			}
			if string(part.InlineData.Data) != string(imagem) {
				t.Error("os bytes da imagem chegaram alterados")
			}
		}
	}
	if !achouResposta {
		t.Error("o FunctionResponse sumiu — a imagem não pode substituir o texto, tem que somar")
	}
	if !achouImagem {
		t.Error("a imagem foi descartada em silêncio, que é exatamente o bug desta task")
	}
}

func TestParaGenaiContentsNaoInventaImagemQuandoNaoHa(t *testing.T) {
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
			{Nome: "eco", Saida: "resultado"},
		}},
	}

	for _, part := range paraGenaiContents(mensagens)[0].Parts {
		if part.InlineData != nil {
			t.Error("resultado sem imagem gerou InlineData")
		}
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
go test ./internal/agente/gemini/ -run Imagem -v
```

Esperado: FAIL com `unknown field Imagem in struct literal of type agente.ResultadoFerramenta`.

- [ ] **Step 3: Adicionar o campo em `internal/agente/tipos.go`**

```go
type ResultadoFerramenta struct {
	ID    string
	Nome  string
	Saida string
	Erro  string
	// Imagem é preenchida só por ferramenta que implementa
	// ferramentas.FerramentaComImagem — hoje só ver_tela.
	Imagem []byte
}
```

- [ ] **Step 4: Alterar o ramo `PapelFerramenta` em `mapeamento.go`**

Substituir o `for _, r := range m.Resultados` do `case agente.PapelFerramenta` por:

```go
			for _, r := range m.Resultados {
				saida := r.Saida
				if r.Erro != "" {
					saida = "erro: " + r.Erro
				}
				parts = append(parts, &genai.Part{FunctionResponse: &genai.FunctionResponse{
					Name:     r.Nome,
					Response: map[string]any{"output": saida},
				}})
				// A imagem entra como Part separado no mesmo Content:
				// FunctionResponse não tem campo pra binário. Sem isto, o
				// resultado de ver_tela chegaria ao modelo como texto e a
				// tela seria descartada sem erro nenhum.
				if len(r.Imagem) > 0 {
					parts = append(parts, &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: r.Imagem}})
				}
			}
```

- [ ] **Step 5: Rodar e confirmar que passa**

```bash
go test ./internal/agente/... -v
```

Esperado: todos passam.

- [ ] **Step 6: Commit**

```bash
git add internal/agente/tipos.go internal/agente/gemini/mapeamento.go internal/agente/gemini/mapeamento_test.go
git commit -m "fix(gemini): imagem de resultado de ferramenta para de ser descartada em silencio"
```

---

## Task 5: `FerramentaComImagem` e o despacho no loop

**Files:**
- Modify: `internal/ferramentas/tipos.go`
- Modify: `internal/agente/loop.go:107-138`
- Test: `internal/agente/loop_test.go`

- [ ] **Step 1: Escrever o teste que falha**

Adicionar em `internal/agente/loop_test.go`:

```go
// ferramentaComImagemFalsa implementa as duas interfaces, igual ver_tela
// vai implementar: o Registro só aceita Ferramenta, mas o loop prefere
// ExecutarComImagem quando ela existe.
type ferramentaComImagemFalsa struct {
	nome            string
	saida           string
	imagem          []byte
	usouCaminhoComImagem bool
}

func (f *ferramentaComImagemFalsa) Declaracao() ferramentas.Declaracao {
	return ferramentas.Declaracao{Nome: f.nome}
}
func (f *ferramentaComImagemFalsa) Nivel() ferramentas.Nivel { return ferramentas.NivelSeguro }
func (f *ferramentaComImagemFalsa) Executar(ctx context.Context, args map[string]any) (string, error) {
	return f.saida, nil
}
func (f *ferramentaComImagemFalsa) ExecutarComImagem(ctx context.Context, args map[string]any) (string, []byte, error) {
	f.usouCaminhoComImagem = true
	return f.saida, f.imagem, nil
}

func TestDespachoPrefereExecutarComImagem(t *testing.T) {
	tool := &ferramentaComImagemFalsa{nome: "ver_tela", saida: "lista", imagem: []byte{1, 2, 3}}
	registro := ferramentas.NovoRegistro()
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("erro ao registrar: %v", err)
	}

	loop := &Loop{Ferramentas: registro, MaxIteracoes: 3}
	resultado := loop.despacharSemHook(context.Background(),
		ChamadaFerramenta{ID: "1", Nome: "ver_tela"}, permissao.Privilegiado, nil)

	if !tool.usouCaminhoComImagem {
		t.Error("o loop chamou Executar em vez de ExecutarComImagem")
	}
	if resultado.Saida != "lista" {
		t.Errorf("Saida = %q, queria lista", resultado.Saida)
	}
	if len(resultado.Imagem) != 3 {
		t.Errorf("Imagem tem %d bytes, queria 3", len(resultado.Imagem))
	}
}

func TestDespachoDeFerramentaSemImagemSegueIgual(t *testing.T) {
	tool := &ferramentaFalsa{nome: "eco", saida: "oi", nivel: ferramentas.NivelSeguro}
	registro := ferramentas.NovoRegistro()
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("erro ao registrar: %v", err)
	}

	loop := &Loop{Ferramentas: registro, MaxIteracoes: 3}
	resultado := loop.despacharSemHook(context.Background(),
		ChamadaFerramenta{ID: "1", Nome: "eco"}, permissao.Privilegiado, nil)

	if resultado.Saida != "oi" {
		t.Errorf("Saida = %q, queria oi", resultado.Saida)
	}
	if resultado.Imagem != nil {
		t.Error("ferramenta sem imagem produziu imagem")
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
go test ./internal/agente/ -run Imagem -v
```

Esperado: FAIL — `ferramentas.FerramentaComImagem` ainda não existe, e o loop não usa o caminho novo.

- [ ] **Step 3: Declarar a interface em `internal/ferramentas/tipos.go`**

Adicionar depois de `ExigeConfirmacao`:

```go
// FerramentaComImagem é implementada pela ferramenta cujo resultado inclui
// uma imagem — hoje só ver_tela.
//
// Mesmo contrato aditivo de ExigeConfirmacao: ferramenta que não implementa
// se comporta exatamente como antes, e o despacho só usa esta interface pra
// ENRIQUECER o resultado, nunca pra mudar o gate de permissão. A alternativa
// (trocar a assinatura de Ferramenta.Executar) forçaria mudança nas cinco
// ferramentas que não têm imagem, sem ganho nenhum pra elas.
//
// Quem implementa esta interface precisa implementar Ferramenta também: o
// Registro só aceita Ferramenta.
type FerramentaComImagem interface {
	ExecutarComImagem(ctx context.Context, args map[string]any) (string, []byte, error)
}
```

- [ ] **Step 4: Usar a interface no despacho, em `internal/agente/loop.go`**

Substituir as linhas finais de `despacharSemHook` (o bloco a partir de `saida, err := tool.Executar(...)`) por:

```go
	if comImagem, ok := tool.(ferramentas.FerramentaComImagem); ok {
		saida, imagem, err := comImagem.ExecutarComImagem(ctx, chamada.Args)
		if err != nil {
			return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: err.Error()}
		}
		return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Saida: saida, Imagem: imagem}
	}

	saida, err := tool.Executar(ctx, chamada.Args)
	if err != nil {
		return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: err.Error()}
	}
	return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Saida: saida}
```

- [ ] **Step 5: Rodar e confirmar que passa**

```bash
go test ./internal/... -v
```

Esperado: todos passam.

- [ ] **Step 6: Commit**

```bash
git add internal/ferramentas/tipos.go internal/agente/loop.go internal/agente/loop_test.go
git commit -m "feat(agente): interface opcional FerramentaComImagem no despacho"
```

---

## Task 6: Hub carrega superfície, elemento e imagem

**Files:**
- Modify: `internal/dispositivo/hub.go`
- Modify: `internal/ferramentas/dispositivo.go:47-51`
- Test: `internal/dispositivo/hub_test.go`

- [ ] **Step 1: Escrever o teste que falha**

Adicionar em `internal/dispositivo/hub_test.go`:

```go
func TestEnviarAcaoDevolveImagemDoResultado(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	imagem := []byte{0x89, 'P', 'N', 'G'}
	go func() {
		for {
			envelope, ok := conexao.primeiroEnviado()
			if !ok {
				time.Sleep(time.Millisecond)
				continue
			}
			var recebido struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(envelope, &recebido); err != nil {
				return
			}
			h.ResolverResultado(recebido.ID, "e1 Button \"Salvar\" (820,455)", "", imagem)
			return
		}
	}()

	resultado, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "ver_tela"})
	if err != nil {
		t.Fatalf("EnviarAcao devolveu erro: %v", err)
	}
	if resultado.Saida == "" {
		t.Error("Saida veio vazia")
	}
	if string(resultado.Imagem) != string(imagem) {
		t.Errorf("Imagem = %v, queria %v", resultado.Imagem, imagem)
	}
}

func TestAcaoTelaSerializaCamposDeElementoESuperficie(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	go func() {
		time.Sleep(20 * time.Millisecond)
		envelope, ok := conexao.primeiroEnviado()
		if !ok {
			return
		}
		var recebido struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(envelope, &recebido); err != nil {
			return
		}
		h.ResolverResultado(recebido.ID, "ok", "", nil)
	}()

	_, err := h.EnviarAcao(context.Background(), AcaoTela{
		Acao: "clicar_elemento", Superficie: "DISPLAY7", ElementoID: "e7", Nome: "Salvar", Tipo: "Button",
	})
	if err != nil {
		t.Fatalf("EnviarAcao devolveu erro: %v", err)
	}

	envelope, _ := conexao.primeiroEnviado()
	var enviado struct {
		Dados struct {
			Acao       string `json:"acao"`
			Superficie string `json:"superficie"`
			ElementoID string `json:"elemento_id"`
			Nome       string `json:"nome"`
			Tipo       string `json:"tipo"`
		} `json:"dados"`
	}
	if err := json.Unmarshal(envelope, &enviado); err != nil {
		t.Fatalf("envelope não decodifica: %v", err)
	}
	if enviado.Dados.Superficie != "DISPLAY7" || enviado.Dados.ElementoID != "e7" {
		t.Errorf("dados chegaram incompletos ao device: %+v", enviado.Dados)
	}
}

func TestTimeoutPorAcaoSobrepoeOGlobal(t *testing.T) {
	h := NovoHub()
	h.Timeout = 10 * time.Millisecond
	h.TimeoutPorAcao = map[string]time.Duration{"ver_tela": 300 * time.Millisecond}
	h.Registrar(&conexaoFalsa{})

	inicio := time.Now()
	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "ver_tela"})
	decorrido := time.Since(inicio)

	if err == nil {
		t.Fatal("esperava timeout, veio nil")
	}
	if decorrido < 250*time.Millisecond {
		t.Errorf("desistiu em %v — usou o Timeout global em vez do da ação", decorrido)
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
go test ./internal/dispositivo/ -v
```

Esperado: FAIL — `Resultado` e `TimeoutPorAcao` não existem, e `ResolverResultado` tem aridade diferente.

- [ ] **Step 3: Alterar `hub.go`**

Trocar `AcaoTela`, `resultadoAcao`, `Ponte`, `Hub`, `EnviarAcao` e `ResolverResultado`:

```go
// AcaoTela é o pedido de ação de tela que vira mensagem WS pro james-device.
// X e Y são LOCAIS à superfície (D15 da spec da Fase 5) — o device soma a
// origem do monitor antes de agir.
type AcaoTela struct {
	Acao       string `json:"acao"`
	X          int    `json:"x,omitempty"`
	Y          int    `json:"y,omitempty"`
	Texto      string `json:"texto,omitempty"`
	Superficie string `json:"superficie,omitempty"`
	ElementoID string `json:"elemento_id,omitempty"`
	Nome       string `json:"nome,omitempty"`
	Tipo       string `json:"tipo,omitempty"`
}

// Resultado é o que o device devolve. A imagem só vem preenchida em
// ver_tela; as outras ações devolvem só texto.
type Resultado struct {
	Saida  string
	Imagem []byte
}

// Ponte é o que ferramentas.ManipularInterface e ferramentas.VerTela
// precisam — *Hub implementa.
type Ponte interface {
	EnviarAcao(ctx context.Context, acao AcaoTela) (Resultado, error)
}

type resultadoAcao struct {
	saida  string
	erro   string
	imagem []byte
}
```

No `Hub`, acrescentar o campo:

```go
	// TimeoutPorAcao permite teto maior pra ação cara: ver_tela varre a
	// árvore UIA de um monitor inteiro, o que não cabe no mesmo limite de um
	// clique. Ação sem entrada aqui usa Timeout. Mesma regra de Timeout:
	// ajustar antes do Hub entrar em uso concorrente, não em runtime.
	TimeoutPorAcao map[string]time.Duration
```

Acrescentar o resolvedor e trocar as duas funções:

```go
func (h *Hub) timeoutDe(acao string) time.Duration {
	if t, ok := h.TimeoutPorAcao[acao]; ok && t > 0 {
		return t
	}
	return h.Timeout
}

func (h *Hub) EnviarAcao(ctx context.Context, acao AcaoTela) (Resultado, error) {
	h.mu.Lock()
	conexao := h.conexao
	h.mu.Unlock()
	if conexao == nil {
		return Resultado{}, ErroDispositivoAusente
	}

	id := uuid.NewString()
	espera := make(chan resultadoAcao, 1)
	h.mu.Lock()
	h.pendentes[id] = espera
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pendentes, id)
		h.mu.Unlock()
	}()

	envelope, err := json.Marshal(map[string]any{"tipo": "acao_tela", "id": id, "dados": acao})
	if err != nil {
		return Resultado{}, err
	}
	if err := conexao.Enviar(envelope); err != nil {
		return Resultado{}, fmt.Errorf("erro ao enviar ação pro device: %w", err)
	}

	select {
	case r := <-espera:
		if r.erro != "" {
			return Resultado{}, fmt.Errorf("device: %s", r.erro)
		}
		return Resultado{Saida: r.saida, Imagem: r.imagem}, nil
	case <-ctx.Done():
		return Resultado{}, ctx.Err()
	case <-time.After(h.timeoutDe(acao.Acao)):
		return Resultado{}, fmt.Errorf("timeout esperando resultado_acao do device")
	}
}

func (h *Hub) ResolverResultado(id, saida, erro string, imagem []byte) {
	h.mu.Lock()
	espera, ok := h.pendentes[id]
	if ok {
		delete(h.pendentes, id)
	}
	h.mu.Unlock()
	if !ok {
		return
	}
	select {
	case espera <- resultadoAcao{saida: saida, erro: erro, imagem: imagem}:
	default:
	}
}
```

- [ ] **Step 4: Ajustar `ManipularInterface` ao tipo novo**

Em `internal/ferramentas/dispositivo.go`, o `Executar` passa a ler o campo, e a declaração ganha os parâmetros novos:

```go
func (m *ManipularInterface) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "manipular_interface",
		Descricao: "Interações com a interface gráfica. Prefira 'clicar_elemento' com o id ou nome vindo de ver_tela; clique por coordenada só quando o elemento não aparecer na lista.",
		Parametros: map[string]Parametro{
			"acao":        {Tipo: "string", Descricao: "'clicar_elemento', 'click' para clique por coordenada, ou 'tecla'"},
			"elemento_id": {Tipo: "string", Descricao: "id do elemento vindo de ver_tela (ex: e7), para acao 'clicar_elemento'"},
			"nome":        {Tipo: "string", Descricao: "nome do elemento, alternativa ao elemento_id"},
			"tipo":        {Tipo: "string", Descricao: "tipo do controle (ex: Button), usado junto com nome para desambiguar"},
			"x":           {Tipo: "integer", Descricao: "coordenada X local à superfície (para acao 'click')"},
			"y":           {Tipo: "integer", Descricao: "coordenada Y local à superfície (para acao 'click')"},
			"texto":       {Tipo: "string", Descricao: "tecla a pressionar (para acao 'tecla', ex: 'space', 'enter')"},
			"superficie":  {Tipo: "string", Descricao: "id do monitor. Omitir para usar o monitor da janela em foco"},
		},
		Obrigatorios: []string{"acao"},
	}
}

func (m *ManipularInterface) Executar(ctx context.Context, args map[string]any) (string, error) {
	acao, ok := args["acao"].(string)
	if !ok || acao == "" {
		return "", fmt.Errorf("parâmetro acao ausente ou vazio")
	}
	x, _ := args["x"].(float64)
	y, _ := args["y"].(float64)
	texto, _ := args["texto"].(string)
	superficie, _ := args["superficie"].(string)
	elementoID, _ := args["elemento_id"].(string)
	nome, _ := args["nome"].(string)
	tipo, _ := args["tipo"].(string)

	r, err := m.Ponte.EnviarAcao(ctx, dispositivo.AcaoTela{
		Acao: acao, X: int(x), Y: int(y), Texto: texto,
		Superficie: superficie, ElementoID: elementoID, Nome: nome, Tipo: tipo,
	})
	if err != nil {
		return "", err
	}
	return r.Saida, nil
}
```

- [ ] **Step 5: Corrigir as quatro chamadas que param de compilar**

**`internal/api/dispositivo.go:70`** — quarto argumento. A Task 7 refaz este bloco inteiro; aqui é só voltar a compilar:

```go
			s.DispositivoHub.ResolverResultado(envelope.ID, dados.Saida, dados.Erro, nil)
```

**`internal/ferramentas/dispositivo_test.go:11-20`** — a ponte falsa devolve `Resultado` e ganha campo de imagem, que a Task 9 vai reusar:

```go
type ponteFalsa struct {
	acaoRecebida dispositivo.AcaoTela
	saida        string
	imagem       []byte
	erro         error
}

func (p *ponteFalsa) EnviarAcao(ctx context.Context, acao dispositivo.AcaoTela) (dispositivo.Resultado, error) {
	p.acaoRecebida = acao
	return dispositivo.Resultado{Saida: p.saida, Imagem: p.imagem}, p.erro
}
```

**`internal/dispositivo/hub_test.go:66, 95, 141, 142`** — `ResolverResultado` com quatro argumentos:

```go
		h.ResolverResultado(envelope["id"].(string), "clique feito", "", nil)
```

```go
		h.ResolverResultado(envelope["id"].(string), "", "tecla inválida", nil)
```

```go
		h.ResolverResultado(id, "primeiro", "", nil)
		h.ResolverResultado(id, "segundo", "", nil) // pendente já devia ter sido removido — tem que ser no-op
```

**`internal/dispositivo/hub_test.go:69-77`** — `EnviarAcao` devolve struct:

```go
	resultado, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click", X: 10, Y: 20})
	if err != nil {
		t.Fatalf("EnviarAcao devolveu erro: %v", err)
	}
	if resultado.Saida != "clique feito" {
		t.Errorf("saida = %q, queria clique feito", resultado.Saida)
	}
```

Confirmar que não sobrou nada:

```bash
go build ./... && go vet ./...
```

- [ ] **Step 6: Rodar e confirmar que passa**

```bash
go test ./... 2>&1 | tail -20
```

Esperado: todos os pacotes `ok`.

- [ ] **Step 7: Commit**

```bash
git add internal/dispositivo/ internal/ferramentas/dispositivo.go internal/api/dispositivo.go
git commit -m "feat(dispositivo): hub carrega superficie, elemento e imagem, com timeout por acao"
```

---

## Task 7: `resultado_acao` decodifica a imagem no endpoint WS

**Files:**
- Modify: `internal/api/dispositivo.go:61-70`
- Test: `internal/api/dispositivo_test.go`

- [ ] **Step 1: Escrever o teste que falha**

Adicionar em `internal/api/dispositivo_test.go`:

```go
func TestResultadoAcaoDecodificaImagemBase64(t *testing.T) {
	imagem := []byte{0x89, 'P', 'N', 'G', 0x0D}
	dados := map[string]string{
		"saida":      "e1 Button \"Salvar\" (820,455)",
		"imagem_b64": base64.StdEncoding.EncodeToString(imagem),
	}
	bruto, err := json.Marshal(dados)
	if err != nil {
		t.Fatalf("erro ao montar dados: %v", err)
	}

	saida, erro, decodificada := decodificarResultadoAcao(bruto)

	if erro != "" {
		t.Errorf("erro = %q, queria vazio", erro)
	}
	if saida == "" {
		t.Error("saida veio vazia")
	}
	if string(decodificada) != string(imagem) {
		t.Errorf("imagem = %v, queria %v", decodificada, imagem)
	}
}

func TestResultadoAcaoComBase64InvalidaSegueSemImagem(t *testing.T) {
	bruto := []byte(`{"saida":"ok","imagem_b64":"não é base64!!!"}`)

	saida, erro, imagem := decodificarResultadoAcao(bruto)

	// Imagem quebrada não pode derrubar o turno: o texto ainda serve.
	if saida != "ok" {
		t.Errorf("saida = %q, queria ok", saida)
	}
	if erro != "" {
		t.Errorf("erro = %q, queria vazio", erro)
	}
	if imagem != nil {
		t.Error("base64 inválida virou imagem")
	}
}
```

Acrescentar `"encoding/base64"` aos imports do arquivo de teste.

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
go test ./internal/api/ -run ResultadoAcao -v
```

Esperado: FAIL com `undefined: decodificarResultadoAcao`.

- [ ] **Step 3: Extrair a decodificação em `internal/api/dispositivo.go`**

Acrescentar a função e usá-la no switch:

```go
// decodificarResultadoAcao lê o payload de resultado_acao. A imagem chega em
// base64 porque o envelope é JSON. Base64 quebrada não vira erro do turno:
// o texto do resultado ainda é útil sozinho, e derrubar tudo por causa de um
// PNG mal formado seria pior que seguir sem ele.
func decodificarResultadoAcao(bruto []byte) (saida, erro string, imagem []byte) {
	var dados struct {
		Saida     string `json:"saida"`
		Erro      string `json:"erro"`
		ImagemB64 string `json:"imagem_b64"`
	}
	if err := json.Unmarshal(bruto, &dados); err != nil {
		return "", "", nil
	}
	if dados.ImagemB64 != "" {
		decodificada, err := base64.StdEncoding.DecodeString(dados.ImagemB64)
		if err != nil {
			slog.Warn("imagem_b64 inválida no resultado_acao, seguindo sem imagem", "erro", err)
		} else {
			imagem = decodificada
		}
	}
	return dados.Saida, dados.Erro, imagem
}
```

E o `case`:

```go
		case "resultado_acao":
			saida, erroDoDevice, imagem := decodificarResultadoAcao(envelope.Dados)
			s.DispositivoHub.ResolverResultado(envelope.ID, saida, erroDoDevice, imagem)
```

Acrescentar `"encoding/base64"` aos imports do arquivo.

- [ ] **Step 4: Pôr um teto explícito na mensagem do device (R8 da spec)**

Verificado no fonte do gorilla v1.5.3: `conn.go:924` só corta `if c.readLimit > 0`, e o James nunca chamou `SetReadLimit` — o valor é zero, então não há limite nenhum hoje. Não é regressão, mas com imagem trafegando passa a existir expectativa de tamanho, e um teto ancora essa expectativa.

Declarar a constante junto das outras do pacote e aplicar logo depois do `Upgrade`, em `wsDispositivo`:

```go
// tamanhoMaximoMensagemDevice é folgado de propósito: um PNG de 1920x1080 em
// base64 fica na casa de centenas de KB, e o teto precisa caber vários com
// sobra. O ponto não é apertar — é deixar de aceitar qualquer tamanho, que é
// o comportamento padrão do gorilla quando SetReadLimit nunca é chamado.
const tamanhoMaximoMensagemDevice = 32 << 20 // 32 MB
```

```go
	conn.SetReadLimit(tamanhoMaximoMensagemDevice)

	conexao := &conexaoDispositivo{ws: conn}
```

- [ ] **Step 5: Rodar e confirmar que passa**

```bash
go test ./internal/api/ -v
```

Esperado: todos passam.

- [ ] **Step 6: Commit**

```bash
git add internal/api/dispositivo.go internal/api/dispositivo_test.go
git commit -m "feat(api): resultado_acao decodifica imagem em base64, com teto de mensagem"
```

---

## Task 8: `captura.py` e a ação `ver_tela` no device

**Files:**
- Create: `dispositivo/james_device/captura.py`
- Modify: `dispositivo/james_device/tela.py`
- Test: `dispositivo/tests/test_tela.py`

- [ ] **Step 1: Escrever o teste que falha**

Adicionar em `dispositivo/tests/test_tela.py`:

```python
def test_ver_tela_devolve_imagem_em_base64(monkeypatch):
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY1)
    monkeypatch.setattr(tela.captura, "capturar", lambda s: b"\x89PNG-falso")
    monkeypatch.setattr(tela, "_listar_elementos", lambda s: "e1  Button  \"Salvar\"  (820,455)")

    resultado = tela.executar_acao({"acao": "ver_tela"})

    assert "erro" not in resultado
    assert resultado["imagem_b64"] == base64.b64encode(b"\x89PNG-falso").decode()


def test_ver_tela_declara_a_superficie_no_cabecalho(monkeypatch):
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY7)
    monkeypatch.setattr(tela.captura, "capturar", lambda s: b"png")
    monkeypatch.setattr(tela, "_listar_elementos", lambda s: "")

    resultado = tela.executar_acao({"acao": "ver_tela"})

    primeira_linha = resultado["saida"].splitlines()[0]
    assert "DISPLAY7" in primeira_linha
    assert "1280x720" in primeira_linha


def test_ver_tela_sobrevive_a_falha_da_varredura(monkeypatch):
    # Captura e varredura falham por motivos diferentes: uma tela sem árvore
    # de acessibilidade ainda produz imagem útil, e o modelo precisa receber
    # a imagem mesmo assim.
    def explode(_s):
        raise RuntimeError("sem arvore UIA")

    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela.captura, "capturar", lambda s: b"png")
    monkeypatch.setattr(tela, "_listar_elementos", explode)

    resultado = tela.executar_acao({"acao": "ver_tela"})

    assert "erro" not in resultado
    assert resultado["imagem_b64"] != ""
    assert "sem arvore UIA" in resultado["saida"]
```

Acrescentar `import base64` no topo do arquivo de teste.

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
cd dispositivo && python -m pytest tests/test_tela.py -k ver_tela -v
```

Esperado: FAIL — `module 'james_device.tela' has no attribute 'captura'`.

- [ ] **Step 3: Criar `captura.py`**

```python
"""Screenshot de uma superfície, em PNG na resolução nativa.

PNG e não JPEG (D13 da spec da Fase 5): o custo de token no Gemini é por
área e não por byte, então comprimir mais não economiza token nenhum — e
JPEG borra texto pequeno, que é justamente o que precisa ficar legível numa
interface. O MIME em internal/agente/gemini/mapeamento.go é a constante
"image/png", e esta escolha é o que a mantém verdadeira.

Separado de elementos.py de propósito: as duas coisas falham por motivos
diferentes, e uma tela sem árvore de acessibilidade ainda produz imagem útil.
"""

import io

import pyautogui


def capturar(superficie) -> bytes:
    """PNG da superfície inteira. A região usa coordenada GLOBAL, que é o
    espaço em que o pyautogui trabalha."""
    imagem = pyautogui.screenshot(
        region=(superficie.origem_x, superficie.origem_y, superficie.largura, superficie.altura)
    )
    buffer = io.BytesIO()
    imagem.save(buffer, format="PNG")
    return buffer.getvalue()
```

- [ ] **Step 4: Acrescentar a ação em `tela.py`**

Adicionar os imports (`base64`, `from james_device import captura`) e o ramo no `executar_acao`, mais as duas funções:

```python
        if acao == "ver_tela":
            return _ver_tela(dados)
```

```python
def _listar_elementos(superficie) -> str:
    """Lista de elementos da superfície. Vira a varredura de verdade na Task
    10; até lá devolve vazio, o que já deixa ver_tela útil só com a imagem."""
    return ""


def _ver_tela(dados: dict) -> dict:
    superficie, erro = _resolver_superficie(dados)
    if erro:
        return {"erro": erro}

    imagem = captura.capturar(superficie)

    cabecalho = (
        f"superficie: {superficie.id}"
        f"{' (primaria)' if superficie.primaria else ''} "
        f"{superficie.largura}x{superficie.altura} "
        f"escala {int(superficie.escala * 100)}%"
    )

    # A varredura falhando não pode custar a imagem: são falhas
    # independentes, e a imagem sozinha já serve pro modelo se orientar.
    try:
        lista = _listar_elementos(superficie)
    except Exception as erro_varredura:
        logger.exception("falha ao varrer elementos, devolvendo só a imagem")
        lista = f"(não consegui ler os elementos desta tela: {erro_varredura})"

    return {
        "saida": cabecalho + ("\n" + lista if lista else ""),
        "imagem_b64": base64.b64encode(imagem).decode(),
    }
```

- [ ] **Step 5: Rodar e confirmar que passa**

```bash
cd dispositivo && python -m pytest tests/ -v
```

Esperado: todos passam.

- [ ] **Step 6: Commit**

```bash
git add dispositivo/james_device/captura.py dispositivo/james_device/tela.py dispositivo/tests/test_tela.py
git commit -m "feat(device): captura de tela por superficie na acao ver_tela"
```

---

## Task 9: Ferramenta `ver_tela` — fecha CA4

**Files:**
- Create: `internal/ferramentas/vertela.go`
- Create: `internal/ferramentas/vertela_test.go`
- Modify: `cmd/jamesd/main.go:79-82`

- [ ] **Step 1: Escrever o teste que falha**

Criar `internal/ferramentas/vertela_test.go`. **Não declarar `ponteFalsa` de novo** — ela já existe em `dispositivo_test.go:11`, no mesmo pacote, e a Task 6 já a adaptou para devolver `Resultado` com imagem. Redeclarar é erro de compilação.

```go
package ferramentas

import (
	"context"
	"testing"
)

func TestVerTelaDevolveSaidaEImagem(t *testing.T) {
	ponte := &ponteFalsa{
		saida:  "superficie: DISPLAY1 (primaria) 1920x1080 escala 100%",
		imagem: []byte{1, 2, 3},
	}
	v := NovoVerTela(ponte)

	saida, imagem, err := v.ExecutarComImagem(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if saida == "" {
		t.Error("saida veio vazia")
	}
	if len(imagem) != 3 {
		t.Errorf("imagem tem %d bytes, queria 3", len(imagem))
	}
	if ponte.acaoRecebida.Acao != "ver_tela" {
		t.Errorf("acao = %q, queria ver_tela", ponte.acaoRecebida.Acao)
	}
}

func TestVerTelaRepassaSuperficiePedida(t *testing.T) {
	ponte := &ponteFalsa{}
	v := NovoVerTela(ponte)

	if _, _, err := v.ExecutarComImagem(context.Background(), map[string]any{"superficie": "DISPLAY7"}); err != nil {
		t.Fatalf("erro: %v", err)
	}
	if ponte.acaoRecebida.Superficie != "DISPLAY7" {
		t.Errorf("superficie = %q, queria DISPLAY7", ponte.acaoRecebida.Superficie)
	}
}

func TestVerTelaEhSegura(t *testing.T) {
	// Decisão D5 da spec: NivelSeguro, sem aprovação, qualquer perfil. A
	// consequência (o celular enxerga a tela do desktop) foi aceita
	// explicitamente pelo Arthur e está registrada em R7.
	if NovoVerTela(&ponteFalsa{}).Nivel() != NivelSeguro {
		t.Error("ver_tela deveria ser NivelSeguro")
	}
}

func TestVerTelaImplementaAsDuasInterfaces(t *testing.T) {
	var _ Ferramenta = NovoVerTela(&ponteFalsa{})
	var _ FerramentaComImagem = NovoVerTela(&ponteFalsa{})
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
go test ./internal/ferramentas/ -run VerTela -v
```

Esperado: FAIL com `undefined: NovoVerTela`.

- [ ] **Step 3: Criar `internal/ferramentas/vertela.go`**

```go
package ferramentas

import (
	"context"

	"james/internal/dispositivo"
)

// VerTela é a sexta ferramenta do registro. Devolve a imagem de uma
// superfície mais a lista de elementos clicáveis dela, e é o que faz o item
// "VISÃO PROATIVA" do system prompt deixar de apontar pro vazio.
//
// NivelSeguro por D5: não altera estado. A ferramenta lê dado sensível e o
// envia à nuvem, e essa consequência foi aceita explicitamente (R7).
type VerTela struct {
	Ponte dispositivo.Ponte
}

func NovoVerTela(ponte dispositivo.Ponte) *VerTela {
	return &VerTela{Ponte: ponte}
}

func (v *VerTela) Declaracao() Declaracao {
	return Declaracao{
		Nome: "ver_tela",
		Descricao: "Captura a tela e lista os elementos clicáveis nela, cada um com id, nome, tipo e posição. " +
			"Chame antes de manipular_interface para saber onde as coisas estão em vez de adivinhar coordenadas.",
		Parametros: map[string]Parametro{
			"superficie": {Tipo: "string", Descricao: "id do monitor (ex: DISPLAY1). Omitir para usar o monitor da janela em foco"},
		},
	}
}

func (v *VerTela) Nivel() Nivel { return NivelSeguro }

func (v *VerTela) ExecutarComImagem(ctx context.Context, args map[string]any) (string, []byte, error) {
	superficie, _ := args["superficie"].(string)

	r, err := v.Ponte.EnviarAcao(ctx, dispositivo.AcaoTela{Acao: "ver_tela", Superficie: superficie})
	if err != nil {
		return "", nil, err
	}
	return r.Saida, r.Imagem, nil
}

// Executar existe porque o Registro só aceita Ferramenta. O loop prefere
// ExecutarComImagem quando a ferramenta a implementa, então este caminho só
// roda pra quem chamar a ferramenta fora do loop — e aí a imagem se perde,
// que é o comportamento certo pra um consumidor que não sabe o que fazer
// com ela.
func (v *VerTela) Executar(ctx context.Context, args map[string]any) (string, error) {
	saida, _, err := v.ExecutarComImagem(ctx, args)
	return saida, err
}
```

- [ ] **Step 4: Registrar em `cmd/jamesd/main.go`**

Depois do registro de `NovoManipularInterface`:

```go
	if err := registro.Registrar(ferramentas.NovoVerTela(dispositivoHub)); err != nil {
		slog.Error("erro ao registrar ver_tela", "erro", err)
		os.Exit(1)
	}
```

E, junto de onde o hub é criado, dar folga ao timeout da varredura:

```go
	dispositivoHub := dispositivo.NovoHub()
	// Varrer a árvore UIA de um monitor inteiro é ordens de grandeza mais
	// caro que um clique (R1). O teto de 10s do hub é apertado pra isso.
	dispositivoHub.TimeoutPorAcao = map[string]time.Duration{
		"ver_tela":         30 * time.Second,
		"clicar_elemento":  20 * time.Second,
	}
```

- [ ] **Step 5: Rodar e confirmar que passa**

```bash
go build ./... && go test ./... 2>&1 | tail -20
```

Esperado: build ok, todos os pacotes `ok`.

- [ ] **Step 6: Commit**

```bash
git add internal/ferramentas/vertela.go internal/ferramentas/vertela_test.go cmd/jamesd/main.go
git commit -m "feat(ferramentas): ver_tela como sexta ferramenta do registro"
```

---

## Task 10: `elementos.py` — varredura por superfície

**Files:**
- Create: `dispositivo/james_device/elementos.py`
- Create: `dispositivo/tests/test_elementos.py`
- Modify: `dispositivo/james_device/tela.py`
- Modify: `dispositivo/pyproject.toml`

- [ ] **Step 1: Escrever os testes que falham**

Criar `dispositivo/tests/test_elementos.py`:

```python
"""A árvore UIA de verdade depende de tela; aqui ela é injetada. O que está
sendo testado é a lógica em cima da árvore — filtro, formatação, ids — não a
biblioteca."""

from james_device.elementos import Elemento, Mapa, formatar
from james_device.superficies import Superficie

DISPLAY1 = Superficie(id="DISPLAY1", largura=1920, altura=1080, origem_x=0, origem_y=0, escala=1.0, primaria=True)


def elemento(identificador, nome, tipo, esquerda, topo, direita, base):
    return Elemento(id=identificador, nome=nome, tipo=tipo, retangulo=(esquerda, topo, direita, base))


def test_formatar_uma_linha_por_elemento():
    elementos = [
        elemento("e1", "Salvar", "Button", 800, 440, 840, 470),
        elemento("e2", "Nome do arquivo", "Edit", 600, 400, 680, 420),
    ]

    linhas = formatar(elementos).splitlines()

    assert len(linhas) == 2
    assert "e1" in linhas[0]
    assert "Button" in linhas[0]
    assert "Salvar" in linhas[0]
    # Centro do retângulo, não o canto: (800+840)/2, (440+470)/2
    assert "(820,455)" in linhas[0]


def test_formatar_mantem_elemento_sem_nome():
    # Botão só de ícone. É o caso que torna o id necessário: sem ele, esse
    # elemento seria inendereçável.
    linhas = formatar([elemento("e3", "", "Button", 1170, 50, 1190, 70)]).splitlines()
    assert "e3" in linhas[0]
    assert '""' in linhas[0]


def test_mapa_resolve_por_id():
    e = elemento("e7", "Salvar", "Button", 800, 440, 840, 470)
    mapa = Mapa(superficie_id="DISPLAY1", janela_em_foco=123, elementos=[e])
    assert mapa.por_id("e7") is e
    assert mapa.por_id("e99") is None


def test_mapa_resolve_por_nome_e_tipo():
    salvar = elemento("e1", "Salvar", "Button", 0, 0, 10, 10)
    outro = elemento("e2", "Salvar", "MenuItem", 20, 20, 30, 30)
    mapa = Mapa(superficie_id="DISPLAY1", janela_em_foco=123, elementos=[salvar, outro])

    assert mapa.por_nome("Salvar", "Button") is salvar
    assert mapa.por_nome("Salvar", "MenuItem") is outro


def test_mapa_por_nome_ambiguo_devolve_none():
    # Dois botões "OK" na mesma tela: preferir errar a chutar um deles.
    a = elemento("e1", "OK", "Button", 0, 0, 10, 10)
    b = elemento("e2", "OK", "Button", 20, 20, 30, 30)
    mapa = Mapa(superficie_id="DISPLAY1", janela_em_foco=123, elementos=[a, b])

    assert mapa.por_nome("OK", "Button") is None


def test_mapa_valido_so_para_a_mesma_superficie_e_janela():
    mapa = Mapa(superficie_id="DISPLAY1", janela_em_foco=123, elementos=[])

    assert mapa.valido_para("DISPLAY1", 123)
    assert not mapa.valido_para("DISPLAY7", 123)
    assert not mapa.valido_para("DISPLAY1", 999)


def test_centro_local_converte_do_global():
    # A árvore UIA dá retângulo em coordenada global; a lista mostra local.
    e = elemento("e1", "Salvar", "Button", 2000, 400, 2040, 430)
    display7 = Superficie(id="DISPLAY7", largura=1280, altura=720, origem_x=1920, origem_y=255, escala=1.0, primaria=False)
    assert e.centro_local(display7) == (100, 160)
```

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
cd dispositivo && python -m pytest tests/test_elementos.py -v
```

Esperado: FAIL com `ModuleNotFoundError: No module named 'james_device.elementos'`.

- [ ] **Step 3: Declarar a dependência**

Em `dispositivo/pyproject.toml`, acrescentar `uiautomation` à lista de `dependencies`:

```toml
dependencies = ["websockets>=13.0", "pyautogui>=0.9.54", "SpeechRecognition>=3.10", "playsound>=1.2.2,<1.3.0", "uiautomation>=2.0.18"]
```

Instalar:

```bash
cd dispositivo && pip install -e ".[dev]"
```

- [ ] **Step 4: Criar `elementos.py`**

```python
"""Leitura da árvore de acessibilidade do Windows (UI Automation).

É o que faz o clique parar de adivinhar coordenada (D2 da spec da Fase 5): a
posição sai do sistema operacional, nunca de um palpite do modelo.

Fronteira deliberada: `uiautomation` só é tocada em `varrer`. Todo o resto é
lógica sobre a dataclass Elemento, testável sem tela. Se a biblioteca
precisar ser trocada por pywinauto, só `varrer` muda.
"""

import logging
from dataclasses import dataclass, field

from james_device import superficies

logger = logging.getLogger(__name__)

# Tipos de controle que valem a pena oferecer ao modelo. A lista existe pra
# conter R2 (tela densa gerando lista enorme): painel, texto estático e
# separador ocupariam a maior parte da saída sem serem clicáveis.
TIPOS_INTERESSANTES = frozenset(
    {
        "Button",
        "CheckBox",
        "ComboBox",
        "Edit",
        "Hyperlink",
        "ListItem",
        "MenuItem",
        "RadioButton",
        "Slider",
        "Tab",
        "TabItem",
        "TreeItem",
    }
)

PROFUNDIDADE_MAXIMA = 12


@dataclass(frozen=True)
class Elemento:
    """Um controle clicável. `retangulo` é (esquerda, topo, direita, base) em
    coordenada GLOBAL, do jeito que a UIA entrega."""

    id: str
    nome: str
    tipo: str
    retangulo: tuple[int, int, int, int]

    def centro_global(self) -> tuple[int, int]:
        esquerda, topo, direita, base = self.retangulo
        return ((esquerda + direita) // 2, (topo + base) // 2)

    def centro_local(self, superficie) -> tuple[int, int]:
        x, y = self.centro_global()
        return superficies.para_local(superficie, x, y)


@dataclass
class Mapa:
    """Resultado de uma varredura. Descartável de propósito: é um retrato,
    não uma fonte da verdade paralela. Quem clica revalida contra a árvore
    viva antes de agir."""

    superficie_id: str
    janela_em_foco: int
    elementos: list[Elemento] = field(default_factory=list)

    def valido_para(self, superficie_id: str, janela_em_foco: int) -> bool:
        """Invalidação de D9: mudou a superfície ou a janela em foco, o
        retrato morreu. Trocar de janela reordena o que está visível no
        monitor, então o mapa não vale mais nem no mesmo monitor."""
        return self.superficie_id == superficie_id and self.janela_em_foco == janela_em_foco

    def por_id(self, identificador: str) -> Elemento | None:
        for elemento in self.elementos:
            if elemento.id == identificador:
                return elemento
        return None

    def por_nome(self, nome: str, tipo: str = "") -> Elemento | None:
        """None quando não acha E quando acha mais de um: com dois botões
        "OK" na tela, chutar um seria reintroduzir o palpite que esta fase
        existe pra eliminar."""
        candidatos = [
            e for e in self.elementos if e.nome == nome and (not tipo or e.tipo == tipo)
        ]
        if len(candidatos) == 1:
            return candidatos[0]
        return None


def formatar(elementos: list[Elemento], superficie=None) -> str:
    """Uma linha por elemento. Só o centro vai pro modelo — o retângulo
    inteiro custaria quatro vezes mais token e quem precisa dele é o device."""
    linhas = []
    for elemento in elementos:
        x, y = elemento.centro_local(superficie) if superficie else elemento.centro_global()
        linhas.append(f'{elemento.id}  {elemento.tipo}  "{elemento.nome}"  ({x},{y})')
    return "\n".join(linhas)


def _dentro(retangulo: tuple[int, int, int, int], superficie) -> bool:
    esquerda, topo, direita, base = retangulo
    centro_x = (esquerda + direita) // 2
    centro_y = (topo + base) // 2
    return (
        superficie.origem_x <= centro_x < superficie.origem_x + superficie.largura
        and superficie.origem_y <= centro_y < superficie.origem_y + superficie.altura
    )


def varrer(superficie, janela_em_foco: int) -> Mapa:
    """Percorre a árvore UIA e devolve o mapa da superfície. Fala com o SO —
    sem teste automatizado, coberto por CA4."""
    import uiautomation

    encontrados: list[Elemento] = []
    contador = 0

    def descer(no, profundidade: int) -> None:
        nonlocal contador
        if profundidade > PROFUNDIDADE_MAXIMA:
            return
        for filho in no.GetChildren():
            try:
                if not filho.IsOffscreen and filho.ControlTypeName in TIPOS_INTERESSANTES:
                    r = filho.BoundingRectangle
                    retangulo = (r.left, r.top, r.right, r.bottom)
                    if r.right > r.left and r.bottom > r.top and _dentro(retangulo, superficie):
                        contador += 1
                        encontrados.append(
                            Elemento(
                                id=f"e{contador}",
                                nome=filho.Name or "",
                                tipo=filho.ControlTypeName,
                                retangulo=retangulo,
                            )
                        )
                descer(filho, profundidade + 1)
            except Exception:
                # Um controle que some no meio da varredura (app fechando,
                # menu colapsando) não pode matar a varredura inteira.
                logger.debug("elemento ilegível durante a varredura, pulando", exc_info=True)

    descer(uiautomation.GetRootControl(), 0)
    return Mapa(superficie_id=superficie.id, janela_em_foco=janela_em_foco, elementos=encontrados)
```

- [ ] **Step 5: Ligar em `tela.py`**

Trocar o `_listar_elementos` provisório da Task 8, e guardar o mapa:

```python
from james_device import captura, elementos, superficies

_mapa_atual: elementos.Mapa | None = None


def _handle_janela_em_foco() -> int:
    import ctypes

    return int(ctypes.windll.user32.GetForegroundWindow())


def _listar_elementos(superficie) -> str:
    global _mapa_atual
    _mapa_atual = elementos.varrer(superficie, _handle_janela_em_foco())
    if not _mapa_atual.elementos:
        return "(nenhum elemento clicável legível nesta tela — use clique por coordenada)"
    return elementos.formatar(_mapa_atual.elementos, superficie)
```

- [ ] **Step 6: Rodar e confirmar que passa**

```bash
cd dispositivo && python -m pytest tests/ -v
```

Esperado: todos passam.

- [ ] **Step 7: Commit**

```bash
git add dispositivo/james_device/elementos.py dispositivo/tests/test_elementos.py dispositivo/james_device/tela.py dispositivo/pyproject.toml
git commit -m "feat(device): varredura da arvore UIA por superficie"
```

---

## Task 11: `clicar_elemento` com revalidação — fecha CA5 e CA6

**Files:**
- Modify: `dispositivo/james_device/tela.py`
- Test: `dispositivo/tests/test_tela.py`

- [ ] **Step 1: Escrever os testes que falham**

Adicionar em `dispositivo/tests/test_tela.py`:

```python
from james_device.elementos import Elemento, Mapa


def _preparar_mapa(monkeypatch, superficie, itens, handle=123):
    monkeypatch.setattr(tela, "_mapa_atual", Mapa(superficie_id=superficie.id, janela_em_foco=handle, elementos=itens))
    monkeypatch.setattr(tela, "_handle_janela_em_foco", lambda: handle)


def test_clicar_elemento_por_id_acerta_o_centro(monkeypatch):
    chamadas = []
    salvar = Elemento(id="e7", nome="Salvar", tipo="Button", retangulo=(800, 440, 840, 470))
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    _preparar_mapa(monkeypatch, DISPLAY1, [salvar])
    monkeypatch.setattr(tela, "_revalidar", lambda e: None)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)

    resultado = tela.executar_acao({"acao": "clicar_elemento", "elemento_id": "e7"})

    assert chamadas == [(820, 455)]
    assert "Salvar" in resultado["saida"]


def test_clicar_elemento_sem_varredura_pede_ver_tela(monkeypatch):
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela, "_mapa_atual", None)

    resultado = tela.executar_acao({"acao": "clicar_elemento", "elemento_id": "e7"})

    assert "erro" in resultado
    assert "ver_tela" in resultado["erro"]


def test_clicar_elemento_com_tela_mudada_devolve_erro_e_nao_clica(monkeypatch):
    # Núcleo do CA6: a janela em foco mudou desde o ver_tela, então o mapa
    # não vale mais. Clicar aqui seria clicar no lugar errado com certeza
    # absoluta, que é o bug que esta fase inteira existe pra matar.
    chamadas = []
    salvar = Elemento(id="e7", nome="Salvar", tipo="Button", retangulo=(800, 440, 840, 470))
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    _preparar_mapa(monkeypatch, DISPLAY1, [salvar], handle=123)
    monkeypatch.setattr(tela, "_handle_janela_em_foco", lambda: 999)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: chamadas.append("click"))

    resultado = tela.executar_acao({"acao": "clicar_elemento", "elemento_id": "e7"})

    assert "erro" in resultado
    assert "mudou" in resultado["erro"]
    assert chamadas == []


def test_clicar_elemento_que_sumiu_da_arvore_devolve_erro(monkeypatch):
    chamadas = []
    salvar = Elemento(id="e7", nome="Salvar", tipo="Button", retangulo=(800, 440, 840, 470))
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    _preparar_mapa(monkeypatch, DISPLAY1, [salvar])
    monkeypatch.setattr(tela, "_revalidar", lambda e: "o elemento não está mais na tela")
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))

    resultado = tela.executar_acao({"acao": "clicar_elemento", "elemento_id": "e7"})

    assert "erro" in resultado
    assert chamadas == []


def test_clicar_elemento_por_nome_ambiguo_devolve_erro_util(monkeypatch):
    a = Elemento(id="e1", nome="OK", tipo="Button", retangulo=(0, 0, 10, 10))
    b = Elemento(id="e2", nome="OK", tipo="Button", retangulo=(20, 20, 30, 30))
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    _preparar_mapa(monkeypatch, DISPLAY1, [a, b])

    resultado = tela.executar_acao({"acao": "clicar_elemento", "nome": "OK"})

    assert "erro" in resultado
    assert "OK" in resultado["erro"]


def test_clicar_elemento_no_segundo_monitor_converte_coordenada(monkeypatch):
    # CA8 em forma de teste unitário: elemento com retângulo global no
    # DISPLAY7. Se a conversão sumir, o clique vai pro monitor errado.
    chamadas = []
    botao = Elemento(id="e1", nome="Play", tipo="Button", retangulo=(2000, 400, 2040, 430))
    _fixar_superficies(monkeypatch, [DISPLAY1, DISPLAY7], DISPLAY7)
    _preparar_mapa(monkeypatch, DISPLAY7, [botao])
    monkeypatch.setattr(tela, "_revalidar", lambda e: None)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append((x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)

    tela.executar_acao({"acao": "clicar_elemento", "elemento_id": "e1"})

    # O retângulo já é global, então o centro global é direto (2020,415).
    assert chamadas == [(2020, 415)]
```

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
cd dispositivo && python -m pytest tests/test_tela.py -k clicar_elemento -v
```

Esperado: FAIL — a ação `clicar_elemento` ainda não existe.

- [ ] **Step 3: Implementar em `tela.py`**

Acrescentar o ramo no `executar_acao`:

```python
        if acao == "clicar_elemento":
            return _clicar_elemento(dados)
```

E as funções:

```python
def _revalidar(elemento) -> str | None:
    """Confere o elemento contra a árvore VIVA, não contra o retrato.
    Devolve o motivo da recusa, ou None se pode clicar.

    É esta função que produz o erro real que faltava: sem ela o device
    respondia sucesso incondicional e o modelo nunca sabia que errou.
    Fala com o SO — sem teste automatizado, coberto por CA6."""
    import uiautomation

    x, y = elemento.centro_global()
    try:
        vivo = uiautomation.ControlFromPoint(x, y)
    except Exception as erro:
        return f"não consegui ler a tela nesse ponto: {erro}"

    if vivo is None:
        return "o elemento não está mais na tela"
    if vivo.IsOffscreen:
        return "o elemento está fora da área visível"
    try:
        if not vivo.IsEnabled:
            return "o elemento está desabilitado"
    except Exception:
        pass

    r = vivo.BoundingRectangle
    if (r.left, r.top, r.right, r.bottom) != elemento.retangulo:
        return "o elemento mudou de posição ou de tamanho desde o ver_tela"
    return None


def _clicar_elemento(dados: dict) -> dict:
    superficie, erro = _resolver_superficie(dados)
    if erro:
        return {"erro": erro}

    if _mapa_atual is None:
        return {"erro": "não tenho varredura desta tela — chame ver_tela antes"}

    handle = _handle_janela_em_foco()
    if not _mapa_atual.valido_para(superficie.id, handle):
        return {"erro": "a tela mudou desde o ver_tela — chame ver_tela de novo"}

    identificador = dados.get("elemento_id", "")
    nome = dados.get("nome", "")
    tipo = dados.get("tipo", "")

    if identificador:
        elemento = _mapa_atual.por_id(identificador)
        if elemento is None:
            return {"erro": f"não achei o elemento {identificador} na última varredura"}
    elif nome:
        elemento = _mapa_atual.por_nome(nome, tipo)
        if elemento is None:
            quantos = sum(1 for e in _mapa_atual.elementos if e.nome == nome)
            if quantos > 1:
                return {"erro": f'achei {quantos} elementos chamados "{nome}" — use o elemento_id da lista'}
            return {"erro": f'não achei nenhum elemento chamado "{nome}" na última varredura'}
    else:
        return {"erro": "informe elemento_id ou nome"}

    recusa = _revalidar(elemento)
    if recusa:
        return {"erro": recusa}

    x, y = elemento.centro_global()
    pyautogui.moveTo(x, y, duration=1.0)
    pyautogui.click()

    local_x, local_y = elemento.centro_local(superficie)
    return {"saida": f'cliquei em "{elemento.nome}" ({elemento.tipo}) em ({local_x},{local_y}) de {superficie.id}'}
```

- [ ] **Step 4: Rodar e confirmar que passa**

```bash
cd dispositivo && python -m pytest tests/ -v
```

Esperado: todos passam.

- [ ] **Step 5: Commit**

```bash
git add dispositivo/james_device/tela.py dispositivo/tests/test_tela.py
git commit -m "feat(device): clicar_elemento com revalidacao contra a arvore viva"
```

---

## Task 12: Feedback honesto no clique por coordenada — fecha CA7

**Files:**
- Modify: `dispositivo/james_device/tela.py`
- Test: `dispositivo/tests/test_tela.py`

- [ ] **Step 1: Escrever os testes que falham**

```python
def test_click_reporta_o_elemento_atingido(monkeypatch):
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: None)
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)
    monkeypatch.setattr(tela, "_descrever_ponto", lambda x, y: 'o Button "Salvar"')

    resultado = tela.executar_acao({"acao": "click", "x": 820, "y": 455})

    assert 'Button "Salvar"' in resultado["saida"]


def test_click_em_area_vazia_diz_que_nao_achou_nada(monkeypatch):
    # CA7: o retorno de hoje é sucesso incondicional. Isso é o que faz o
    # modelo nunca perceber que errou.
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: None)
    monkeypatch.setattr(tela.pyautogui, "click", lambda: None)
    monkeypatch.setattr(tela, "_descrever_ponto", lambda x, y: None)

    resultado = tela.executar_acao({"acao": "click", "x": 5, "y": 5})

    assert "nenhum elemento" in resultado["saida"]
```

O `test_executar_acao_click_chama_pyautogui` da Task 2 precisa passar a fixar `_descrever_ponto` também:

```python
def test_executar_acao_click_chama_pyautogui(monkeypatch):
    chamadas = []
    _fixar_superficies(monkeypatch, [DISPLAY1], DISPLAY1)
    monkeypatch.setattr(tela, "_descrever_ponto", lambda x, y: None)
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append(("moveTo", x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: chamadas.append(("click",)))

    resultado = tela.executar_acao({"acao": "click", "x": 100, "y": 200})

    assert resultado["saida"].startswith("cliquei em (100,200)")
    assert ("moveTo", 100, 200) in chamadas
    assert ("click",) in chamadas
```

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
cd dispositivo && python -m pytest tests/test_tela.py -k "reporta or vazia" -v
```

Esperado: FAIL — `_descrever_ponto` não existe.

- [ ] **Step 3: Implementar**

Acrescentar a função e usar no `_clicar`:

```python
def _descrever_ponto(x: int, y: int) -> str | None:
    """O que existe na coordenada GLOBAL (x,y), ou None. Fala com o SO —
    sem teste automatizado, coberto por CA7.

    Vale mesmo no caminho de pixel cru, que é onde não há lista de elementos
    pra consultar: em canvas e jogo, saber que não há nada identificável ali
    já é informação honesta, e é infinitamente melhor que o sucesso
    incondicional de antes."""
    try:
        import uiautomation

        controle = uiautomation.ControlFromPoint(x, y)
    except Exception:
        return None
    if controle is None:
        return None
    nome = controle.Name or ""
    tipo = controle.ControlTypeName or "elemento"
    if not nome:
        return f"um {tipo} sem nome"
    return f'o {tipo} "{nome}"'
```

E o final de `_clicar`:

```python
    global_x, global_y = superficies.para_global(superficie, x, y)
    pyautogui.moveTo(global_x, global_y, duration=1.0)
    pyautogui.click()

    atingido = _descrever_ponto(global_x, global_y)
    onde = f", que é {atingido}" if atingido else ", nenhum elemento identificável nesse ponto"
    return {"saida": f"cliquei em ({x},{y}) de {superficie.id}{onde}"}
```

- [ ] **Step 4: Rodar e confirmar que passa**

```bash
cd dispositivo && python -m pytest tests/ -v
```

Esperado: todos passam.

- [ ] **Step 5: Commit**

```bash
git add dispositivo/james_device/tela.py dispositivo/tests/test_tela.py
git commit -m "fix(device): clique por coordenada reporta o que atingiu, nao sucesso incondicional"
```

---

## Task 13: Log passivo de quase-acertos da wake word

**Files:**
- Modify: `dispositivo/james_device/wake.py`
- Modify: `dispositivo/james_device/main.py`
- Modify: `.gitignore`
- Test: `dispositivo/tests/test_wake.py`

- [ ] **Step 1: Escrever os testes que falham**

Adicionar em `dispositivo/tests/test_wake.py`, seguindo o import direto que o arquivo já usa (`from james_device.wake import contem_wake_word, normalizar` na linha 3 — acrescentar os dois nomes novos lá):

```python
from james_device.wake import (
    contem_wake_word,
    normalizar,
    quase_acerto,
    registrar_quase_acerto,
)


def test_quase_acerto_pega_palavra_parecida():
    # "games" é o falso negativo clássico: o STT em pt-BR devolve isso com
    # frequência e a lista conservadora de VARIANTES_PADRAO não cobre.
    # SequenceMatcher("games", "james") dá 0.8, acima do limiar de 0.6.
    assert quase_acerto("games abre o spotify") == "games"


def test_quase_acerto_ignora_palavra_distante():
    assert quase_acerto("abre a janela da casa") is None


def test_quase_acerto_nao_dispara_quando_a_wake_word_bateu():
    # Se bateu, não é candidato a calibração — é ativação normal.
    assert quase_acerto("james abre o spotify") is None


def test_registrar_quase_acerto_grava_linha(tmp_path):
    caminho = tmp_path / "wake_candidatos.log"

    registrar_quase_acerto("games", caminho=caminho)
    registrar_quase_acerto("jeimis", caminho=caminho)

    linhas = caminho.read_text(encoding="utf-8").strip().splitlines()
    assert len(linhas) == 2
    assert linhas[0].endswith("\tgames")
    assert linhas[1].endswith("\tjeimis")


def test_registrar_quase_acerto_nunca_levanta(tmp_path):
    # Log de calibração é conveniência: falhar ao gravar não pode derrubar a
    # escuta, que é a função principal do device.
    caminho = tmp_path / "nao" / "existe" / "wake.log"
    registrar_quase_acerto("games", caminho=caminho)
```

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
cd dispositivo && python -m pytest tests/test_wake.py -v
```

Esperado: FAIL com `AttributeError: module 'james_device.wake' has no attribute 'quase_acerto'`.

- [ ] **Step 3: Implementar em `wake.py`**

Acrescentar aos imports do topo (`re` e `unicodedata` já estão lá):

```python
import difflib
import logging
from datetime import datetime
from pathlib import Path
```

E ao fim do arquivo:

```python
logger = logging.getLogger(__name__)

# Arquivo de calibração (D10 da spec da Fase 5). Fica fora do Vault de
# propósito: o Vault é lido pelo modelo a cada turno e não deve receber esse
# ruído. Está no .gitignore — é transcrição do que o Arthur falou.
CAMINHO_LOG = Path("dispositivo/wake_candidatos.log")

# 0.6 pega "games" (0.8 contra "james") e deixa passar palavra distante.
# Calibrável depois, com o próprio log em mãos.
LIMIAR_QUASE_ACERTO = 0.6


def quase_acerto(texto: str, variantes: list[str] | None = None) -> str | None:
    """Palavra que chegou perto de uma variante sem bater, ou None.

    Existe porque a alternativa — sessão de calibração guiada — tem um furo
    lógico: o Arthur falando de propósito no microfone é exatamente a
    condição em que a detecção já funciona. Os erros acontecem longe do mic,
    com ruído, na fala rápida. Só o log passivo coleta esses.
    """
    variantes = variantes or VARIANTES_PADRAO
    if contem_wake_word(texto, variantes):
        return None
    for palavra in normalizar(texto).split():
        for variante in variantes:
            if difflib.SequenceMatcher(None, palavra, variante).ratio() >= LIMIAR_QUASE_ACERTO:
                return palavra
    return None


def registrar_quase_acerto(palavra: str, caminho: Path | None = None) -> None:
    """Anexa a palavra ao log de calibração. Nunca levanta: log é
    conveniência, e falhar aqui não pode derrubar a escuta."""
    destino = Path(caminho) if caminho else CAMINHO_LOG
    linha = f"{datetime.now().isoformat(timespec='seconds')}\t{palavra}\n"
    try:
        with open(destino, "a", encoding="utf-8") as arquivo:
            arquivo.write(linha)
    except OSError:
        logger.debug("não consegui gravar o log de wake word em %s", destino, exc_info=True)
```

- [ ] **Step 4: Ligar no loop de escuta**

Em `dispositivo/james_device/main.py`, dentro de `escutar_e_enviar`, trocar o bloco final do `while True`:

```python
            if contem_wake_word(texto):
                playsound("dispositivo/assets/entrada.mp3")
                comando = _ouvir_comando(recognizer, source)
                if comando:
                    conexao.enviar_entrada_voz(comando)
            else:
                candidato = quase_acerto(texto)
                if candidato:
                    registrar_quase_acerto(candidato)
```

E o import:

```python
from james_device.wake import contem_wake_word, quase_acerto, registrar_quase_acerto
```

- [ ] **Step 5: Ignorar o log no git**

Acrescentar ao `.gitignore`:

```
dispositivo/wake_candidatos.log
```

- [ ] **Step 6: Rodar e confirmar que passa**

```bash
cd dispositivo && python -m pytest tests/ -v
```

Esperado: todos passam.

- [ ] **Step 7: Commit**

```bash
git add dispositivo/james_device/wake.py dispositivo/james_device/main.py dispositivo/tests/test_wake.py .gitignore
git commit -m "feat(device): log passivo de quase-acertos da wake word"
```

---

## Task 14: System prompt aponta pra um recurso que existe

**Files:**
- Modify: `prompts/sistema.md:9`
- Test: `internal/api/memoria_test.go` (padrão do espião de system prompt)

- [ ] **Step 1: Escrever o teste que falha**

Criar `internal/api/vertela_prompt_test.go`:

```go
package api

import (
	"os"
	"strings"
	"testing"
)

// O item de visão do system prompt instruía o modelo a analisar um print
// que nada produzia — o encanamento existia, mas nenhum caminho o
// alimentava. Agora que ver_tela existe, o prompt tem que nomear a
// ferramenta, senão continua sendo instrução sem referente.
func TestSystemPromptNomeiaVerTela(t *testing.T) {
	conteudo, err := os.ReadFile("../../prompts/sistema.md")
	if err != nil {
		t.Fatalf("erro ao ler o system prompt: %v", err)
	}

	texto := string(conteudo)
	if !strings.Contains(texto, "ver_tela") {
		t.Error("o system prompt não menciona ver_tela")
	}
	if !strings.Contains(texto, "manipular_interface") {
		t.Error("o system prompt não liga ver_tela a manipular_interface — a sequência é o que evita o palpite de coordenada")
	}
}
```

- [ ] **Step 2: Rodar e confirmar que falha**

```bash
go test ./internal/api/ -run SystemPrompt -v
```

Esperado: FAIL com "o system prompt não menciona ver_tela".

- [ ] **Step 3: Reescrever o item 2 de `prompts/sistema.md`**

Substituir:

```
2. VISÃO PROATIVA: se houver um print da tela, analise antes de agir.
```

Por:

```
2. OLHE ANTES DE CLICAR: você não enxerga a tela por padrão. Antes de
   qualquer manipular_interface, chame ver_tela — ela devolve o print e a
   lista de elementos clicáveis, cada um com id, nome, tipo e posição.
   Depois clique pelo id ou pelo nome do elemento, nunca por coordenada
   adivinhada. Coordenada crua é último recurso, só quando o alvo não
   aparecer na lista (jogo, canvas, área desenhada).
```

- [ ] **Step 4: Rodar e confirmar que passa**

```bash
go test ./internal/api/ -run SystemPrompt -v
```

Esperado: PASS.

- [ ] **Step 5: Commit**

```bash
git add prompts/sistema.md internal/api/vertela_prompt_test.go
git commit -m "feat(prompt): item de visao passa a nomear ver_tela e a sequencia certa"
```

---

## Task 15: Verificação ao vivo — fecha CA3 a CA8

Nenhum critério ao vivo pode ser declarado fechado sem a saída real colada no commit. A regra do projeto é explícita: nada é declarado funcionando sem o comando ter sido rodado e a saída lida.

**Files:** nenhum alterado, salvo correções que a verificação revelar.

- [ ] **Step 1: Subir a infraestrutura**

```bash
docker compose up -d && go run ./cmd/jamesd
```

Esperado: sobe sem erro de embed e sem erro de conexão com o Postgres.

- [ ] **Step 2: Conectar o device**

Noutro terminal:

```bash
cd dispositivo && python -m james_device.main
```

Esperado: log `conectado ao jamesd em ws://127.0.0.1:8080/ws/dispositivo`.

- [ ] **Step 3: CA3 — conferir a fórmula de tiles contra a API real**

A aritmética de `EstimarTokensImagem` veio da documentação e do exemplo 960x540, **não de medição**. Este passo é o que a torna verificada.

Primeiro, conferir a assinatura de `CountTokens` no SDK instalado, em vez de assumir:

```bash
go doc google.golang.org/genai.Models.CountTokens
```

Criar `scripts/verificar_tokens_imagem/main.go` com a assinatura que o `go doc` mostrou. O esqueleto, seguindo como o projeto já cria o cliente em [`gemini.go:24`](../../../internal/agente/gemini/gemini.go):

```go
// Comando verificar_tokens_imagem confere a tabela de tiles de
// memoria.EstimarTokensImagem contra o que a API do Gemini realmente cobra.
// Existe porque a fórmula foi derivada da documentação, e documentação não é
// medição. Se divergir, a fórmula cede — não a API.
package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"

	"google.golang.org/genai"

	"james/internal/memoria"
)

func pngDe(largura, altura int) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, largura, altura)))
	return buf.Bytes()
}

func main() {
	ctx := context.Background()
	cliente, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  os.Getenv("GEMINI_API_KEY"),
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		fmt.Println("erro ao criar cliente:", err)
		os.Exit(1)
	}

	casos := []struct{ largura, altura int }{{300, 200}, {960, 540}, {1920, 1080}}
	for _, c := range casos {
		imagem := pngDe(c.largura, c.altura)
		contents := []*genai.Content{{Role: "user", Parts: []*genai.Part{
			{InlineData: &genai.Blob{MIMEType: "image/png", Data: imagem}},
		}}}

		resp, err := cliente.Models.CountTokens(ctx, "gemini-2.5-flash", contents, nil)
		if err != nil {
			fmt.Printf("%dx%d: erro %v\n", c.largura, c.altura, err)
			continue
		}
		nosso := memoria.EstimarTokensImagem(c.largura, c.altura)
		fmt.Printf("%dx%d: API=%d nosso=%d diferenca=%d\n",
			c.largura, c.altura, resp.TotalTokens, nosso, int(resp.TotalTokens)-nosso)
	}
}
```

Rodar:

```bash
go run ./scripts/verificar_tokens_imagem
```

Esperado: as três linhas com `diferenca` pequena (a API cobra alguns tokens de overhead do envelope além da imagem; o que importa é a ordem de grandeza bater — 258 e 1548, não 75.000).

Se a divergência for grande, **ajustar `EstimarTokensImagem` ao que a API respondeu**, atualizar os valores esperados em `TestEstimarTokensImagemSegueTabelaDoGemini` e rodar `go test ./internal/memoria/` de novo. Colar a saída real no commit.

- [ ] **Step 4: CA4 — `ver_tela` de verdade**

Abrir o Bloco de Notas no monitor primário e pedir ao James, por voz ou pela SPA: "o que tem na minha tela?"

Esperado: o `ver_tela` dispara, o cabeçalho da saída declara `DISPLAY1 ... 1920x1080`, e a lista traz elementos reconhecíveis do Bloco de Notas.

- [ ] **Step 5: CA5 — clique por nome**

Com uma caixa de diálogo aberta (Salvar Como, por exemplo): "clica no botão Cancelar".

Esperado: a caixa fecha. Confirmar na tela, não no log.

- [ ] **Step 6: CA6 — o critério mais importante**

Pedir `ver_tela`, **trocar de janela à mão**, e então pedir o clique num elemento da lista anterior.

Esperado: erro nomeado ("a tela mudou desde o ver_tela"), e o mouse **não se move**. Se clicar mesmo assim, o bug central da fase continua vivo e a fase não fecha.

- [ ] **Step 7: CA7 — clique em área vazia**

"clica na coordenada 5,5" com a área do desktop vazia naquele ponto.

Esperado: a resposta contém "nenhum elemento identificável nesse ponto", não um sucesso seco.

- [ ] **Step 8: CA8 — o segundo monitor**

Mover uma janela para o `DISPLAY7`, deixá-la em foco, e pedir `ver_tela` seguido de um clique por elemento.

Esperado: o cabeçalho declara `DISPLAY7 ...` com a resolução real que `superficies.listar()` reportar para essa máquina (confirmada na Task 1 como **1920x1080 @ escala 150%**, não os `1280x720 @ 100%` que uma medição sem DPI awareness mostraria — não travar a expectativa num número específico aqui; o que importa é que bata com o que a própria `listar()` já provou devolver). O clique acerta o alvo **no segundo monitor**. Se a soma de origem `(1920,255)` estiver errada, o mouse vai para o monitor primário — este é o teste que pega isso.

- [ ] **Step 9: Rodar a suíte inteira uma última vez**

```bash
go test ./... && cd dispositivo && python -m pytest tests/ -v
```

Esperado: todos os pacotes `ok` e todos os testes Python passando.

- [ ] **Step 10: Commit de fechamento**

```bash
git add -A
git commit -m "docs: fecha a Fase 5 com verificacao ao vivo de CA3 a CA8"
```

Colar no corpo do commit a saída real de cada critério — em especial a do CA6 e a do CA8, que são os dois que provam que o bug do Anexo A #20 morreu.

---

## Riscos durante a execução

| Risco | Sinal | O que fazer |
|---|---|---|
| Varredura UIA lenta demais (R1) | `ver_tela` estoura os 30s do `TimeoutPorAcao` | Reduzir `PROFUNDIDADE_MAXIMA` em `elementos.py`, e cortar por área mínima do elemento. Medir antes de mexer |
| Lista de elementos gigante (R2) | Saída de `ver_tela` com centenas de linhas | Apertar `TIPOS_INTERESSANTES`, e descartar elemento com área abaixo de um piso |
| `uiautomation` falha em app real (D4) | Exceção ou árvore vazia em app específico | A biblioteca só é tocada em `elementos.varrer` e `_revalidar`/`_descrever_ponto` — trocar por `pywinauto` é localizado |
| Ids de monitor com formato diferente do esperado | `listar()` devolve `\\.\DISPLAY1` em vez de `DISPLAY1` | Ajustar o `replace` em `superficies.listar` (Step 5 da Task 1 existe pra pegar isso cedo) |
| CA6 falha: o clique acontece mesmo com a tela mudada | O mouse se move no Step 6 da Task 15 | Não seguir. É o bug central da fase; investigar `Mapa.valido_para` e `_handle_janela_em_foco` antes de qualquer outra coisa |
