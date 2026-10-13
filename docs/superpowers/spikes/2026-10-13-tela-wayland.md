# Spikes do B2: tela e janelas no Wayland (2026-10-13)

Máquina: Fedora 44, KDE Plasma/KWin 6.7.5, sessão Wayland, dois monitores.
Scripts em `2026-10-13-tela-wayland/`, rodam com o `/usr/bin/python3` do sistema
(`pyatspi` e `gi` vêm do dnf; o venv do device não tem `gi`).

Decisões do Arthur antes dos spikes:
- Escopo: as duas camadas numa spec só (ações por D-Bus + porte da ponte de tela).
- Aprovação: tela e janelas livres, EXCETO ação destrutiva (fechar janela; clicar em
  elemento chamado Excluir/Apagar/Remover/Esvaziar lixeira e afins).
- Apps do AT-SPI: Firefox, Spotify, Dolphin, VS Code.

## Ambiente achado

| Coisa | Estado |
|---|---|
| `org.a11y.Status IsEnabled` | **estava `false`**: só GTK/SWT (DBeaver) expunham elementos. Ligado no spike (D-Bus Set). |
| Portais | `RemoteDesktop` v2, `ScreenCast`, `Screenshot`, `InputCapture` presentes (`xdg-desktop-portal-kde` 6.7.5) |
| KWin | `/Scripting` (loadScript/run) e `/WindowsRunner` (krunner1 Match/Run) |
| MPRIS | `org.mpris.MediaPlayer2.spotify`: Next, Previous, PlayPause, OpenUri, Metadata |
| Layout de teclado do KDE | `us` (o VC é `us-intl`) |
| Telas (lógico) | HDMI-A-1 `0,0 1920x1080` escala 1; HDMI-A-2 `1920,588 1422x800` escala 1,35 |
| Ausentes | `wmctrl`, `kdotool`, `ydotool`, `playerctl`, `qdbus` (nenhum faz falta, ver abaixo) |

## B2.1: AT-SPI cobre os apps de uso diário?

Com a acessibilidade ligada, o barramento passou de 12 para 31 apps.

| App | Resultado | Observação |
|---|---|---|
| Dolphin | 52 elementos úteis, todos com retângulo | Qt: liga na hora, sem reabrir |
| Konsole, plasmashell, KWrite | expõem | plasmashell dá coordenada global (é layer-shell) |
| DBeaver | 383 úteis | GTK/SWT, já expunha antes |
| Firefox | só barra e menus (289 nós) | aberto antes de ligar; **reabrir e medir o conteúdo da página** |
| Spotify, VS Code | ausentes | Electron/Chromium; **reabrir e medir** |

**Coordenadas do AT-SPI no Wayland são relativas à janela** (o app não sabe onde
está). Ex.: Dolphin `Abrir menu` em `889,6`.

## B2.3: janela em foco e lista de janelas pelo KWin

Provado. Um script do KWin (carregado por D-Bus, devolve linhas por `callDBus`) dá:
classe do app, título, pid, ativa, minimizada, monitor e `frameGeometry`/`clientGeometry`
**globais em coordenada lógica**. `workspace.activeWindow = w` traz para frente:
**7–8 ms**, confirmado relendo `activeWindow`. Nada a instalar; `wmctrl` (X11) é
desnecessário e não enxergaria janelas Wayland nativas.

Detalhe: `print()` do script do KWin cai numa categoria de log desligada; a saída
tem que voltar por D-Bus.

## B2.2: teclado e mouse pelo portal RemoteDesktop

| Medida | Resultado |
|---|---|
| 1ª sessão: diálogo do KDE, o Arthur clicou Permitir | `Start` 12,4 s (tempo humano), `devices=3` (teclado+mouse) |
| Sessões seguintes com `restore_token` (persist_mode=2) | **`Start` 0,01–0,02 s, sem diálogo** |
| Fluxos do ScreenCast | posição e tamanho lógicos **idênticos** às telas do KWin |
| Mouse absoluto (`NotifyPointerMotionAbsolute`) | erro 0,5 px no alvo; **0,00 px** no centro dos dois monitores, inclusive o de escala 1,35 |
| Clique em elemento achado pelo AT-SPI (botão "Novo" do KWrite) | abriu o editor em 0,20 s |
| Digitar por keysym (`NotifyKeyboardKeysym`) | **FALHOU**: `"James teste ação 123"` virou `"jAMES TESTE A<o 123"` |
| Escrever pelo AT-SPI `EditableText.insertText` | **ok**: `"Olá, James! Ação, coração, pão e maçã — 100% ok?"` exato, 5 ms |

