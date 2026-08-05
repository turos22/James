# James — Fase 3: Qualidade das ferramentas

**Data:** 2026-08-05
**Autor:** Arthur Orosco (design colaborativo)
**Estado:** aprovado, aguardando revisão do documento escrito
**Projeto:** `C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts`
**Base:** [`2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`](2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md) (Anexo A — bugs mapeados por fase) e [`HANDOFF-inicio-fase-3.md`](../HANDOFF-inicio-fase-3.md). Este documento é autocontido: não é necessário ler os anteriores para implementar a Fase 3.

---

## 1. Contexto

As Fases 0 e 1 fecharam e estão mergeadas em `main`. As quatro ferramentas do agente (`executar_comando_sistema`, `perguntar_ao_claude_code`, `manipular_interface`, `pesquisar_internet`) entraram na Fase 1 **com o comportamento ruim que tinham no `app.go` monolítico** — a Fase 1 moveu cada uma para trás da interface `Ferramenta` sem melhorar o que elas fazem. Os comentários no topo de [`terminal.go`](../../../internal/ferramentas/terminal.go), [`busca.go`](../../../internal/ferramentas/busca.go) e [`claudecode.go`](../../../internal/ferramentas/claudecode.go) registram isso explicitamente e apontam para esta fase.

A Fase 3 existe para consertar o **comportamento** dessas ferramentas.

A "Fase 2" não existe: o número foi absorvido pela Fase 1 (Postgres) e pela Fase 4 (pgvector), e fica vago de propósito para não invalidar a numeração já documentada.

### 1.1 Estado real dos itens do Anexo A, verificado nesta sessão

| # | Item do Anexo A | Estado verificado em 2026-08-05 |
|---|---|---|
| 15 | `toolBusca` (Google Search grounding) declarada e nunca usada | **Já resolvido.** Busca por `toolBusca`, `grounding`, `googleSearch` e `google_search` não retorna nenhuma ocorrência em arquivo `.go` — só em documentos. O código morreu junto com `app.go` na Fase 1. Fora do escopo desta fase. |
| 16 | Tavily em `search_depth basic`, sem `include_answer`, devolve JSON bruto | **Confirmado.** [`busca.go:44`](../../../internal/ferramentas/busca.go) envia `"search_depth": "basic"` sem `include_answer`; [`busca.go:66`](../../../internal/ferramentas/busca.go) devolve `string(respostaBruta)` direto para o modelo. |
| 17 | Claude Code sequestrado para o OpenRouter | **Parcialmente resolvido.** [`rodar_claude.ps1:7`](../../../rodar_claude.ps1) já remove `ANTHROPIC_BASE_URL`, `ANTHROPIC_API_KEY` e `ANTHROPIC_AUTH_TOKEN`. Falta provar ao vivo que a sessão sobe na assinatura Pro, e a limpeza precisa migrar para código Go testável. |
| 18 | Claude Code fire-and-forget, não captura saída | **Confirmado.** [`claudecode.go:21`](../../../internal/ferramentas/claudecode.go) usa `cmd /C start`, que retorna imediatamente; o retorno para o modelo é a string fixa `"Claude Code iniciado. Tarefa enviada para o agente."`. O agente nunca sabe se a tarefa deu certo. |

### 1.2 Itens fora do Anexo A, encontrados nesta sessão

- **`executar_comando_sistema` sem contenção alguma.** [`terminal.go:20-23`](../../../internal/ferramentas/terminal.go) passa a string crua para `powershell -Command`, sem timeout, sem limite de saída, com `CombinedOutput` misturando stdout e stderr, e descartando o exit code.
- **Cadeia de injeção de dois saltos.** `pesquisar_internet` é `NivelSeguro`, devolve conteúdo bruto de página web para o modelo, e o mesmo modelo dispõe de `executar_comando_sistema`, que no perfil Privilegiado executa sem confirmação nenhuma ([`permissao.go:20-23`](../../../internal/permissao/permissao.go) — `Privilegiado.Pode` devolve `true` para qualquer nível). Uma página web com texto endereçado ao modelo fecha o circuito.
- **`http.DefaultClient` na busca.** [`busca.go:22`](../../../internal/ferramentas/busca.go) usa `http.DefaultClient`, que não tem timeout. Uma Tavily lenta pendura o turno indefinidamente.
- **Aprovação remota não executa a ação.** Registrado no handoff, não investigado: aprovar uma ação perigosa vinda do celular (VPN Tailscale) não executa a ação de verdade.

