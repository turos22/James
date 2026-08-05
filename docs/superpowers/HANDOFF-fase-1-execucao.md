# Handoff — Fase 1 em execução (subagent-driven-development)

Escrito em 2026-08-04, no meio da execução da Fase 1. Uma nova conversa deve
ler este arquivo inteiro antes de continuar — ele substitui a necessidade de
reler o histórico da sessão anterior.

## Onde está tudo

- **Worktree ativo:** `C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts\.claude\worktrees\fase-1-nucleo`
  (branch `worktree-fase-1-nucleo`, criado a partir de `main` via `EnterWorktree`).
  **Todo trabalho da Fase 1 acontece aqui, nunca no checkout principal.**
- **Spec:** `docs/superpowers/specs/2026-08-04-james-fase-1-nucleo-design.md`
- **Plano:** `docs/superpowers/plans/2026-08-04-james-fase-1-nucleo.md` — 25 tasks,
  cada uma com texto completo (testes + implementação), ordem testável-primeiro.
  **Esse arquivo é a fonte da verdade de cada task** — não resuma de memória,
  releia a seção da task antes de dispatchar.
- **Handoff da Fase 0** (contexto anterior, ainda válido): `docs/superpowers/HANDOFF.md`

## Estado exato: 23 de 25 tasks fechadas

HEAD do worktree: `66d5c09` (branch `worktree-fase-1-nucleo`).

| Task | O que é | Status |
|---|---|---|
| 1 | `internal/higiene` detecção genérica | ✅ fechada |
| 2 | `internal/config` | ✅ fechada |
| 3 | `internal/ferramentas` tipos e registro | ✅ fechada |
| 4 | `internal/permissao` | ✅ fechada |
| 5 | `internal/agente` loop | ✅ fechada |
| 6 | `internal/agente/gemini` | ✅ fechada |
| 7 | `ferramentas.ComandoSistema` | ✅ fechada |
| 8 | `ferramentas.ClaudeCode` | ✅ fechada |
| 9 | `ferramentas.Busca` | ✅ fechada |
| 10 | `internal/dispositivo` hub | ✅ fechada |
| 11 | `ferramentas.ManipularInterface` | ✅ fechada |
| 12 | migrações SQL (`internal/armazenamento`) | ✅ fechada |
| 13 | repositórios Postgres (Sessao/Mensagem/Aprovacao) | ✅ fechada |
| 14 | `internal/memoria` | ✅ fechada |
| 15 | `internal/voz` | ✅ fechada |
| 16 | `internal/api` REST | ✅ fechada |
| 17 | `internal/api` WS clientes + aprovação | ✅ fechada (commits `abccca9` implementação + `105ebd0` fix pós-revisão) |
| 18 | `internal/api` WS dispositivo | ✅ fechada (commits `c8d63af` implementação + `2e0b72f` fix pós-revisão) |
| 19 | `prompts/sistema.md` + `cmd/jamesd` | ✅ fechada (commits `b2fd281` implementação + `d1ee120` fix pós-revisão) |
| 20 | `cmd/migrar-historico` | ✅ fechada (commits `2355bde` implementação + `c119acd` fix pós-revisão) |
| 21 | `james_device/conexao.py` | ✅ fechada (commits `c0619cc` implementação + `bcd0742` fix pós-revisão) |
| 22 | `james_device/tela.py` + `main.py` | ✅ fechada (commits `e991cac` implementação + `55a46b0` fix pós-revisão) |
| 23 | `web/` rewire + card de aprovação | ✅ fechada (commits `7546fef` rename + `66d5c09` conteúdo, zero fix pós-revisão) |
| **24** | **SPA embutida + `shell/`** | 🔶 **próxima — não começada** |
| 25 | remoção de código morto e fechamento | pendente |

