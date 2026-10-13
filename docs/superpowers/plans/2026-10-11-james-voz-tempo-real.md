# Voz em tempo real: plano de implementação

> **Para quem executa:** um task por vez, na ordem. TDD em todos: escrever o teste, rodar e **ver falhar pelo motivo certo**, implementar, rodar e ver passar, commitar. A saída de cada "rodar" vai no relato. Não rodou, diz que não rodou. Modo trabalho: `/code-review` e `/security-review` antes do merge (Task 20).

**Objetivo:** do fim da fala do Arthur ao primeiro som do James em ≤ 2 s de mediana (CA6), com Claude Haiku 5.5, Whisper na GPU e Piper, e uma voz só nos turnos de voz e de texto.

**Spec:** [`../specs/2026-10-10-james-voz-tempo-real-design.md`](../specs/2026-10-10-james-voz-tempo-real-design.md) (aprovada em 2026-10-11).
**Medições de base:** [`../spikes/2026-10-10-latencia-voz.md`](../spikes/2026-10-10-latencia-voz.md).
**Branch:** `feat/james-tempo-real`.

---

## Antes de começar: leia isto

Fatos medidos ou conferidos nesta sessão. Cada um já custou tempo uma vez.

1. **A chave da Anthropic se chama `CLAUDE_API_KEY`** no `.env`, não `ANTHROPIC_API_KEY`. Nunca imprimir o valor; conferir só presença e prefixo `sk-ant-`.
2. **O Haiku 5.5 pensa por padrão** (pensamento adaptativo, `effort` padrão `medium`). Se `max_tokens` for baixo, o pensamento come o limite e a resposta sai cortada (`stop_reason=max_tokens`). Por isso:
   - turno de voz: `thinking` desligado (`ThinkingConfigParamUnion{OfDisabled: ...}`), aceito com effort até `high`;
   - turno de texto: adaptativo com effort `low`;
   - `max_tokens` = 4096.
3. **SDK Go da Anthropic** (`github.com/anthropics/anthropic-sdk-go` v1.80.0, exige Go ≥ 1.24):
   - streaming: `client.Messages.NewStreaming(ctx, params)`, `for stream.Next()`, `event.AsAny().(type)`, `anthropic.ContentBlockDeltaEvent`, `anthropic.TextDelta`;
   - mensagem final: `message.Accumulate(stream.Current())`;
   - ferramentas: `anthropic.ToolParam` dentro de `ToolUnionParam{OfTool: ...}`, `anthropic.NewToolResultBlock(id, conteudo, isError)`, `variant.JSON.Input.Raw()`;
   - pensamento adaptativo: `ThinkingConfigParamUnion{OfAdaptive: &...}`.
   - **O nome Go do campo de effort (em `OutputConfig`) não está na referência que li.** Confira com `go doc github.com/anthropics/anthropic-sdk-go.MessageNewParams` antes de escrever. Não adivinhe.
4. **Ferramentas antigas no Postgres têm `ID` igual ao nome** (o Gemini preenchia assim; ex.: `"ID": "executar_comando_sistema"`). A Anthropic exige ids únicos por mensagem e pareamento `tool_use` ↔ `tool_result`. O mapeamento tem de desambiguar ids repetidos (Task 3).
5. **VAD: `pysilero-vad` 3.4.0** (MIT, **sem dependências**, sem PyTorch; o `silero-vad` oficial puxa torch e CUDA).
   - API: `SileroVoiceActivityDetector()`; chamar o objeto com **1024 bytes** (512 amostras, 32 ms a 16 kHz) devolve a probabilidade de voz. `chunk_bytes()` e `reset()` existem.
   - No áudio de teste: 164 blocos em 8,6 ms.
6. **Piper 1.8.0** (`piper-tts`): `PiperVoice.load(caminho)`; `voz.synthesize(texto)` itera `AudioChunk` com `.audio_int16_bytes` e `.sample_rate`. Carga: 0,7 s.
7. **`whisper-server`** (whisper.cpp v1.9.5, Vulkan):
   - binário e bibliotecas em `~/Downloads/stt-teste/whisper.cpp/build/bin`, com **RUNPATH absoluto para essa pasta**. Movido, só funciona com `LD_LIBRARY_PATH` apontando para a pasta nova;
   - flags: `-m <modelo> -l pt -ac 512 -t 6 --port 8179`;
   - `POST /inference` multipart (`file`, `response_format=json`, `temperature=0.0`) devolve `{"text": ...}`;
   - modelo: `ggml-large-v3-turbo-q8_0.bin` (874 MB).
