# Tela e janelas no Wayland (B2): plano de implementação

> **Para quem executa:** um task por vez, na ordem. TDD em todos que têm código: escrever o teste, rodar e **ver falhar pelo motivo certo**, implementar, rodar e ver passar, commitar. A saída de cada "rodar" vai no relato. Não rodou, diz que não rodou. Modo trabalho: `/code-review` e `/security-review` antes do merge (Task 19).

**Objetivo:** no Fedora (KDE, Wayland), o James foca janelas, controla mídia, abre coisas, vê a tela, clica e escreve em português, pedindo aprovação só para ação destrutiva. O Windows não muda.

**Spec:** [`../specs/2026-10-13-james-tela-wayland-design.md`](../specs/2026-10-13-james-tela-wayland-design.md) (aprovada em 2026-10-13).
**Spikes:** [`../spikes/2026-10-13-tela-wayland.md`](../spikes/2026-10-13-tela-wayland.md) e os scripts em `../spikes/2026-10-13-tela-wayland/`. **São a referência de como cada API se comporta de verdade**; reaproveitar o que funcionou lá.
**Branch:** `feat/tela-wayland`.

---

## Antes de começar: leia isto

Fatos medidos nos spikes. Cada um já custou tempo uma vez.

1. **Use `gi.repository.Atspi` direto, não `pyatspi`.** O `pyatspi` é wrapper do sistema (Python 3.14) e não entra no venv (Python 3.13, mise). Nomes conferidos por introspecção: `Atspi.get_desktop(0)`, `acc.get_child_count()`, `acc.get_child_at_index(i)`, `acc.get_role_name()`, `acc.get_name()`, `acc.get_state_set().contains(Atspi.StateType.SHOWING)`, `acc.get_extents(Atspi.CoordType.WINDOW)`, `acc.get_process_id()`, `acc.get_text_iface()`, `acc.get_editable_text_iface()`, `Atspi.Text.get_text(t, 0, -1)`, `Atspi.EditableText.insert_text(e, pos, texto, len_bytes)`. Antes de usar qualquer outro, confira com `python -c "from gi.repository import Atspi; help(Atspi.X)"`. Não adivinhe.
2. **Papel de botão é `"button"`** no AT-SPI atual (o antigo `"push button"` ainda aparece). No spike, `pyatspi.ROLE_BUTTON` não existia, o critério levantou `AttributeError` dentro do `try` e a subárvore inteira foi pulada em silêncio. **Compare pelo nome do papel, e nunca deixe um `except` engolir erro de programação** (pegue só `GLib.Error` nas chamadas ao AT-SPI).
3. **Coordenada do AT-SPI no Wayland é relativa à janela** (`CoordType.WINDOW`); a origem vem do `clientGeometry` do KWin. Exceção: o plasmashell (painel, layer-shell) já vem global.
4. **`print()` de script do KWin não aparece em lugar nenhum** (categoria de log desligada). A saída volta por `callDBus("<nome do device>", "/", "<interface>", "receber", texto)` para um objeto D-Bus que o device registra.
5. **`node_id` dos fluxos do ScreenCast muda entre sessões.** Case fluxo e monitor **pela posição**.
6. **Keysym não serve para texto:** com layout `us`, `ç` virou `<` e `ã` sumiu. Texto = `insert_text`, com fallback para colar.
7. **A barra de endereço do Firefox ignora `insert_text` e `set_text_contents`**: devolvem `True` e nada muda. Por isso a estratégia sempre **relê** depois de escrever.
8. **`wl-copy` vira daemon e segura o stdout herdado**: o comando do spike "travou" por isso. Use o portal `Clipboard` (D6), não `wl-copy` em subprocesso.
9. **App aberto antes de ligar a acessibilidade não monta a árvore.** Firefox e Qt precisaram ser reabertos; VS Code e Spotify não expõem nem reabertos.
10. **`kwrite --new` não existe nesse KWrite**: o processo sai na hora. O KWrite abre na tela de boas-vindas, sem editor: clique em "Novo" primeiro. Serve de alvo de teste ao vivo.
11. **Restore token do portal** = controle de teclado e mouse sem perguntar. Arquivo 0600, nunca em log, nunca no repositório. O do spike está no scratchpad da sessão: a implementação gera o seu.
12. **Arquivos com CRLF** (vieram do Windows): `gramatica.py`, `wake.py`, `test_escuta.py`, `test_gramatica.py`, `test_wake.py`, `prompts/sistema.md`. O `Edit` falha neles; edite preservando o fim de linha.
13. **`pkill -f`/`pgrep -f` casam com a própria linha de comando.** Mate por PID.
14. **Comandos de suíte:**
    - Go (com o Postgres de pé, `docker compose up -d`):
      ```bash
      go vet ./... && GOOS=windows go vet ./... && GOOS=windows go build ./... && go test -count=1 ./...
      ```
    - Device:
      ```bash
      dispositivo/.venv/bin/python -m pytest -q -rs dispositivo
      ```
    - Front:
      ```bash
      npm test --prefix web
      ```
    Base em `main` (`a8061c0`): Go verde, pytest **249 passed, 1 skipped**, vitest **117 passed**.
15. **Testes que falam com o desktop de verdade** ficam atrás de `JAMES_TESTE_DESKTOP=1` (pulados por padrão, como o `JAMES_TESTE_ANTHROPIC`). Eles abrem e fecham um KWrite próprio e **nunca** mexem em outra janela do Arthur além de focar.

---

## Estrutura de arquivos