**Rastreamento formal:** existe uma lista de tasks no harness (TaskList) com IDs
#1-32 espelhando exatamente essa tabela (mais as 6 tasks de brainstorming/plano
que já fecharam antes). Numa nova conversa, `TaskList` não persiste — recriar
mentalmente o equivalente ou usar `TaskCreate`/`TaskUpdate` de novo se quiser
o rastreamento visual; não é obrigatório, o que importa é a tabela acima e os
commits no worktree.

## Como continuar (workflow exato usado até aqui)

Skill: **`superpowers:subagent-driven-development`**. Para cada task pendente,
nesta ordem:

1. `git rev-parse HEAD` no worktree — anotar o SHA base.
2. Dispatch um subagente **implementador** (`Agent` tool, `subagent_type: general-purpose`,
   `run_in_background: false`) com:
   - Texto **completo** da task copiado do plano (não resuma, cole o markdown
     da task inteira — testes + implementação, exatamente como está no arquivo).
   - Contexto: em que task estamos, do que ela depende (quais commits já
     existem e o que eles exportam), convenção do repo (português, Go 1.26).
   - Regras não-negociáveis: nunca `go build .`/`go build ./...` sem `-o`;
     nunca imprimir/logar valor de credencial.
   - Se a task envolve Postgres: mencionar que **este worktree tem um `.env`
     real** (ver seção abaixo) e que os testes devem rodar de verdade, nunca
     pular — se pularem, é BLOCKED, não sucesso.
   - Pedir: seguir TDD dado, rodar de verdade, commitar, self-review, reportar
     com Status (DONE/DONE_WITH_CONCERNS/BLOCKED/NEEDS_CONTEXT) + saída real
     de teste colada + SHA do commit.
3. Dispatch um subagente **revisor de spec** (mesmo padrão) que releia o código
   real (não confie no relato do implementador), rode os testes de novo, e
   devolva ✅ Spec compliant ou ❌ issues específicos.
4. Dispatch um subagente **revisor de qualidade** (mesmo padrão, seguindo o
   template de `requesting-code-review/code-reviewer.md`) com o diff
   `BASE_SHA..HEAD`, pedindo Strengths/Issues (Critical/Important/Minor)/
   Assessment (`Ready to merge? Yes|No|With fixes`).
5. Se achou Critical/Important: **use `SendMessage` pro mesmo agentId do
   implementador** (não um agente novo) pra ele corrigir com contexto
   preservado, depois re-dispatch só o revisor relevante pra confirmar o fix.
   Ou, se o fix for pequeno e bem entendido (como aconteceu várias vezes),
   aplique direto você mesmo (Edit + rodar teste real + commit) — isso é
   aceito e já foi feito várias vezes nesta sessão, desde que sempre com
   evidência real rodada, nunca "deve funcionar".
6. Só marcar a task como fechada quando o revisor de qualidade disser
   "Ready to merge: Yes" sem pendência.

**Isso já achou e corrigiu bugs reais em produção**, não só nitpicks — exemplos:
role `"function"` inexistente na API Gemini (era `"user"`, corrigido via
verificação no context7 contra a doc oficial do SDK), deadlock em
`ResolverResultado` num resolve duplicado, `MensagemRepositorio.Salvar` que
podia duplicar mensagem em retry, `internal/memoria` tratando screenshot como
custo zero. **Leve os revisores a sério — eles pegam coisa de verdade.**

## Ambiente deste worktree — importante

- **`.env` real foi copiado pra dentro do worktree** (`cp`, nunca lido/impresso
  o conteúdo) porque o Arthur não estava em casa pra copiar manualmente e
  autorizou explicitamente. Está gitignored, nunca vai aparecer em commit.
  Se uma nova sessão abrir este worktree e o `.env` não estiver lá por algum
  motivo, perguntar ao Arthur antes de tentar recriar — nunca inventar
  credencial nem pedir pra colar no chat.
- **Postgres real já está rodando**: container `james-postgres`, saudável,
  `127.0.0.1:5433`. `docker compose up -d` já foi rodado; deve continuar de pé,
  mas confirmar com `docker ps` no início de uma nova sessão.