8. **Arquivos com CRLF no worktree** (vieram do Windows): `tela.py`, `main.py`, `test_tela.py` e outros. O índice do git é LF. Ao editar por script, preserve o fim de linha ou normalize só o arquivo tocado; o `Edit` falha em CRLF.
9. **`pkill -f`/`pgrep -f` casam com a própria linha de comando** e já mataram o shell duas vezes nesta sessão. Use `pgrep -x <nome>`.
10. **Comandos de suíte:**
    - Go:
      ```bash
      go vet ./... && GOOS=windows go vet ./... && go test -count=1 ./...
      ```
      Precisa do Postgres de pé (`docker compose up -d`).
    - Device:
      ```bash
      dispositivo/.venv/bin/python -m pytest -q -rs dispositivo
      ```
    - Front:
      ```bash
      npm test --prefix web
      ```

---

## Estrutura de arquivos

```
internal/config/config.go, config_test.go          MODIFICAR  provedor, CLAUDE_API_KEY, [voz] motor/tts_url
internal/agente/tipos.go                           MODIFICAR  Requisicao.Modo, Resposta.Uso
internal/agente/loop.go, loop_test.go              MODIFICAR  AoDelta -> GerarStream
internal/agente/anthropic/                         CRIAR
  mapeamento.go, mapeamento_test.go                  Requisicao <-> SDK (puro)
  anthropic.go, anthropic_test.go                    Provider (Gerar/GerarStream) contra httptest
  integracao_test.go                                 CA3, opt-in JAMES_TESTE_ANTHROPIC=1
  testdata/*.sse                                     streams reais gravados (sem chave)
internal/voz/fala.go, fala_test.go                 CRIAR      QuebradorDeFrases, LimparParaFala
internal/voz/sintetizador.go, _test.go             MODIFICAR  executor Piper, extensão por motor
internal/api/ws.go, dispositivo.go, eventos.go     MODIFICAR  modo por canal, fala_* ao device, sem audio_pronto na voz
internal/api/rest.go                               MODIFICAR  servir .wav/.mp3 com Content-Type explícito
cmd/jamesd/main.go                                 MODIFICAR  escolher provedor e motor de voz
dispositivo/pyproject.toml                         MODIFICAR  piper-tts, pysilero-vad, requests
dispositivo/james_device/
  sintese.py              CRIAR  Piper: carregar uma vez, sintetizar(texto) -> (pcm, taxa)
  fala.py                 CRIAR  fila de frases, reprodução, estado ocupado/falando
  servidor_tts.py         CRIAR  POST /sintetizar, GET /saude, só loopback
  vad.py                  CRIAR  wrapper do pysilero-vad (blocos de 0,25 s -> quadros de 32 ms)
  medidor.py              CRIAR  marcos por turno -> .executar/latencia.tsv
  escuta.py               MODIFICAR  fim de fala por VAD, especulação, ocupado, continuação
  conexao.py              MODIFICAR  fala_frase / fala_fim / fala_erro
  main.py                 MODIFICAR  fiação, transcritor whisper|google
dispositivo/tests/test_{sintese,fala,servidor_tts,vad,medidor}.py   CRIAR
dispositivo/tests/test_escuta.py, test_conexao.py                    MODIFICAR
executar.sh, .gitignore, .env.example              MODIFICAR
```

---

## Fatia 0: ambiente

> **Achado na execução (2026-10-13):** o `edge-tts` está falhando com **HTTP 403** do servidor da Microsoft (`WSServerHandshakeError 403` em `speech.platform.bing.com`), até em frase simples. É o TTS que o James usa hoje na `main`, então o James em `main` está **sem voz de resposta no texto** neste momento. O Piper (Task 9) resolve isso de vez; até lá, `[voz] motor = "edge"` não é opção de volta confiável. O motor `edge` fica no código só por compatibilidade.

### Task 0: whisper.cpp, voz do Piper e dependências em lugar estável

Não é código, então não há TDD. A prova são os comandos de verificação.

- [ ] **Passo 1: whisper.cpp num caminho estável.** Copiar `build/bin` (binários e `.so`) para `~/.local/share/james/whisper.cpp/bin` e o modelo turbo q8 para `~/.local/share/james/modelos/`. Fica fora do repo. Não apagar nada de `~/Downloads/stt-teste`.

  ```bash
  mkdir -p ~/.local/share/james/whisper.cpp ~/.local/share/james/modelos && cp -a ~/Downloads/stt-teste/whisper.cpp/build/bin ~/.local/share/james/whisper.cpp/ && cp ~/Downloads/stt-teste/whisper.cpp/models/ggml-large-v3-turbo-q8_0.bin ~/.local/share/james/modelos/
  ```

- [ ] **Passo 2: provar que roda da pasta nova, sem a de `Downloads`.**

  ```bash
  W=~/.local/share/james/whisper.cpp/bin; LD_LIBRARY_PATH=$W ldd $W/whisper-server | grep -E 'ggml|whisper'
  ```

  Esperado: todas as `.so` resolvidas em `~/.local/share/james/...`. Depois, subir na porta 8179 e mandar o `pergunta.wav` do spike (regerar com os comandos do doc do spike, se o scratchpad sumiu). Esperado: o texto da pergunta e ~0,2 s.

