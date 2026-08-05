# Handoff — Fase 1 fechada, começando a Fase 3

Escrito em 2026-08-05, no fim da execução da Fase 1 (já mergeada em `main`)
e antes de qualquer trabalho da Fase 3 começar. Uma nova conversa deve ler
este arquivo inteiro antes de fazer qualquer coisa.

## Onde está tudo

Não tem worktree ativo — todo trabalho acontece direto em `main`, em
`C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts`. O worktree usado pra
executar a Fase 1 já foi mergeado e removido nesta sessão.

- **Handoffs anteriores** (contexto histórico, ainda válidos):
  `docs/superpowers/HANDOFF.md` (Fase 0), `docs/superpowers/HANDOFF-fase-1-execucao.md` (execução das 25 tasks da Fase 1).
- **Specs**: `docs/superpowers/specs/2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`
  (spec original, com o Anexo A — bugs mapeados por fase) e
  `docs/superpowers/specs/2026-08-04-james-fase-1-nucleo-design.md` (spec detalhado da Fase 1).
- **Plano executado**: `docs/superpowers/plans/2026-08-04-james-fase-1-nucleo.md` (25 tasks, todas fechadas).

## Estado do projeto: Fase 0 e Fase 1 completas, mergeadas em `main`

25/25 tasks da Fase 1 fechadas, cada uma revisada (implementador → revisor
de spec → revisor de qualidade), com achados reais corrigidos antes de
fechar. Merge fast-forward pra `main` sem conflito. Suíte inteira passando
nas 3 stacks (Go, Python, TypeScript) no checkout principal, não só no
worktree.

**O que existe hoje:**
- `cmd/jamesd` — back-end Go: REST + WebSocket, Postgres, loop do agente
  (Gemini), 4 ferramentas (`executar_comando_sistema`,
  `perguntar_ao_claude_code`, `manipular_interface`, `pesquisar_internet`),
  perfil Privilegiado (loopback) vs Restrito (qualquer outra origem) com
  fluxo de aprovação humana pra ações perigosas.
- `dispositivo/james_device/` — cliente Python que roda na máquina física:
  conecta no `jamesd` via WS, reconecta sozinho, wake word, reconhecimento
  de voz, controle de tela via pyautogui.
- `web/` — SPA React, servida pelo próprio `jamesd` (SPA embutida via
  `go:embed`), fala com o back-end por HTTP/WS.
- `shell/` — invólucro Wails mínimo: janela nativa (bolinha flutuante que
  expande em chat), sem lógica de aplicação própria — só aponta pro
  `jamesd` local e agora também expõe um servidor de controle (ver abaixo).
- `cmd/migrar-historico` — importa o `Athena.db` antigo pro Postgres,
  idempotente e atômico (rollback em falha parcial).

**Removido nesta fase:** `app.go`, `database.go`, `main.go` (raiz, Wails
monolítico antigo), `processador_voz.py`, `run_athena.bat`, `vosk_api.h`.

## Correções feitas DEPOIS do merge, testando a aplicação de verdade

O merge sozinho não bastou — testar a aplicação de verdade (browser real
contra o `jamesd` rodando, depois `wails dev` de verdade) achou vários bugs
que nenhum teste automatizado pegaria, porque dependem de comportamento
real do WebView2/Wails. Todos corrigidos e commitados direto em `main`
(sem worktree, já que a Fase 1 tinha acabado de fechar):

1. **`dispositivo/pyproject.toml`** — descoberta de pacote quebrada desde
   que a Task 22 criou `assets/` (nunca reinstalado do zero antes). `commit
   03831f5`.
2. **`certs/`** (certificado TLS da VPN Tailscale) — gitignorado, e o guard
   de credenciais (`internal/higiene`) ganhou a mesma exceção que `.env`
   já tinha. `commit 6110d85`.
3. **Arraste e resize da janela nunca funcionavam de verdade** —
   `window.runtime` do Wails só é injetado em páginas servidas pelo
   próprio AssetServer do Wails, nunca na página que o `jamesd` serve
   (processo separado). Corrigido com um servidor de controle local em
   `shell/controle.go` (só loopback, porta `17313`) que a SPA chama via
   `fetch()` — o lado Go, que tem o `ctx` real da janela, executa a
   chamada de verdade via `pkg/runtime` do Wails. `commit 1146c5d`.
