# Handoff — Fase 4 fechada, começando a Fase 5

Escrito em 2026-08-07, no fim da execução da Fase 4 (já mergeada em `main`)
e antes de qualquer trabalho da Fase 5 começar. Uma nova conversa deve ler
este arquivo inteiro antes de fazer qualquer coisa.

## Onde está tudo

Não tem worktree ativo — Fase 4 rodou direto em `main`, mesmo padrão da
Fase 3. Por padrão a Fase 5 também roda em `main`, mas **isso é uma decisão
a confirmar com o Arthur no brainstorm**, não presumir (as Fases 1 e 3/4 já
usaram os dois padrões neste projeto: worktree isolado vs. `main` direto).

- **Handoffs anteriores** (contexto histórico, ainda válidos):
  `docs/superpowers/HANDOFF.md` (Fase 0),
  `docs/superpowers/HANDOFF-fase-1-execucao.md` (execução da Fase 1),
  `docs/superpowers/HANDOFF-inicio-fase-3.md` (início da Fase 3),
  `docs/superpowers/HANDOFF-fase-3-execucao.md` (fechamento da Fase 3),
  `docs/superpowers/HANDOFF-inicio-fase-4.md` (início da Fase 4).
- **Specs**: `docs/superpowers/specs/2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`
  (spec original — tem o Anexo A com os bugs mapeados por fase, citado
  abaixo) e `docs/superpowers/specs/2026-08-06-james-fase-4-memoria-duravel-design.md`
  (spec da Fase 4, inclusive a seção 7 "Implantação em nuvem", relevante se
  a Fase 5 tocar em rede/dispositivo remoto).
- **Plano da Fase 4**: `docs/superpowers/plans/2026-08-06-james-fase-4-memoria-duravel.md`
  (13 tasks, todas fechadas, com subagent-driven-development e dois estágios
  de revisão por task).

## Estado do projeto: Fases 0, 1, 3 e 4 completas

| Fase | Conteúdo | Status |
|---|---|---|
| 0 | Fundação: segredos, git, testes, Docker | ✅ fechada |
| 1 | Núcleo: back-end como serviço | ✅ fechada, mergeada |
| ~~2~~ | *vaga — absorvida pela 1 (Postgres) e pela 4 (pgvector, depois adiado)* | não existe |
| 3 | Qualidade das ferramentas | ✅ fechada, mergeada |
| 4 | Memória durável (Obsidian/markdown, sem pgvector) | ✅ fechada, mergeada, verificada ao vivo |
| **5** | **Máquina física: controle de tela e voz** | 🔶 próxima |
| 6 | Front-end novo | pendente |

**O que existe hoje:**
- `cmd/jamesd` — back-end Go de serviço: REST + WebSocket, Postgres, loop do
  agente (Gemini), 5 ferramentas (`executar_comando_sistema`,
  `perguntar_ao_claude_code`, `pesquisar_internet`, `manipular_interface`,
  `salvar_na_memoria` — a última chegou na Fase 4).
- `internal/memoria/` — `Janela` (recência da conversa) e `Vault` (memória
  durável em markdown, `Vault/Memoria/*.md`, escrita só pela ferramenta
  `salvar_na_memoria`, lida a cada turno e injetada no system prompt).
  Fechado e verificado ao vivo nesta sessão anterior — não é escopo da
  Fase 5, só contexto.
- `internal/dispositivo/hub.go` — ponte Go↔Python já testada e correta:
  WS com correlação por id, timeout, erro dedicado sem device conectado
  (`ErroDispositivoAusente`). **Isso não parece ser problema da Fase 5** —
  verificado nesta sessão, o hub em si está sólido.
- `dispositivo/james_device/` — cliente Python: `main.py`, `conexao.py`,
  `tela.py` (executa clique/tecla via `pyautogui`), `wake.py` (detecção de
  wake word).
- `internal/ferramentas/dispositivo.go` — ferramenta `manipular_interface`,
  `NivelPerigoso`, ainda recebe X/Y crus do modelo.

## O que a spec original já disse sobre a Fase 5 — ponto de partida real

