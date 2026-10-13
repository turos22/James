# James: tela e janelas no Wayland (B2)

Status: **rascunho para aprovação do Arthur** (2026-10-13)
Branch: `feat/tela-wayland` (a partir de `main` em `a8061c0`)
Spikes: `docs/superpowers/spikes/2026-10-13-tela-wayland.md` (commit `3a09ad0`)
Plano-mãe: `docs/superpowers/plans/2026-10-09-james-analise-stt-e-fedora.md`, item B2

## 1. Contexto

### 1.1 O que o Arthur pediu

No aceite da voz em tempo real: "a única coisa que me incomoda é que ele ainda não
tem tanta autonomia". No teste, o James abriu o Firefox mas não conseguiu dar foco
nele, e sugeriu instalar o `wmctrl`. O Arthur também pediu para ele trocar a música
do Spotify.

### 1.2 Como é hoje (lido no código em 2026-10-13)

- No Linux, a ponte de tela está **desligada**: `plataforma/linux.py` responde
  `"tela indisponível neste sistema (Linux)"` a toda ação, e o device loga
  `ponte de tela desligada neste sistema (linux)` ao subir.
- O protocolo jamesd → device é `dispositivo.AcaoTela{Acao, X, Y, Texto, Superficie,
  ElementoID, Nome, Tipo}` → `Resultado{Saida, Imagem}`. Ações: `click`, `tecla`,
  `digitar`, `ver_tela`, `clicar_elemento` (`tela.py`, só Windows: pyautogui +
  uiautomation).
- Ferramentas no Go: `ver_tela` (`NivelSeguro`, devolve imagem) e
  `manipular_interface` (`NivelPerigoso`). Nenhuma das duas implementa
  `ExigeConfirmacao`. O provedor Anthropic já manda a imagem do `ver_tela` como bloco
  `image/png` no `tool_result`.
- No Windows as coordenadas do protocolo são **pixels físicos locais ao monitor**
  (DPI awareness per-monitor), e a imagem vai na resolução nativa.

### 1.3 O que os spikes provaram (Fedora 44, KDE Plasma 6.7.5, Wayland)

| Ponto | Como | Medida |
|---|---|---|
| Listar janelas com geometria, pid e monitor | script do KWin por D-Bus, saída por `callDBus` | ok, coordenada global **lógica** |
| Focar janela | `workspace.activeWindow = w` | **7–10 ms**, confirmado relendo |
| Teclado e mouse | portal RemoteDesktop + ScreenCast de monitores | 1ª vez com diálogo; depois `restore_token`: **0,01 s, sem diálogo** |
| Mouse absoluto | `NotifyPointerMotionAbsolute` por fluxo | erro **0 px** nos dois monitores, inclusive escala 1,35 |
| Clicar em elemento | retângulo AT-SPI (relativo à janela) + origem do cliente no KWin | botão "Novo" do KWrite clicado, efeito em 0,20 s |
| Captura de um monitor | quadro do ScreenCast (`pipewiresrc`) → PNG | **21–39 ms**, 1920x1080 nativo |
| Texto em português | AT-SPI `EditableText.insertText` | exato, 5 ms (Qt) |
| Texto onde `insertText` é ignorado (barra do Firefox) | área de transferência + Ctrl+V pelo portal | exato, 0,17 s; área de transferência devolvida intacta |
| Texto por keysym | `NotifyKeyboardKeysym` | **falha** (`ç`→`<`, `ã` some; layout `us`) |
| Mídia | MPRIS (`org.mpris.MediaPlayer2.spotify`) | Next/Previous/PlayPause/Metadata disponíveis |
| Acessibilidade | `org.a11y.Status IsEnabled` | **estava desligada**: só GTK expunha árvore |
| VS Code, Spotify | Electron/CEF | não expõem árvore (nem com flag) |

## 2. Escopo

### 2.1 Entra