- [ ] **Passo 3: voz do Piper em `vozes/`** (fora do git). O download já foi autorizado pelo Arthur em 2026-10-10. Conferir o tamanho (63.201.294 bytes) e o início do sha256 (`858555e3a064209c`).

  ```bash
  mkdir -p vozes && U=https://huggingface.co/rhasspy/piper-voices/resolve/main/pt/pt_BR/faber/medium && curl -sSL -o vozes/pt_BR-faber-medium.onnx "$U/pt_BR-faber-medium.onnx" && curl -sSL -o vozes/pt_BR-faber-medium.onnx.json "$U/pt_BR-faber-medium.onnx.json" && sha256sum vozes/pt_BR-faber-medium.onnx | cut -c1-16
  ```

- [ ] **Passo 4:** `.gitignore` ganha `vozes/`. `pyproject.toml` ganha `piper-tts>=1.8`, `pysilero-vad>=3.4` e `requests>=2.32`, com comentário do porquê (sem torch: o `silero-vad` oficial puxa CUDA). Reinstalar o venv do device e rodar o pytest: tem de continuar **101 passed, 1 skipped** nesta branch (os 19 testes do script de corpus moram na `feat/stt-whisper`).
- [ ] **Passo 5: não se aplica aqui.** `go mod tidy` remove o SDK da Anthropic enquanto nada o importa. Ele entra no `go.mod` na Task 3, com `go get github.com/anthropics/anthropic-sdk-go@v1.80.0` (exige Go ≥ 1.24; o projeto está em 1.26).
- [ ] **Passo 6: commit.**

  ```bash
  git add .gitignore dispositivo/pyproject.toml && git commit -m "build: dependencias da voz em tempo real (Anthropic SDK, Piper, pysilero-vad)"
  ```

---

## Fatia 1: cérebro Claude (Go)

### Task 1: configuração do provedor e da voz

**Arquivos:** `internal/config/config.go`, `config_test.go`, `.env.example`, `config.toml`

- [ ] **Testes (tabela):**

  | Caso | Toml e env | Esperado |
  |---|---|---|
  | padrão | sem `[agente] provedor` | `Provedor == "anthropic"`, `Modelo == "claude-haiku-5-5"` |
  | anthropic sem chave | `CLAUDE_API_KEY=""` | erro citando `CLAUDE_API_KEY` |
  | anthropic com chave | `CLAUDE_API_KEY=sk-ant-x`, `GEMINI_API_KEY=""` | sobe (Gemini não é mais obrigatório) |
  | gemini sem chave | `provedor="gemini"`, `GEMINI_API_KEY=""` | erro citando `GEMINI_API_KEY` |
  | modelo descartado | `provedor="gemini"`, `modelo="gemini-2.5-flash-lite"` | erro, como hoje |
  | voz padrão | sem `[voz]` | `MotorVoz == "piper"`, `TTSURL == "http://127.0.0.1:8180"` |
  | provedor inválido | `provedor="openai"` | erro claro |

  Ajustar os testes atuais que zeram só a `GEMINI_API_KEY` para também definir `CLAUDE_API_KEY`.
- [ ] **Rodar e ver falhar** (campos inexistentes).
- [ ] **Implementar:**
  - campos `Provedor`, `ClaudeAPIKey`, `MotorVoz`, `TTSURL`;
  - toml `[agente] provedor`, `[voz] motor`, `tts_url`;
  - validação por provedor;
  - a regra do `flash-lite` vale só para `gemini`.
  - No `config.toml`: `provedor = "anthropic"`, `modelo = "claude-haiku-5-5"`, `[voz] motor = "piper"`. A `voz_tts` fica, comentada como "só para motor = edge".
  - No `.env.example`: a linha `CLAUDE_API_KEY=` com comentário.
- [ ] **Verde**, `GOOS=windows go vet ./...`, commit: `feat(config): provedor anthropic por padrao e motor de voz piper`.

### Task 2: `Modo` na requisição e `Uso` na resposta

**Arquivos:** `internal/agente/tipos.go`, `tipos_test.go`

- [ ] **Teste:** o valor zero de `Modo` é `ModoTexto` (comportamento conservador: pensa). `ModoVoz` existe. `Resposta.Uso` tem `Entrada`, `Saida`, `CacheLeitura`, `Pensamento` (int) e valor zero.
- [ ] **Implementar:** `type Modo int` com `ModoTexto` (= 0) e `ModoVoz`, mais o campo `Requisicao.Modo` e o tipo `Uso`. O Gemini ignora `Modo`, com um comentário dizendo isso.
- [ ] Verde e commit.

### Task 3: mapeamento James ↔ Anthropic (puro)