Do Anexo A da spec de 03/08 (`docs/superpowers/specs/2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md:645-680`),
os itens marcados pra Fase 5:

| # | Problema | Local original | Status verificado nesta sessão |
|---|---|---|---|
| 19 | Screenshot só é enviado se o texto contiver a palavra "tela" | `frontend/src/App.tsx:59` (app antigo), anotado "1 move / 5 conserta" | **Ver seção "Achado desta sessão" abaixo — o mecanismo pode nem existir mais** |
| 20 | Coordenadas X/Y adivinhadas pelo modelo; 1920x1080 fixo; PyAutoGUI sem DPI awareness | `app.go:156`, `app.go:323` (app antigo) | **Confirmado ainda verdadeiro**: `dispositivo/james_device/tela.py` de hoje é `pyautogui.moveTo(x, y)` cru, comentário no topo do arquivo confirma "coordenadas continuam adivinhadas pelo modelo (Anexo A #20), consertar é Fase 5" |

Da tabela de decisões da mesma spec (`:60`):

> **D5** | Destino do Python | Vira agente de dispositivo | Bibliotecas de UI
> Automation em Python são muito superiores às de Go; **é o caminho para
> consertar o clique na Fase 5**

Isso é uma direção técnica que a spec original já apontou, não uma decisão
tomada — vale trazer pro brainstorm como opção (UI Automation por
acessibilidade/elemento, em vez de coordenada de pixel adivinhada), mas sem
presumir que é a escolha final.

Também da spec original (`:37-40` e ecoado em `docs/superpowers/specs/2026-08-04-james-fase-1-nucleo-design.md:464`):

> A calibração [da wake word] com gravações reais do Arthur fica pra Fase 5.

**Confirmado nesta sessão:** `dispositivo/james_device/wake.py` de hoje tem
uma lista de 5 variantes (`james`, `jeimes`, `jaimes`, `jeims`, `geimes`),
comentário no arquivo confirma que é a lista "deliberadamente conservadora"
portada na Fase 0/1, e que calibrar com gravações reais é trabalho da
Fase 5, ainda não feito.

## Achado desta sessão: screenshot pode não existir em lugar nenhum hoje

Verificado por busca, não suposição: `grep` por `screenshot|imagem|print`
em `dispositivo/james_device/*.py` (main.py, tela.py, conexao.py) não
encontra nada — nenhum código Python de hoje captura tela.

Do lado Go, o **encanamento existe e está correto**: `agente.Mensagem.Imagem`
é um campo real, `internal/memoria/janela.go:30` conta o custo dele no
orçamento de token (com teste — `TestMontarContaImagemComoCusto`), e
`internal/agente/gemini/mapeamento.go:16-17` monta o `genai.Part` com
`InlineData` quando `m.Imagem` não é vazio. Ou seja: se um `[]byte` de PNG
chegasse em `Mensagem.Imagem`, o resto do pipeline já sabe o que fazer com
ele.