**Camada 1: ações por significado (D-Bus, sem pixel)**
- `janelas`: listar, focar, fechar (KWin).
- `midia`: tocar, pausar, alternar, próxima, anterior, estado (MPRIS, qualquer player).
- `abrir`: app pelo nome, arquivo ou pasta, URL; arquivo numa linha no VS Code
  (`code -g`).

**Camada 2: porte da ponte de tela para o Wayland**
- `ver_tela`, `click`, `clicar_elemento`, `digitar`, `tecla` no Linux, com o mesmo
  protocolo e o mesmo formato de resposta do Windows.

**Transversal**
- Aprovação para ação destrutiva (seção 3, D10).
- O device liga a acessibilidade ao subir (D11).
- Fluxo de permissão do portal na primeira vez e quando for revogada (D5).

### 2.2 Não entra

- Autonomia de arquivos (ler livre, criar/editar/apagar com aprovação): spec própria,
  anotada em `docs/superpowers/decisoes/2026-10-13-james-autonomia-no-sistema.md`.
- Árvore de elementos do VS Code e do Spotify (decisão do Arthur: CLI e imagem).
- Sessão X11, GNOME, outros compositores: o device detecta e fica com a tela
  desligada, como hoje.
- Mudar o comportamento do Windows. As ações novas respondem "indisponível neste
  sistema" lá (D13). O desvio de coordenada da imagem no Windows (seção 9, R8) vira
  dívida anotada.
- Câmera (spec 3 da iniciativa de agilidade).

## 3. Decisões

