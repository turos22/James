# Handoff — Fase 3 fechada (qualidade das ferramentas)

Escrito em 2026-08-06, ao fim da execução da Fase 3. Uma nova conversa deve
ler este arquivo inteiro antes de continuar.

## Onde está tudo

Sem worktree — Fase 3 rodou direto em `main` (decisão D8 da spec), como os
fixes pós-merge da Fase 1.

- **Spec:** `docs/superpowers/specs/2026-08-05-james-fase-3-qualidade-das-ferramentas-design.md`
  (atualizada durante a execução com D10 e a correção da sessão persistente —
  ver seção "Decisões tomadas durante a execução, fora da spec original")
- **Plano:** `docs/superpowers/plans/2026-08-05-james-fase-3-qualidade-das-ferramentas.md`
  — 20 tasks
- **Spikes desta fase:**
  `docs/superpowers/spikes/2026-08-05-tavily-api.md`,
  `docs/superpowers/spikes/2026-08-05-claude-code-headless.md`,
  `docs/superpowers/spikes/2026-08-06-bug-aprovacao-remota.md` (o mais
  importante — diagnóstico completo do bug da aprovação remota, incluindo um
  terceiro bug achado só durante a verificação ao vivo, fora do escopo
  original)

## Estado: 20 de 20 tasks fechadas

| Task | O que é | Status |
|---|---|---|
| 1 | `ExigeConfirmacao` (interface aditiva) + gate no loop | ✅ `2101286` |
| 2 | `ComandoEhLeitura` (classificador PowerShell) | ✅ `2a8b891` + fix `d93963c` |
| 3 | `ComandoSistema.ConfirmacaoNecessaria` | ✅ `e0129da` |
| 4 | Truncagem de saída (`truncar`) | ✅ `0910d17` + reforço de teste `e0a63fc` |
| 5 | `ResultadoComando` (stdout/stderr/exit code) | ✅ `8565e62` |
| 6 | `-EncodedCommand` + timeout real | ✅ `0f1409c` |
| 7 | Seção `[ferramentas]` no config | ✅ `ab5d771` |
| 8 | Injeção de config no `executar_comando_sistema` | ✅ `a628906` + extração pra função pura `9355184` |
| 9 | Spike da API da Tavily | ✅ `6861eae` |
| 10 | Busca com parsing tipado e payload melhorado | ✅ `e2eb100` |
| 11 | Timeout da busca + injeção no `jamesd` | ✅ `66a8219` + fix status HTTP `9f392fd` |
| 12 | `AmbienteSemAnthropic` + `MontarArgs` | ✅ `b3e4870` |
| 13 | Spike do Claude Code headless | ✅ `a8a2f39` |
| 14 | Claude Code headless devolve resposta real | ✅ `1608319` (inclui D10: `ConfirmacaoNecessaria` sempre `true`) |
| 15 | Injeção de diretório/timeout no Claude Code | ✅ `10dbf3b` |
| 16 | Sessão persistente do Claude Code via `context` | ✅ `4b1bf90` |
| 17 | Verificação ao vivo do Claude Code | ✅ verificado sem UI, via WS direto — ver seção própria |
| 18 | Diagnóstico do bug da aprovação remota | ✅ `79ac529`, reaberto e completado `f6ebc5f` |
| 19 | Correção do bug da aprovação remota | ✅ `43419e5` (Bug 1 + Bug 2) + `35ac155` (Bug 3, achado na Task 17) |
| 20 | Fechamento — `executar.bat`, verificação ao vivo, handoff | ✅ este documento |

## O que a Fase 3 mudou, resumo

- **Comando PowerShell de escrita agora exige confirmação humana**, mesmo no
  perfil Privilegiado (desktop). Leitura continua livre. Restrito (celular)
  continua confirmando tudo, sem exceção — a fase só aperta, nunca afrouxa.
- **`executar_comando_sistema`** ganhou timeout (default 60s, mata o
  processo de verdade), limite de saída com truncagem sempre marcada
  (default 64 KiB por corrente), stdout/stderr separados, exit code, e
  `-EncodedCommand` com UTF-8 forçado (o `powershell.exe` sem console real
  emite no codepage OEM da máquina — texto acentuado voltava corrompido).
- **`pesquisar_internet`** usa `search_depth advanced` + `include_answer`,
  autentica por header `Authorization: Bearer` (a forma antiga, `api_key` no
  corpo, é legada), devolve texto formatado com fontes numeradas — nunca
  mais JSON bruto — e marca todo conteúdo vindo da web como
  `<conteudo_externo_nao_confiavel>`, com neutralização de tag falsa
  injetada por uma página maliciosa. Rejeita status HTTP de erro em vez de
  mascarar como "nenhuma fonte encontrada".
