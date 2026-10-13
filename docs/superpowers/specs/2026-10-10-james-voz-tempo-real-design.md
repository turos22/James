# James: voz em tempo real

**Data:** 2026-10-10
**Autor:** Arthur Orosco (design colaborativo)
**Estado:** aprovado pelo Arthur em 2026-10-11
**Projeto:** `/home/arthurorosco/SI/Projetos/Athena(Go)/meu-app-ts`
**Base:** [`decisoes/2026-10-10-james-agilidade-e-inteligencia.md`](../decisoes/2026-10-10-james-agilidade-e-inteligencia.md) (as decisões do Arthur) e [`spikes/2026-10-10-latencia-voz.md`](../spikes/2026-10-10-latencia-voz.md) (as medições).

Esta é a primeira de três specs da iniciativa "James mais ágil e mais esperto". As outras duas ficam para depois: memória com dois modelos e câmera.

---

## 1. Contexto

### 1.1 O que o Arthur pediu
Que o James responda **o mais rápido possível, "tempo real ou o mais próximo disso"**, sem precisar dizer "enviar". Também que a inteligência seja "boa como o Claude", com custo de API de R$ 20–30 por mês no máximo.

### 1.2 Como é hoje (lido no código em 2026-10-10)

| Etapa | Hoje | Onde |
|---|---|---|
| Fim do comando | Janela fixa de **8 s**, ou a palavra "enviar" | `dispositivo/james_device/escuta.py` |
| STT | Google, API web gratuita, só depois que a janela fecha | `main.py`, `criar_transcritor` |
| LLM | Gemini 2.5 Flash, **sem streaming**: `GerarStream` chama `Gerar` e devolve tudo de uma vez | `internal/agente/gemini/gemini.go:48` |
| Loop de ferramentas | Só chama `Provider.Gerar` | `internal/agente/loop.go:86` |
| TTS | edge-tts gera o MP3 inteiro; o front baixa e toca | `internal/voz/sintetizador.go`, `web/src/App.tsx` |
| jamesd → device | Só `acao_tela` | `internal/api/dispositivo.go`, `dispositivo/james_device/conexao.py` |

### 1.3 O que os spikes mediram

Do fim da fala ao primeiro som, com áudio sintético e limpo, na máquina do Arthur:

| Configuração | Mediana | Faixa |
|---|---|---|
| Gemini Live | 1,44 s | 1,35–1,55 |
| **Pipeline: Whisper turbo na GPU + Claude Haiku 5.5 sem pensamento + Piper** | **1,44 s** | 1,38–1,53 |

Por etapa, no pipeline:

| Etapa | Mediana | Nota |
|---|---|---|
| Espera de silêncio W | 0,70 s | parâmetro |
| STT | 0,18 s | roda dentro de W |
| Haiku até a 1ª frase | 0,70 s | |
| Piper até o 1º áudio | 0,03 s | |

O custo estimado é ≈ R$ 6 por mês com 40 perguntas por dia.

---

## 2. Escopo

### 2.1 Entra
1. **Fim de fala automático**, com "enviar" como atalho opcional.
2. **STT local na GPU** (whisper.cpp com Vulkan, `large-v3-turbo-q8_0`). O Google continua disponível por `JAMES_STT=google`.
3. **Claude Haiku 5.5 como LLM**, com **streaming real** até o device.
4. **TTS local com Piper** (`pt_BR-faber-medium`), sintetizado e tocado **no device**, frase a frase.
5. **Instrumentação** das etapas, para medir a latência em uso real.
6. **`executar.sh`** sobe e derruba os servidores de STT e de TTS.
7. **Uma voz só:** o Piper fala também a resposta do turno de **texto** (digitado no app). O edge-tts sai do caminho padrão (D8).
8. **Continuação sem wake word:** depois que o James termina de falar, há 6 s para responder sem dizer "James" (D13).

### 2.2 Não entra
- Memória consolidada, modelo avançado e escalonamento: spec 2.
- Câmera: spec 3.
- Interromper o James enquanto ele fala (barge-in).
- Streaming de **texto** para o app: no turno digitado, a resposta continua aparecendo inteira, no `resposta_final`.
- Windows: fora do escopo funcional. O código não pode quebrar a compilação Windows (`GOOS=windows go vet ./...`), mas não há verificação lá.
- Tela no Wayland (B2).

