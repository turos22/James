# Spike: latência de voz, Gemini Live × pipeline otimizado

**Data:** 2026-10-10
**Máquina:** Fedora 44, Ryzen 5 9600X, RX 6650 XT 8 GB (Vulkan, sem matrix cores), rede doméstica.
**Pergunta:** quanto tempo passa do fim da fala até o primeiro som da resposta em cada caminho da [decisão de 2026-10-10](../decisoes/2026-10-10-james-agilidade-e-inteligencia.md).
**Scripts:** [`2026-10-10-latencia-voz/`](2026-10-10-latencia-voz/). Leem a `GEMINI_API_KEY` do `.env`, sem copiá-la.

## Método

- **Áudio de teste:** a frase "Qual é a capital da Austrália e quantas horas de diferença ela tem para Brasília?", **sintética** (edge-tts `pt-BR-AntonioNeural`), com 5,26 s, dos quais 4,40 s têm voz.
  - Sintética de propósito: nenhum dado pessoal vai para o Google, e o áudio é idêntico entre rodadas.
  - **Limite:** áudio limpo e sem ruído não mede precisão de STT, só tempo.
- **Medida:** do fim da última amostra com voz até o primeiro pedaço de áudio da resposta. 5 rodadas por configuração.
- Para regerar o áudio:

```bash
edge-tts --voice pt-BR-AntonioNeural --text "Qual é a capital da Austrália e quantas horas de diferença ela tem para Brasília?" --write-media pergunta.mp3
```

```bash
ffmpeg -i pergunta.mp3 -ar 16000 -ac 1 -f s16le pergunta.raw && ffmpeg -f s16le -ar 16000 -ac 1 -i pergunta.raw pergunta.wav
```

## Spike 1: Gemini Live (`gemini-3.8-live`, google-genai 2.29.0)

- **Como roda:** o áudio vai em tempo real, em blocos de 20 ms, como um microfone, seguido de silêncio contínuo. A detecção de fim de fala é do servidor, então essa espera **está incluída** no número.
- **Resultado:** mediana **1,44 s**, mínima 1,35 s, máxima 1,55 s.
- **Transcrição:** perfeita nas 5 rodadas.
- **Respostas** (transcrição do próprio Live): todas disseram Camberra/Canberra.
  - O fuso **variou e às vezes saiu errado**: "11 horas", "13 horas", "13 ou 14", "11 a 14".
  - O certo é 13 h, ou 14 h no horário de verão australiano.
  - Rapidez não garante acerto factual.
- **Áudio das respostas:** 5,6 a 9,1 s.

## Spike 2: pipeline otimizado

**Substituição declarada:** não há chave da API da Anthropic nesta máquina. O LLM rápido medido foi o **Gemini 2.5 Flash com thinking desligado**, como substituto do Claude Haiku 5.5. O Haiku **não foi medido**.

| Etapa | Configuração | Mediana | Faixa |
|---|---|---|---|
| STT | whisper.cpp `large-v3-turbo-q8_0`, Vulkan, contexto completo | 2,55 s | 2,52–2,59 |
| STT | idem com `-ac 512` (contexto reduzido) | **1,07 s** | 1,00–1,11 |
| LLM | Gemini 2.5 Flash, primeiro pedaço | 0,62 s | 0,58–0,67 |
| LLM | Gemini 2.5 Flash, primeira frase completa | **0,68 s** | 0,63–0,72 |
| TTS | edge-tts Andrew Multilingual, primeiro áudio | **0,76 s** | 0,73–**1,84** |

- As duas configurações de STT transcreveram a frase perfeitamente.
- **Encadeado real** (STT `-ac 512` → LLM até a 1ª frase → TTS até o 1º áudio), com W = 0,7 s de espera de silêncio (parâmetro, não medido):

| Variante | Mediana | Faixa |
|---|---|---|
| STT só depois da espera W | 3,37 s | 3,15–4,76 |
| STT começando no primeiro silêncio, em paralelo a W | **2,67 s** | 2,45–4,06 |

- Numa primeira execução do mesmo script, as medianas foram 4,26 s e 3,56 s. O TTS chegou a **2,5 s** de primeiro áudio.

## Leitura

1. **Live: ~1,4 s e estável. Pipeline: ~2,7 s, com picos de 4 s.** Hoje o Live é cerca de 1,2 s mais rápido e bem mais previsível.
2. **No pipeline, a variância vem do edge-tts** (0,73 a 2,5 s): é um serviço da Microsoft pela rede, sem garantia de serviço. O STT na GPU e o LLM foram estáveis.
3. **O contexto reduzido derruba o STT de 2,55 para 1,07 s** sem errar esta frase. Ainda não se sabe se ele erra em fala real com ruído; isso é trabalho para o corpus.
4. **O pipeline ainda tem ganho possível, só estimado:**
   - transcrever enquanto a pessoa fala, deixando só a cauda para o fim;
   - TTS local, mais rápido e sem a rede, mas com outra voz;
   - conexão do TTS já aberta.
   - Estimativa de piso: max(0,7, STT da cauda) + 0,6 + ~0,2 ≈ 1,5 s. **Não medido.**