- **Nunca imprimir/logar valor de credencial**, nem em relatório de subagente.
- **Nunca `go build .` ou `go build ./...` sem `-o`** — o módulo é `james`;
  sem `-o` deposita um `.exe` na raiz com segredo do fonte compilado dentro.
- `go build ./...`/`go vet ./...` no repo inteiro **sempre falha** em
  `main.go:14` (`//go:embed all:frontend/dist`, diretório não construído) —
  isso é esperado e pré-existente até a Task 24 (SPA embutida) fechar; não é
  regressão de nenhuma task. Rodar `go build -o /dev/null ./internal/...`
  ou o pacote específico em vez do repo inteiro.
- Uma vez, um subagente caiu por limite de sessão da API (transitório,
  resetava às 18:40 horário de SP). Se acontecer de novo: não insistir cego,
  perguntar ao Arthur como prefere seguir (retry / assumir eu mesmo com o
  mesmo rigor / pausar até liberar).

## Backlog acumulado (não bloqueia, já registrado no plano)

O plano (seção final, "Backlog aberto ao fechar a fase") já lista os achados
de design maiores (allowlist de peer Tailscale, WebView2 contra HTTPS real,
rename manual da pasta, streaming token-a-token do Gemini) e agora também
uma lista consolidada de achados menores dos revisores das Tasks 1-16
(vazamento de `err.Error()` na API REST, `json.RawMessage` vs `[]byte`,
falta de índice em `aprovacoes(sessao_id)`, etc.) — releia essa seção antes
de decidir se algum desses vale resolver agora ou fica mesmo pro fechamento
(Task 25) ou pra Fase 3.

## Task 17 — o que ficou registrado pra quem ler depois

Fechada com o ciclo completo (implementador → revisor de spec ✅ compliant →
revisor de qualidade achou 1 Critical + 2 Important → fix aplicado direto por
mim, sem novo subagente implementador → revisor de qualidade re-rodou e deu
"Ready to merge: Yes"). Os achados corrigidos, todos reais e testados contra
Postgres de verdade (não teatro):
- **Critical**: desconexão do cliente (`ctx.Done()`) durante uma aprovação
  pendente nunca gravava o estado final no banco — ficava `pendente` pra
  sempre. Corrigido chamando `Aprovacoes.Decidir(context.Background(), ...,
  "expirada")` nesse branch (precisa de `context.Background()` porque o `ctx`
  original já está cancelado).
- **Important**: tanto o branch de timeout quanto o de desconexão vazavam a
  entrada em `registroAprovacoesPendentes.canais` pra sempre (só `resolver()`
  limpava, e só é chamado quando "aprovar"/"negar" chega). Corrigido com um
  novo método `remover(id)`.
- **Important**: `BroadcastAprovacaoRequerida` segurava o mutex do hub durante
  escrita bloqueante no socket — um cliente com socket travado bloquearia
  Registrar/Remover de todas as outras conexões. Corrigido com snapshot da
  lista sob lock + escrita fora do lock, e `SetWriteDeadline(5s)` em
  `conexaoCliente.Enviar` como cinto e suspensório.

Testes novos que provam os fixes (Postgres real, sem skip):
`TestCriarSolicitarAprovacaoExpiraELimpaAoEstourarTimeout` e
`TestCriarSolicitarAprovacaoExpiraELimpaAoDesconectar`
(`internal/api/criar_solicitar_aprovacao_test.go`), mais
`TestRemoverDescartaEntradaSemVazarNoMapa`/`TestRemoverIDDesconhecidoNaoQuebra`.

**Ressalvas de robustez registradas pelo revisor (não bloqueiam, ficam pro
backlog da fase — seção "Backlog acumulado" abaixo)**:
- O `Decidir(context.Background(), ...)` no branch de desconexão não tem
  timeout próprio — se o Postgres estiver lento nesse instante, essa limpeza
  pode travar sem bound (diferente do branch de timeout normal, que ainda usa
  o `ctx` original válido).