```
dispositivo/pyproject.toml                          MODIFICAR  PyGObject
dispositivo/james_device/
  superficies.py           MODIFICAR  Superficie.fator; para_global/para_local com fator
  laco_glib.py             CRIAR      GLib.MainLoop numa thread; executar(fn, timeout)
  kwin.py                  CRIAR      script do KWin, parsing, Janela, focar/fechar, por_alvo
  portal.py                CRIAR      sessão RemoteDesktop+ScreenCast+Clipboard, token, mouse, teclado
  teclas.py                CRIAR      nome/combinação -> keysyms (puro)
  captura_linux.py         CRIAR      quadro do ScreenCast -> PNG de 1280 px
  acessibilidade.py        CRIAR      ligar IsEnabled; varrer janelas -> elementos.Elemento
  texto.py                 CRIAR      digitar: insert_text -> reler -> colar (restaura)
  midia.py                 CRIAR      MPRIS
  abrir.py                 CRIAR      URL, arquivo, pasta, app, code -g
  tela_linux.py            CRIAR      executar_acao(dados): mesmo contrato de tela.py
  plataforma/linux.py      MODIFICAR  TELA_DISPONIVEL por detecção; ponte_de_tela -> tela_linux
  plataforma/windows.py    MODIFICAR  ponte responde "indisponível" às ações novas
  main.py                  MODIFICAR  sobe laço, acessibilidade e portal no Linux
dispositivo/tests/test_{laco_glib,kwin,portal,teclas,captura_linux,acessibilidade,texto,midia,abrir,tela_linux}.py  CRIAR
dispositivo/tests/test_superficies.py, test_plataforma.py, test_main_voz.py                                  MODIFICAR
internal/dispositivo/hub.go, hub_test.go           MODIFICAR  AcaoTela.Alvo/Linha, Resultado.Elementos, último mapa
internal/api/dispositivo.go, _test.go              MODIFICAR  decodificar "elementos"
internal/ferramentas/destrutivo.go, _test.go       CRIAR      vocabulário destrutivo (puro)
internal/ferramentas/dispositivo.go, _test.go      MODIFICAR  ManipularInterface implementa ExigeConfirmacao
internal/ferramentas/janelas.go, midia.go, abrir.go (+ _test.go)   CRIAR
internal/ferramentas/tipos.go                      MODIFICAR  interface DescreveConfirmacao
internal/api/ws.go, _test.go                       MODIFICAR  descrição no pedido de aprovação
web/src/...                                        MODIFICAR  cartão de aprovação mostra a descrição
cmd/jamesd/main.go                                 MODIFICAR  registrar janelas, midia, abrir
prompts/sistema.md                                 MODIFICAR  quando usar cada ferramenta (CRLF)
```

---

## Fatia 0: ambiente

### Task 0: PyGObject no venv do device (PORTÃO)

Sem código de produto. A prova são os comandos. **Se o Passo 2 falhar, pare e volte à D12 da spec com o Arthur.** Não improvise outro caminho.

- [ ] **Passo 1 (o Arthur roda, precisa de sudo):**
  ```bash
  sudo dnf install gobject-introspection-devel glib2-devel
  ```
- [ ] **Passo 2:** `pyproject.toml` do device ganha `PyGObject>=3.50`, com comentário: os typelibs (`Atspi-2.0`, `Gst-1.0`, `Gio-2.0`) vêm do sistema; o `pyatspi` do dnf não serve porque é para o Python do sistema. Reinstalar o venv e provar:
  ```bash
  dispositivo/.venv/bin/python -c "import gi; gi.require_version('Atspi','2.0'); gi.require_version('Gst','1.0'); from gi.repository import Atspi, Gst, Gio, GLib; Gst.init(None); print('ok', gi.__version__, Gst.version_string())"
  ```
  Esperado: `ok` com as versões. Também `pycairo`, se o pip puxar, tem de compilar (o `cairo-devel` já está instalado).
- [ ] **Passo 3:** pytest do device continua **249 passed, 1 skipped**.
- [ ] **Passo 4: commit.**
  ```bash
  git add dispositivo/pyproject.toml && git commit -m "build(device): PyGObject para AT-SPI, GStreamer e D-Bus no Linux"
  ```

---

## Fatia 1: blocos do device (Python)

### Task 1: `Superficie.fator` (D9)

- **Arquivos:** `superficies.py`, `tests/test_superficies.py`.
- **Testes novos:**
  - `fator` padrão é `1.0`, e todos os testes atuais de `para_global`/`para_local` continuam valendo sem mudança (Windows).
  - Com `fator=1.5` (HDMI-A-1: lógico 1920 → imagem 1280): `para_global(s, 640, 360) == (960, 540)`; `para_local` é o inverso.
  - Com o HDMI-A-2 (origem 1920,588; lógico 1422x800; imagem 1280x720; `fator = 1422/1280`): o centro da imagem vai para `(2631, 988)`, que é o centro medido no spike.
  - Arredondamento: `para_global` devolve `int` arredondado (não truncado). Ponto (1279, 719) continua dentro do monitor.
  - Nova função pura `tamanho_da_imagem(largura_logica, altura_logica, largura=1280) -> (w, h, fator)`.
- **Implementação:** campo `fator: float = 1.0` no fim da dataclass (não quebra construção posicional). `largura`/`altura` da `Superficie` do Linux são os **da imagem**.
- **Commit:** `feat(device): Superficie.fator para coordenada no espaço da imagem`

### Task 2: laço GLib numa thread (D12)