**Arquivos:** `internal/agente/anthropic/mapeamento.go`, `mapeamento_test.go`

- [ ] **Testes** (sem rede, comparando o JSON serializado dos params do SDK):
  1. Sistema vira `System` com **um** bloco de texto e `cache_control` efêmero.
  2. `Declaracao` vira `ToolParam` com `Properties`, `Required` e `Description` (tipos `string`/`integer`/`boolean`/`number` como no Gemini).
  3. Ordem `Usuario` → `Modelo` (texto + 2 chamadas) → `Ferramenta` (2 resultados): `user`, `assistant` com `text` e dois `tool_use`, e `user` com dois `tool_result`, ids pareados.
  4. **Ids legados repetidos:** duas chamadas com `ID == "executar_comando_sistema"` na mesma mensagem viram ids únicos e estáveis (`executar_comando_sistema`, `executar_comando_sistema_2`), e os resultados seguem o mesmo pareamento, por posição.
  5. Resultado com `Erro` vira `tool_result` com `is_error: true`, e o conteúdo é a mensagem de erro.
  6. Resultado com `Imagem` (PNG) vira conteúdo com bloco de imagem base64 mais o texto.
  7. Modelo com texto vazio e só chamadas não gera bloco `text` vazio (a API rejeita).
  8. Resposta: `Message` com `text` + `tool_use` (input `{"comando":"ls"}`) vira `Resposta{Texto, Chamadas[{ID, Nome, Args}]}`. Blocos `thinking` são ignorados. `Uso` vem do `usage`.
- [ ] **Rodar e ver falhar.**
- [ ] **Implementar** `paraParams(req, modelo) anthropic.MessageNewParams` e `paraResposta(msg anthropic.Message) agente.Resposta`. Nomes de tipo do SDK: consultar `go doc` ao escrever.
- [ ] Verde e commit: `feat(agente): mapeamento entre as mensagens do James e a API da Anthropic`.

### Task 4: provedor Anthropic, streaming e pensamento por modo

**Arquivos:** `internal/agente/anthropic/anthropic.go`, `anthropic_test.go`, `testdata/*.sse`

- [ ] **Passo 1: gravar streams reais como fixtures.** Com `curl --no-buffer` direto na API, usando a chave lida do `.env` dentro de um script que não a imprime:
  - (a) pergunta simples, só texto;
  - (b) pedido que exige ferramenta, com a declaração de `executar_comando_sistema`.

  Salvar **só o corpo da resposta** em `testdata/texto.sse` e `testdata/ferramenta.sse`. Conferir com grep que não há `sk-ant` em nenhum arquivo. Se o stream (b) vier sem `tool_use`, registrar: isso já antecipa o risco do CA3.
- [ ] **Testes** (`httptest.Server` servindo a fixture; cliente com `option.WithBaseURL` e `option.WithAPIKey("teste")`):
  1. `GerarStream` chama `emitir` com os deltas **na ordem**, e a concatenação é igual ao `Resposta.Texto`.
  2. Fixture de ferramenta: `Resposta.Chamadas` com nome e args decodificados.
  3. O servidor falso captura o corpo da requisição:
     - `ModoVoz` manda `"thinking":{"type":"disabled"}`;
     - `ModoTexto` manda thinking `adaptive` e effort `low`;
     - `max_tokens` = 4096;
     - `stream` = true.
  4. Stream com `stop_reason: "refusal"` vira erro tipado `ErroRecusa`, com mensagem ao usuário.
  5. Servidor devolvendo 529/500 vira erro (o SDK pode retentar: limitar `max_retries` a 1 para o turno de voz não esperar demais).
  6. `Gerar` (sem stream) devolve o mesmo que `GerarStream` com `emitir` nulo.
- [ ] **Rodar e ver falhar.**
- [ ] **Implementar** `Novo(apiKey, modelo string, opts ...option.RequestOption) *Provider`, `Gerar` e `GerarStream`, com `message.Accumulate` para a resposta final.
- [ ] Verde e commit: `feat(agente): provedor Anthropic com streaming real e pensamento por modo`.

### Task 5: **PORTÃO: ferramentas com pensamento desligado (CA3)**

O risco número 1 da spec. Tudo o que vem depois assume que este portão passou.

**Arquivo:** `internal/agente/anthropic/integracao_test.go` (`//go:build integracao`, e só roda com `JAMES_TESTE_ANTHROPIC=1`).

- [ ] **Teste:**
  - sistema = `prompts/sistema.md` real;
  - ferramentas = as declarações reais do registro do jamesd, montadas como em `cmd/jamesd`;
  - `ModoVoz`;
  - 10 pedidos que exigem ferramenta ("lista os arquivos da pasta atual", "que horas são no sistema?" e outros).
  - **Esperado:** em 10 de 10, `Resposta.Chamadas` não vazia. Em nenhuma, o `Texto` contém algo que imite chamada (`executar_comando_sistema(`, `{"comando"`, `<tool`).