| # | Decisão | Por quê |
|---|---|---|
| D1 | As duas camadas passam pela **mesma ponte WS** jamesd → device. O device é quem fala com o desktop. | Um caminho só, já com timeout, reconexão e testes. O jamesd não precisa saber de D-Bus. |
| D2 | Janelas pelo **script do KWin** (carregado por `org.kde.KWin /Scripting`, saída por `callDBus` para um objeto D-Bus do device). Sem `wmctrl`, `kdotool` ou `ydotool`. | Provado no spike; enxerga janelas Wayland nativas, dá geometria e foca em 8 ms. `wmctrl` é X11. |
| D3 | Mídia pelo **MPRIS**, escolhendo o player que está tocando (ou o último ativo, ou o pedido pelo nome). | Funciona com qualquer player, sem tela, sem clique errado. |
| D4 | `abrir`: URL e arquivo pelo app padrão (`Gio.AppInfo.launch_default_for_uri`); app pelo nome (`Gio.DesktopAppInfo.search` sobre os `.desktop`); arquivo + linha → `code -g arquivo:linha`. Abrir não pede aprovação. | Caminhos do próprio desktop. Abrir não destrói nada. |
| D5 | Entrada por **portal RemoteDesktop** (teclado + mouse) + **ScreenCast** de todos os monitores, `persist_mode=2`. O `restore_token` fica em `~/.local/share/james/portal-token`, modo 0600, fora do git. Sessão aberta quando o device sobe e mantida enquanto ele vive. | Provado. O token dispensa o diálogo nas próximas vezes. Ele dá controle de teclado e mouse sem perguntar, então é tratado como segredo local. |
| D5a | Sem token, ou com token recusado: o device abre o diálogo **uma vez** ao subir e loga. Até o Arthur permitir, toda ação de tela responde "preciso de permissão: clique em Permitir no aviso do KDE". Nada trava; a voz segue. | O pedido de permissão é ação do Arthur, nunca do James. |
| D6 | **Texto**: `digitar` tenta `EditableText.insertText` no campo em foco e confere relendo. Se o texto não mudou, cai para **colar**: grava a área de transferência pelo portal `Clipboard`, manda Ctrl+V, confere e **sempre** devolve o conteúdo anterior (`finally`). Keysym **nunca** é usado para texto. `tecla` usa keysym só para tecla nomeada e atalho (`enter`, `tab`, `esc`, setas, `ctrl+l`...). | Spike: keysym quebra `ç`/`ã`; `insertText` é exato onde funciona; colar cobre o Firefox. Colar sem devolver a área de transferência estragaria o que o Arthur copiou. |
| D7 | **Elementos** pelo `Atspi` (typelib do sistema, via `gi`). Varre a janela ativa primeiro, depois as outras janelas visíveis do monitor, e marca `em_foco` (o `elementos.formatar` existente já prioriza). AT-SPI e KWin são casados por pid + título. **`clicar_elemento` foca a janela dona do elemento antes de clicar.** | No Wayland o AT-SPI não sabe se uma janela está coberta por outra. Focar antes (8 ms) garante que o clique cai nela. |
| D8 | **Coordenada global** = retângulo AT-SPI (relativo à janela) + origem do `clientGeometry` do KWin, em lógico. O portal recebe lógico relativo ao fluxo do monitor. Fluxo e monitor são casados **pela posição**, nunca pelo `node_id`. | Provado com erro 0 px. O `node_id` troca entre sessões. |
| D9 | **Espaço de coordenada do protocolo no Linux = pixels da imagem que o modelo recebe.** O device reduz a captura para **1280 px de largura** (altura proporcional) antes de mandar, e lista os elementos nesse mesmo espaço. `Superficie` ganha `fator` (imagem → lógico); `para_global(x, y) = origem + (x, y) * fator`. No Windows `fator = 1` e nada muda. | A API reduz imagens acima de 1568 px no lado maior. Mandar 1920 faria o clique por coordenada (o único caminho no Spotify e no VS Code) cair cerca de 22% fora. A Anthropic recomenda esta faixa de resolução para uso de computador. |
| D10 | **Aprovação** (decisão do Arthur: livre, exceto destrutivo). Pedem aprovação: (a) `janelas fechar`; (b) `clicar_elemento` cujo nome contém palavra destrutiva; (c) `tecla` com `delete` (inclui `shift+delete`). Para o Go saber o nome antes de executar, o `ver_tela` passa a devolver também a lista estruturada `[{id, nome, tipo}]`, que o jamesd guarda como "último mapa" do device. O vocabulário fica no Go, com teste: excluir, apagar, remover, deletar, delete, remove, esvaziar, lixeira, descartar, desinstalar, formatar, "não salvar", "don't save" (comparação sem acento e sem caixa, por palavra). (d) No Linux, **`click` por coordenada que cai em cima de um elemento listado é recusado** com "use clicar_elemento com o id": o clique por pixel fica só para onde não há árvore. | `ExigeConfirmacao` decide antes de executar, olhando os argumentos; o id `e7` sozinho não diz o que é. (d) impede que o clique por coordenada contorne (b). |
| D11 | O device **liga a acessibilidade ao subir** (`org.a11y.Status IsEnabled = true`) e loga. Quando o app da janela ativa não está no barramento AT-SPI, o `ver_tela` avisa: "este app não expõe elementos (talvez aberto antes do James: reabra-o) — use a imagem e clique por coordenada". | Decisão do Arthur. Apps abertos antes de ligar não montam a árvore. |
| D12 | `gi` no device: **PyGObject instalado no venv por pip**, usando os typelibs do sistema (`Atspi-2.0`, `Gst-1.0`, `Gio-2.0`). Dependências de build pelo dnf, rodadas pelo Arthur (`sudo dnf install gobject-introspection-devel glib2-devel`). Um laço `GLib.MainLoop` numa thread própria atende os sinais do portal; as ações da ponte chamam esse laço e esperam com timeout. | Um processo só, sem IPC novo. O venv é Python 3.13 (mise) e o `pyatspi` do sistema é para 3.14, por isso não dá para reaproveitar o do sistema. Verificado na Task 0 do plano; se o build falhar, o plano para e volta a esta decisão. |
| D13 | **Windows**: as ações novas (`janelas`, `midia`, `abrir`) respondem `"indisponível neste sistema (Windows)"`. O caminho atual do Windows (`tela.py`) não muda. A escolha de backend fica em `plataforma/`. | "Windows não pode quebrar." Portar a camada 1 para o Windows é outra fatia. |
| D14 | Ferramentas novas no Go: `janelas` (`NivelPerigoso`, `fechar` com `ExigeConfirmacao`), `abrir` (`NivelPerigoso`), `midia` (`NivelSeguro`: só mexe no player). | Mesma régua de `manipular_interface`; mídia não altera nada fora do player. |