- **Arquivos:** `laco_glib.py`, `tests/test_laco_glib.py` (usa GLib de verdade, sem desktop).
- **Testes:**
  - `executar(fn)` roda `fn` na thread do laço (confere `threading.get_ident()`) e devolve o valor.
  - Exceção dentro de `fn` chega a quem chamou, com o tipo original.
  - `fn` que não termina → `TimeoutError` em `timeout` segundos, e o laço continua atendendo a próxima chamada.
  - `esperar_sinal`: helper que registra uma assinatura, devolve um objeto que espera com timeout. Teste com um sinal emitido por um objeto D-Bus de teste no barramento de sessão próprio (`Gio.TestDBus`).
  - `encerrar()` para o laço e a thread termina.
- **Implementação:** `GLib.MainContext` próprio + `GLib.MainLoop`, `GLib.idle_add`/`context.invoke_full` para marshal, `threading.Event` para a volta. Nenhuma chamada D-Bus do device sai fora deste laço (R9).
- **Commit:** `feat(device): laço GLib em thread própria para o D-Bus`

### Task 3: KWin (D2)

- **Arquivos:** `kwin.py`, `tests/test_kwin.py`.
- **Testes (puros, com executor de script falso):**
  - O JS gerado para `listar` contém `workspace.windowList()` e devolve uma linha por janela com campos separados por um separador que não aparece em título (use `\x1f`). Título com `|`, aspas e acento sobrevive ao parsing.
  - Parsing → `Janela(id, app, titulo, pid, ativa, minimizada, normal, monitor, frame, cliente)`; `id` é o `internalId` do KWin (UUID).
  - `telas()` → nome, geometria lógica, escala.
  - `por_alvo(janelas, "firefox")` casa por app ou título, sem caixa e sem acento; ignora janela não normal; dois casamentos → devolve os dois (quem chama decide); zero → vazio.
  - `focar(id)` gera JS que ativa a janela pelo `internalId` e devolve o id da ativa; `focar` falha com mensagem clara se o id não existir mais.
  - `fechar(id)` usa `closeWindow()`.
  - **Texto do modelo nunca entra no JS.** O casamento por alvo (`por_alvo`) roda em Python sobre a lista; o JS de `focar`/`fechar` só recebe um `internalId` que passou por regex de UUID (`^\{?[0-9a-f-]{36}\}?$`). Id fora do formato → erro, sem gerar script. Teste com alvo `'"); workspace.activeWindow.closeWindow(); ("'`: nenhum script é gerado com esse texto.
  - Script que não responde em 3 s → erro claro, e o script é descarregado (`unloadScript`) **sempre**.
- **Implementação:** como o `kwin()` do `spike2_portal_entrada.py`, mas dentro do laço da Task 2, com nome de script único por chamada e arquivo temporário apagado no `finally`.
- **Ao vivo (atrás de `JAMES_TESTE_DESKTOP=1`):** `listar` traz a janela do próprio terminal do teste; `focar` numa janela de KWrite aberta pelo teste confirma `ativa`.
- **Commit:** `feat(device): janelas pelo script do KWin`

### Task 4: portal RemoteDesktop + ScreenCast + Clipboard (D5, D5a, D8)

- **Arquivos:** `portal.py`, `tests/test_portal.py`.
- **Testes (puros e com portal falso):**
  - Token: grava com modo **0600** (conferir `stat`), escrita atômica (arquivo temporário + `os.replace`), lê ignorando espaço; token ausente → sessão pede diálogo.
  - `casar_fluxos(fluxos, telas)` por posição: os dois monitores do spike casam mesmo com `node_id` trocado; fluxo sem tela correspondente vira erro.
  - `para_fluxo(gx, gy)`: ponto global lógico → (node, x relativo, y relativo); ponto fora de todo monitor → erro.
  - Estado `SEM_PERMISSAO` (resposta do portal com código ≠ 0): toda ação devolve `"preciso de permissão: clique em Permitir no aviso do KDE"`.
  - Sessão fechada por fora (sinal `Closed`): a próxima ação tenta reabrir **uma** vez com o token; se falhar, `SEM_PERMISSAO`.
  - `clicar` manda botão 0x110 apertado e solto; `tecla(keysyms)` aperta em ordem e solta na ordem inversa.
  - Clipboard: `RequestClipboard` entra **antes** do `Start`; `gravar(texto)` e `ler()` funcionam pelo portal falso.
- **Ao vivo (`JAMES_TESTE_DESKTOP=1`):** abre a sessão com o token, mostra os dois fluxos e move o mouse para o centro de cada monitor; confere pelo KWin (erro ≤ 1 px).
- **Implementação:** como `abrir_sessao`/`pedir`/`notificar` do spike, dentro do laço. Token em `~/.local/share/james/portal-token`.
- **Commit:** `feat(device): sessão do portal com permissão persistente`

### Task 5: teclas (puro)

- **Arquivos:** `teclas.py`, `tests/test_teclas.py`.
- **Testes (tabela):** `enter`, `tab`, `esc`, `space`, `backspace`, `delete`, setas, `home`, `end`, `pageup`, `f1`–`f12`; combinações `ctrl+l`, `ctrl+shift+t`, `alt+tab`, `super`; caixa e espaços ignorados (`"Ctrl + L"`); letra simples em combinação (`ctrl+v` → `0xFFE3, ord('v')`); nome desconhecido → erro com a lista de nomes aceitos; **frase** (`"olá mundo"`) → erro "tecla aceita só uma tecla ou combinação; para texto use digitar".
- **Commit:** `feat(device): mapa de teclas para keysym`