4. **Áudio de resposta (TTS) nunca tocava** — o evento `audio_pronto`
   nunca teve handler nenhum no frontend desde a Task 17. Agora toca via
   `<audio>` e chama `DELETE /api/audio/{id}` (endpoint novo) ao terminar
   — nunca acumula arquivo em `audios/`. `commit 1146c5d`.
5. **Chat abrindo pra fora da tela** quando a bolinha está perto da borda
   (ex.: barra de tarefas) — resize simples sempre cresce a partir do
   canto superior-esquerdo. Trocado por resize ancorado no canto onde a
   bolinha já está (`calcularRedimensionamentoAncorado`, função pura e
   testada), com clamp contra o tamanho real da tela via `ScreenGetAll`.
   `commit 2049b18`.
6. **Fundo da janela ficando rosa/magenta sólido em vez de transparente**
   — `WindowIsTranslucent` não é o que o nome sugere (liga blur-behind/
   Mica, não transparência real) e combinado com `WS_EX_NOREDIRECTIONBITMAP`
   deixava a composição da janela quebrada. Removido — `WebviewIsTransparent`
   sozinho já é o mecanismo certo. `commit 951de3a`.
7. **Condição de corrida no arraste** — `aoIniciarArraste` buscava a
   posição da janela de forma assíncrona antes de marcar
   `arrastando.current = true`; num clique rápido, o `mouseup` podia
   chegar antes dessa resposta, deixando o estado de arraste preso em
   `true` e movendo a janela sem o usuário pedir (isso desalinhava o
   resize ancorado do item 5, e era a causa provável do chat não voltar
   pro mesmo lugar ao fechar). Corrigido com uma guarda síncrona
   (`mouseApertado`). Mesmo commit `951de3a`.

**Confirmado funcionando ao vivo** (browser real contra `jamesd` rodando):
chat abre, manda mensagem, recebe resposta do Gemini; `manipular_interface`
sem nenhum `james-device` conectado devolve erro claro
("James-device não conectado") em vez de travar, e o `jamesd`/a sessão
continuam normais depois.

## Item aberto, não investigado — Arthur pediu pra deixar pra depois

Testando pelo celular na VPN Tailscale: a aprovação de uma ação perigosa
(card no desktop, Aprovar) **não estava executando a ação de verdade**
mesmo depois de aprovada. Arthur disse explicitamente "não vamos nos
preocupar com isso agora" — não foi investigado. Vale olhar antes ou
durante a Fase 3, já que pode estar relacionado à qualidade da ferramenta
`manipular_interface` (que é justamente parte do escopo da Fase 3 — ver
abaixo) ou a algo na ponte `internal/dispositivo`/`james_device`.

## IMPORTANTE: a "Fase 2" não existe — não criar spec pra ela