- [ ] **Rodar:**

  ```bash
  JAMES_TESTE_ANTHROPIC=1 go test -tags integracao -count=1 -v -run Integracao ./internal/agente/anthropic/
  ```

  O teste lê a `CLAUDE_API_KEY` do `.env` sem imprimir. Custo: frações de centavo.
- [ ] **Decisão pela saída:**
  - **10/10:** segue o plano.
  - **Qualquer falha:** repetir com `ModoVoz` = adaptativo, effort `low`. Se passar, mudar o mapeamento do `ModoVoz` para isso, anotar na spec (§9 já prevê) e seguir. Se também falhar, **parar e chamar o Arthur**.
- [ ] Commit do teste (o portão fica versionado): `test(agente): portao do CA3, ferramentas com pensamento desligado`.

### Task 6: loop com streaming e escolha do provedor

**Arquivos:** `internal/agente/loop.go`, `loop_test.go`, `cmd/jamesd/main.go`, `internal/api/ws.go`

- [ ] **Testes (loop):**
  - com `AoDelta` não nulo, o loop usa `GerarStream` e repassa os deltas de **todas** as iterações, inclusive o texto antes de uma chamada de ferramenta;
  - com `AoDelta` nulo, usa `Gerar` (comportamento atual);
  - `Modo` chega ao provedor;
  - o `Uso` das iterações é somado na resposta final.
- [ ] **Implementar:**
  - `Loop.AoDelta func(string)` e `Loop.Modo agente.Modo`;
  - `processarTurno` recebe o modo; `tratarMensagem` passa `ModoTexto`;
  - `main.go` constrói `anthropic.Novo(cfg.ClaudeAPIKey, cfg.Modelo)` ou `gemini.Novo(...)` conforme `cfg.Provedor`, e loga o provedor e o modelo no boot;
  - log por turno: `turno provedor=… modelo=… entrada=… saida=… cache=… dur=…`.
- [ ] Verde e commit: `feat(agente): loop com streaming e jamesd escolhendo o provedor pela config`.

---

## Fatia 2: fala

### Task 7: quebrador de frases e limpeza para fala (puro)

**Arquivos:** `internal/voz/fala.go`, `fala_test.go`

- [ ] **Testes (tabela):**

  | Entrada | Esperado |
  |---|---|
  | deltas `"A capital"`, `" é Canberra. Ela"`, `" fica"` | frase 1 = `A capital é Canberra.`; `Fim()` devolve `Ela fica` |
  | `"Sim! Claro? Ok."` num delta só | três frases |
  | `"3.14 é pi."` | uma frase (ponto entre dígitos não quebra) |
  | `"Dr. Silva chegou."` | limite aceito: pode quebrar em `Dr.`; o teste documenta o comportamento |
  | 300 caracteres sem pontuação | corta no último espaço antes de N = 200 |
  | `"linha um\nlinha dois"` | quebra na linha |
  | `LimparParaFala("A capital é **Canberra**.")` | `A capital é Canberra.` |
  | lista `"- item um\n- item dois"` | `item um. item dois.` |
  | `` "use `ls -la`" `` | `use ls -la` |
  | bloco ```` ``` ```` com código | `veja o código na tela.` |
  | `"[docs](https://x.invalid)"` | `docs` |
  | URL solta `https://x.invalid/a` | `o link está na tela` |
  | `"# Título"` | `Título` |
- [ ] **Implementar** `QuebradorDeFrases` (`Acrescentar(delta) []string` e `Fim() string`) e `LimparParaFala(string) string`.
- [ ] Verde e commit: `feat(voz): quebrador de frases e limpeza de Markdown para a fala`.

### Task 8: turno de voz fala pelo device e não gera `audio_pronto`

**Arquivos:** `internal/api/dispositivo.go`, `eventos.go`, `ws.go`, testes em `internal/api`

- [ ] **Testes** (padrão dos testes atuais de `dispositivo_test.go`, com provedor falso que faz streaming):
  1. O device manda `entrada_voz` e recebe, **na própria conexão**, `fala_frase` com `indice` 0, 1… na ordem e o texto já limpo, depois `fala_fim`.
  2. A primeira `fala_frase` chega **antes** do fim do turno: o provedor falso segura a segunda parte até o teste ler a primeira frase.
  3. Turno de voz: **nenhum** `audio_pronto` no broadcast; `transcricao` e `resposta_final` continuam (CA8).
  4. Turno de texto (`tratarMensagem`): `audio_pronto` continua vindo, com sintetizador falso.
  5. Erro do provedor num turno de voz: o device recebe `fala_erro` com a mensagem segura (a mesma de `mensagemDeErroDoTurno`).
  6. Turno de voz passa `ModoVoz` ao loop; turno de texto passa `ModoTexto`.