### Task 6: captura (D9)

- **Arquivos:** `captura_linux.py`, `tests/test_captura_linux.py`.
- **Testes:**
  - O pipeline gerado tem `pipewiresrc fd=<fd> path=<node> num-buffers=2 always-copy=true`, `videoscale` com caps `width=1280,height=<h>` e `pngenc snapshot=true`.
  - `h` vem de `tamanho_da_imagem` (Task 1): 720 para os dois monitores do Arthur.
  - Erro do GStreamer ou timeout (10 s) → exceção com a mensagem do GStreamer; o pipeline vai para `NULL` sempre.
- **Ao vivo (`JAMES_TESTE_DESKTOP=1`):** captura cada monitor; PNG 1280x720; desvio padrão de cinza > 5 (não é quadro preto). O teste **não grava** a imagem em disco fora do `tmp_path`.
- **Commit:** `feat(device): captura de monitor pelo ScreenCast`

### Task 7: acessibilidade (D7, D11)

- **Arquivos:** `acessibilidade.py`, `tests/test_acessibilidade.py`.
- **Testes (com árvore falsa que imita a interface do `Atspi.Accessible`):**
  - `ligar()` faz `Set org.a11y.Status IsEnabled true` e devolve se já estava ligada.
  - `varrer(apps, janelas_kwin, superficie)`:
    - só nós `SHOWING` de papel interessante, com área > 0;
    - retângulo WINDOW + origem do cliente da janela KWin → global lógico; janela não normal (plasmashell) → retângulo já global (R4);
    - casamento app AT-SPI ↔ janela KWin por pid + título; pid diferente (flatpak) → só título (R1);
    - elementos da janela ativa marcados `em_foco`;
    - só entra elemento cujo centro cai na superfície;
    - teto de nós por app e de tempo total (400 ms, relógio injetado): estoura → devolve o que achou + aviso;
    - `GLib.Error` num nó pula o nó e conta; **`AttributeError`/`TypeError` não é engolido** (fato 2).
  - `tipo` exibido: `"button"` → `"Button"`, `"push button"` → `"Button"`, `"entry"` → `"Edit"`, `"combo box"` → `"ComboBox"`, `"link"` → `"Hyperlink"`, `"menu item"` → `"MenuItem"`, `"page tab"` → `"TabItem"`, `"check box"` → `"CheckBox"`, `"list item"` → `"ListItem"`, `"tree item"` → `"TreeItem"`, `"slider"` → `"Slider"`. Assim o vocabulário de tipos fica igual ao do Windows, sem o sufixo `Control`.
  - `app_expoe(janela_ativa)`: False quando o app da janela ativa não está no barramento (D11).
  - `revalidar(elemento)` contra a árvore viva: sumiu, não está mais `SHOWING`, não está `ENABLED` ou mudou de retângulo → motivo da recusa.
- **Ao vivo (`JAMES_TESTE_DESKTOP=1`):** KWrite aberto pelo teste aparece com o botão "Novo".
- **Commit:** `feat(device): varredura de elementos pelo AT-SPI`

### Task 8: texto (D6)

- **Arquivos:** `texto.py`, `tests/test_texto.py`.
- **Testes (campo, área de transferência e teclado falsos):**
  - `insert_text` funciona → o texto entra, a área de transferência **não é tocada**.
  - `insert_text` devolve True mas o texto não muda (caso Firefox) → cola: grava, Ctrl+V, relê, confere; área de transferência devolvida com o conteúdo anterior.
  - Colar falha (texto não aparece) → erro honesto e a área de transferência **ainda** é devolvida.
  - Área de transferência original vazia → depois de colar, fica vazia de novo (não fica o texto do James).
  - Campo em foco sem interface de texto (app sem AT-SPI) → vai direto para colar.
  - Texto com `ç`, `ã`, travessão, emoji e quebra de linha sai igual.
- **Ao vivo (`JAMES_TESTE_DESKTOP=1`):** no KWrite do teste, `"Olá, ação, coração e maçã — 100%"` sai exato.
- **Commit:** `feat(device): escrever texto sem depender do layout do teclado`

### Task 9: mídia (D3)

- **Arquivos:** `midia.py`, `tests/test_midia.py`.
- **Testes (barramento falso):**
  - Lista players por `ListNames` com prefixo `org.mpris.MediaPlayer2.`.
  - Escolha: o pedido pelo nome; senão o que está `Playing`; senão o único; vários parados sem nome → lista e pede para escolher.
  - `tocar`/`pausar`/`alternar`/`proxima`/`anterior` chamam `Play`/`Pause`/`PlayPause`/`Next`/`Previous`.
  - `estado` → `"Spotify: tocando 'Hopeful' de Ajmw"` a partir do `Metadata` e do `PlaybackStatus`.
  - Nenhum player → `"nenhum player de mídia aberto"`.
  - `CanGoNext=false` → avisa em vez de fingir sucesso.
- **Commit:** `feat(device): controle de mídia pelo MPRIS`

### Task 10: abrir (D4)

- **Arquivos:** `abrir.py`, `tests/test_abrir.py`.
- **Testes (lançadores falsos):**
  - Classificação: `https://...`/`http://...` → URL; caminho existente (com `~` expandido) → arquivo ou pasta; o resto → nome de app.
  - URL e arquivo → app padrão. Pasta → app padrão de pasta (Dolphin).
  - App: busca nos `.desktop` sem caixa e sem acento; um resultado → abre; vários → lista; nenhum → erro com sugestão.
  - `linha` com arquivo → `["code", "-g", f"{arquivo}:{linha}"]` (lista de argumentos, **nunca** shell); `linha` sem arquivo existente → erro.
  - Caminho inexistente que parece caminho (`/` ou `~/` no começo) → erro "não existe", não tenta abrir como app.
