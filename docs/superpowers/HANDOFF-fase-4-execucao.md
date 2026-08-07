# Handoff — Fase 4 fechada (memória durável)

Escrito em 2026-08-06, ao fim da execução da Task 13 (verificação ao vivo do
CA3, a task que fecha a fase).

## Onde está tudo

- **Spec:** `docs/superpowers/specs/2026-08-06-james-fase-4-memoria-duravel-design.md`
- **Plano:** `docs/superpowers/plans/2026-08-06-james-fase-4-memoria-duravel.md`
  — 13 tasks
- **Teste de verificação:** `internal/api/verificar_memoria_test.go` (gated
  por `JAMES_AO_VIVO=1`, não roda na suíte normal)

## Estado: 13 de 13 tasks fechadas

Tasks 1–12 (Vault, `SalvarNaMemoria`, wiring no system prompt, instrução no
prompt) já estavam em `main` antes desta sessão. Esta sessão executou só a
Task 13 — a verificação ao vivo do CA3, que é o critério que prova a fase
inteira.

## CA3 — o que ele prova

`historico` é por conexão WS e começa vazio (`internal/api/ws.go:37`); uma
conexão nova sempre cria sessão nova (`ws.go:116`). Sem memória durável, uma
pergunta numa conexão nova **não pode**, por construção, saber de algo dito
numa conexão anterior já fechada. O CA3 prova que, com o Vault de memória
durável escrito nas 12 tasks anteriores, essa barreira foi furada de
propósito: o bloco de memória (não a janela de conversa) entra no system
prompt a cada turno, lido do disco.

## Ambiente usado

- `james-postgres` (Docker) já estava no ar antes da sessão, confirmado
  saudável antes e depois — não foi tocado.
- `jamesd` real (`go build ./cmd/jamesd`, binário rodado direto, não
  `go run`) subiu em background nesta sessão e foi derrubado ao fim do
  Step 6, como pedido.
- Chave real do Gemini via `.env` (não inspecionada, só confirmada presente
  pelo controlador antes da sessão).

### Boot do jamesd — log real

```
2026/08/06 23:38:46 INFO servindo em loopback endereco=127.0.0.1:8080
2026/08/06 23:38:46 INFO servindo na VPN com TLS endereco=100.96.219.29:8443
```

Sem erro fatal de boot. `curl http://127.0.0.1:8080/api/saude` respondeu
`{"status":"ok"}` com HTTP 200 antes de qualquer outro passo.

## Step 3 — a verificação ao vivo (o passo que mais importa)

Comando: `JAMES_AO_VIVO=1 go test ./internal/api/ -run TestAoVivo -v`

Saída real, colada:

```
=== RUN   TestAoVivoMemoriaAtravessaConexao
    verificar_memoria_test.go:24: ferramenta chamada: {"nome":"salvar_na_memoria"}
    verificar_memoria_test.go:25: conexão 1 respondeu: Beleza, guardei aqui: o Arthur tem um Fusca 1971 azul.
    verificar_memoria_test.go:28: conexão 2 respondeu: Você tem um Fusca 1971 azul.
--- PASS: TestAoVivoMemoriaAtravessaConexao (9.36s)
PASS
ok  	james/internal/api	10.040s
```

Passou de primeira, sem retrabalho: o Gemini chamou `salvar_na_memoria`
sozinho na primeira conexão (nenhum prompt extra além da instrução normal do
sistema), e a **segunda conexão — WS novo, sessão nova, histórico vazio —**
respondeu corretamente sobre o Fusca sem que ninguém tivesse repetido a
informação.

## Step 4 — confirmação em disco

Comando: `ls -la Vault/Memoria/` seguido de `cat Vault/Memoria/*.md`

Saída real:

```
total 5
drwxr-xr-x 1 Arthur 197121  0 Aug  6 23:39 .
drwxr-xr-x 1 Arthur 197121  0 Aug  6 23:39 ..
-rw-r--r-- 1 Arthur 197121 43 Aug  6 23:39 carro.md

== Vault/Memoria/carro.md ==
- O carro do Arthur é um Fusca 1971 azul.
```

Uma nota, um arquivo, uma linha começando com `- `, exatamente como a spec
descreve.

## Step 5 — gitignore

Comando: `git status --short`

`Vault/Memoria/` não aparece na saída — coberto pelo `.gitignore` desde a
Task 10. Confirmado.

Observação fora do escopo desta task: dois `.mp3` novos apareceram em
`audios/` (gerados pelo TTS da resposta durante o teste ao vivo) e ficaram
como untracked — `.gitignore` só cobre `audios/audio_jarvis.mp3`, não o
diretório inteiro. Não mexi nisso; registrando para quem cuidar do
`.gitignore` de áudio depois, não é problema desta fase.

## Step 6 — suíte completa, três stacks

**Go** (`go test ./...`):

```
ok  	james/cmd/jamesd	(cached)
ok  	james/cmd/migrar-historico	(cached)
ok  	james/internal/agente	(cached)
ok  	james/internal/agente/gemini	(cached)
ok  	james/internal/api	1.201s
?   	james/internal/armazenamento	[no test files]
ok  	james/internal/armazenamento/postgres	(cached)
ok  	james/internal/config	(cached)
ok  	james/internal/dispositivo	(cached)
ok  	james/internal/ferramentas	(cached)
ok  	james/internal/higiene	0.248s
ok  	james/internal/memoria	(cached)
ok  	james/internal/permissao	(cached)
ok  	james/internal/voz	(cached)
ok  	james/shell	(cached)
?   	james/web	[no test files]
```

**Web** (`npm test --prefix web`, roda `vitest run`):

```
 Test Files  5 passed (5)
      Tests  24 passed (24)
   Start at  23:39:54
   Duration  2.11s
```

**Dispositivo** (`python -m pytest`, em `dispositivo/`):

```
collected 29 items

tests\test_conexao.py ..........                                         [ 34%]
tests\test_tela.py ....                                                  [ 48%]
tests\test_wake.py ...............                                       [100%]

============================= 29 passed in 1.18s ==============================
```

Tudo verde nas três stacks.

## Encerramento

`jamesd` (PID confirmado via `tasklist`) foi derrubado com
`Stop-Process -Force` ao fim do Step 6. Confirmado que não sobrou processo
(`tasklist` vazio pro binário) e que `james-postgres` continuou `Up ...
(healthy)` sem interrupção.

## Conclusão

CA3 passou ao vivo, sem ajuste de prompt nem mudança de código de produção
necessária — as 12 tasks anteriores já tinham fechado a lacuna
corretamente. Fase 4 fechada.