- `ws.go` faz `defer conn.Close()` direto no `*websocket.Conn`, fora do mutex
  `conexaoCliente.mu` que serializa as escritas — o gorilla/websocket exige
  um único escritor concorrente incluindo `Close`, então existe uma race
  teórica entre um `Enviar` em andamento (via broadcast) e o `Close` no fim
  da conexão. Pré-existente, não introduzida por este fix, não avaliada como
  bloqueante.

## Task 18 — resumo

Fechada com o ciclo completo, achado **Important** único (não Critical desta
vez): `conexaoDispositivo.Enviar` (novo, `internal/api/dispositivo.go`) não
tinha `SetWriteDeadline` — mesmo padrão de bug já corrigido na Task 17 pro
lado do cliente, reaparecendo no socket do device. Agravante: `dispositivo.Hub.
EnviarAcao` chama `Enviar` de forma síncrona ANTES do próprio `select` de
timeout/cancelamento, então sem deadline o travamento escapava até do
timeout do turno chamador. Corrigido reusando a constante `tempoLimiteEscrita`
(5s) já definida em `clientes.go`. `BroadcastParaPrivilegiados` nasceu
correta desde a implementação (já usando o padrão snapshot-fora-do-lock
da Task 17, sem repetir o bug antigo).

## Task 19 — resumo

`cmd/jamesd/main.go` amarra tudo (config, Postgres, Gemini, ferramentas,
API). Achou 2 **Important**, ambos sobre a filosofia de degradação
graciosa que o próprio código já declarava mas não seguia por completo:
- Erro de `ListenAndServeTLS` (cert do Tailscale ainda não emitido, cenário
  plausível de primeiro deploy) escrevia no mesmo canal fatal do loopback e
  matava o processo inteiro via `os.Exit`, mesmo com o loopback já servindo
  com sucesso. Corrigido: canal fatal (`errosServidor`, agora tamanho 1) só
  recebe do loopback; a goroutine da VPN, em erro, só loga e segue.
- `resolverIPTailscale` não checava `net.FlagUp` — interface Tailscale
  presente mas down (deslogada) podia reter IP obsoleto de sessão anterior
  (comum no Windows) e gerar um erro de bind confuso em vez do erro claro
  que a função foi desenhada pra dar. Corrigido com checagem de `FlagUp`
  antes de consultar `Addrs()`, mais teste novo cobrindo esse caminho.

Nenhuma das duas tasks teve desvio do texto do plano na implementação —
os dois ciclos de fix vieram só da rodada de revisão de qualidade, não de
erro de leitura da spec.

## Task 20 — resumo

Achado **Important** único: idempotência de `executar()` era só "sessão já
existe → pula tudo" — se a migração falhasse no meio do laço de inserts, a
sessão ficava parcial no banco, e rodar de novo achava essa sessão e pulava
o resto silenciosamente (exit 0, perda de dado de histórico sem sinal
nenhum). Corrigido envolvendo criação da sessão + todos os inserts numa
única transação Postgres (bypassando os repositórios `SessaoRepositorio`/
`MensagemRepositorio` só nesse caminho de escrita, já que eles só falam
com `*pgxpool.Pool`, não `pgx.Tx`). Teste novo prova o rollback forçando
falha real com um byte nulo no conteúdo de uma mensagem (Postgres rejeita
`\x00` em texto).

## Mudança de stack a partir daqui

As Tasks 1-20 são todas Go (`internal/`, `cmd/`). A partir da **Task 21** o
trabalho é **Python** (`dispositivo/james_device/`) — o cliente que roda na
própria máquina do Arthur, conecta no `jamesd` via WebSocket, e expõe voz +
controle de tela pro sistema operacional. O workflow de subagent-driven-
development (implementador → revisor de spec → revisor de qualidade)
continua o mesmo, só troca a stack — adaptar as regras de ambiente Go
(nunca `go build` sem `-o`, etc.) pras equivalentes Python (rodar
`pip install -e ".[dev]"` dentro de `dispositivo/`, testes via `pytest`,
sem regra de build binário).