- [ ] **Implementar:**
  - `tratarEntradaDeVoz` recebe a `*conexaoDispositivo`;
  - monta `AoDelta`: `QuebradorDeFrases` → `LimparParaFala` → `conexao.Enviar(fala_frase)`;
  - no fim, manda o resto e `fala_fim`;
  - pula `gerarAudioDaResposta`.
  - Payloads nomeados (struct, como os existentes): `payloadFalaFrase{Texto, Indice}`, `payloadFalaErro{Mensagem}`.
- [ ] Verde e commit: `feat(api): turno de voz fala frase a frase pelo device, sem audio_pronto`.

### Task 9: Piper no turno de texto (jamesd)

**Arquivos:** `internal/voz/sintetizador.go`, `_test.go`, `internal/api/rest.go`, testes, `cmd/jamesd/main.go`

- [ ] **Testes:**
  1. `NovoSintetizadorPiper(dir, url)`: `Gerar` faz `POST {url}/sintetizar` com `{"texto": ...}` **já limpo** (`LimparParaFala`) e grava `id.wav` com os bytes devolvidos (`httptest`).
  2. Servidor de TTS fora do ar: `Gerar` devolve erro (o chamador já trata como melhor esforço).
  3. O edge-tts atual continua gerando `id.mp3` (teste existente intacto).
  4. `GET /api/audio/{id}` serve `id.wav` com `Content-Type: audio/wav`, e `id.mp3` com `audio/mpeg`. O Go não conhece `.wav` na tabela embutida de MIME, por isso o cabeçalho é **explícito**.
  5. `DELETE /api/audio/{id}` apaga a extensão que existir.
- [ ] **Implementar:**
  - `Sintetizador.Extensao`;
  - executor Piper com `http.Client` com timeout de 10 s;
  - `servirAudio` e `deletar` procuram `.wav` e depois `.mp3`;
  - `main.go` escolhe pelo `cfg.MotorVoz`.
- [ ] Verde e commit: `feat(voz): turno de texto fala com o Piper pelo servidor de TTS local`.

### Task 10: síntese e fala no device

**Arquivos:** `dispositivo/james_device/sintese.py`, `fala.py`, `tests/test_sintese.py`, `tests/test_fala.py`

- [ ] **Testes `sintese`:**
  - `Sintetizador(caminho)` carrega a voz uma vez (`PiperVoice.load` falso conta as chamadas);
  - `sintetizar("oi.")` devolve `(pcm_bytes, taxa)` juntando os `AudioChunk`;
  - um teste real, marcado `skip` se `vozes/` não existir, sintetiza "teste." e confere taxa 22050 e duração > 0.
- [ ] **Testes `fala`** (reprodutor falso que só registra; sintetizador falso):
  1. Frases fora de ordem (`indice` 1 antes do 0) tocam **na ordem**.
  2. `ocupado()` fica `True` desde `comando_enviado()` até a última frase tocada depois de `fala_fim`.
  3. `fala_erro` toca a mensagem e libera o `ocupado`.
  4. Sem `fala_fim` em `JAMES_TIMEOUT_RESPOSTA_S` (padrão 30 s, com relógio falso), `ocupado` libera com log.
  5. `nao_entendi()` toca "Não entendi, pode repetir?" sem passar pelo jamesd (D10).
  6. Falha do reprodutor não derruba a fila: loga e segue para a próxima frase.
  7. `medidor.marca("primeiro_audio")` é chamado só no primeiro áudio do turno.
- [ ] **Implementar:** reprodutor real por stream de saída do PyAudio, aberto por resposta (D12), em thread própria. Fila com `threading`, sem `asyncio`.
- [ ] Verde e commit: `feat(device): sintese com Piper e fila de fala na ordem das frases`.

### Task 11: o device recebe `fala_*`

**Arquivos:** `dispositivo/james_device/conexao.py`, `tests/test_conexao.py`

- [ ] **Testes:**
  - `_tratar` repassa `fala_frase` (texto, indice), `fala_fim` e `fala_erro` aos callbacks injetados;
  - `acao_tela` continua indo ao `manipulador_acao`;
  - tipo desconhecido é ignorado sem erro.
- [ ] Implementar, verde e commit: `feat(device): conexao entrega fala_frase, fala_fim e fala_erro`.

### Task 12: servidor de TTS

**Arquivos:** `dispositivo/james_device/servidor_tts.py`, `tests/test_servidor_tts.py`

- [ ] **Testes** (servidor real em porta efêmera, sintetizador falso):
  1. `POST /sintetizar {"texto":"oi."}` devolve 200, `Content-Type: audio/wav` e WAV válido (abre com `wave`; 1 canal, 16 bits, taxa do sintetizador).
  2. Texto vazio ou JSON inválido devolve 400.
  3. `GET /saude` devolve 200.
  4. **Escuta só em `127.0.0.1`:** construir com `0.0.0.0` levanta erro.