- **Commit:** `feat(device): abrir app, arquivo, pasta e URL`

### Task 11: `tela_linux.executar_acao`

- **Arquivos:** `tela_linux.py`, `tests/test_tela_linux.py`.
- **Testes (todos os colaboradores falsos):**
  - **Nunca levanta**: qualquer exceção vira `{"erro": ...}` (mesma regra do `tela.py`).
  - `ver_tela`: monitor pedido ou o da janela ativa; cabeçalho com id, tamanho da imagem e escala; lista formatada pelo `elementos.formatar` com coordenada no espaço da imagem; **`elementos: [{id, nome, tipo}]`** no resultado; varredura falhando não custa a imagem; app sem árvore → aviso da D11 e só a imagem; sem permissão do portal → mensagem da D5a.
  - `click`: fora dos limites da imagem → erro; **cai em elemento listado → recusa "use clicar_elemento com o id eX"** (D10 d); senão converte e clica, descreve o ponto.
  - `clicar_elemento`: sem mapa → "chame ver_tela antes"; mapa de outro monitor ou outra janela ativa → "a tela mudou"; por id e por nome (nome repetido → pede id); revalidação recusa → mensagem; **foca a janela dona antes de clicar** (ordem conferida no falso).
  - `digitar` → `texto.py`; `tecla` → `teclas.py` (frase vira erro).
  - `janelas_listar`/`janela_focar`/`janela_fechar` → `kwin.py`; alvo ambíguo devolve a lista.
  - `midia` → `midia.py`; `abrir` → `abrir.py`.
  - Ação desconhecida → erro com o nome.
  - Cada ação loga a duração (CA12).
- **Commit:** `feat(device): ponte de tela do Linux`

### Task 12: fiação no device e no Windows (D11, D13)

- **Arquivos:** `plataforma/linux.py`, `plataforma/windows.py`, `main.py`, `tests/test_plataforma.py`, `tests/test_main_voz.py`.
- **Testes:**
  - `TELA_DISPONIVEL` no Linux: `True` só com `XDG_SESSION_TYPE=wayland`, `XDG_CURRENT_DESKTOP` contendo `KDE` e `gi` importável; cada condição faltando → `False` com o motivo no log (GNOME, X11, sem `gi`).
  - Ao subir com tela: liga a acessibilidade (loga se ligou agora e avisa para reabrir apps abertos antes), inicia o laço, abre o portal; falha em qualquer passo → tela desligada, **voz segue** (mesma regra do `configurar_ou_degradar`).
  - Windows: a ponte responde `"indisponível neste sistema (Windows)"` a `janelas_*`, `midia` e `abrir`, e delega todo o resto ao `tela.executar_acao` **sem mudança** (teste com `tela` falso; roda no Linux).
- **Ao vivo:** `./executar.sh` sobe e o log do device mostra a tela ligada no lugar de `ponte de tela desligada`.
- **Commit:** `feat(device): ponte de tela ligada no KDE Wayland`

---

## Fatia 2: jamesd (Go)

### Task 13: protocolo e último mapa

- **Arquivos:** `internal/dispositivo/hub.go`, `hub_test.go`, `internal/api/dispositivo.go`, `dispositivo_test.go`.
- **Testes:**
  - `AcaoTela` serializa `alvo` e `linha` só quando preenchidos (`omitempty`): JSON das ações antigas fica **byte a byte igual**.
  - `decodificarResultadoAcao` lê `elementos: [{id, nome, tipo}]`; ausente → lista vazia; malformado → segue sem elementos, com log (mesma tolerância da imagem).
  - `Resultado.Elementos` chega a quem chamou `EnviarAcao`.
  - O Hub guarda o último mapa quando a ação foi `ver_tela` e devolve com `UltimoMapa()`; troca a cada `ver_tela`; **zera quando o device desconecta** (device novo não herda o mapa do velho).
- **Commit:** `feat(dispositivo): elementos no resultado do ver_tela e último mapa no Hub`

### Task 14: vocabulário destrutivo e aprovação do `manipular_interface` (D10)

- **Arquivos:** `internal/ferramentas/destrutivo.go`, `destrutivo_test.go`, `dispositivo.go`, `dispositivo_test.go`.
- **Testes:**
  - `Destrutivo(nome)`: casa `Excluir`, `excluir arquivo`, `Mover para a lixeira`, `Esvaziar lixeira`, `Apagar`, `Remover`, `Deletar`, `Delete`, `Remove`, `Descartar`, `Desinstalar`, `Formatar`, `Não salvar`, `Don't Save`; sem caixa e sem acento (`EXCLUÍR`/`excluir`). **Não** casa: `Removedor de manchas` (palavra inteira), `Salvar`, `Abrir`, `Excelente`.
  - `ConfirmacaoNecessaria`:
    - `clicar_elemento` com `elemento_id` cujo nome no último mapa é destrutivo → `true`; com nome inócuo → `false`;
    - `clicar_elemento` com `nome` destrutivo nos argumentos → `true`, mesmo sem mapa;
    - `elemento_id` que não está no mapa → `false` (o device recusa sozinho);
    - `tecla` `delete`, `Delete`, `shift+delete` → `true`; `enter` → `false`;
    - `click`, `digitar` → `false`.
  - `NovoManipularInterface` recebe algo que dá o último mapa (interface pequena, o Hub implementa).
