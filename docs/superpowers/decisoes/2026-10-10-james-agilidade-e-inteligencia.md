# Decisão: James mais ágil e mais esperto (brainstorming)

**Data:** 2026-10-10
**Estado:** brainstorming em andamento. Nada aqui é spec ainda.
**Origem:** pedido do Arthur com quatro melhorias:
1. Fim de fala automático, sem precisar dizer "enviar".
2. Memória constante do perfil dele e dois modelos (um rápido por padrão, um avançado).
3. Resposta falada mais rápida.
4. Câmera.

---

## Decisões do Arthur (2026-10-10)

| Tema | Decisão |
|---|---|
| Latência | **O mais rápido possível, "tempo real ou o mais próximo disso".** Meta inicial citada: 2 a 3 s do fim da fala ao primeiro som. |
| Prioridade do STT | **Velocidade acima de precisão.** Substitui a linha "Prioridade" da tabela A.1 do [plano de análise](../plans/2026-10-09-james-analise-stt-e-fedora.md), que dizia o contrário. Piso recomendado, ainda não vetado: o STT novo não pode errar mais que o Google erra hoje. |
| Custo de API | **R$ 20 a 30 por mês no máximo, e o menor possível.** |
| Qualidade | "Tem que ser algo bom que nem o Claude." |
| Modelo avançado | O Arthur **assina o Claude**. O modelo avançado pode sair pela assinatura, via Claude Code headless (`claude -p`), sem custo de API. |
| Nuvem | Com esse teto, **nada contínuo na nuvem** (câmera em fluxo, por exemplo). Chamadas sob demanda, sim. |

## Fatos levantados (com fonte)

- **Hoje** (lido no código em 2026-10-10):
  - janela de comando fixa de 8 s (`escuta.py`);
  - LLM sem streaming (`gemini.go:48`);
  - edge-tts gera o arquivo inteiro antes de tocar;
  - memória só grava quando o modelo chama `salvar_na_memoria`. O Vault tem 2 notas, com fato contraditório (dois carros) e duplicata (Marvel 3 vezes).
- **Medido:**
  - edge-tts levou 1,9 s numa frase curta;
  - Whisper `large-v3` na CPU levou 5,9 s para 9,3 s de áudio (benchmark do Arthur).
- **Preços oficiais** (USD por milhão de tokens, conferidos em 2026-10-10):
  - Claude Haiku 5.5: $0,10 de entrada e $0,50 de saída;
  - Sonnet 5.5: $2 e $10;
  - Gemini 2.5 Flash: $0,30 e $2,50;
  - Gemini 3.8 Flash: $0,75 e $3,75 até dez/26, depois $1,50 e $7,50;
  - Gemini 3.8 Live: $0,005/min ouvindo e $0,018/min falando.
  - Fontes: [Claude](https://platform.claude.com/docs/en/about-claude/pricing), [Gemini](https://ai.google.dev/gemini-api/docs/pricing).
- **Free tier do Gemini:** o conteúdo "used to improve our products" (página oficial). No pago, não é.
- **`gemini-2.5-flash`:** sem data de desligamento (página oficial de deprecations). Um blog dizia 16/10/2026, e estava errado.

## A bifurcação em aberto

| | A. Pipeline otimizado | B. API de voz em tempo real (Gemini Live) |
|---|---|---|
| Inteligência | Claude Haiku 5.5, mais o Claude avançado pela assinatura | Gemini, com o Claude como ferramenta |
| Custo estimado | ≈ R$ 5–15/mês | ≈ R$ 20–40/mês pago, ou grátis com o Google usando o conteúdo |
| Voz do Arthur | Fica no PC | Vai para o Google |
| Latência | A medir | A medir |

**Próximo passo:** dois spikes que medem a latência do fim da fala ao primeiro som, na máquina do Arthur:
- **Spike 1:** Gemini Live.
- **Spike 2:** Whisper turbo na GPU + LLM com streaming + TTS com streaming.

Resultados em `docs/superpowers/spikes/`. A escolha entre A e B sai dos números.

## Resultado dos spikes (2026-10-10)

Detalhes em [`spikes/2026-10-10-latencia-voz.md`](../spikes/2026-10-10-latencia-voz.md).

- **Gemini Live:** 1,44 s de mediana, estável (1,35–1,55).
- **Pipeline otimizado** (com o Gemini Flash no lugar do Haiku): 2,67 s de mediana, com picos de 4 s causados pelo edge-tts.
- **Ainda não medidos:** o Claude Haiku 5.5 (falta chave de API) e um TTS local.

## Decisão: pipeline (2026-10-10)

O Arthur **fechou com o pipeline (caminho A)**. O Live sai cerca de 5x mais caro por turno (medido no Spike 1b) e estoura o teto com uso moderado. O Arthur aceita 2 s de latência.

- **Spike 3** (Piper local no lugar do edge-tts): o pipeline encadeado mediu **1,53 s** (1,44–1,63), na prática empatado com o Live (1,44 s).
- **Pendente:**
  - a escolha da voz (Piper ou edge-tts), de ouvido, com as amostras em `.executar/comparar-voz/`;
  - medir o Claude Haiku 5.5, que depende de uma chave de API da Anthropic.
