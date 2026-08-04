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

## Estado exato: 16 de 25 tasks fechadas

HEAD do worktree: `e4f7783` (branch `worktree-fase-1-nucleo`).

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
| **17** | **`internal/api` WS clientes + aprovação** | 🔶 **próxima — não começada** |
| 18 | `internal/api` WS dispositivo | pendente |
| 19 | `prompts/sistema.md` + `cmd/jamesd` | pendente |
| 20 | `cmd/migrar-historico` | pendente |
| 21 | `james_device/conexao.py` | pendente |
| 22 | `james_device/tela.py` + `main.py` | pendente |
| 23 | `web/` rewire + card de aprovação | pendente |
| 24 | SPA embutida + `shell/` | pendente |
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

## Próxima ação concreta

Abrir `docs/superpowers/plans/2026-08-04-james-fase-1-nucleo.md`, ir na seção
"## Task 17: `internal/api` — WebSocket de clientes, aprovação e integração
com o agente", copiar o texto completo, e dispatchar o subagente implementador
seguindo o workflow acima. Essa é a task mais complexa do lote restante
(envelope WS, `ClientesHub`, fila de aprovações pendentes, `processarTurno`
compartilhado com a Task 18) — dar contexto extra sobre o que as Tasks
13-16 já entregam (repositórios reais, `Servidor` de Task 16 que essa task
estende) e sobre o que a Task 18 vai reusar de `processarTurno` (não
implementar Task 18 agora, só deixar a função pronta pra ser reusada).