- **Commit:** `feat(ferramentas): aprovação para clique e tecla destrutivos`

### Task 15: ferramentas `janelas`, `midia`, `abrir` (D14)

- **Arquivos:** `internal/ferramentas/{janelas,midia,abrir}.go` e `_test.go`, `cmd/jamesd/main.go`, `prompts/sistema.md` (CRLF).
- **Testes (ponte falsa):**
  - Declaração com descrições em português e `acao` com valores fechados; argumento faltando → erro claro sem chamar a ponte.
  - Ações mapeiam para `janelas_listar`, `janela_focar`, `janela_fechar`, `midia` (subação em `Texto`), `abrir` (`Alvo`, `Linha`).
  - `Nivel`: `janelas` e `abrir` → `NivelPerigoso`; `midia` → `NivelSeguro`.
  - `Janelas.ConfirmacaoNecessaria` → `true` só para `fechar`.
  - `main.go` registra as três; o teste de registro existente (se houver) conta as novas.
- **Prompt:** regra de preferência da spec 5.3, mais "para digitar texto use `digitar`; `tecla` é só uma tecla ou atalho".
- **Ao vivo (opcional, gasta centavos):** rodar o `integracao_test.go` existente e conferir que nada piorou (10 de 10 `tool_use`).
- **Commit:** `feat(ferramentas): janelas, mídia e abrir`

### Task 16: descrição no pedido de aprovação

- **Arquivos:** `internal/ferramentas/tipos.go`, `dispositivo.go`, `janelas.go`, `internal/api/ws.go`, `ws_test.go`, componente do cartão de aprovação no `web/` e o teste dele.
- **Testes:**
  - Interface opcional `DescreveConfirmacao{ DescricaoDaConfirmacao(args) string }` (aditiva, como `ExigeConfirmacao`).
  - `ManipularInterface` → `clicar em "Excluir" (Button)`, `apertar a tecla delete`; `Janelas` → `fechar a janela "Word anotações — Dolphin"`.
  - O evento de aprovação no WS leva `descricao` quando a ferramenta implementa; sem ela, o evento é **igual ao de hoje**.
  - Front: o cartão mostra a descrição em destaque quando vem, e os argumentos como hoje (vitest).
  - Lembrete: `npm run build --prefix web` **e reiniciar o `jamesd`** antes de ver ao vivo (a SPA vai embutida no binário).
- **Commit:** `feat: cartão de aprovação diz o que vai acontecer`

---

## Fatia 3: verificação

### Task 17: verificação ao vivo pelo executor (sem voz)

Roda o device de verdade e chama `tela_linux.executar_acao` por um script de verificação em `docs/superpowers/spikes/2026-10-13-tela-wayland/verificar_b2.py` (fica versionado como prova reproduzível). O script só mexe num KWrite que ele mesmo abre, e foca janelas existentes sem fechá-las.

- [ ] CA2: `janelas_listar` traz as janelas reais; `janela_focar` no Firefox foca; alvo ambíguo (`konsole`, se houver dois) devolve a lista.
- [ ] CA3: `midia estado`; `midia proxima` muda o `Metadata`; `midia anterior` volta. **Só com o OK do Arthur na hora** (mexe na música dele).
- [ ] CA4: `abrir` Dolphin pelo nome, uma URL, `~/Downloads` e um arquivo do repo numa linha no VS Code.
- [ ] CA5: `ver_tela` nos dois monitores: PNG 1280x720, elementos da janela ativa primeiro; clicar no centro listado do botão "Novo" do KWrite em cada monitor acerta.
- [ ] CA6: `clicar_elemento` no KWrite, no Dolphin (um botão inócuo, como "Dividir" ou um item de menu que só abre e fecha) e no Firefox; KWrite coberto por outra janela ainda recebe o clique.
- [ ] CA7: `digitar` `"Olá, ação, coração e maçã — 100%"` no KWrite e num campo do Firefox (o script limpa o que escreveu); a área de transferência é a mesma antes e depois.
- [ ] CA8 (lado device): `click` por coordenada em cima do botão "Novo" é recusado com orientação.
- [ ] CA10: Spotify em foco → `ver_tela` avisa e devolve só a imagem.
- [ ] CA12: durações no log dentro dos tetos.
- Registrar a saída no fim deste plano.
- **Commit:** `docs: verificação ao vivo do B2`

### Task 18: aceite ao vivo com o Arthur

Só o Arthur pode fazer: voz, aprovação no app, diálogo do KDE.

- [ ] CA9: apagar o token, subir o James, o diálogo aparece uma vez, a voz funciona antes de permitir; permitir; reiniciar o James e subir **sem** diálogo; `ls -l ~/.local/share/james/portal-token` mostra `-rw-------`.
- [ ] CA8 (ponta a ponta): pedir por voz para fechar uma janela e para clicar em "Excluir" num arquivo de teste no Dolphin: o cartão de aprovação aparece com a descrição; negar não executa; aprovar executa.
- [ ] CA13: "abre o Firefox e coloca o foco nele" e "troca a música", por voz.
- [ ] CA11: `GOOS=windows go build ./...` verde (dívida do Windows real segue anotada).

### Task 19: revisão e fechamento