---

## 2. Escopo

### 2.1 Entra

1. Gate de confirmação para comando de escrita, inclusive no perfil Privilegiado.
2. Robustez do `executar_comando_sistema`: timeout, limite de saída, stdout/stderr separados, exit code, encoding.
3. `perguntar_ao_claude_code` headless, síncrono, com saída capturada e sessão persistente (Anexo A #17 e #18).
4. `pesquisar_internet` com `search_depth advanced`, `include_answer`, saída formatada e marcada como conteúdo externo não confiável (Anexo A #16).
5. Diagnóstico e correção do bug da aprovação remota.

### 2.2 Não entra

- **Anexo A #15** — já resolvido pela Fase 1 (evidência na seção 1.1).
- **Refatoração transversal das quatro ferramentas** (contrato de erro unificado, política de timeout compartilhada, revisão geral de schema). Avaliado e descartado nesta sessão: cada item acima é tratado no seu próprio lugar. Se um padrão comum emergir durante a implementação, ele é extraído então — não antecipado agora.
- **`manipular_interface`.** Coordenadas adivinhadas pelo modelo são Anexo A #20, marcado para a Fase 5. Esta fase só toca essa ferramenta se o diagnóstico do bug da aprovação apontar para lá.
- **Sandbox de processo, container ou usuário separado** para o PowerShell. Desproporcional para uma máquina pessoal de dono único; o gate de confirmação é a contenção escolhida.

---

## 3. Decisões desta sessão

| # | Decisão | Escolha | Por quê |
|---|---|---|---|
| D1 | Escopo da fase | Anexo A #16/#17/#18 + segurança do terminal + bug da aprovação | #15 já está resolvido; o item de segurança do terminal está marcado como Fase 3 no próprio código, mas não constava do handoff |
| D2 | Contenção do `executar_comando_sistema` | Robustez sempre + confirmação humana para comando de escrita, inclusive no Privilegiado | Allowlist estrita de comandos foi descartada: a descrição da própria ferramenta pressupõe comando arbitrário ("se não souber o comando, pesquise primeiro"), então a allowlist bateria na parede constantemente e acabaria afrouxada — segurança que se abandona é pior que nenhuma |
| D3 | Piso do perfil Restrito | Preservado: Restrito continua pedindo aprovação para **qualquer** PowerShell, leitura inclusive | A classificação existe para apertar o Privilegiado, não para liberar o celular. A fase só aperta, nunca afrouxa |
| D4 | Mecanismo do gate | Interface opcional aditiva `ExigeConfirmacao`, em vez de trocar `Nivel()` por `Nivel(args)` | Consequência direta de D3: um valor de nível só não consegue significar "leitura, o dono só confirma" e "perigoso, o celular precisa de aprovação" ao mesmo tempo. Ver seção 4.1 |
| D5 | Forma de execução do Claude Code | Headless síncrono (`claude -p --output-format json`) com sessão persistente por `--session-id` | Resolve #18 na raiz: o agente passa a receber o resultado real. Sessão persistente permite "ajusta aquilo que você acabou de fazer" |
| D6 | Poder do Claude Code headless | `--permission-mode acceptEdits`, diretório de trabalho vindo do `config.toml` | `bypassPermissions` fecharia a cadeia de injeção de dois saltos para execução irrestrita de código. Diretório configurado explicitamente porque hoje o cwd herdado é a raiz do repo do próprio James |
| D7 | Melhoria da busca | `advanced` + `include_answer` + saída formatada e marcada como não confiável | A marcação é defesa em profundidade; o gate de D2 continua sendo a barreira real |
| D8 | Local de execução da fase | Direto em `main`, commit por task | Escolha do Arthur. Permite testar ao vivo a cada passo sem trocar de checkout |
| D9 | Momento do diagnóstico do bug da aprovação | Durante a execução, como task do plano, e por último | É o único item de duração desconhecida; na frente, bloquearia trabalho já totalmente especificado |

---

## 4. Arquitetura: o gate de confirmação

### 4.1 Interface opcional `ExigeConfirmacao`

O contrato `Ferramenta` em [`tipos.go`](../../../internal/ferramentas/tipos.go) **não muda**:

```go
type Ferramenta interface {
	Declaracao() Declaracao
	Nivel() Nivel
	Executar(ctx context.Context, args map[string]any) (string, error)
}
```

Uma interface opcional é adicionada no mesmo pacote:

```go
// ExigeConfirmacao é implementada por ferramentas em que uma chamada
// específica precisa de confirmação humana mesmo vinda de um perfil
// Privilegiado. Nunca dispensa a aprovação que o Nivel() já exige — só
// acrescenta. Ferramenta que não implementa se comporta como hoje.
type ExigeConfirmacao interface {
	ConfirmacaoNecessaria(args map[string]any) bool
}
```

O despacho em [`loop.go:91`](../../../internal/agente/loop.go) passa a ser:

```go
precisaAprovar := !perfil.Pode(tool.Nivel())
if c, ok := tool.(ExigeConfirmacao); ok && c.ConfirmacaoNecessaria(chamada.Args) {
	precisaAprovar = true
}
```

**Por que aditiva e não substitutiva.** A regra é `precisaAprovar` só receber `true`, nunca `false`. Isso torna o piso de D3 uma propriedade estrutural do código, não uma disciplina que alguém precisa lembrar de manter: nenhuma implementação futura de `ConfirmacaoNecessaria` consegue liberar uma chamada que o `Nivel()` já bloqueava. Trocar `Nivel()` por `Nivel(args)` daria a qualquer ferramenta o poder de rebaixar o próprio nível conforme os argumentos — exatamente o que D3 proíbe.

Custo assumido: passam a existir dois lugares que falam de perigo. A semântica é genuinamente diferente e fica documentada — `Nivel()` responde "que tipo de coisa esta ferramenta é", `ConfirmacaoNecessaria(args)` responde "esta chamada específica precisa de um humano, mesmo o dono".

Só `ComandoSistema` implementa a interface nesta fase. As outras três ferramentas não são tocadas.

### 4.2 Matriz de decisão resultante

| Perfil | Comando de leitura | Comando de escrita |
|---|---|---|
| Privilegiado (desktop, loopback) | Executa direto — **igual a hoje** | Pede confirmação — **novo** |
| Restrito (celular, VPN) | Pede aprovação — igual a hoje | Pede aprovação — igual a hoje |

A única célula que muda é a do comando de escrita no desktop. O fluxo de aprovação inteiro já existe e não precisa de nada novo: [`ws.go:115`](../../../internal/api/ws.go) já passa `criarSolicitarAprovacao` para **toda** sessão, inclusive a Privilegiada — a plumbing está lá desde a Fase 1, apenas nunca foi acionada para o desktop. O card de aprovação, o WS e a persistência em `internal/armazenamento/postgres/aprovacao.go` funcionam sem alteração.

### 4.3 Classificador de comando PowerShell

É a peça de maior risco da fase, então mora numa função pura, sem `exec`, testada por tabela:

```go
// ComandoEhLeitura devolve true apenas para comandos comprovadamente sem
// efeito colateral. Fail closed: qualquer dúvida devolve false.
func ComandoEhLeitura(comando string) bool
```

**Postura: allowlist de leitura, não blocklist de perigo.** Não se tenta enumerar o mal — enumera-se o pouco que é comprovadamente inofensivo. Falso positivo custa um clique; falso negativo custa a máquina.

Algoritmo, nesta ordem:

1. **Rejeição por token proibido em qualquer posição.** Presença de qualquer um destes devolve `false` imediatamente, mesmo que o verbo pareça leitura:
   `;` · `&` · crase (backtick) · `$(` · `@(` · `{` · `}` · `>` · `>>` · `<` · `--%` · `::`
   As chaves entram na lista porque um `ScriptBlock` executa código arbitrário. Isso rejeita `Where-Object { $_.CPU -gt 10 }`, que é de fato leitura — perda aceita e deliberada sob a postura fail closed.

2. **Divisão por `|`.** Todo estágio do pipeline precisa passar; um único estágio fora da allowlist reprova o comando inteiro.

3. **Allowlist do primeiro token de cada estágio**, comparação case-insensitive:
   - Prefixo `get-` (o verbo `Get` é definido como somente-leitura pelas diretrizes de verbo do PowerShell).
   - Nomes completos: `test-path`, `resolve-path`, `split-path`, `join-path`, `select-object`, `select-string`, `sort-object`, `group-object`, `compare-object`, `measure-object`, `format-list`, `format-table`, `format-wide`, `out-string`, `convertto-json`, `convertfrom-json`, `write-output`.
   - Aliases: `dir`, `ls`, `gci`, `cat`, `gc`, `type`, `pwd`, `echo`, `select`, `sort`, `measure`, `ft`, `fl`.
   - Sem parâmetro: `whoami`, `hostname`, `date`.

4. **Qualquer outra coisa devolve `false`** — incluindo comando vazio, caminho de executável, chamada por operador `&`, e qualquer cmdlet de módulo de terceiro.

`ComandoSistema.ConfirmacaoNecessaria(args)` devolve `!ComandoEhLeitura(comando)`. Argumento ausente, vazio ou de tipo errado devolve `true` (fail closed) — a validação de argumento continua acontecendo em `Executar`.

A allowlist fica numa única variável de pacote, para poder ser apertada ou afrouxada num lugar só.

**Risco documentado:** o prefixo `get-` é uma decisão deliberada de conveniência sobre rigor. Um módulo de terceiro pode definir um `Get-*` com efeito colateral. Aceito porque a alternativa (lista exaustiva de `Get-*` do core) quebraria em qualquer instalação diferente e empurraria o Arthur a afrouxar a regra inteira.

---

## 5. `executar_comando_sistema` — robustez

Vale para **toda** execução, aprovada ou não:

- **Timeout** via `exec.CommandContext`, valor de configuração (default 60s). Estourou, o processo é morto e o modelo recebe um erro explícito dizendo que o comando excedeu o tempo — nunca uma saída parcial passando por sucesso.
- **stdout e stderr em buffers separados.** `CombinedOutput` sai de cena. O modelo recebe as duas correntes rotuladas.
- **Exit code no retorno.** Comando que falha com exit code diferente de zero devolve erro contendo o código e o stderr.
- **Teto de bytes na saída**, valor de configuração (default 64 KiB). Truncagem sempre marcada explicitamente no texto devolvido, com o total original em bytes. Truncagem silenciosa é proibida — o modelo precisa saber que não viu tudo.
- **`-EncodedCommand` em vez de `-Command`**, com o comando em UTF-16LE base64. Elimina a classe inteira de bugs de aspas, acento e caractere especial atravessando duas camadas de parsing de shell.

O campo `Executor` injetável de [`terminal.go:12`](../../../internal/ferramentas/terminal.go) é preservado — é o que mantém os testes de unidade sem tocar no sistema. A assinatura cresce para carregar exit code e as duas correntes.

`Nivel()` continua devolvendo `NivelPerigoso`.

---

## 6. `perguntar_ao_claude_code`

`rodar_claude.ps1` **deixa de existir**. A limpeza das variáveis de ambiente da Anthropic migra do PowerShell para Go, virando código testável em vez de um script que ninguém verifica:

- O `exec.Cmd.Env` é montado a partir do ambiente do processo **removendo** `ANTHROPIC_BASE_URL`, `ANTHROPIC_API_KEY` e `ANTHROPIC_AUTH_TOKEN`. É assim que o Claude Code autentica pela assinatura Pro via OAuth em vez de ser sequestrado para outro provider (Anexo A #17).

**Invocação:**

```
claude -p <instrucao> --output-format json --permission-mode acceptEdits [--session-id <uuid> | --resume <uuid>]
```

Flags confirmadas por `claude --help` nesta máquina em 2026-08-05: `-p/--print`, `--output-format text|json|stream-json`, `--permission-mode` (aceita `acceptEdits`, `auto`, `bypassPermissions`, `default`, `dontAsk`, `plan`), `--session-id <uuid>`, `--resume`.

**Diretório de trabalho** vem do `config.toml`. Não é herdado do cwd do `jamesd` — hoje isso significa a raiz do repo do próprio James, ou seja, o James pode mandar o Claude Code editar o código-fonte do James. Sem configuração, a ferramenta falha com erro claro no momento da chamada. A falha é na chamada e não no boot do `jamesd`: o James funciona normalmente sem nunca invocar o Claude Code, então um campo não preenchido não deve impedir o serviço de subir.

**Timeout próprio**, de configuração, na casa de minutos — o Claude Code é ordens de grandeza mais lento que um comando de shell, e o timeout do terminal não serve.

**Sessão persistente:** o UUID da sessão do James é usado diretamente como `--session-id`, o que dispensa qualquer tabela de mapeamento ou estado novo. Se reusar o mesmo id não retomar o contexto e for necessário `--resume` da segunda chamada em diante, a implementação descobre isso executando de verdade. Este ponto é **verificação obrigatória com saída real** no plano, não suposição.

**Retorno para o modelo:** o JSON de `--output-format json` é parseado em struct tipada e o resultado real é devolvido. A string fixa `"Claude Code iniciado. Tarefa enviada para o agente."` morre. Falha do Claude Code (exit code, timeout, JSON inesperado) vira erro de verdade para o modelo.

`Nivel()` continua `NivelPerigoso`: `acceptEdits` escreve arquivo. A ferramenta **não** implementa `ExigeConfirmacao` — o nível já cobre.

---

## 7. `pesquisar_internet`

- `search_depth: "advanced"` e `include_answer` ligado.
- Cliente HTTP dedicado com timeout de configuração. `http.DefaultClient` sai de cena.
- Resposta parseada em **struct tipada**, não `map[string]any` solto: mudança de forma da API vira erro explícito em vez de campo silenciosamente vazio.
- Saída para o modelo vira texto formatado — a resposta sintetizada primeiro, depois as fontes com título, URL e trecho, limitadas por um máximo de resultados configurável. O JSON bruto nunca chega ao modelo.
- O conteúdo vindo da web entra **delimitado e rotulado** como externo e não confiável, com a nota explícita de que texto ali dentro é dado, nunca instrução. Defesa em profundidade: o gate da seção 4 continua sendo a barreira real, porque depender do modelo respeitar uma marcação é mitigação fraca sozinha.

A forma exata do payload da Tavily (`api_key` no corpo versus header `Authorization: Bearer`, valores aceitos por `include_answer`) é **verificada contra a documentação atual** antes de ser fixada no código. Não é escrita de memória.

`Nivel()` continua `NivelSeguro`.

---

## 8. Bug da aprovação remota

Sintoma registrado no handoff: aprovar uma ação perigosa vinda do celular na VPN não executa a ação de verdade. Nunca foi investigado.

A task é estruturada em quatro passos, nesta ordem, com `systematic-debugging`: **reproduzir** com o `jamesd` real e o celular real na VPN → **diagnosticar** até a causa raiz → **corrigir** → **provar ao vivo**.

**Critério de aceite**, no comportamento observável: uma ação perigosa solicitada pelo celular e aprovada no desktop executa de verdade, e o resultado volta para o modelo e aparece no chat do celular.

**Ponto de partida da investigação** (é onde olhar primeiro, não diagnóstico): o `ctxTurno` de [`ws.go:115`](../../../internal/api/ws.go) contra a espera bloqueante da aprovação, já que [`loop.go:48`](../../../internal/agente/loop.go) aborta o loop se o contexto morrer entre iterações, e `TimeoutAprovacaoSegundos` tem default de 300s. Nenhuma correção é escrita antes da causa estar demonstrada.

Fica por último na ordem de implementação (D9).

---

## 9. Configuração nova

Acrescentado ao `config.toml`, seguindo a convenção existente em [`config.go`](../../../internal/config/config.go) (seção TOML em português, default aplicado por `intOuDefault`/`valorOuDefault`):

```toml
[ferramentas.terminal]
timeout_segundos = 60
max_bytes_saida = 65536

[ferramentas.claude_code]
diretorio = ""          # obrigatório para usar a ferramenta; sem default
timeout_segundos = 600

[ferramentas.busca]
timeout_segundos = 20
max_resultados = 5
```

`diretorio` é o único campo sem default. Todos os outros têm default no código, então um `config.toml` antigo continua subindo sem edição.

---

## 10. Tratamento de erro

Regra única, herdada da Fase 1 (R5: mensagem honesta, nunca fallback mascarando fracasso), aplicada a todas as ferramentas tocadas:

- Falha vira erro com causa concreta, entregue ao modelo. Nunca string genérica de sucesso.
- Timeout é reportado **como timeout**, distinguível de erro de execução.
- Truncagem de saída é sempre visível no texto, com o tamanho original.
- Falha de forma na resposta de API externa (Tavily, JSON do Claude Code) é reportada como tal, não como resultado vazio.

---

## 11. Estratégia de teste

TDD por task, no mesmo padrão das Fases 0 e 1: teste primeiro, implementação depois.

**Unitário** — concentrado no que é decisão pura, sem I/O:
- `ComandoEhLeitura`: tabela extensa cobrindo cada regra da seção 4.3 — aceitos, rejeitados por token proibido, rejeitados por verbo fora da allowlist, pipeline misto (um estágio de escrita reprova o todo), comando vazio, caixa alta e baixa.
- `ConfirmacaoNecessaria`: argumento ausente, vazio e de tipo errado devolvem `true`.
- Despacho no `loop`: prova de que `ExigeConfirmacao` só acrescenta aprovação e nunca remove — em particular, que Restrito continua precisando de aprovação para comando de leitura (D3).
- Truncagem de saída, incluindo a marca de truncagem e o tamanho original.
- Parser da resposta da Tavily, incluindo resposta malformada.
- Montagem do argv e do `Env` do Claude Code, via `Executor` injetável — inclusive a prova de que as três variáveis `ANTHROPIC_*` não aparecem no ambiente montado.

**Integração, contra as coisas reais** (não só mock):
- Comando de verdade estourando o timeout de verdade, e o processo sendo morto.
- Comando de verdade com acento e aspas atravessando `-EncodedCommand`.
- `claude -p` de verdade, provando saída capturada e autenticação pela assinatura Pro.
- Tavily de verdade, com a chave do `.env`.

**Ao vivo, no fim da fase:** `jamesd` + SPA rodando, disparando cada ferramenta pelo chat, confirmando que o card de confirmação aparece no desktop para comando de escrita e **não** aparece para comando de leitura, e que o fluxo do celular continua pedindo aprovação para os dois casos.

Nada é declarado funcionando sem o comando ter sido rodado e a saída lida, conforme a regra do `CLAUDE.md`.

---

## 12. Critérios de aceitação

1. `Get-Process` pedido pelo desktop executa sem confirmação; `Stop-Process` pedido pelo desktop abre card de confirmação e só executa depois do "Aprovar".
2. Qualquer PowerShell pedido pelo celular, leitura inclusive, continua pedindo aprovação.
3. Comando que trava é morto no timeout e o modelo recebe erro de timeout, não saída parcial.
4. Saída maior que o teto chega ao modelo truncada e explicitamente marcada como tal.
5. `perguntar_ao_claude_code` devolve ao modelo a resposta real do Claude Code, e a sessão está autenticada pela assinatura Pro.
6. Duas chamadas seguidas ao Claude Code na mesma sessão do James compartilham contexto.
7. `pesquisar_internet` devolve texto formatado com resposta sintetizada e fontes, com o conteúdo web delimitado e marcado — nunca JSON bruto.
8. Uma ação perigosa pedida pelo celular e aprovada no desktop executa de verdade e o resultado aparece no chat do celular.
9. Suíte inteira passando nas três stacks (Go, Python, TypeScript) em `main`.

---

## 13. Ordem de implementação

1. `ExigeConfirmacao` + `ComandoEhLeitura` + despacho no `loop` (a fundação; nada depende de I/O externo).
2. Robustez do `executar_comando_sistema` (timeout, buffers, exit code, truncagem, `-EncodedCommand`) e `ConfirmacaoNecessaria`.
3. `pesquisar_internet` (independente dos anteriores; verificação da doc da Tavily primeiro).
4. `perguntar_ao_claude_code` (o maior, e o que mais depende de verificação ao vivo).
5. Bug da aprovação remota (duração desconhecida — por último, D9).
6. Revisão do `executar.bat` e do `config.toml` de exemplo, já que a configuração ganhou campos.

Trabalho direto em `main`, um commit por task, com o ciclo implementador → revisor de spec → revisor de qualidade usado nas 25 tasks da Fase 1.

---

## 14. Riscos conhecidos

| Risco | Mitigação |
|---|---|
| O gate de confirmação vira fricção diária e o Arthur desliga | A matriz de 4.2 foi desenhada para que consulta (o caso comum) permaneça sem fricção. Se a fricção aparecer mesmo assim, a allowlist está num lugar só e é ajustável sem redesenho |
| Prefixo `get-` aceitando um `Get-*` de terceiro com efeito colateral | Aceito e documentado em 4.3. A alternativa quebraria em instalações diferentes |
| `--session-id` reusado não retomar contexto | Verificação obrigatória com execução real durante a implementação; o fallback é `--resume` da segunda chamada em diante |
| API da Tavily ter mudado de forma desde o código atual | A doc atual é consultada antes de escrever o payload; a struct tipada faz uma mudança futura falhar alto em vez de silenciosamente |
| O bug da aprovação ser mais profundo que o previsto e estourar o prazo da fase | Está por último; os outros quatro itens fecham independentemente dele |