A numeração original (spec `2026-08-03-...-design.md`, seção "Numeração
das fases") reservou os números 0–6, mas a Fase 2 (migração pra Postgres)
foi absorvida pela Fase 1 de propósito — o número fica **vago**, não
renumerado, pra não invalidar tudo que já foi decidido/documentado com
essa numeração. Mapa vigente:

| Fase | Conteúdo | Status |
|---|---|---|
| 0 | Fundação: segredos, git, testes, Docker | ✅ fechada |
| 1 | Núcleo: back-end como serviço | ✅ fechada (mergeada) |
| ~~2~~ | *vaga — absorvida pela 1 (Postgres) e pela 4 (pgvector)* | não existe |
| **3** | **Qualidade das ferramentas** | 🔶 próxima |
| 4 | Obsidian como cérebro de contexto (RAG, pgvector) | pendente |
| 5 | Máquina física: controle de tela e voz (coordenadas reais, não adivinhadas) | pendente |
| 6 | Front-end novo | pendente |

## Escopo conhecido da Fase 3, direto do Anexo A do spec original

O Anexo A ("Bugs mapeados", lido em 2026-08-03) já marca explicitamente
4 problemas como pertencentes à Fase 3 — isso não é exaustivo (é onde
começar a brainstormar, não a lista final), mas é ponto de partida real,
não invenção:

| # | Problema | Onde estava (código antigo, já removido) |
|---|---|---|
| 15 | `toolBusca` (Google Search grounding) declarada e nunca usada — código morto | `app.go:135` vs `app.go:485` |
| 16 | Tavily em `search_depth basic`, sem `include_answer`, devolve JSON bruto pro modelo | `app.go:226`, hoje `internal/ferramentas/busca.go` |
| 17 | Claude Code sequestrado pro OpenRouter (modelo NVIDIA grátis) em vez da assinatura Pro real | `rodar_claude.ps1:5-7` |
| 18 | Claude Code fire-and-forget: abre janela nova, não captura saída nenhuma | `app.go:183`, hoje `internal/ferramentas/claudecode.go` |

O comando que desfaz o sequestro do Claude Code (achado registrado no spec
original, seção "Fato relevante: assinatura Pro ≠ API"):
```powershell
Remove-Item Env:\ANTHROPIC_BASE_URL, Env:\ANTHROPIC_API_KEY, Env:\ANTHROPIC_AUTH_TOKEN -ErrorAction SilentlyContinue; claude
```
seguido de `/login`.

A spec da Fase 1 (`2026-08-04-james-fase-1-nucleo-design.md:234`) também
registra: as 4 ferramentas entraram na Fase 1 "com o comportamento ruim
que têm hoje" — melhorar o *comportamento* delas (não só a estrutura) é
exatamente o que a Fase 3 existe pra fazer.

## Workflow a seguir (mesmo das Fases 0 e 1)

1. **`brainstorming`** primeiro — explorar com o Arthur o que exatamente
   entra na Fase 3 além dos 4 itens do Anexo A (ex.: o item aberto da
   aprovação que não executa, se ele quiser incluir), decisões de design,
   trade-offs.
2. **`writing-plans`** — depois do brainstorm aprovado, produzir o plano
   task-por-task (mesmo formato das Fases 0/1: cada task com texto
   completo de teste + implementação, ordem testável-primeiro).
3. **`subagent-driven-development`** — executar o plano com o ciclo
   implementador → revisor de spec → revisor de qualidade por task, igual
   foi feito nas 25 tasks da Fase 1. Fix pequeno e bem entendido pode ser
   aplicado direto (Edit + teste real + commit); senão, `SendMessage` pro
   mesmo agente implementador.
4. Convenção de nome: spec em
   `docs/superpowers/specs/2026-08-05-james-fase-3-....md` (ou data real
   de quando o brainstorm rodar), plano em
   `docs/superpowers/plans/2026-08-05-james-fase-3-....md`.
5. Se o trabalho for de múltiplas sessões, criar um
   `docs/superpowers/HANDOFF-fase-3-execucao.md` (mesmo padrão da Fase 1)
   assim que a primeira task fechar.

**Decidir com o Arthur no brainstorm**: trabalhar direto em `main` (como
esta sessão fez pros fixes pós-merge) ou abrir um worktree novo pra
isolar a Fase 3 (como a Fase 1 fez)? Ambos os padrões já foram usados
neste projeto — não presumir, perguntar.

## Ambiente confirmado nesta sessão (main, não worktree)

- Postgres real: container `james-postgres`, `127.0.0.1:5433`, saudável,
  volume persistente `james_pgdata` (dados sobrevivem a `docker compose
  down`/`up`).
- `.env` e `config.toml` já existem no checkout principal e funcionam
  (GEMINI_API_KEY, TAVILY_API_KEY, DATABASE_URL válidos — testado rodando
  `jamesd` de verdade).
- `certs/tailscale.crt`/`certs/tailscale.key` gerados e funcionando pra
  VPN — celular real testado com sucesso na VPN (chat abre, conversa
  funciona em modo Restrito).
- `executar.bat` (raiz do repo) é o launcher de conveniência do Arthur:
  sobe Postgres, builda a SPA, roda `jamesd` numa janela separada, e abre
  `wails dev` do shell. Ele pediu pra manter esse arquivo atualizado
  conforme o fluxo de teste mudar — não esquecer disso ao longo da Fase 3
  se o jeito de rodar a aplicação mudar.
- Node v24.12.0, npm 11.6.2, `wails` CLI v2.11.0, Python 3.13.9 (sem venv,
  pacotes globais) — todos confirmados funcionando nesta sessão.

## Regra que vale pra qualquer sessão futura neste projeto

CLAUDE.md do Arthur é claro: nunca declarar algo funcionando/pronto sem
ter rodado o comando e lido a saída **nesta mensagem**. Isso valeu a
sessão inteira — os fixes pós-merge só foram confirmados depois de
`wails build` real, testes reais, e (quando possível) teste ao vivo via
browser contra o `jamesd` rodando de verdade. Manter esse padrão na
Fase 3.