## Task 21 — resumo

Achou 2 **Important**: (1) `json.loads`/`manipulador_acao` levantando
exceção derrubava a sessão E o `except (OSError, ConnectionClosed)` de
`executar()` não capturava — matava o loop de reconexão pra sempre,
contradizendo a promessa central do módulo; (2) `parar()` só setava o
`Event`, não desbloqueava um `recv()` ocioso — sessão conectada e sem
tráfego nunca via o sinal de parada. Corrigidos: try/except por mensagem
dentro de `_sessao` (uma mensagem ruim não derruba a sessão), `except
Exception` genérico em `executar()` como rede de segurança final, e
`_ws_atual`/`_lock_ws_atual` pra `parar()` conseguir fechar a conexão ativa
de fora. 4 testes novos provam os dois fixes.

**Débito técnico registrado pro backlog (não bloqueou o fechamento)**:
existe uma race residual bem estreita — se `parar()` for chamado exatamente
entre `executar()` sair do `while` e `_sessao` setar `_ws_atual`, a sessão
que conecta em seguida não vê o sinal de parada e pode ficar bloqueada se
não chegar nenhuma mensagem. Fix sugerido pelo revisor: checar
`self._parar.is_set()` logo no início de `_sessao`. Também: o catch-all de
`executar()` usa `logger.info` sem traceback — perde debugabilidade de bugs
de programação reais que agora só reconectam silenciosamente; trocar por
`logger.exception` quando for mexer nesse arquivo de novo.

## Nota de ambiente Python — vale pra Task 22 também

O pacote `james-device` é instalado em modo editable, e o "Editable project
location" pode estar apontando pro checkout principal
(`C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts\dispositivo`) em vez do
worktree, dependendo do que rodou por último no Python do sistema (sem
venv). **Sempre conferir com `pip show james-device` antes de testar** — se
não apontar pro worktree, rodar `pip install -e ".[dev]"` de dentro de
`dispositivo/` do worktree pra reapontar. Sem isso, `pytest` pode
silenciosamente testar código do checkout principal.

## Task 22 — resumo

Achou 1 **Critical** real: `main.py` referenciava `dispositivo/assets/
faaah.wav` e `dispositivo/assets/entrada.mp3`, mas essa pasta nunca tinha
sido criada em NENHUMA task anterior — o processo derrubava na primeira
wake word detectada. A spec (`2026-08-04-james-fase-1-nucleo-design.md:126`)
já dizia que esses assets deveriam ser versionados em `dispositivo/assets/`,
separado de `audios/` (que fica só com saída efêmera de TTS), mas a
migração nunca tinha sido feita. Corrigido copiando `entrada.mp3`,
`faaah.wav`, `um_minuto.mp3` de `audios/` pra `dispositivo/assets/` (cópia,
não move — `audios/` continua igual, usado pelo `app.go` do lado
Wails/desktop). Também achou 1 **Important**: `sr.Microphone()` sem
PyAudio/microfone crashava sem log tratado — corrigido com `logger.exception`
antes do `raise` (comportamento de falha continua o mesmo, só ganhou log).

**`playsound` pinado em `>=1.2.2,<1.3.0`** (não `>=1.3.0` como o plano
original pedia) — bug confirmado no sdist 1.3.0 sob Python 3.13 (`setup.py`
quebrado, `inspect.getsource` falha em build isolation). Registrado como
dívida técnica pra fase futura: `playsound` tem manutenção fraca havia
anos, vale trocar por alternativa mantida quando mexer nisso de novo.

## Mudança de stack de novo: Task 23 é TypeScript/React