**O que falta, e não foi verificado a fundo nesta sessão:** nenhum caminho
encontrado que realmente populam `Mensagem.Imagem` a partir de uma captura
de tela real — nem no WS handler (`internal/api/ws.go`), nem em lugar
nenhum do device Python. Item 2 do `prompts/sistema.md` ("VISÃO PROATIVA:
se houver um print da tela, analise antes de agir") pode estar orientando o
modelo pra um recurso que não é acionado por nada hoje. **Confirmar isso a
fundo é o primeiro passo do brainstorm da Fase 5**, antes de desenhar
qualquer coisa — se for verdade, é bug adicional pra registrar (não estava
no Anexo A original com esse enquadramento), e muda o que "Fase 5" precisa
cobrir.

## Objetivo final do Arthur — fora de escopo até o fim de TODAS as fases

Em 2026-08-06 o Arthur declarou explicitamente que o objetivo final é
instalar o James como **aplicativo nativo no celular**, e que isso só entra
em pauta **depois que as Fases 5 e 6 fecharem**. Também disse, na mesma
sessão, que pretende **implantar em nuvem**, com app desktop e app de
celular acessando o mesmo `jamesd`, restrito por IP/rede a essas duas
máquinas. Não antecipar essa conversa, não sugerir trabalho nela — mas
manter em mente que a Fase 5 mexe em "máquina física" (device local), o que
pode ter tensão real com esse objetivo de nuvem declarado; vale nomear essa
tensão no brainstorm em vez de ignorá-la, sem resolvê-la agora.

## Item aberto herdado (não bloqueante, mas não tratar como resolvido)

O bug da aprovação remota (celular pedindo ação perigosa, desktop aprovando)
foi corrigido com teste na Fase 3, mas o **fluxo de ponta a ponta pelo
celular real nunca foi revalidado ao vivo** depois das correções. Detalhe em
`docs/superpowers/HANDOFF-fase-3-execucao.md` e no spike
`docs/superpowers/spikes/2026-08-06-bug-aprovacao-remota.md`.

## Workflow a seguir (mesmo das Fases 0, 1, 3 e 4)

1. **`brainstorming`** primeiro — explorar com o Arthur o escopo exato da
   Fase 5. Pontos que precisam de decisão dele, não presumidos:
   - O achado do screenshot acima: confirmar se o mecanismo de visão existe
     de verdade em algum lugar não encontrado nesta busca, ou se é lacuna
     real a fechar.
   - Direção técnica pro clique: seguir a sugestão original da spec (UI
     Automation por elemento/acessibilidade) ou outra abordagem (captura de
     tela + coordenada corrigida por DPI, sem trocar o mecanismo de clique)?
   - Calibração da wake word: como o Arthur pretende fornecer "gravações
     reais" — processo manual, sessão de calibração guiada, outro jeito?
   - Worktree isolado ou direto em `main`?
2. **`writing-plans`** — depois do brainstorm aprovado, plano task-por-task
   no mesmo formato das fases anteriores.
3. **`subagent-driven-development`** — implementador → revisor de spec →
   revisor de qualidade por task. Nas Fases 1, 3 e 4 esse processo achou
   bugs reais que review de código sozinho não pegaria (na Fase 4, entre
   outros: um ciclo de import genuíno entre `ferramentas`↔`memoria`↔`agente`,
   e um teste de concorrência que passava 100% das vezes mesmo com o mutex
   removido de propósito). Não pular, mesmo sob pressa.

## Ambiente confirmado nesta sessão (2026-08-06/07, `main`)

- Postgres real: container `james-postgres`, `127.0.0.1:5433`, saudável.
- `.env` com `GEMINI_API_KEY`, `TAVILY_API_KEY`, `POSTGRES_PASSWORD`,
  `DATABASE_URL` — todas presentes, testado ao vivo na Fase 4 (chamada real
  ao Gemini, tool `salvar_na_memoria` disparada de verdade).
- `web/dist/` com build da SPA presente — `go run ./cmd/jamesd` sobe sem
  erro de embed.
- `.gitignore` tem um gap conhecido, não bloqueante: cobre só
  `audios/audio_jarvis.mp3`, não a pasta inteira — arquivos `.mp3` gerados
  pelo TTS em teste ao vivo aparecem como untracked. Registrado, não
  corrigido (decisão do Arthur: "deixa passar").

## Regra que vale pra qualquer sessão futura neste projeto

CLAUDE.md do Arthur é claro: nunca declarar algo funcionando/pronto sem ter
rodado o comando e lido a saída **nesta mensagem**. Isso valeu as quatro
fases anteriores inteiras — a Fase 4 em particular só fechou depois de uma
verificação ao vivo real contra o `jamesd` rodando de verdade (WebSocket,
duas conexões separadas, Gemini real, nota gravada em disco, tudo
conferido, não simulado). A Fase 5 mexe em hardware/dispositivo físico
(mouse, teclado, microfone) — verificação ao vivo aqui é ainda mais
essencial que nas fases anteriores, porque é exatamente a categoria de bug
("funciona no código, falha na tela real") que motivou a fase inteira
existir (Anexo A #20, evidência de 10/11 respostas fallback em
`Vault/Logs_Diarios/2026-07-08.md`, citada na spec original).
