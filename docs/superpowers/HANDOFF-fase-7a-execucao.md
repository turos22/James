# Handoff — Fase 7A fechada (o James vira aplicativo)

Escrito em 2026-08-14, ao fim da verificação ao vivo. Uma nova conversa deve
ler este arquivo inteiro antes de fazer qualquer coisa.

## Onde está tudo

- **Spec**: `docs/superpowers/specs/2026-08-11-james-fase-7a-aplicativo-design.md`
  — 16 decisões (D1-D16), 12 riscos (R1-R12), e a seção **12.1** com o
  resultado da verificação ao vivo critério por critério, inclusive o que
  não foi exercitado
- **Plano**: `docs/superpowers/plans/2026-08-11-james-fase-7a-aplicativo.md`
  — 21 tasks, todas executadas
- **Handoff da fase anterior**: `docs/superpowers/HANDOFF-fase-6-execucao.md`
- **Referência de design**: `docs/superpowers/referencias/painel-de-controle-james/`
  (cópia do v0.dev do Arthur + `LEIA-ME.md` com os desvios deliberados)

## Estado: 58 commits em `main`, de `0a7be70` a `078aa7b`

Executado com `subagent-driven-development`: implementador → revisor de
conformidade → revisor de qualidade por task, direto em `main` (autorizado
explicitamente pelo Arthur, "Direto em main, como sempre").

**O que existe agora, que não existia:**

- A bolinha flutuante morreu. `shell/main.go` monta uma janela Wails de
  verdade (1100×760, redimensionável, com os três círculos como controles
  reais), não mais um widget 100×100 sempre-no-topo.
- `web/src/shell/aplicativo.tsx` + `barraLateral.tsx` — o app vira cinco
  abas (James, Mensagens, Memória, Histórico, DashBoard) com sidebar
  colapsável e contador de aprovação pendente compondo o nome acessível do
  botão.
- `web/src/shell/orbe.tsx` — canvas reativo à saída de áudio real via
  `AnalyserNode`, sem botão de tocar. `web/src/lib/audio.ts` fecha a cadeia
  `<audio>` → `MediaElementAudioSourceNode` → `AnalyserNode` →
  `ctx.destination`.
- `web/src/abas/james.tsx` — aprovação, atividade da ferramenta (nome,
  estado, duração) e tela de `ver_tela` convergem num único painel, com
  precedência `aprovação > ferramenta > tela > repouso`.
- `web/src/abas/historico.tsx` — paginação por cursor (`ListarPaginado`,
  keyset em `atualizada_em`), agrupamento por dia, busca no servidor
  debounced.
- `web/src/lib/sessao.ts` + `internal/api/ws.go` (`tratarNovaSessao`) — ciclo
  de vida de sessão: fechar a janela e reabrir começa sessão nova; F5 e
  queda de rede retomam a mesma; `nova_sessao` zera cliente **e servidor**.
- `internal/memoria/resumo.go` — `Resumidor`, resumo rolante da sessão,
  ligado a `processarTurno` quando a janela transborda. Ancorado por
  `mensagens.id` (Postgres, monotônico) via `IDDeCorte`/`ListarNaoResumidas`,
  não por contagem em memória — o histórico retomado é só a cauda de 200
  linhas, e contar sobre ele resumiria o trecho errado, em silêncio.
- `internal/armazenamento/migracoes/0002_resumo_sessao.sql` (colunas
  `resumo`, `resumido_ate`) e `0003_indice_mensagens_id.sql` (índice
  `sessao_id, id`, sem o qual `IDDeCorte` varria a sessão inteira).
- `web/e2e/james.spec.ts` + `web/playwright.config.ts` — ponta a ponta de
  verdade contra o `jamesd` real, prometido na Fase 6 e nunca entregue até
  agora.

Front: 63 arquivos alterados, +2944/-952 linhas (líquido: crescimento menor
que parece, porque `trilha.tsx`, `visualizador.tsx`, `abas/conversa.tsx`,
`abas/atividade.tsx`, `abas/tela.tsx` e `abas/sessoes.tsx` morreram). Go:
suíte inteira verde contra Postgres real. 106 testes de vitest, 5 de
Playwright.

## O que a verificação ao vivo provou

Detalhe por critério está em 12.1 da spec. Feita com `jamesd` real (build
`npm run build --prefix web` + `go run ./cmd/jamesd`), Postgres real no
Docker, chamadas reais ao Gemini e TTS real — sem device Python nem shell
Wails aberto (ver limitações abaixo).

**Passam, exercitados ao vivo:**