- [ ] `/code-review` (nível high) contra `main` e `/security-review` (manual, se a skill falhar sem `origin`: `git diff main...HEAD`). Pontos de segurança para olhar com atenção: token do portal (permissão, log, caminho), argumentos do `code -g` e do lançador (lista, nunca shell), script do KWin montado com texto vindo do modelo (**o alvo nunca entra no JS como código**: passar por `JSON.stringify`/escape e testar com título contendo `"); evil(); ("`), recusa do clique por coordenada.
- [ ] Achados corrigidos com TDD e registrados aqui, com `ReportFindings` de novo com os desfechos.
- [ ] Suítes completas (fato 14) verdes, saída colada.
- [ ] Memória do projeto atualizada.
- [ ] Merge em `main` **só com o OK explícito do Arthur**, sem push.

---

## Fora deste plano

- Autonomia de arquivos (spec própria).
- Camada 1 no Windows (`janelas`/`midia`/`abrir`) e o desvio de coordenada da imagem nativa no Windows (R8 da spec).
- Árvore de elementos do VS Code e do Spotify.
- GNOME, X11, outros compositores.

---

## Registro da execução

### Desvios e achados

- **Task 2:** "o laço continua atendendo" só vale depois que a função lenta termina (uma thread só). O teste prova isso: quem chamou recebe `TimeoutError` na hora, e o laço atende a próxima chamada quando a lenta acaba. Mutação (tirar `push_thread_default`) pega 2 testes.
- **Task 3:** mutação (tirar a validação do id antes de montar o JS) pega 4 testes. `focar` ao vivo: 3 ms.
- **D6 mudou: área de transferência pelo Klipper, não pelo portal `Clipboard`.** O portal é assíncrono (o device teria de servir os dados a cada `SelectionTransfer` e ficaria dono da área de transferência enquanto vivesse). O Klipper (`org.kde.klipper /klipper setClipboardContents/getClipboardContents`) é síncrono e guarda o conteúdo depois que o device sai. Medido em 2026-10-13: o texto gravado aparece na hora no `wl-paste`, e o conteúdo original volta byte a byte. Conteúdo que não é texto (detectado por `wl-paste --list-types`, que não vira daemon) faz o James **recusar** o caminho de colar, para não destruir uma imagem copiada. Fica na Task 8; a Task 4 não tem área de transferência.
- **Task 4, permissão do KDE:** a tabela `remote-desktop` do `PermissionStore` guarda as concessões para o app id `''` (todo programa fora de sandbox). A sessão do teste ao vivo, sem token, abriu em 1,5 s e criou uma concessão nova. Consequência para o CA9: depois da primeira permissão, o diálogo pode não aparecer de novo nem sem token. A fronteira de segurança continua sendo a conta do sistema (como já registrado na Fase 6); o token segue 0600 por defesa em profundidade.
- **Task 6, achado: o `pipewiresrc` trava no `set_state(PLAYING)`** com descritor que não responde (descoberto pelo teste de erro, que travou em vez de falhar). Um descritor velho (sessão fechada, PipeWire reiniciado) congelaria a ponte de tela inteira. A captura agora roda numa thread com prazo (5 s): estourou, o erro volta na hora; a captura presa trava só o próprio fluxo (pedido seguinte no mesmo fluxo é recusado sem empilhar threads), e o portal reaberto traz fluxos novos. A imagem vai direto para a memória (`appsink`), sem arquivo. Ao vivo: 26–40 ms por monitor, 1280x720.
- **Task 8, achado que vale para tudo: o cache da libatspi.** Ela guarda a árvore e só atualiza processando eventos no contexto GLib a que está ligada; o device só gira o contexto do laço, então a árvore ficava congelada no primeiro acesso (o clique em "Novo" abriu o editor e o James continuava vendo a tela de boas-vindas; no spike não apareceu porque o script girava o contexto padrão enquanto esperava). Correção: `acessibilidade.vincular_ao_laco(laco)` (`Atspi.set_main_context`) ao subir, **toda chamada ao Atspi dentro do laço**, e quem espera a tela mudar (o `digitar`) espera **fora** do laço, com cada leitura entrando e saindo (`CampoAtspi(no, laco.executar)`). **A Task 11 tem de seguir esta regra.**
- **Task 8, API:** `get_text_iface()` devolve o próprio `Accessible`; `.get_text(0, -1)` nele é um método antigo de outra assinatura. Usar `Atspi.Text.get_text(obj, 0, -1)`, `Atspi.Text.get_caret_offset(obj)`, `Atspi.EditableText.insert_text(obj, pos, texto, bytes)`. Pego pelo teste ao vivo (o `TypeError` subiu, como deve).
- **Task 8, corrida do colar:** o app busca o conteúdo depois do Ctrl+V de forma assíncrona; restaurar cedo faria colar o texto antigo. Com campo legível, restaura só depois de o texto aparecer; sem campo, espera 0,5 s. Ao vivo no KWrite: AT-SPI e colar forçado, frase exata nos dois, área de transferência igual antes e depois.
- **Task 9, achado no barramento real:** o Firefox publica player MPRIS e aparece como `Playing` com um curso em vídeo (artista `""`), junto com o Spotify. "Troca a música" responderia "diga qual". Regra nova: entre vários tocando, prefere o único com artista (música); artista vazio não conta. Lido ao vivo (sem tocar em nada): `Spotify: tocando 'New Chapter' de Jikuru`.
- **Task 11:** sem `superficie`, o `clicar_elemento` usa o monitor da **última varredura**, não o da janela ativa: o id `eN` sempre se refere a ela (o teste de clicar no Firefox, no outro monitor, recusava com "a tela mudou"). Depois de focar a janela dona, o retrato continua válido (focar não move nada) e o `janela_em_foco` do mapa é atualizado. Captura e varredura falham de forma independente: sem imagem ainda vai a lista, e sem lista ainda vai a imagem.
- **Task 17, BUG SÉRIO achado na verificação ao vivo: caracteres nulos no documento.** O Qt conta o comprimento do `insert_text` em **caracteres**; com o comprimento em bytes (o que a libatspi documenta, e o que a Task 8 passava), ele inseria o texto **e mais caracteres `\0`** (2 nulos para "ação "). O `get_text` corta no `\0`, então a inserção seguinte "sumia" e o `digitar` caía para colar. O GTK conta em bytes; `-1` não insere nada no Qt. Correção: `inserir_conferindo` tenta em caracteres, confere que o documento cresceu exatamente o texto e que o trecho é o texto, **desfaz** o que entrou errado (`delete_text`) e tenta em bytes. Testes com falsos de Qt, GTK e "ignora" (Firefox); ao vivo: frase de 32 caracteres sem nulo e duas inserções seguidas. Só documentos descartáveis do teste receberam os nulos.
- **Task 17, corrida do Klipper:** depois do `setClipboardContents`, o Ctrl+V imediato às vezes colava o conteúdo antigo. Agora o `digitar` espera (até 0,5 s) a leitura devolver o texto do James antes do Ctrl+V; senão, erro e nada é colado. Ao vivo: 3 de 3 com o colar forçado, área de transferência devolvida nas três.
- **Task 17, recusas certas:** com uma imagem na área de transferência, o James recusou colar por cima (o teste agora confere essa recusa quando há imagem). A barra do Firefox cresce ao ganhar foco ("breakout"), e o segundo `clicar_elemento` foi recusado com "mudou de lugar" pela revalidação: proteção funcionando, o modelo chama `ver_tela` de novo.
- **Task 17, tela de boas-vindas do KWrite** tem duas versões ("Novo" na barra, ou "Novo arquivo" com recentes): testes ao vivo aceitam os dois e exigem o botão visível.