---

## 3. Decisões

| # | Decisão | Motivo |
|---|---|---|
| D1 | **Pipeline**, não API de voz em tempo real. | Mesma latência medida, cerca de 5x mais barato, Claude, voz do Arthur fica local. Decisão do Arthur. |
| D2 | **Claude Haiku 5.5** (`claude-haiku-5-5`), chave em `CLAUDE_API_KEY`. O Gemini continua implementado e selecionável por config. | "Bom como o Claude" e o mais barato que mede bem. Manter o Gemini dá um caminho de volta sem reescrever. |
| D3 | **Turno de voz: pensamento desligado** (`thinking: {type: "disabled"}`). **Turno de texto: pensamento adaptativo** com `effort` `low`. | Sem pensamento: 0,72–0,82 s até a 1ª frase. Com pensamento: 1,5–2 s. No texto a latência pesa menos e a qualidade mais. |
| D4 | O jamesd quebra a resposta em **frases** e manda cada uma ao device assim que fica pronta. O device sintetiza e toca. | O primeiro som sai na primeira frase, não no fim da resposta. Quebrar no jamesd (Go, testável) deixa o device simples. |
| D5 | **Markdown sai do texto falado.** O jamesd limpa antes de mandar a frase. O texto na tela continua com Markdown. | O spike pegou `**Canberra**`; um TTS lê os asteriscos. |
| D6 | **Fim de fala por VAD local** no device. Fecha depois de **W = 700 ms** de silêncio após ter havido voz. Teto de duração e "enviar" continuam valendo. | 700 ms é a faixa usual para não cortar pausa no meio da frase. É parâmetro, calibrado com uso real. |
| D7 | **STT especulativo.** No primeiro silêncio, o device já manda o áudio ao Whisper; se a voz voltar antes de W, descarta. Se a transcrição terminar numa palavra de ligação ("e", "então", "pra", "que", "de", "tipo"…), a espera estica para **W longo = 1,5 s**. | O STT (0,18 s) some dentro de W. A dica de frase incompleta vem de graça da transcrição que já ia acontecer (o Vosk, em gramática fechada, não serve para isso). |
| D8 | **Uma voz só, a mais rápida: Piper nos dois caminhos.** Turno de **voz**: o device fala com o Piper carregado nele, frase a frase, e o jamesd **não** gera `audio_pronto`. Turno de **texto**: o jamesd sintetiza a resposta inteira com o Piper, por um **servidor de TTS local**, e emite **um** `audio_pronto`, como hoje. As duas pontas usam o mesmo arquivo de voz (`JAMES_PIPER_VOZ`). O edge-tts fica no código como opção de config, fora do padrão. | Pedido do Arthur (2026-10-11): voz igual no texto e na voz, usando a mais rápida. No texto não precisa ser frase a frase: o Piper gera 1 s de fala em ~17 ms, então uma resposta de 10 s sai em ~0,2 s. Sem `audio_pronto` no turno de voz, não há voz dupla (device + app). |
| D9 | **Half-duplex.** Enquanto o device fala, a escuta da wake word fica pausada. | O microfone ouve o próprio James. Sem isso, a fala dele pode disparar a wake word ou o VAD. Barge-in fica fora (§2.2). |
| D10 | **Falha ou transcrição vazia: o device diz "Não entendi, pode repetir?"** com o Piper, local, sem depender do jamesd. Sem cair para o Google. | Decisão da Parte A (A.1 e A7), mantida. |
| D11 | **Whisper via `whisper-server`** (whisper.cpp, Vulkan), processo separado e sempre carregado, chamado por HTTP local. Contexto reduzido (`-ac`) **dimensionado para cobrir o teto de duração do comando** (§6.3). | Foi o que mediu 0,17 s. Processo separado mantém o modelo quente e isola a GPU do device. |
| D12 | **Reprodução pelo PyAudio** (stream de saída), aberto uma vez por resposta, frases em sequência. | Já é dependência do device (é o microfone). Sem subprocesso por frase, sem buraco entre frases. |
| D13 | **Continuação sem wake word, ligada por padrão.** Quando o James termina de falar, o device abre uma janela de **6 s** em que voz detectada pelo VAD conta como novo comando, sem "James". Passou a janela sem voz, volta a exigir a wake word. `JAMES_CONTINUACAO_S` (padrão 6; 0 desliga). | Pedido do Arthur (2026-10-11, "vamos ver como fica"): deixa a conversa natural. O risco (TV ou conversa paralela disparando) se resolve desligando por env. |