- **CA4** — pedi a exclusão real de um arquivo de teste via
  `executar_comando_sistema`. O card de aprovação apareceu na aba James, a
  sidebar mostrou "James, 1 aprovação pendente", aprovei, e o arquivo foi
  apagado do disco de verdade. O contador voltou a zero depois.
- **CA5** — a mesma chamada ficou registrada na trilha de atividade com
  nome (`executar_comando_sistema`), estado (✓) e duração real (`20,8 s`,
  incluindo o tempo que fiquei parado decidindo aprovar).
- **CA6** — matei o processo do `jamesd` com a aba Mensagens aberta: o
  status trocou de `online` para `Reconectando… (tentativa 2)` de verdade,
  religuei o processo e ele voltou a `online` sozinho, sem perder a
  conversa.
- **CA8**, as três pernas: **F5** (recarreguei a página inteira — a mesma
  conversa continuou ali); **queda de rede** (é o mesmo caminho do CA6,
  confirmado); **janela nova** (abri uma aba com `sessionStorage` vazio,
  mandei mensagem, e o Postgres registrou um `sessao_id` novo,
  `9b210e43-...`, diferente da sessão anterior).
- **CA9** — cliquei Nova sessão, mandei mensagem, e o Postgres mostrou um
  `sessao_id` novo (`9e38055b-...`) — verificado com
  `SELECT id, titulo, criada_em FROM sessoes ORDER BY criada_em DESC`, não
  só olhando a tela.
- **CA12** — com histórico real crescendo (chegou a >20 sessões durante a
  própria verificação), `document.querySelectorAll('main ul li button').length`
  deu **50**, não a lista inteira — confirma a paginação (`POR_PAGINA = 50`).
  Agrupamento por "Hoje" apareceu. Busquei por "CA9" e só a sessão certa
  ficou na lista — busca no servidor funcionando, não filtro no cliente.
- **CA13** — Memória listou a nota real do vault ("carro" — Fusca 1971
  azul). DashBoard mostrou o texto sobre a Fase 7B, sem gráfico nem número
  inventado.
- **CA14** — as três suítes, coladas de verdade:
  ```
  go vet ./...  → limpo
  go test ./... → todos os pacotes ok, contra Postgres real
  npm test --prefix web → Test Files 17 passed (17) | Tests 106 passed (106)
  npm run e2e --prefix web → 5 passed (10.0s)
  ```

**Passam, mas só parcialmente exercitados:**

- **CA1** — confirmado o shell abrindo direto (sem bolinha), as cinco abas,
  e o recolher/expandir do menu (label muda de "Recolher menu" para
  "Expandir menu"). **Não** subi o shell Wails de verdade nesta rodada —
  a composição do app inteiro fora do Wails já é a maior parte do
  critério, mas a parte "roda dentro do executável" não foi reexercitada
  agora (foi na Task 7, aprovada).
- **CA3** — confirmei a metade que dá pra confirmar sem olhar a tela: o
  áudio real tocou (`GET /api/audio/... → 206 Partial Content`, TTS de
  verdade, não mock), o que prova que a cadeia `<audio>` →
  `MediaElementAudioSourceNode` → `AnalyserNode` disparou de ponta a ponta
  contra o Gemini e o sintetizador reais. **Não confirmei visualmente a
  deformação do orbe** — o pane de screenshot não compôs frames neste
  ambiente ("Browser pane is not displayed"). A matemática de
  `energiaDaFaixa`/`raioDoOrbe` foi verificada por mim, manualmente, antes
  do commit da Task 8, e revisada duas vezes — mas isso é garantia de
  unidade, não de "eu vi a bolha respirar".

**Cobertos só por teste automatizado, não exercitados ao vivo nesta rodada:**

- **CA10** — a suíte de integração da Task 18
  (`TestTurnoQueTransbordaGravaOResumo`) prova o mecanismo contra Postgres
  real, mas com orçamento artificialmente pequeno (40 tokens) pra
  transbordar rápido. Não gerei uma conversa real de milhares de tokens
  contra o orçamento de produção (8000) — custaria dezenas de chamadas
  reais ao Gemini só para esta verificação, e o mecanismo já está provado
  contra o banco de verdade.
- **CA11** — mesma suíte, `TestFalhaDoResumidorNaoDerrubaOTurno`. Não
  provoquei uma falha real do resumidor ao vivo (não há como forçar o
  Gemini a falhar sob demanda sem mockar), mas nada quebrou durante os
  vários turnos reais desta verificação, o que é consistente.