Por que o keysym falhou: o KWin converte keysym em tecla do layout ativo (`us`), onde
`ç` e `ã` não existem; a inversão de caixa ficou sem causa confirmada (o LED do Caps
Lock físico está apagado). Conclusão: **texto entra por `EditableText`**; teclado do
portal só para tecla nomeada (Enter, Tab, Esc, atalhos). Fallback a medir: área de
transferência + Ctrl+V, para campo que não expõe `EditableText`.

O `restore_token` dá controle de teclado e mouse sem perguntar: tratar como segredo
local (arquivo 0600 fora do git).

## B2.4: coordenadas

Provado no mesmo sistema: retângulo do AT-SPI (relativo à janela) + origem do
`clientGeometry` do KWin = coordenada global lógica, que é exatamente o que o portal
aceita (por fluxo: global − posição do fluxo). Escala fracionária não atrapalha
porque tudo está em lógico.

## ver_tela: captura da imagem

| Caminho | Tempo | Imagem | Veredito |
|---|---|---|---|
| `spectacle -b -n -f -o` | 0,56–0,70 s | **6684x2776** (os dois monitores, em escala 2) | grande e lento demais |
| Quadro do ScreenCast da sessão do portal (`OpenPipeWireRemote` + `pipewiresrc num-buffers=2 ! pngenc`) | **21–39 ms** | 1920x1080 por monitor, nativo | **escolhido** |

Conferido que o quadro não sai preto: desvio padrão de cinza 17–21, 254–256 níveis
(tema escuro). Sem diálogo novo: o ScreenCast já vem na sessão do RemoteDesktop.

Pegadinha: o `node_id` de cada fluxo **muda entre sessões** (138/142 trocaram de
monitor). Casar fluxo e monitor pela posição, nunca pelo id.

## Os 4 apps do Arthur, depois de reabertos com a acessibilidade ligada

| App | AT-SPI | Escrever texto | Caminho recomendado |
|---|---|---|---|
| Dolphin | 52 elementos, todos com retângulo | `EditableText` (Qt: ok, provado no KWrite) | AT-SPI + portal |
| Firefox | página exposta (289 → 1349 nós) | barra de endereço **ignora** `insertText` e `setTextContents` (devolvem True e nada muda); **colar** (área de transferência + Ctrl+V pelo portal) ok em 0,17 s | AT-SPI + portal; texto por colar |
| VS Code | aparece com 2 nós, sem árvore | — | `code -g arquivo:linha` (CLI) + imagem da tela. A flag `--force-renderer-accessibility` não chegou ao processo; o único jeito de forçar é `editor.accessibilitySupport: on`, que põe o editor em modo leitor de tela (efeito colateral no uso do Arthur) |
| Spotify (flatpak, CEF) | não aparece nem com a flag | — | MPRIS para mídia; imagem da tela + clique por coordenada para o resto |

`gsettings org.gnome.desktop.interface toolkit-accessibility` = `true` depois do spike.

### Texto: ordem de preferência

1. `EditableText.insertText` (Qt/GTK, campos de página): exato, 5 ms, não toca na área
   de transferência.
2. Colar: grava a área de transferência, Ctrl+V pelo portal, **devolve** o conteúdo
   anterior. Provado no Firefox; a área de transferência do Arthur voltou intacta.
   Pegadinha: `wl-copy` vira daemon e segura o stdout herdado (o comando do spike
   "travou" por isso). Na implementação, usar o portal `Clipboard` (existe, exige
   `RequestClipboard` antes do `Start`) ou o Klipper por D-Bus
   (`org.kde.klipper /klipper setClipboardContents`), não `wl-copy` em subprocesso.
3. Keysym do portal: só tecla nomeada e atalho. Nunca texto (falha em `ç`/`ã`).

## Outros achados úteis para a spec

- Portal `Trash` existe: "apagar" pode virar "mandar para a lixeira" (serve à spec de
  autonomia de arquivos).
- O `/WindowsRunner` do KWin acha janela por texto do título sem script; o script do
  KWin é mais completo (geometria, pid, monitor) e foi o usado.
- O device roda em Python 3.13 (mise) e o `gi`/`pyatspi` do sistema é para 3.14:
  decidir na spec como o device importa `gi` (PyGObject no venv, venv no Python do
  sistema, ou um processo auxiliar no Python do sistema).

## Em aberto (decisão de spec, não de spike)

- Ligar a acessibilidade de forma persistente: o device faz o `Set IsEnabled=true` ao
  subir? Apps abertos antes disso precisam ser reabertos.
- Onde guardar o `restore_token` (0600, fora do git) e o que fazer quando o portal
  revoga (pedir de novo pela voz: "preciso de permissão, clique em Permitir").