- **`perguntar_ao_claude_code`** é headless de verdade agora
  (`claude -p --output-format json --permission-mode acceptEdits`), devolve
  a resposta real (não mais uma string fixa de disparo), mantém contexto
  entre chamadas da mesma conversa (`--session-id` na primeira, `--resume`
  nas seguintes — verificado ao vivo, reusar o mesmo id dá erro), roda num
  diretório configurável que nunca é herdado do cwd do `jamesd`, e **exige
  confirmação humana sempre**, inclusive do Privilegiado (decisão D10,
  tomada durante a execução — ver abaixo). `rodar_claude.ps1` foi apagado.
- **Bug da aprovação remota**: três causas raiz encontradas e corrigidas,
  nenhuma delas simples suposição — todas demonstradas com teste antes do
  fix. Detalhe completo no spike `2026-08-06-bug-aprovacao-remota.md`.

## Decisões tomadas durante a execução, fora da spec original

### D10 — `perguntar_ao_claude_code` exige confirmação sempre

A spec original (D6) escolheu `--permission-mode acceptEdits` com a premissa
de que ele "edita arquivos sem perguntar, mas não executa Bash arbitrário
sem permissão". **Essa premissa estava errada.** Um spike ao vivo
(`2026-08-05-claude-code-headless.md`, item 6) provou que `acceptEdits`
**executa** comando de shell em modo headless, sem recusa,
`permission_denials` vazio.

Corrigido tratando o Claude Code igual ao comando PowerShell de escrita: a
ferramenta implementa `ExigeConfirmacao` devolvendo sempre `true`. Isso por
sua vez **causou** o Bug 3 (abaixo) — achado na mesma sessão, ao tentar
verificar a correção ao vivo.

### Sessão persistente via `context`, não campo de struct

A spec original (§6) dizia "sem estado novo" pra sessão persistente. Isso
mudou ao escrever o plano: `ClaudeCode` é construído **uma vez** e
compartilhado por todas as conversas — um campo `SessaoID` no struct seria
global e misturaria conversas diferentes. O id da sessão do James viaja pelo
`context.Context` (`ferramentas.ComSessao`/`SessaoDo`), carimbado em
`internal/api/ws.go` antes do loop rodar. Isso é o único estado novo real: um
`map[string]bool` em memória (não persistido) dizendo quais sessões já foram
iniciadas no Claude Code, pra escolher entre `--session-id` e `--resume`.

## O bug da aprovação remota — os três bugs, resumido (detalhe no spike)

O Arthur reproduziu ao vivo em 2026-08-06: celular pedindo ação perigosa,
card de aprovação nunca aparecia no desktop, processo travado no celular.

**Duas hipóteses testadas e refutadas antes de qualquer fix** (a skill de
debug sistemático foi seguida à risca): chat fechado no desktop (refutada —
estava aberto) e texto do card invisível por contraste CSS (refutada —
testado ao vivo com `vite dev` e inspeção de estilo computado real, texto
branco sobre fundo escuro, legível).

**Três bugs confirmados por leitura de código e corrigidos com teste:**

1. **`dados.id` sempre vazio** no evento `aprovacao_requerida` — o backend
   mandava o id só no envelope, nunca dentro de `dados`, e o frontend lê
   `dados.id`. Clicar Aprovar mandaria id vazio, que o backend não encontra
   no mapa de pendências. Quebra a aprovação mesmo que o card aparecesse.
2. **`TimeoutTurno` embrulhava o turno inteiro**, não só a chamada ao
   Provider como o comentário do código prometia — encurtava a janela real
   de aprovação de 300s pro que sobrasse do timeout de 30s do turno.
   Corrigido com `Loop.TimeoutProvider`, aplicado só em volta de
   `Provider.Gerar`; o campo `TimeoutTurno` foi removido de `Servidor`.
3. **A mesma conexão WS trava tentando aprovar a si própria** — achado ao
   verificar Claude Code ao vivo sob D10 (item novo, não fazia parte do
   plano original). O desktop usa uma única conexão pra mandar mensagem e
   pra aprovar; o laço de leitura processava "mensagem" de forma síncrona,
   e nunca chegava a ler o "aprovar" da mesma conexão até o timeout de
   aprovação inteiro estourar. **Confirmado ao vivo: exatos 5 minutos**,
   batendo com o default de 300s. Corrigido decompondo a leitura em goroutine
   dedicado (lê e resolve aprovar/negar na hora) + canal pro processamento
   sequencial de "mensagem".