- **CA7** — auto-scroll condicional já tem teste de unidade da Task 4.
  Exercitar isso ao vivo exige um evento concorrente chegando enquanto o
  usuário está rolado pra cima — não montei esse cenário nesta rodada
  (mesma lacuna que a Fase 6 deixou aberta para seu próprio CA7, por razão
  parecida).

**Não verificados nesta rodada — limitação do ambiente, não do produto:**

- **CA2** — confirmei a metade que dá pra confirmar no navegador: os três
  círculos **não aparecem** fora do shell Wails
  (`document.querySelectorAll('[aria-label*="Minimizar"], ...')` deu 0).
  **Não testei os círculos de verdade** — minimizar, maximizar, fechar,
  arrastar a borda para redimensionar, confirmar que a janela não fica
  sempre no topo. Isso exige o `shell.exe` (ou `wails dev`) rodando como
  janela real do Windows, interação de mouse no chrome do SO, e nenhuma
  ferramenta deste ambiente alcança isso — só o Browser pane, que é
  page-only. As três rotas HTTP (`/minimizar`, `/maximizar`, `/fechar`)
  foram testadas por unidade na Task 6 e aprovadas por dois revisores; o
  que falta é a mão do Arthur clicando de verdade.
- Nada que dependa do device Python (`dispositivo/james_device`) foi
  exercitado nesta verificação — não subi o agente. Não é necessário para
  nenhum CA desta fase (CA1-CA14 não citam `ver_tela`/`manipular_interface`
  diretamente), mas registro que ficou de fora.

## Achados abertos, não corrigidos

1. **A deformação visual do orbe nunca foi vista por um humano ou por
   captura de tela nesta verificação.** O caminho de dados está provado
   (áudio real chega, `AnalyserNode` está no lugar certo, a matemática foi
   auditada duas vezes) mas isso é diferente de confirmar que o CSS/canvas
   renderiza a deformação de forma perceptível. Pedido ao Arthur: abrir o
   app e ouvir/ver uma resposta longa.
2. **Os três círculos da janela Wails nunca foram clicados de verdade nesta
   fase, por ninguém, desde a Task 6.** A revisão foi só por HTTP
   (`curl`/teste automatizado) contra as rotas `/minimizar` etc. Pedido ao
   Arthur: abrir o `.exe`, clicar os três, arrastar a borda, e comparar
   com `127.0.0.1:8080` no Chrome pra confirmar que os círculos somem.
3. **CA10 nunca rodou contra o orçamento de produção (8000 tokens).** O
   mecanismo está provado com orçamento artificial; uma conversa real longa
   o suficiente para transbordar 8000 tokens nunca foi tentada. Baixo
   risco (a lógica é a mesma, só muda o número), mas é a lacuna mais barata
   de fechar quando o Arthur tiver uma sessão longa de verdade — vale
   perguntar sobre algo dito no início da conversa depois.
4. **CA7 (auto-scroll condicional) segue sem exercício ao vivo**, mesma
   lacuna que a Fase 6 já tinha deixado para o CA7 daquela fase.
5. **`web/e2e/` não tem verificação de tipos própria.** `web/tsconfig.json`
   restringe `include` a `src/`, e o projeto não tem `@types/node` — sem
   ele, nenhum `tsc` alcançaria `web/e2e/` mesmo que o `include` mudasse
   (os tipos internos do `@playwright/test` referenciam `Buffer`/`fs`).
   Achado pela revisão de qualidade da Task 20; não bloqueante hoje porque
   o `esbuild` do Playwright faz strip-de-tipos sem checar, mas é um buraco
   de cobertura real.
6. **Race entre conexões concorrentes na mesma sessão** (Task 18): duas
   abas ou dois dispositivos dando `retomar` na mesma sessão podem fazer
   `atualizarResumo` (read-modify-write sem lock) retroceder
   `resumido_ate`, causando resumo redundante (não perda de dado). É
   sintoma de uma lacuna arquitetural maior — o app já não trata bem duas
   conexões simultâneas na mesma sessão — não uma regressão desta fase.
7. **Suíte Go continua escrevendo no Postgres de trabalho sem limpar**
   (R7, herdado da Fase 6, ainda aberto). A verificação ao vivo desta fase
   também empilhou sessões de teste reais (visíveis no `SELECT` acima) —
   é o mesmo padrão, não um novo problema.

## Correções de fato em documentos anteriores

Nenhuma encontrada nesta fase. Os handoffs da Fase 5 e 6 não foram
contestados por nada observado durante a Fase 7A.

## Lição de processo, com números

O plano tinha 21 tasks e acumulou **23 correções** durante a execução (25
commits `docs:` no total, descontando a spec e o plano originais). As que
teriam custado mais caro se não pegas antes de produção:

- **Task 17**: o resumidor descartava toda mensagem sem `Texto` —
  exatamente as de `PapelFerramenta` (nunca têm `Texto`, só `Resultados`) e
  as de `PapelModelo` em function-calling puro. Ligado à Task 18, isso
  faria o James "esquecer" sistematicamente qualquer ferramenta que rodou
  assim que a conversa transbordasse — silêncio, sem erro, exatamente a
  classe de bug mais cara deste projeto.
- **Task 16**: faltava o índice `(sessao_id, id)` em `mensagens`. Com uma
  sessão de 100k linhas, `IDDeCorte` levava ~9,8ms sem índice contra
  ~0,045ms com — ~200× de diferença, e é uma consulta que roda a cada
  turno que transborda.
- **Task 20 (achado nº1)**: o teste "envia mensagem e recebe resposta"
  usava `text=/pong/i` sem escopo, que casava com a **própria bolha do
  usuário** (a pergunta continha "pong"). O teste passava em ~200ms — sem
  nunca esperar a resposta real do Gemini. Um teste ponta a ponta que
  "passa" sem exercitar o caminho que deveria provar é pior que não ter
  teste: dá falsa confiança. Corrigido escopando à bolha do assistente.
- **Task 20 (achado nº2)**: essa mesma task, já corrigida e aprovada,
  ainda quebrava `npm test` — o glob padrão do vitest capturava
  `e2e/james.spec.ts` (arquivo do Playwright) e a suíte inteira saía com
  exit code 1, mesmo com os 106 testes de verdade passando. Pego pela
  revisão de qualidade rodando o comando de verdade, não só lendo o diff.
- **Task 14**: um `useEffect` de espelho sobrescrevia
  `sessaoAtualRef.current` com `null` no mount, antes mesmo da conexão WS
  ter chance de ler o valor recém-carregado do `sessionStorage` — o F5
  nunca teria retomado sessão nenhuma, silenciosamente.

**Quase nenhuma dessas apareceu relendo o documento.** Apareceram lendo o
código real que o plano descrevia, rodando os testes de verdade (não
confiando no relato do relato), ou — no caso do achado nº1 da Task 20 —
notando que um teste passou rápido *demais* para ser verdade. A régua deste
projeto ("relato errado é mais caro que código errado") se pagou de novo:
em pelo menos três tasks (17, 18, 20) o revisor discordou do relato do
implementador, e em todas as três o revisor estava certo, confirmado por
verificação independente do orquestrador antes de aceitar.

## Ambiente confirmado (2026-08-14)

- Docker Desktop e `james-postgres` de pé o tempo todo, sem incidente nesta
  fase (diferente de fases anteriores — só uma reinicialização de Docker
  Desktop durante a Task 12, resolvida com kill+restart simples).
- `jamesd` sobe com `go run ./cmd/jamesd`; serve loopback `127.0.0.1:8080`
  e VPN `100.96.219.29:8443`.
- `web/embed.go` continua usando `//go:embed all:dist`: mudou o front?
  `npm run build --prefix web` **e reinicie o `jamesd`**.
- `wails` está instalado (`/c/Users/Arthur/go/bin/wails`), e `shell/shell.exe`
  já existe compilado — mas não foi reaberto nesta verificação (ver
  achados abertos, item 2).
- O device Python (`dispositivo/james_device`) não foi subido nesta
  verificação — nenhum CA desta fase depende dele.
- `web/e2e/james.spec.ts` roda contra o Postgres real e persistente
  (não um container descartável por rodada) — títulos de sessão de teste
  se acumulam para sempre (mesmo padrão do R7).

## O que vem depois

Fases 0 a 7A fechadas. O app deixou de ser uma bolinha: cinco abas, sessão
com ciclo de vida real, resumo rolante ligado ao turno, ponta a ponta
automatizado pela primeira vez.

Antes da Fase 7B (instrumentação e DashBoard de custo), vale:

1. Arthur abrir o `.exe` do Wails uma vez e confirmar os três círculos e o
   orbe visualmente — os dois achados mais baratos de fechar e os únicos
   que exigem olho/mão humana, não ferramenta.
2. Decidir se a race de `atualizarResumo` entre conexões concorrentes
   (achado 6) e a paginação da suíte Go que nunca limpa o banco (achado 7,
   herdado) valem um item de escopo próprio ou ficam como dívida
   registrada.
3. Fase 7C (grafo de memória com wikilinks) segue depois da 7B, na ordem
   que o Arthur já escolheu ("app usável primeiro").