5. **Achado lateral:** o Gemini Flash devolveu `**Canberra**`, em Markdown. Um TTS lê isso literalmente, a menos que o texto seja limpo antes.

## O que este spike NÃO responde

- A latência e a qualidade do **Claude Haiku 5.5**: falta uma chave de API.
- STT com **a voz do Arthur**, ruído e distância: falta o corpus.
- O custo real do Live por conversa: os números do plano são estimativa por minuto.
- A latência de um TTS local (Piper ou outro).

## Spike 1b: custo medido do Gemini Live (`spike1b_custo.py`)

- **Montagem:** uma sessão com o `prompts/sistema.md` real do James como instrução de sistema e dois turnos (a pergunta e "E qual é a moeda de lá?").
- **Envio:** silêncio só até a resposta começar; depois o envio para.
- **Fonte dos números:** o `usage_metadata` que o servidor devolve:

| Turno | Texto entrada | Áudio entrada | Áudio saída | Pensamento | US$ (preço oficial) |
|---|---|---|---|---|---|
| 1 | 1278 | 334 | 168 | 244 | 0,0051 |
| 2 | 1308 | **543** | 77 | 49 | 0,0038 |

- **O histórico é cobrado de novo a cada turno:** o áudio de entrada do turno 2 (543) inclui os 334 do turno 1. Sessão longa fica mais cara por turno.
- **O modelo "pensa" por padrão** e cobra por isso. Assumi o preço de saída de texto (US$ 4,50/M) para o pensamento. A US$ 12/M, o turno 1 sai US$ 0,0069.
- **Faltam no número:** a declaração das ferramentas, a memória e o resumo, que o James manda e este teste não mandou. Por isso uso US$ 0,006 por turno como estimativa realista.

**Comparação mensal** (US$ 1 = R$ 5, antes de IOF):

| Perguntas/dia | Live (US$ 0,005/turno) | Live (US$ 0,006) | Pipeline com Haiku 5.5 (US$ 0,001, estimado) |
|---|---|---|---|
| 20 | R$ 15 | R$ 18 | R$ 3 |
| 40 | R$ 30 | R$ 36 | R$ 6 |
| 100 | R$ 75 | R$ 90 | R$ 16 |

O pipeline sai cerca de 5x mais barato por turno. O STT local e o edge-tts não custam nada; o custo é só o LLM. O preço do Haiku é o oficial, mas o volume de tokens é estimado (9k de entrada e 300 de saída por turno), não medido.

## Spike 3: TTS local com Piper (`spike3_piper.py`)

- **Voz:** `pt_BR-faber-medium.onnx` (63,2 MB, `rhasspy/piper-voices` no Hugging Face, sha256 começa com `858555e3a064209c`).
- **Runtime:** `piper-tts` 1.8.0 com onnxruntime na CPU.
- **Carga da voz:** 0,70 s, uma vez só, no boot.
- **Primeiro áudio** (frase inteira, 5 frases × 2): mediana **0,05 s**, faixa 0,025–0,116 s.
- **RTF mediano:** 0,017. Gera 1 s de fala em cerca de 17 ms.
- **Pipeline encadeado real com Piper** (10 rodadas; STT `-ac 512` → Gemini 2.5 Flash até a 1ª frase → Piper 1º áudio):

| Variante | Mediana | Faixa |
|---|---|---|
| STT só depois da espera W | 1,71 s | 1,62–1,81 |
| STT começando no primeiro silêncio | **1,53 s** | **1,44–1,63** |

- **Comparação:** no mesmo método, o Gemini Live deu 1,44 s (1,35–1,55). **O pipeline com Piper empata com o Live na prática** e fica estável.
- **Como se divide o 1,53 s:** espera de silêncio W = 0,70 (parâmetro), LLM ~0,8, Piper ~0,03. O STT (0,17 s) cabe dentro de W.

**Variação sem explicação no STT.** Neste spike, o STT `-ac 512` mediu **0,17 s** (0,16–0,19). Conferi com o texto impresso, via Python e via `curl`, com um e com dois servidores carregados: sempre correto e sempre 0,17 s. No Spike 2, a mesma configuração tinha medido 1,07 s. A hipótese dos dois servidores carregados foi testada e descartada; a causa segue desconhecida. A instrumentação do pipeline real precisa medir isso continuamente.

**O que falta decidir:** a voz. O Piper é ~25x mais rápido no primeiro áudio e não depende da rede, mas a voz é outra. As amostras para comparar de ouvido estão em `.executar/comparar-voz/` (fora do git): `comparar_N_piper.wav` e `comparar_N_edge.mp3`, com as mesmas 5 frases.

**Ainda não medido:** o Claude Haiku 5.5 no lugar do Gemini Flash.