## 4. Arquitetura

```
jamesd (Go)                                  device (Python, Linux)
──────────                                   ──────────────────────
ferramentas: ver_tela, manipular_interface,  plataforma/linux.py  → escolhe backend
             janelas*, midia*, abrir*          tela_linux.py       → executar_acao(dados) (mesmo contrato de tela.py)
  │  ExigeConfirmacao (D10)                     ├─ kwin.py          janelas, foco, geometria (script KWin + callDBus)
  │  último mapa de elementos por device        ├─ portal.py        sessão RemoteDesktop+ScreenCast+Clipboard, token
  ▼                                             ├─ acessibilidade.py varredura Atspi → elementos.Elemento
dispositivo.Hub ── WS acao/resultado_acao ──►   ├─ captura_linux.py quadro do ScreenCast → PNG 1280 px
  AcaoTela{+Alvo, +Linha}                       ├─ midia.py         MPRIS
  Resultado{+Elementos}                         ├─ abrir.py         Gio.AppInfo / DesktopAppInfo / code -g
                                                └─ laco_glib.py     GLib.MainLoop numa thread + chamada com timeout
```

- `superficies.Superficie` ganha `fator` (D9). `para_global` e `para_local` passam a
  usá-lo; no Windows o valor é 1 e os testes atuais continuam valendo.
- `elementos.Elemento`, `Mapa` e `formatar` são reaproveitados. Só o `varrer` muda
  por plataforma: o Windows continua com uiautomation, e o Linux ganha o seu em
  `acessibilidade.py`.
- `main.py`: `TELA_DISPONIVEL` no Linux passa a ser `True` quando a sessão é Wayland
  + KDE e o `gi` importa. Em qualquer outro caso, fica desligada com log, como hoje.

### 4.1 Protocolo (aditivo)

- `AcaoTela` ganha `Alvo string` (`json:"alvo,omitempty"`) e `Linha int`
  (`json:"linha,omitempty"`). Ações novas: `janelas_listar`, `janela_focar`,
  `janela_fechar`, `midia` (subação em `Texto`), `abrir`.
- O `resultado_acao` do `ver_tela` ganha `elementos: [{id, nome, tipo}]`. O device do
  Windows pode mandar a mesma lista (custo zero, ele já tem o `Mapa`). Se não mandar,
  o Go trata como lista vazia, e então (b) da D10 só vale pelo `nome` passado.
- Campo desconhecido é ignorado dos dois lados: device velho com jamesd novo continua
  funcionando, e vice-versa.

## 5. jamesd (Go)

### 5.1 Ferramentas novas

- `janelas {acao: listar|focar|fechar, alvo}`: `alvo` é trecho do título ou nome do
  app, sem diferenciar caixa nem acento. Com mais de uma janela casando, `focar`
  devolve a lista e pede para escolher; nunca chuta.
- `midia {acao: tocar|pausar|alternar|proxima|anterior|estado, player?}`: `estado`
  devolve player, título, artista e se está tocando.
- `abrir {alvo, linha?}`: com `linha` e `alvo` sendo arquivo, abre no VS Code nessa
  linha.

### 5.2 Aprovação (D10)

- `ManipularInterface` passa a implementar `ExigeConfirmacao`:
  `clicar_elemento` → resolve o nome pelo `elemento_id` no último mapa (ou usa o
  `nome` dos argumentos) e casa com o vocabulário; `tecla` → casa `delete`.
- `Janelas` implementa `ExigeConfirmacao` para `fechar`.
- O último mapa é guardado por device no Hub e trocado a cada `ver_tela`. Sem mapa e
  sem nome, `clicar_elemento` não pede aprovação: o device recusa de qualquer jeito
  ("chame ver_tela antes").