### Task 17: verificação ao vivo pelo executor (2026-10-13)

Script: `docs/superpowers/spikes/2026-10-13-tela-wayland/verificar_b2.py` (mesmas peças do `plataforma/linux.py`). Última rodada, depois das correções, com `VERIFICAR_SEM_ABRIR=1` (o CA4 já tinha passado na rodada anterior e não foi repetido para não abrir outra aba e outro arquivo no VS Code do Arthur):

| CA | Resultado |
|---|---|
| CA2 | ok: 8 janelas; focar o Firefox ok (a lista ambígua não aparece com um Firefox só) |
| CA4 | ok (rodada anterior): KWrite pelo nome, `~/Downloads`, `https://example.com`, arquivo do repo na linha 3 no VS Code; janelas abertas pelo teste fechadas pelo id |
| CA5 | ok: PNG 1280x720, botão Novo listado, elementos estruturados |
| CA6 | ok no KWrite com outra janela na frente (a dona foi focada antes); ok no Firefox numa rodada isolada (dois cliques na barra); Dolphin não estava aberto nesta rodada |
| CA7 | ok no KWrite (AT-SPI, sem nulo; colar forçado 3 de 3); **Firefox pendente**: na última rodada ele estava em tela cheia (sem barra visível), e numa anterior a área de transferência tinha imagem (recusa correta). Fica para a Task 18 |
| CA8 (device) | ok: `click` em cima do "Novo" recusado com "use clicar_elemento com o id e12" |
| CA10 | ok: Spotify em foco → aviso de app sem elementos + imagem |
| CA12 | ok: focar 12 ms, `clicar_elemento` 8 ms, `digitar` 23 ms, `ver_tela` 154 ms (medianas) |
| CA3 | não rodado: mexe na música do Arthur, fica para a Task 18 |

### Task 19: code-review (xhigh, 2026-10-13): 8 achados

| # | Achado | Desfecho |
|---|---|---|
| 1 | Aprovação do clique usa o mapa do Hub, que diverge do device (WS reconecta a cada 2 s e zera; ver_tela com timeout) | **corrigido**: id que o Hub não conhece conta como destrutivo e pede aprovação (falha para o lado seguro); ver_tela com erro, timeout ou cancelamento zera o mapa |
| 2 | Recusa do `click` por coordenada só valia com o mapa válido (troca de janela a desliga) | **corrigido**: o `click` varre a tela NA HORA (~150 ms); sem árvore ou AT-SPI fora, o clique por pixel segue |
| 3 | Restauração da área de transferência achata HTML e `text/uri-list` | **corrigido**: formato rico (imagem, HTML, arquivos) bloqueia o colar |
| 4 | Espera de 0,3 s depois de inserção que já falhou | **corrigido**: o retorno de `inserir` pula a espera |
| 5 | `_travas_por_fluxo` nunca limpa | **não aplicado**: um dict de inteiros por sessão do portal, sem efeito prático; anotado |
| 6 | Estado "abrindo" dizia "falta permissão" | **corrigido**: mensagem própria |
| 7 | `_normalizar` duplicada em kwin.py e abrir.py | **não aplicado**: 3 linhas, e juntar criaria dependência entre módulos sem relação |
| 8 | `abrir` aceita qualquer esquema de URL | **corrigido**: só http, https e mailto |

Efeito colateral útil: o `ExecutorKWin` ganhou caminho D-Bus único por instância (os testes ao vivo juntos colidiam na conexão compartilhada). Mutação do achado 2 (voltar ao mapa velho) derruba 2 testes.