**O que continua em aberto:** nenhum dos três bugs explica sozinho "o card
nunca aparece" no cenário original (celular pedindo, desktop aprovando —
duas conexões diferentes, não afetadas pelo Bug 3). Candidato não testado,
por falta de acesso ao vivo: `ClienteWS` (`web/src/lib/ws.ts`) não tem
lógica de reconexão nenhuma — um socket caído no desktop fica morto em
silêncio, sem indicador visual.

## Verificação ao vivo feita nesta sessão

Sem UI do desktop disponível, a verificação foi feita conectando direto no
WS do `jamesd` real (`ws://127.0.0.1:8080/ws`) via um programa Go
descartável (nunca commitado — fazia o papel do cliente Privilegiado:
manda mensagem, aprova automaticamente qualquer `aprovacao_requerida`).

**Confirmado, com evidência real:**
1. `Get-Process | Select-Object -First 3` executa **sem** pedir aprovação.
2. `Stop-Process -Name notepad -Force` **pede** aprovação; aprovada, mata o
   processo de verdade (confirmado abrindo um Notepad real antes e
   verificando com `tasklist` depois — processo sumiu).
3. `Start-Sleep -Seconds 90` é morto aos 60s (`comando excedeu o tempo
   limite de 1m0s e foi encerrado`), não os 90s completos.
4. Saída de ~1MB (`1..20000 | ForEach-Object {...}`) trunca — o próprio
   Gemini relatou "a saída foi truncada por exceder o limite de bytes" ao
   ver a marca no resultado da ferramenta.
5. `perguntar_ao_claude_code`, testado com `ANTHROPIC_BASE_URL`,
   `ANTHROPIC_API_KEY` e `ANTHROPIC_AUTH_TOKEN` **sequestrados de
   propósito** no ambiente do próprio `jamesd`: a chamada completou e
   devolveu a resposta real, provando que `AmbienteSemAnthropic` limpou o
   ambiente e a sessão autenticou pela assinatura Pro via OAuth, não pelo
   endpoint falso. Sessão persistente confirmada com duas mensagens na
   mesma conexão: a segunda corretamente lembrou o que a primeira tinha
   listado.

**Não verificado — pendente, é o único item que precisa do celular real:**
6. Ação perigosa pedida pelo celular na VPN, aprovada no desktop, executando
   de verdade e o resultado voltando ao chat do celular. Arthur vai testar
   isso depois desta sessão, com as correções dos três bugs já aplicadas.
   Se ainda falhar, o próximo passo é instrumentar
   `BroadcastAprovacaoRequerida`/`Enviar` com log — hoje o erro de escrita
   no socket é descartado em silêncio (`_ = c.Enviar(...)`).

**Suíte completa, nas três stacks, rodada nesta sessão:**
- Go: `go test -count=1 ./internal/... ./cmd/...` — 14 pacotes `ok`, nenhum FAIL.
- Web: `npm test` em `web/` — 5 arquivos, 24 testes, todos PASS.
- Python: `python -m pytest` em `dispositivo/` — 29 testes PASS.

## `executar.bat`

Revisado — não precisou de nenhuma mudança. Não referencia `rodar_claude.ps1`
(a única ocorrência restante no repo é um comentário histórico dentro de
`internal/ferramentas/claudecode.go`, explicando por que a limpeza de
ambiente existe). O fluxo de subida (`docker compose`, build da SPA,
`jamesd`, `wails dev`) continua válido — nenhuma das mudanças desta fase
alterou como o James é iniciado.

## O que a Fase 4 (ou quem retomar) precisa saber

1. **Reteste do celular pendente** (critério 6 acima) — é o item mais
   importante a fazer primeiro. Os três bugs corrigidos devem resolver o
   sintoma original, mas isso não foi confirmado ao vivo com o celular real
   ainda.
2. Se o reteste do celular falhar mesmo com os três bugs corrigidos, o
   próximo suspeito é a falta de log em `Enviar`/`BroadcastAprovacaoRequerida`
   — hoje um erro de escrita no socket desaparece silenciosamente.
3. `ClienteWS` sem reconexão automática é uma lacuna real, documentada mas
   não corrigida — considerar pra uma fase futura ou pro reteste do celular
   revelar se importa na prática.
4. O diretório do Claude Code está configurado em `config.toml` como
   `C:\Users\Arthur\SI\Projetos\james-workspace` (escolha do Arthur —
   workspace dedicado, isolado de projetos reais). Trocar isso é editar uma
   linha do `config.toml`, sem tocar em código.
5. Regra do CLAUDE.md do Arthur vale pra qualquer sessão futura: nunca
   declarar algo funcionando sem ter rodado o comando e lido a saída. Isso
   valeu a sessão inteira — os três bugs da aprovação remota só foram
   encontrados porque cada hipótese foi testada de verdade (inclusive duas
   que pareciam plausíveis e foram refutadas com evidência, não descartadas
   por palpite).