- O texto que o Arthur vê no cartão de aprovação diz o que vai acontecer:
  `clicar em "Excluir" (botão) no Dolphin`.

### 5.3 Prompt

`prompts/sistema.md` ganha a regra de preferência: para mídia, janela e abrir,
use a ferramenta específica; só use `ver_tela`/`manipular_interface` quando precisar
olhar ou agir dentro de um app.

## 6. Device (Python, Linux)

### 6.1 Ciclo de vida

1. Ao subir: liga a acessibilidade (D11), inicia o laço GLib (D12), abre a sessão do
   portal com o token (D5). Sem token, abre o diálogo e segue (D5a).
2. Cada ação vem pela ponte como hoje e roda em `tela_linux.executar_acao`, que
   **nunca levanta** (mesma regra do `tela.py`: erro vira `{"erro": ...}`).
3. Sessão do portal fechada por fora (revogada, logout do compositor): na próxima
   ação, o device tenta reabrir uma vez com o token; se falhar, volta para D5a.

### 6.2 `ver_tela`

Monitor alvo: o pedido, ou o da janela ativa (KWin). Captura o quadro do fluxo
desse monitor, reduz para 1280 px de largura, varre os elementos (D7) com teto de
tempo de 400 ms e de nós por app (a varredura que estoura devolve o que achou e
avisa). Cabeçalho igual ao do Windows, com `escala` e o tamanho da imagem.

### 6.3 `clicar_elemento`

Mesmo fluxo do Windows: mapa válido para (monitor, janela ativa), resolve por id ou
nome, **revalida contra a árvore viva** (o elemento ainda existe, `SHOWING`,
`ENABLED`, mesmo retângulo), foca a janela dona (D7), move e clica. A saída diz o
que foi clicado e onde.

### 6.4 `click` por coordenada

Converte imagem → lógico → global (D9), valida os limites do monitor e aplica a
recusa (d) da D10 quando o ponto cai em elemento listado. Depois do clique, descreve
o que há no ponto, se o AT-SPI souber.

### 6.5 `digitar` e `tecla`

D6. O mapa de nomes de tecla → keysym é tabela pura, com teste. `tecla` aceita
combinação com `+` (`ctrl+l`, `alt+tab`).

## 7. Ambiente

- Dependências de build do PyGObject pelo dnf (o Arthur roda o `sudo`), depois `pip
  install` no venv do device. Plugins do GStreamer (`pipewiresrc`, `videoconvert`,
  `videoscale`, `pngenc`) já estão instalados (conferido com `gst-inspect-1.0`).
- Token do portal em `~/.local/share/james/portal-token` (0600), fora do repositório:
  não há o que ignorar no git.
- Nada de `wmctrl`, `ydotool` (daemon root + uinput) ou `kdotool`.

## 8. Critérios de aceite

