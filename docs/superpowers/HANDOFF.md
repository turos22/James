# Handoff — James (ex-Athena)

Documento de retomada. Escrito em 2026-08-04 ao final da Fase 0, pra nova
conversa continuar sem reler a sessão inteira. Conteúdo completo de decisão
já vive nos dois docs abaixo — este arquivo só aponta pra eles e guarda o
que não está em nenhum outro lugar.

## Onde está tudo

- **Spec (Fase 0+1):** [`docs/superpowers/specs/2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`](specs/2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md)
- **Plano (Fase 0, fechado):** [`docs/superpowers/plans/2026-08-03-james-fase-0-fundacao.md`](plans/2026-08-03-james-fase-0-fundacao.md) — tem seção "Status final" no fim, com evidência da verificação e backlog
- **Spike do Tailscale:** [`docs/superpowers/spikes/2026-08-03-tailscale.md`](spikes/2026-08-03-tailscale.md)

## Estado

Fase 0 fechada. 12 commits, `6f18fa8`..`5da99ec`, branch `main`, working tree limpo.
Suítes verdes: Go (`go test ./...`), Python (`dispositivo/`, pytest), TypeScript
(`frontend/`, vitest). Postgres local de pé via `docker-compose.yml`. Rename
Athena → James em andamento (módulo Go já é `james`; rename completo de pastas
e persona é Fase 1).

**Próximo passo:** brainstorming → spec → plano da Fase 1 (núcleo: `jamesd`,
interface `Provider` + Gemini, ferramentas, permissão, API REST+WS,
armazenamento, memória, TTS entregue ao cliente). Ainda não começado.

## Regras que não estão em nenhum doc, só nesta sessão

- **Nunca `go build .` ou `go build ./...` sem `-o`.** Módulo é `james` — sem
  `-o`, deposita `james.exe` na raiz com qualquer segredo do fonte compilado
  dentro. Já aconteceu uma vez, foi pego na revisão e corrigido. Sempre
  `go build -o /dev/null .`.
- **Nunca imprimir valor de credencial** em relatório de subagente, log, ou
  comando de debug — nem as já revogadas. Confirmar presença/ausência com
  `grep -c`, nunca com o valor.
- **`.env` real existe e está preenchido** (Gemini, Tavily, `POSTGRES_PASSWORD`,
  `DATABASE_URL`) — só o Arthur edita esse arquivo. Nunca ler nem sugerir
  colar chave no chat.
- **Postgres:** `docker-compose.yml`, container `james-postgres`, porta
  **5434** no host. Era 5433 (escolhida pra não colidir com a instalação
  nativa na 5432), mas a 5433 é usada pelo Postgres do Final-project, que
  sobe com `restart: always` e sempre chega antes — ver o comentário no
  `docker-compose.yml`. Mudou em 2026-09-24,
  loopback only.
- **Tailscale:** binário não está no PATH do Git Bash —
  `"/c/Program Files/Tailscale/tailscale.exe"`. PC = `desktop-dc358rs`,
  celular = `s21-fe-de-arthur`. IP muda a cada relogin, nunca fixar — resolver
  por nome DNS ou runtime. "HTTPS Certificates" precisa estar ligado no
  admin console (`login.tailscale.com/admin/dns`) — já foi ligado.

## Como o trabalho foi conduzido (pra manter o padrão)

- Ciclo: `brainstorming` → `writing-plans` → `subagent-driven-development`.
- Cada task do plano: subagente implementador → revisor de conformidade
  (compara código real contra a spec, não confia no relato) → revisor de
  qualidade → correções voltam pro mesmo implementador → re-revisão →
  fecha. Achado Importante/Crítico sempre corrigido antes de seguir, nunca
  só anotado.
- `AskUserQuestion` usado nos pontos que só o Arthur podia decidir:
  topologia de rede, alcance da VPN, modelo de permissão por cliente,
  destino do processo Python, banco na Fase 1 vs Fase 2, Vault no
  `.gitignore`, resolução do incidente do `james.exe`/`.env.example`.
- `git init` só rodou depois da limpeza de credenciais — nunca antes.
- Linguagem: `caveman lite` em execução e status; normal em design/spec.
  Modo trabalho ativo (SDD + TDD + verificação com evidência obrigatória).

## Backlog aberto (não bloqueia, registrado no plano)

1. Guarda de credenciais (`internal/higiene`) só cobre 4 prefixos conhecidos —
   sem detecção genérica (senha em string de conexão, AWS, GitHub, JWT, PEM).
   Reforçar antes/durante a Fase 1, quando `DATABASE_URL` real vai circular mais.
2. Sem critério formal pra quando uma correção descoberta em execução vira
   seção documentada no plano vs. só um commit solto.
3. WebView2 real ainda não testado contra o HTTPS do tailnet — o spike usou
   um Chromium genérico. Confirmar quando o shell da Fase 1 existir.