Tasks 21-22 foram Python (`dispositivo/`). A partir da **Task 23** o
trabalho é **TypeScript/React** (`web/`, renomeado de `frontend/`) — o
mesmo workflow de subagent-driven-development, adaptado pra `npm`/`vitest`
em vez de `pip`/`pytest` ou `go test`. Verificar se `node`/`npm` estão
disponíveis no ambiente antes de dispatchar o implementador.

## Task 23 — resumo

**Zero achado Critical/Important dos dois revisores** — primeira task da
fase inteira que fechou sem precisar de ciclo de fix. `api.ts`, `ws.ts`,
`CardAprovacao.tsx`, rewire de `App.tsx`, remoção de `web/wailsjs/` — tudo
conferido linha a linha contra o texto do plano e contra o contrato real
do lado Go (`internal/api/ws.go`), sem divergência. Único nit registrado
(cosmético, não bloqueia): `web/package.json` ficou com `"name": "frontend"`
depois do rename.

**Dois desvios operacionais do implementador, ambos corretos e verificados**:
1. `git mv frontend web` não coube num comando só (cwd travado numa sessão
   anterior) — fez rename arquivo-por-arquivo. Confirmei depois que não
   sobrou nada esquecido e removi o diretório `frontend/` vazio que ficou
   no disco (não rastreado pelo git, só resíduo de filesystem).
2. Commit foi dividido em dois (`7546fef` rename puro + `66d5c09`
   conteúdo) porque um commit único (rename + reescrita radical de
   `App.tsx` junto) fazia o git perder detecção de rename — `git log
   --follow` não alcançava o histórico anterior. Confirmei que a divisão
   resolveu isso pros três arquivos testados (`App.tsx`, `package.json`,
   `vite.config.ts`).

## Ambiente confirmado pra Task 24

`wails` CLI v2.11.0 disponível (bate com `go.mod`). `web/` tem `npm run
build` (`tsc && vite build`) funcional. Root `main.go` já está com
`//go:embed all:frontend/dist` quebrado (a pasta `frontend/` nem existe
mais desde a Task 23) — isso é esperado, é exatamente o arquivo que a
Task 25 vai deletar; a Task 24 não mexe nele, cria `web/embed.go` e
`shell/main.go` novos e independentes.

## Próxima ação concreta

Abrir `docs/superpowers/plans/2026-08-04-james-fase-1-nucleo.md`, ir na seção
"## Task 24: `jamesd` serve a SPA embutida + `shell/` Wails mínimo", copiar
o texto completo, e dispatchar o subagente implementador seguindo o
workflow acima. Essa task: cria `web/embed.go` (embute `web/dist` via
`go:embed`), adiciona `EmbutidosSPA fs.FS` ao `Servidor` (`internal/api/
rest.go`) com rota catch-all `/` registrada DEPOIS das rotas `/api/*` e
`/ws*` (prioridade), liga isso em `cmd/jamesd/main.go`, e cria um `shell/`
novo — invólucro Wails mínimo sem UI própria (`shell/frontend/index.html`
só redireciona pra `http://127.0.0.1:8080/`, `shell/main.go` sobe uma
janela frameless/transparente/always-on-top apontando pro `jamesd`
local), com `shell/wails.json` adaptado do `wails.json` da raiz (que
depois é removido). Precisa de `npm run build --prefix web` rodando de
verdade ANTES do `go build` (senão `web/embed.go` não tem `dist/` pra
embutir). O smoke test de janela nativa (Step 9) é manual — não é
responsabilidade do subagente implementador, fica pro fechamento da fase
(Task 25 + verificação final). Atenção à nota do próprio plano: a API
exata de `assetserver.Options`/`options.App` deve ser conferida contra a
versão instalada do Wails (usar context7 se `go build` reclamar de campo
inexistente).

Depois da Task 24, só falta a Task 25 (remoção de código morto +
fechamento formal da fase, com a checklist de 10 critérios de aceitação
da spec seção 9) pra fase inteira fechar.