| # | Critério | Como verificar |
|---|---|---|
| CA1 | Suítes verdes: `go vet ./...`, `GOOS=windows go vet ./...`, `GOOS=windows go build ./...`, `go test ./...`, pytest do device, vitest do front | saída dos comandos |
| CA2 | **Janelas**: `listar` traz as janelas reais com app e título; "traz o Firefox pra frente" foca a janela certa; título ambíguo devolve a lista em vez de chutar | pytest com KWin falso + sessão ao vivo |
| CA3 | **Mídia**: "próxima música" muda a faixa (Metadata antes ≠ depois); "pausa" e "continua" funcionam; `estado` diz o que está tocando | pytest com MPRIS falso + sessão ao vivo |
| CA4 | **Abrir**: um app pelo nome (Dolphin), uma URL, uma pasta, e um arquivo numa linha no VS Code | sessão ao vivo |
| CA5 | **ver_tela**: imagem de 1280 px do monitor certo, mais os elementos da janela ativa primeiro, com coordenadas no espaço da imagem; nos dois monitores (escala 1 e 1,35), clicar no centro listado de um botão acerta | pytest (conversão, formatação) + sessão ao vivo |
| CA6 | **clicar_elemento** no Dolphin, no Firefox e num app Qt produz o efeito esperado; com a janela coberta por outra, o clique ainda cai na janela dona | sessão ao vivo |
| CA7 | **digitar** `"Olá, ação, coração e maçã — 100%"` sai exato num campo do Firefox e num app Qt; a área de transferência do Arthur é a mesma antes e depois | pytest (estratégia e restauração) + sessão ao vivo |
| CA8 | **Aprovação**: `janelas fechar`, `clicar_elemento` em "Excluir"/"Mover para a lixeira" e `tecla delete` pedem aprovação; o resto roda livre; `click` por coordenada em cima de elemento listado é recusado com orientação | `go test` (vocabulário, resolução por id) + pytest (recusa) + sessão ao vivo |
| CA9 | **Permissão**: sem token, o diálogo aparece uma vez e a voz segue; depois sobe sem diálogo; token 0600 fora do git; com a permissão revogada, o James diz que precisa de permissão em vez de travar | sessão ao vivo + `ls -l` |
| CA10 | **Acessibilidade**: ligada ao subir; com o Spotify na frente, `ver_tela` avisa que o app não expõe elementos e devolve só a imagem | sessão ao vivo |
| CA11 | **Windows intacto**: ações novas respondem indisponível; nada no caminho de `tela.py` muda; build e vet para Windows verdes. (CA3 do Windows real continua dívida, como no B1) | `GOOS=windows`, pytest |
| CA12 | **Rapidez** no device (log): focar ≤ 50 ms, `clicar_elemento` ≤ 300 ms, `digitar` ≤ 300 ms, `ver_tela` ≤ 600 ms com varredura | log do device em sessão ao vivo |
| CA13 | **O pedido do teste**, por voz, ponta a ponta: "abre o Firefox e coloca o foco nele" e "troca a música" | sessão ao vivo |

## 9. Riscos

| # | Risco | Mitigação |
|---|---|---|
| R1 | App flatpak tem pid do sandbox no AT-SPI, diferente do pid no KWin | casar por título quando o pid não bate; teste com o Spotify |
| R2 | Árvore AT-SPI grande (página pesada no Firefox) deixa o `ver_tela` lento | teto de tempo e de nós (6.2), aviso honesto de corte |
| R3 | API de script do KWin muda numa atualização do Plasma | script pequeno num arquivo só, com teste de parsing da saída; falha vira erro claro, não clique errado |
| R4 | Coordenada do plasmashell (painel, layer-shell) já vem global, não relativa à janela | detectar janela que não é `normalWindow` e não somar a origem; caso de teste |
| R5 | `restore_token` vazado dá controle de teclado e mouse | 0600, fora do git, nunca em log |
| R6 | Colar sobrescreve a área de transferência se o device cair no meio | restauração em `finally`; janela de risco de ~0,2 s, aceita |
| R7 | PyGObject não compila no venv do mise | Task 0 verifica antes de tudo; se falhar, para e volta à D12 (opção: processo auxiliar no Python do sistema) |
| R8 | **No Windows**, a imagem nativa de 1920 px passa do limite da API e é reduzida, então o clique por coordenada estimado na imagem cai deslocado | fora do escopo (D13); vira dívida anotada no plano |
| R9 | Thread do GLib e thread da ponte disputando o D-Bus | toda chamada D-Bus passa pelo laço (D12); a ponte só espera com timeout |

## 10. Decisões do Arthur (2026-10-13)

- Escopo: as duas camadas juntas.
- Aprovação: livre, exceto ação destrutiva (fechar janela, clicar em
  Excluir/Apagar/Remover e afins).
- Apps do AT-SPI: Firefox, Spotify, Dolphin, VS Code.
- Acessibilidade: o device liga ao subir.
- VS Code: CLI (`code -g`) e imagem da tela; não forçar o modo leitor de tela.