**Substitui, da Parte A:** o backend (faster-whisper `large-v3` na CPU) e a prioridade (precisão primeiro) dão lugar a whisper.cpp turbo na GPU e a velocidade primeiro, por decisão do Arthur em 2026-10-10. **Ficam:** `JAMES_STT=whisper|google`, sem fallback para o Google, e pedir para repetir. O **corpus** da Parte A passa a ser o critério de precisão desta spec (CA7).

---

## 4. Arquitetura

```
 microfone ─► device (Python)                               jamesd (Go)                 Anthropic
              │ wake word (Vosk)
              │ VAD: voz… silêncio ──┐
              │   1º silêncio ───────┼─► whisper-server (GPU) ─ texto
              │   W sem voz ─────────┘   (já pronto)
              │ entrada_voz{texto} ─────────────────────────► processarTurno
              │                                               │ Loop + Provider Anthropic ─► stream
              │                                               │   deltas ─► quebra em frases
              │ ◄──────────────────────── fala_frase{texto} ──┤   (Markdown limpo)
              │ Piper ─► PyAudio ─► alto-falante              │ ferramentas (como hoje)
              │ ◄──────────────────────── fala_fim ───────────┤
              │ continuação 6 s (VAD, sem wake word)          └► SPA: transcricao, ferramenta_*,
              │ depois volta a exigir a wake word                  resposta_final (sem audio_pronto)

 Turno de TEXTO (digitado no app):
 SPA ─ mensagem ─► jamesd ─ Loop + Anthropic (pensamento adaptativo, low) ─► resposta completa
                    └─► servidor de TTS (Piper, local) ─► WAV ─► audio_pronto ─► SPA toca
```

---

## 5. jamesd (Go)

### 5.1 Provedor Anthropic
- **Pacote novo:** `internal/agente/anthropic`, implementando `agente.Provider` com o SDK oficial `github.com/anthropics/anthropic-sdk-go`. Nomes e tipos saem da documentação do SDK, não de memória.
- **Mapeamento:** `Mensagem`, `ChamadaFerramenta` e `ResultadoFerramenta` viram blocos `text`, `tool_use` e `tool_result`. Imagem vira bloco `image`.
- **`GerarStream` faz streaming de verdade:** chama `emitir(delta)` a cada pedaço de texto e devolve a `Resposta` completa (texto mais chamadas) no fim.
- **Pensamento por requisição.** `Requisicao` ganha um campo de modo (voz ou texto): voz manda `thinking: {type: "disabled"}`, texto manda pensamento adaptativo com `output_config.effort: "low"`. O bloco de pensamento não entra no `Texto`.
- **`max_tokens` folgado:** 4096. O spike mostrou que 300 corta a resposta quando há pensamento.
- **Prompt caching** no sistema e nas ferramentas, por `cache_control`. O ganho real (custo e tempo) é medido no plano; se o prefixo for menor que o mínimo cacheável do modelo, não cacheia, e isso é aceitável.
- **Recusa** (`stop_reason == "refusal"`) vira erro tratado, com mensagem ao usuário. Não há fallback no servidor para o Haiku.
- **Config:** `[agente] provedor = "anthropic" | "gemini"`, mais `modelo`. A chave vem de `CLAUDE_API_KEY` no `.env`, nunca do toml. Sem chave com provedor `anthropic`, o jamesd não sobe e diz por quê.

### 5.2 Loop com streaming
- `Loop` ganha um caminho de streaming: um callback de deltas, nil por padrão, que mantém o comportamento atual.
- Texto gerado **antes** de uma chamada de ferramenta também é emitido. "Vou abrir o Firefox." falado antes de a ferramenta rodar é desejável.