- [ ] **Implementar:** `ThreadingHTTPServer`, voz carregada no boot pelo mesmo `sintese.Sintetizador`, porta por `JAMES_TTS_PORTA` (padrão 8180).
- [ ] Verde e commit: `feat(device): servidor de TTS local para o turno de texto`.

---

## Fatia 3: fim de fala e STT

### Task 13: VAD

**Arquivos:** `dispositivo/james_device/vad.py`, `tests/test_vad.py`

- [ ] **Testes:**
  - bloco de 0,25 s (8000 bytes) é dividido em quadros de 1024 bytes, e o resto fica guardado para o próximo bloco;
  - `tem_voz(bloco)` é `True` se algum quadro passar do limiar (padrão 0,5), com detector falso;
  - `reset()` limpa o resto e o estado do detector;
  - teste real com `pysilero-vad`: silêncio dá `False`; o `pergunta.wav` do spike, se existir em `testdata`, dá `True` no meio.
- [ ] Implementar, verde e commit: `feat(device): VAD com pysilero-vad em quadros de 32 ms`.

### Task 14: fim de fala com STT especulativo

**Arquivos:** `dispositivo/james_device/escuta.py`, `tests/test_escuta.py`

`_coletar_janela` (por tempo) vira `coletar_comando(...)`. Tudo injetado: fonte, VAD, relógio, transcritor, executor (síncrono nos testes), reconhecedor (para o "enviar"), parâmetros e medidor.

- [ ] **Testes** (CA5, com fonte, VAD e relógio falsos; os 17 testes atuais da wake word continuam passando):
  1. Voz, depois W de silêncio: fecha e devolve o texto da **especulação**, sem segunda transcrição.
  2. Voz, silêncio menor que W, voz de novo: a especulação é descartada, e a transcrição final usa o áudio completo.
  3. Especulação terminando em palavra de ligação ("e", "então", "pra", "que", "de", "tipo"): não fecha em W; fecha em W longo.
  4. Teto de 10 s fecha mesmo com voz contínua.
  5. "enviar" (reconhecedor falso) fecha na hora, e a palavra sai do texto (comportamento atual).
  6. Sem voz em 5 s depois de acordar: devolve vazio e loga "desisti".
  7. O `medidor` recebe `fim_fala` no começo do silêncio que fechou e `stt_pronto` quando a transcrição usada terminou.
  8. **Guarda do teto contra o `-ac`:** `JAMES_COMANDO_MAX_S` acima da cobertura do `-ac` configurado (512 → 10 s) faz a construção falhar com mensagem clara.
- [ ] **Implementar:** a especulação usa `concurrent.futures.ThreadPoolExecutor(max_workers=1)` em produção. Descartar quer dizer não usar o resultado; não cancela a requisição em voo.
- [ ] Verde e commit: `feat(device): fim de fala por VAD com transcricao especulativa`.

### Task 15: half-duplex e continuação

**Arquivos:** `dispositivo/james_device/escuta.py`, `tests/test_escuta.py`

- [ ] **Testes** (CA12, e a parte de unidade do CA9):
  1. Depois de `ao_comando`, enquanto `ocupado()` for `True`, a escuta **consome e descarta** os blocos: nenhum vai ao Vosk nem ao VAD, e não há acordar.
  2. `ocupado()` vira `False`: Vosk e VAD são zerados; com voz dentro de `JAMES_CONTINUACAO_S` (relógio falso), `coletar_comando` começa **sem** wake word e o comando sai.
  3. Sem voz em 6 s: volta a exigir a wake word, e voz sem "James" não gera comando.
  4. `JAMES_CONTINUACAO_S=0`: não há continuação.
  5. STT vazio ou com falha: chama `ao_nao_entendeu` (fala "Não entendi…") e conta como resposta, entrando em `ocupado` e continuação como um turno normal.
- [ ] Implementar, verde e commit: `feat(device): half-duplex enquanto o James fala e continuacao de 6 s sem wake word`.

### Task 16: transcritor Whisper e fiação do `main.py`

**Arquivos:** `dispositivo/james_device/main.py`, `tests/test_main_transcritor.py`

- [ ] **Testes:**
  - `criar_transcritor()` escolhe pelo `JAMES_STT` (`whisper` é o padrão; `google`; valor inválido levanta erro no boot);
  - o transcritor Whisper monta WAV 16 kHz mono a partir dos bytes e faz POST multipart em `JAMES_WHISPER_URL` (`httptest` com `http.server` local);
  - servidor fora ou HTTP 500 devolvem `""` com log;
  - áudio vazio devolve `""` sem requisição.
