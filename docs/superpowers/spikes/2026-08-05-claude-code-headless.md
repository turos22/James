# Spike — Claude Code headless (`claude -p`)

**Data:** 2026-08-05
**Motivo:** a spec da Fase 3 (§6) proíbe escrever o schema de `--output-format json` de memória. As Tasks 14 e 16 dependem deste documento.
**Método:** execuções reais do `claude` instalado nesta máquina, a partir de `C:\Users\Arthur\SI\Projetos\james-workspace` (o diretório dedicado que a fase adota como cwd da ferramenta).

---

## 1. Campos de topo do JSON, e qual carrega a resposta

Saída literal de `claude -p "responda apenas com a palavra OK, nada mais" --output-format json --permission-mode acceptEdits`, guardada como fixture em [`internal/ferramentas/testdata/claude_code_resposta.json`](../../../internal/ferramentas/testdata/claude_code_resposta.json):

```
type, subtype, is_error, api_error_status, duration_ms, duration_api_ms,
ttft_ms, num_turns, result, stop_reason, session_id, total_cost_usd, usage,
modelUsage, permission_denials, terminal_reason, fast_mode_state, uuid
```

**O texto da resposta está em `result`** (`"OK"` neste caso).

Os três campos que o plano assumiu — `result`, `is_error`, `session_id` — **existem com esses nomes exatos**. A struct `resultadoClaudeCode` da Task 14 pode ser escrita como planejada.

## 2. Campo de erro

`is_error`, booleano. Veio `false` na execução bem-sucedida. Há também `api_error_status` (null no sucesso), `stop_reason` (`"end_turn"`) e `subtype` (`"success"`).

## 3. `session_id` na resposta

Sim, `session_id`, string UUID. Quando `--session-id` é passado, a resposta devolve exatamente o id passado.

## 4. Exit code no sucesso

`0`.

## 5. Reusar o mesmo `--session-id` duas vezes — **erro, `--resume` é obrigatório**

Esta era a pergunta que decidia o desenho da Task 16.

Primeira chamada, `--session-id 7f3a1c20-0000-4000-8000-000000000abc`:
```json
{"is_error":false,"result":"OK","session_id":"7f3a1c20-0000-4000-8000-000000000abc"}
```

Segunda chamada com o **mesmo** `--session-id`:
```
Error: Session ID 7f3a1c20-0000-4000-8000-000000000abc is already in use.
```
exit code `1`.

Segunda chamada com `--resume` no mesmo id, perguntando o número pedido na primeira:
```json
{"is_error":false,"result":"42","session_id":"7f3a1c20-0000-4000-8000-000000000abc"}
```

**Conclusão:** o contexto é preservado, e a distinção `--session-id` na primeira chamada / `--resume` da segunda em diante é **obrigatória**, não opcional. O desenho da Task 16 (campo `retomar`, mapa `sessoesIniciadas`) está correto e não simplifica.

Consequência para a implementação: o erro de sessão já em uso é distinguível (exit code diferente de zero, mensagem no stderr), então marcar `sessoesIniciadas` só no caminho de sucesso — como a Task 16 especifica — é o que evita ficar preso tentando criar uma sessão que já existe.

## 6. `acceptEdits` **executou comando de shell sem pedir permissão**

Este resultado **contraria a premissa da decisão D6 da spec**.

A spec descreve `acceptEdits` como "edita arquivos sem perguntar, mas não executa Bash arbitrário sem permissão", e foi com essa descrição que a opção foi escolhida em vez de `bypassPermissions`.

Teste: `claude -p "rode o comando de shell 'echo teste123' e me diga a saída exata" --output-format json --permission-mode acceptEdits`

Resultado:
```
EXIT: 0
is_error: False
permission_denials: (vazio)
result: Rodei. Saída exata:

```
teste123
```
```

O comando **rodou**, `permission_denials` veio **vazio**, e não houve recusa nem espera. Em modo headless não há ninguém pra aprovar, e o resultado observado é execução, não recusa.

**O que isto significa na prática:** a ferramenta `perguntar_ao_claude_code`, com a configuração que a fase adota, dá ao agente Gemini a capacidade de executar comando de shell arbitrário na máquina — indiretamente, via Claude Code, e **sem passar pelo classificador nem pelo gate de confirmação** que a mesma fase construiu para o `executar_comando_sistema`.

O diretório de trabalho dedicado (`james-workspace`) limita o dano a arquivos, mas **não confina comando de shell**: um `Remove-Item` fora do diretório continua alcançável.

**Ressalva de escopo desta observação:** foi uma amostra, com um comando inofensivo. Não testei se algum comando específico seria bloqueado, nem se o comportamento muda com `--allowedTools`/`--disallowedTools`. O que está estabelecido é que a barreira que a spec supunha existir **não impediu** a execução de shell.

**Decisão pendente do Arthur** (registrada aqui porque muda o cálculo de risco que ele fez ao escolher D6):
- (a) manter `acceptEdits` e aceitar o caminho indireto para shell, com o diretório dedicado como única contenção;
- (b) restringir por `--disallowedTools Bash` (ou allowlist de ferramentas) para fechar o caminho de shell, mantendo a edição de arquivo;
- (c) trocar para `--permission-mode plan`, em que o Claude Code só analisa e planeja, nunca altera nem executa;
- (d) fazer `perguntar_ao_claude_code` também implementar `ExigeConfirmacao`, pedindo confirmação humana em toda chamada.

---

## Comando de referência, validado nesta data

```
claude -p <instrucao> --output-format json --permission-mode acceptEdits --session-id <uuid>     # primeira chamada
claude -p <instrucao> --output-format json --permission-mode acceptEdits --resume <uuid>          # seguintes
```

Ambiente: `ANTHROPIC_BASE_URL`, `ANTHROPIC_API_KEY` e `ANTHROPIC_AUTH_TOKEN` removidos, para a sessão autenticar pela assinatura Pro via OAuth (Anexo A #17). Nas execuções acima essas variáveis não estavam presentes no shell, e `modelUsage` reportou `claude-opus-4-8` — ou seja, sessão real da assinatura, não um provider desviado.