### 5.3 Frases para o device
- **Quebrador de frases** (função pura, testada): acumula deltas e libera uma frase em `. ! ?` seguido de espaço ou fim, em quebra de linha, ou ao passar de **N caracteres sem pontuação** (corta no último espaço, para não travar a voz numa frase sem ponto). O restante sai no fim do turno.
- **Limpeza para fala** (função pura, testada): tira Markdown (`**`, `*`, `_`, `` ` ``, `#`, marcadores de lista, links `[texto](url)` → `texto`) e blocos de código (viram "veja o código na tela"). URLs viram "o link está na tela".
- **Eventos novos, jamesd → device**, na conexão do device que mandou `entrada_voz`:
  - `fala_frase` `{texto, indice}`
  - `fala_fim` `{}`
  - `fala_erro` `{mensagem}`: o device fala uma frase curta de erro.
- **Turno de voz não chama `gerarAudioDaResposta`** (D8). Os eventos para o SPA (`transcricao`, `ferramenta_*`, `resposta_final`) seguem iguais.

### 5.4 Voz do turno de texto (Piper)
- O `voz.Sintetizador` ganha um executor **Piper**: faz POST do texto já limpo de Markdown (mesma função da §5.3) ao servidor de TTS local (`JAMES_TTS_URL`, padrão `http://127.0.0.1:8180`) e grava o WAV devolvido.
- **O arquivo passa a ser `.wav`.** `servirAudio` serve com `Content-Type: audio/wav`, e o descarte depois de tocar (`DELETE`) continua igual.
- **Config:** `[voz] motor = "piper" | "edge"`, padrão `piper`. O executor edge-tts atual fica, testado, como alternativa.
- **Servidor de TTS fora do ar:** o turno segue sem áudio, com log de erro. É o comportamento de hoje quando o edge-tts falha ("melhor esforço", `ws.go:368`).

---

## 6. Device (Python)

### 6.1 Fim de fala (`escuta.py`)
`_coletar_janela` por tempo dá lugar a uma máquina de estados injetável e testável sem microfone:

1. Depois da wake word, espera voz. Se **nenhuma voz em 5 s**, desiste e volta a dormir, com log.
2. Com voz, coleta. No **primeiro bloco de silêncio**, dispara o STT especulativo (D7) com o áudio até ali.
3. Voz volta antes de W: descarta a especulação e continua.
4. **W de silêncio contínuo:** se a especulação terminou numa palavra de ligação, estica para W longo; senão, fecha.
5. **Teto de duração** (§6.3) fecha de qualquer jeito. **"enviar"** continua fechando na hora.
6. **Continuação (D13):** quando a fala do James termina, entra num estado em que voz detectada pelo VAD abre a coleta do passo 2 **sem** wake word. Sem voz em `JAMES_CONTINUACAO_S`, volta ao estado de wake word. A continuação também passa pelo half-duplex: só começa depois do último áudio tocado.

Parâmetros por env, com padrões:

| Variável | Padrão |
|---|---|
| `JAMES_FIM_SILENCIO_MS` | 700 |
| `JAMES_FIM_SILENCIO_LONGO_MS` | 1500 |
| `JAMES_COMANDO_MAX_S` | 10 (§6.3) |
| `JAMES_ESPERA_VOZ_S` | 5 |
| `JAMES_CONTINUACAO_S` | 6 (0 desliga) |

O **VAD** é o Silero via onnxruntime, que já é dependência do Piper. O pacote exato é escolhido no plano, com uma condição dura: **sem PyTorch**.

### 6.2 STT
- `criar_transcritor()` passa a escolher pelo `JAMES_STT`:
  - `whisper` (padrão): POST do WAV ao `whisper-server` em `JAMES_WHISPER_URL` (padrão `http://127.0.0.1:8179`).
  - `google`: o atual.
- O contrato `Callable[[bytes], str]` não muda.
- Falha de rede, servidor fora ou erro devolvem string vazia com log, e caem em D10.

### 6.3 Teto de duração e contexto do Whisper
- `-ac 512` cobre cerca de **10 s** de áudio. Comando mais longo seria cortado.
- **Teto: 10 s com `-ac 512`** (decisão do Arthur, 2026-10-11). Passou de 10 s, a coleta fecha e o que foi dito até ali vai para o STT.
- O teto e o `-ac` andam juntos na config; um teste garante que o teto nunca passa da cobertura do `-ac`.

### 6.4 Fala
- **Módulo `fala.py`:** carrega o Piper uma vez no boot (0,7 s), mantém uma fila de frases, sintetiza e toca pelo PyAudio na ordem de `indice`.
- **Half-duplex (D9):** durante a fala, a escuta descarta o áudio do microfone, mas continua lendo o stream. Ao terminar, zera o Vosk e volta a ouvir a wake word.
- **Frases fixas faladas localmente:** "Não entendi, pode repetir?" (D10) e "Não consegui falar com o servidor." (jamesd fora).
- **Som de entrada** (`entrada.mp3`) fica. O `faaah.wav` de "fechei a janela" fica, mas o plano mede se ele atrasa o primeiro som; se atrasar, sai.
- **Voz:** `JAMES_PIPER_VOZ` (padrão `vozes/pt_BR-faber-medium.onnx`, fora do git, como o `vosk/`).

### 6.5 Servidor de TTS (para o turno de texto)
- **Módulo `servidor_tts.py`** no pacote do device: mesmo venv e mesma dependência do Piper. Roda como processo próprio: `python -m dispositivo.james_device.servidor_tts`.
- Só escuta em **loopback** (`127.0.0.1:8180`), carrega a voz uma vez e expõe `POST /sintetizar` (texto → WAV) e `GET /saude`.
- Reaproveita o mesmo código de síntese do `fala.py`: uma função, duas pontas.
- **Por que um processo à parte**, em vez de o jamesd pedir ao device: o turno de texto pode vir do celular pela VPN, e o áudio tem de tocar **onde a pessoa digitou**, não no alto-falante do PC. O jamesd gera o arquivo e o app toca, como já acontece hoje.

### 6.6 Instrumentação
- O device registra, por turno, em monotônico: fim da fala, STT pronto, `entrada_voz` enviada, primeira `fala_frase` recebida e **primeiro áudio tocado**.
- Linha de log por turno: `latencia turno=<n> fim_fala→1º_som=1.42s stt=0.18 jamesd=0.91 tts=0.03`.
- O jamesd loga o tempo até o primeiro delta e o total do turno.
- Os tempos vão para um arquivo local (`.executar/latencia.tsv`). **Sem texto do comando**, só números: é a voz do Arthur, e o tempo basta.

---

## 7. Ambiente
- **`whisper-server`:** binário do whisper.cpp compilado com Vulkan num caminho estável (`JAMES_WHISPER_SERVIDOR`) e modelo em `JAMES_WHISPER_MODELO`, ambos fora do git.
  - O build atual do Arthur vive em `~/Downloads/stt-teste`; o plano move para um lugar estável e documenta como recompilar.
- **`executar.sh`:** sobe o `whisper-server` e o servidor de TTS antes do device e do jamesd, espera os dois responderem e derruba tudo no fim, como já faz com o jamesd.
  - Se o `whisper-server` não subir, o device sobe com `JAMES_STT=google` e um aviso. O James nunca fica surdo por causa do STT.
  - Se o servidor de TTS não subir, o turno de texto fica sem áudio, com aviso. O turno de voz não depende dele.
- **Dependências do device:** `piper-tts`, o pacote do VAD e `requests`. O Piper traz o onnxruntime.
- **O edge-tts deixa de ser necessário** no caminho padrão. O `pipx install edge-tts` só importa com `[voz] motor = "edge"`.

---

## 8. Critérios de aceite

| # | Critério | Prova |
|---|---|---|
| CA1 | Suítes verdes: `go vet ./...`, `GOOS=windows go vet ./...`, `go test ./...`, pytest do device, vitest do front | saída dos comandos |
| CA2 | Provedor Anthropic: streaming emite deltas na ordem; `tool_use` vira `ChamadaFerramenta`; recusa vira erro tratado | testes de unidade com servidor HTTP falso |
| CA3 | **Ferramenta com pensamento desligado:** chamada real ao Haiku 5.5 com as ferramentas do James pedindo algo que exige ferramenta. Sai `tool_use`, não texto imitando chamada, em 10 de 10 | teste de integração opcional (`JAMES_TESTE_ANTHROPIC=1`), saída colada |
| CA4 | Quebrador de frases e limpeza de Markdown com casos de tabela, incluindo `**Canberra**`, listas, código e URL | `go test` |
| CA5 | Fim de fala: fecha em W após voz; estica com palavra de ligação; descarta especulação quando a voz volta; respeita teto e "enviar"; desiste sem voz em 5 s | pytest com fonte de áudio e relógio falsos |
| CA6 | **Latência em uso real:** em ≥ 20 turnos falados pelo Arthur, mediana de fim da fala → 1º som **≤ 2,0 s** e p95 **≤ 3,0 s** | `.executar/latencia.tsv` |
| CA7 | **Precisão:** no corpus de 30 comandos do Arthur, o WER do Whisper (com a config desta spec) não é pior que o do Google | tabela do benchmark (depende do corpus gravado) |
| CA8 | **Voz única:** turno de voz não gera `audio_pronto` (sem voz dupla); turno de texto gera um `audio_pronto` com WAV do Piper, servido como `audio/wav` | testes em `internal/api` e `internal/voz` |
| CA9 | Half-duplex: a fala do James não dispara a wake word | sessão ao vivo, log |
| CA10 | "Não entendi, pode repetir?" com STT vazio ou fora do ar; `JAMES_STT=google` ainda funciona | pytest e sessão ao vivo |
| CA11 | Custo: o jamesd loga os tokens de cada turno; em uma semana de uso, a projeção mensal fica abaixo de R$ 20 | log de uso |
| CA12 | **Continuação:** depois da fala do James, voz em até 6 s vira comando sem wake word; passados 6 s sem voz, só a wake word acorda; `JAMES_CONTINUACAO_S=0` desliga | pytest com fonte e relógio falsos + sessão ao vivo |
| CA13 | **Mesma voz no texto:** um turno digitado no app toca a resposta com a voz do Piper; o áudio fica pronto em ≤ 0,5 s depois da resposta completa | sessão ao vivo + log do jamesd |

---

## 9. Riscos
- **Ferramentas sem pensamento** (CA3). A documentação registra, em outro modelo, chamada de ferramenta escrita como texto com o pensamento desligado. Se o Haiku 5.5 fizer isso, a saída é `effort: "low"` com pensamento adaptativo no turno de voz. Medido no spike: 1,12 s até a 1ª frase com prompt curto e 1,79 s com o prompt real, ainda dentro de 2 s com folga pequena.
- **Áudio real ≠ sintético.** A voz do Arthur, a distância e o ruído podem piorar o STT e o VAD. O CA6 e o CA7 existem para isso.
- **TV ligada:** o VAD vê voz humana e não fecha. O teto de duração garante o fim; a calibração fica para depois, com dados.
- **Variação do STT:** 1,07 s num spike, 0,17 s nos outros, causa desconhecida. A instrumentação (§6.6) mostra se voltar.
- **Prefixo pequeno demais para cache:** só custo, sem efeito funcional.
- **Continuação disparando sozinha** (D13): TV ou conversa paralela nos 6 s depois da fala do James viram comando. Mitigação: `JAMES_CONTINUACAO_S=0`; os dados de uso dizem se é preciso calibrar.
- **Voz do Piper menos natural** que a Andrew: aceita pelo Arthur ("a gente se acostuma").
- **Mais um processo** (servidor de TTS): se cair, o texto fica mudo. O `executar.sh` avisa, e o turno de voz não é afetado.

---

## 10. Decisões do Arthur (2026-10-11)

| Pergunta | Decisão | Onde entrou |
|---|---|---|
| Continuação sem wake word? | **Sim**, "vamos ver como fica". Ligada por padrão, com desliga por env. | D13, §6.1 passo 6, CA12 |
| Teto do comando | **10 s** (`-ac 512`, já medido) | §6.3 |
| Voz do turno de texto | **Igual à da voz, usando a mais rápida: Piper nos dois.** O edge-tts sai do padrão. | D8, §5.4, §6.5, CA8, CA13 |