- [ ] **Implementar a fiação:**
  - `Sintetizador` carregado uma vez;
  - `Fala` com reprodutor PyAudio;
  - `ConexaoJames` com os callbacks `fala_*`;
  - `escutar(...)` com VAD, executor, `ocupado=fala.ocupado`, `ao_nao_entendeu=fala.nao_entendi` e os parâmetros por env da spec (§6.1);
  - `ao_comando` marca `comando_enviado` no `Fala` e no `Medidor`.
  - O `faaah.wav` de "fechei a janela" continua por enquanto; a Task 19 mede se ele atrasa o primeiro som.
- [ ] Verde (incluindo `test_plataforma`: o `main` continua importando sem `tela`) e commit: `feat(device): transcricao no whisper-server e fiacao da voz em tempo real`.

---

## Fatia 4: medição e ambiente

### Task 17: medidor de latência

**Arquivos:** `dispositivo/james_device/medidor.py`, `tests/test_medidor.py`, `internal/api` (log já feito na Task 6)

- [ ] **Testes:**
  - marcos fora de ordem ou repetidos não quebram;
  - com `fim_fala`, `stt_pronto`, `entrada_enviada`, `primeira_frase` e `primeiro_audio`, a linha TSV tem `turno, fim_fala→1º_som, stt, jamesd, tts` em segundos com 3 casas;
  - turno sem `primeiro_audio` (erro) grava com campo vazio;
  - **a linha nunca contém o texto do comando** (teste: passar um texto com marcador e conferir que ele não aparece no arquivo).
- [ ] **Implementar:** grava em `.executar/latencia.tsv` (cabeçalho na primeira escrita) e loga `latencia turno=… fim_fala→1º_som=…`.
- [ ] Verde e commit: `feat(device): medidor de latencia por turno, so numeros`.

### Task 18: `executar.sh` sobe STT e TTS

**Arquivos:** `executar.sh`, `.env.example`

- [ ] **Antes do jamesd:**
  - sobe o `whisper-server`, com `LD_LIBRARY_PATH` na pasta do binário e caminhos de `JAMES_WHISPER_SERVIDOR`/`JAMES_WHISPER_MODELO`, com padrão em `~/.local/share/james/`;
  - sobe o servidor de TTS (`dispositivo/.venv/bin/python -m dispositivo.james_device.servidor_tts`);
  - espera os dois responderem por até 30 s cada;
  - os dois entram em `PIDS` (o `trap` derruba).
- [ ] **Fallbacks com aviso** (§7 da spec):
  - sem `whisper-server`: exporta `JAMES_STT=google` para o device;
  - sem TTS: só avisa.
- [ ] **Verificar:**

  ```bash
  bash -n executar.sh
  ```

  Depois, subir de verdade e conferir `pgrep -x whisper-server`, `curl -s 127.0.0.1:8180/saude` e o log. A subida completa fica para a Task 19.
- [ ] Commit: `feat(ambiente): executar.sh sobe o whisper-server e o servidor de TTS`.

---

## Fatia 5: verificação

### Task 19: aceite ao vivo

- [ ] **CA1:** as três suítes, com a saída colada.
- [ ] **CA3:** a saída do portão da Task 5, se mudou desde então.
- [ ] **CA6:** o Arthur fala **≥ 20 turnos** reais. Mediana e p95 de `fim_fala→1º_som` calculadas do `.executar/latencia.tsv`. Meta: mediana ≤ 2,0 s e p95 ≤ 3,0 s.
  - Se o `faaah.wav` estiver somando tempo (marcos mostram o atraso), tirar e medir de novo.
- [ ] **CA8 e CA13:** turno digitado no app toca com a voz do Piper; o log mostra o áudio pronto ≤ 0,5 s depois da resposta.
- [ ] **CA9:** durante a fala do James, o log não mostra "acordei".
- [ ] **CA10:** derrubar o `whisper-server` no meio: o device diz "Não entendi…"; com `JAMES_STT=google`, funciona.
- [ ] **CA11:** tokens por turno no log do jamesd; projeção mensal anotada depois de uma semana de uso (fica como pendência datada, se a semana não tiver passado).
- [ ] **CA12:** continuação: responder em até 6 s sem "James" funciona; depois de 6 s, não.
- [ ] **CA7:** depende do corpus. Se o Arthur ainda não gravou, fica registrado como pendente, e não bloqueia o merge da fatia, por decisão do Arthur na revisão.

### Task 20: revisão e fechamento

- [ ] `/code-review` e `/security-review` no diff da branch. Os achados viram tasks de correção com TDD antes do merge.
- [ ] Atualizar o plano com o que fechou e o que ficou aberto, mais a memória do projeto.
- [ ] Merge em `main` **só com o OK do Arthur**.

---

## Fora deste plano
- Spec 2 (memória consolidada, modelo avançado via Claude Code) e spec 3 (câmera).
- Barge-in (interromper o James falando).
- Streaming de texto para o app.
- Windows: compila (`GOOS=windows go vet`), sem verificação funcional.
