# James — Fase 6: Front-end novo

**Data:** 2026-08-09
**Autor:** Arthur Orosco (design colaborativo)
**Estado:** aprovado, aguardando revisão do documento escrito
**Projeto:** `C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts`
**Base:** [`2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`](2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md) (mapa de fases, e a promessa de que "abas verticais e visualizador reativo são a Fase 6"), [`2026-08-06-james-fase-4-memoria-duravel-design.md`](2026-08-06-james-fase-4-memoria-duravel-design.md) (D11 e R8, que marcam esta fase como o prazo de reavaliação do Postgres) e [`HANDOFF-fase-5-execucao.md`](../HANDOFF-fase-5-execucao.md) (nota de metodologia sobre o frontend não renderizar respostas). Este documento é autocontido: não é necessário ler os anteriores para implementar a Fase 6.

**Esta fase é a última antes do objetivo declarado.** A spec original fixou 0 a 6, com o app nativo de celular e a implantação em nuvem fora de pauta até 5 e 6 fecharem. A Fase 5 fechou em 2026-08-08.

---

## 1. Contexto

As Fases 0, 1, 3, 4 e 5 fecharam e estão em `main`. O front-end nunca foi redesenhado: a Fase 1 trocou os bindings do Wails por um cliente HTTP/WS e deixou a UI como estava, de propósito, para não deixar o Arthur sem app funcionando. O resultado é que a interface é a única camada do sistema que não passou por nenhuma fase de qualidade.

Seis levantamentos feitos nesta sessão definem o que a fase precisa ser. Todos foram lidos no código, não presumidos.

### 1.1 A resposta não chega à tela, e o backend não tem culpa

Durante a verificação ao vivo da Fase 5, respostas completavam no backend e não apareciam na interface. Confirmado lendo o Postgres direto: a mensagem existia na tabela, apenas não subia para a tela. O handoff registra que isso quase produziu a conclusão errada de "está travado" quando o turno tinha terminado rápido.

A consequência prática já está institucionalizada como regra: verificação ao vivo neste projeto não pode confiar no SPA, tem que ler o banco por REST. Uma interface em que não se confia é uma interface que não faz o seu trabalho. **É esta a razão de a fase existir.**

### 1.2 O backend fala sete eventos e a SPA escuta quatro

Emitidos hoje pelo servidor:

| Evento | Origem | A SPA trata? |
|---|---|---|
| `resposta_final` | [`ws.go:102`](../../../internal/api/ws.go), [`dispositivo.go:133`](../../../internal/api/dispositivo.go) | sim |
| `erro` | [`ws.go:97`](../../../internal/api/ws.go) | sim |
| `audio_pronto` | [`ws.go:104`](../../../internal/api/ws.go) | sim |
| `aprovacao_requerida` | [`clientes.go:80`](../../../internal/api/clientes.go) | sim |
| `ferramenta_iniciada` | [`ws.go:90`](../../../internal/api/ws.go) | **não** |
| `ferramenta_concluida` | [`ws.go:93`](../../../internal/api/ws.go) | **não** |
| `token` | [`ws.go:101`](../../../internal/api/ws.go) | **não** |

Os dois eventos de ferramenta são exatamente o sinal de "estou vivo, estou fazendo X" que faltou em 1.1. O backend os emite desde a Fase 1 e ninguém nunca os consumiu.

`token` é caso à parte: ele manda `resp.Texto` inteiro, de uma vez, imediatamente antes de `resposta_final` com o mesmo conteúdo. Não é streaming — é um evento duplicado com nome que promete o que não entrega.

### 1.3 O WebSocket não tem reconexão nem sinal de queda

[`ws.ts`](../../../web/src/lib/ws.ts) tem 35 linhas: abre o socket, registra `onmessage`, expõe `on`, `enviar` e `fechar`. **Não há `onclose`, não há `onerror`, não há reconexão, não há fila de envio.** Se o socket cair — VPN oscilando, `jamesd` reiniciado, máquina suspendendo — a página fica silenciosamente morta, e `enviar` passa a lançar exceção não tratada. Nada aparece na tela.

Isso é candidato direto a explicar 1.1, e vale independentemente de ser a causa: uma interface sem estado de conexão não é diagnosticável.

### 1.4 O Postgres tem dois leitores e a SPA não chama nenhum

A Fase 4, em sua seção 1.4, mapeou tabela por tabela quem escreve e quem lê. `mensagens` e `sessoes` são lidas só por [`rest.go`](../../../internal/api/rest.go), para a SPA exibir; `aprovacoes` não é lida por ninguém em produção. As rotas `GET /api/sessoes` e `GET /api/sessoes/{id}/mensagens` existem e estão registradas em [`rest.go:57-59`](../../../internal/api/rest.go).

A SPA não chama nenhuma das duas. [`App.tsx:98`](../../../web/src/App.tsx) inicializa a lista de mensagens com uma saudação fixa em código e nada mais. Recarregar a página perde a conversa da tela e abre sessão nova no backend, porque `sessaoID` é variável local da conexão WS ([`ws.go:36`](../../../internal/api/ws.go)) e nasce vazia.

### 1.5 A Fase 5 produz conteúdo visual que não tem onde aparecer

`ver_tela` devolve um PNG na resolução nativa mais uma lista de até 80 elementos com id, nome, tipo e posição. Isso atravessa device → hub → ferramenta → modelo e morre ali: a interface não tem superfície para exibir imagem nem lista estruturada. O usuário não consegue ver o que o James viu.

### 1.6 Correção de um fato herdado: o timeout do Gemini existe

O handoff da Fase 5 afirma, na lista de ações para a próxima sessão, que não há `context.WithTimeout` nas chamadas ao Gemini e que "hoje nada força o timeout". **Isso está incorreto, e foi verificado nesta sessão:**

- [`loop.go:23-27`](../../../internal/agente/loop.go) declara `TimeoutProvider`, com o comentário "limita cada chamada ao Provider.Gerar. Zero desliga".
- [`loop.go:88-90`](../../../internal/agente/loop.go) aplica `context.WithTimeout` quando o campo é maior que zero.
- [`main.go:115`](../../../cmd/jamesd/main.go) alimenta o campo com `cfg.TimeoutSegundos`, que o [`config.toml`](../../../config.toml) define como **30 segundos**.

Duas consequências. A primeira é de escopo: o item "adicionar timeout ao Gemini", aprovado no brainstorm, deixa de existir como trabalho. A segunda é mais relevante e vale para quem for pegar o R10 (`ver_tela` encadeado com um clique no mesmo turno trava indefinidamente): **se toda chamada ao provider morre em 30 segundos, o travamento indefinido não está na chamada ao Gemini.** Isso elimina uma hipótese e aponta o resto do caminho — device, hub, ou a leitura do WS — como onde procurar. Ver R8.

---

## 2. Escopo

### 2.1 Entra

- **Reescrita de `web/src`** na forma decidida em D1: orbe como estado recolhido, e um painel expandido com trilha vertical de cinco abas mais visualizador reativo.
- **Consumo dos eventos ignorados** de 1.2, com estado de ferramenta em voo visível.
- **Reconexão de WebSocket** com backoff, estado de conexão na tela e fila de envio.
- **Aba Sessões com retomada**, não só arquivo: listar do Postgres e continuar uma conversa anterior.
- **Aba Tela**, exibindo a última captura de `ver_tela` e a lista de elementos.
- **Aba Memória**, exigindo o único endpoint novo desta fase.
- **Tailwind** como sistema de estilo, substituindo as 414 linhas de [`App.css`](../../../web/src/App.css).
- **Remoção do evento `token`** do `ws.go`.
- **Testes**: vitest nos componentes e no estado, Playwright no fluxo real de ponta a ponta.

### 2.2 Não entra

- **Streaming de resposta.** Decisão D4. A dor real de 1.1 é "não sei se está vivo", e quem resolve isso são os eventos de ferramenta que já existem. Streaming arrastaria o provider Gemini, o loop e o protocolo WS para dentro de uma fase de interface. Adiado, não cancelado.
- **Causa raiz do R10.** É reentrância COM/UIA no device Python, não tem relação com front-end. Continua aberto, agora com uma hipótese a menos (1.6).
- **Timeout do Gemini.** Já existe (1.6).
- **`pgvector`, embeddings e retrieval semântico.** Seguem adiados desde a Fase 4.
- **App nativo de celular e implantação em nuvem.** São o objetivo seguinte, depois desta fase.
- **Troca do mecanismo de wake word por detector local.** Segue adiado desde a Fase 5, D10.
- **Leitor para a tabela `aprovacoes`.** Ela continua sem leitor em produção. Registrado, fora de escopo.

---

## 3. Decisões desta sessão

| # | Decisão | Escolha | Por quê |
|---|---|---|---|
| D1 | Forma da interface | **Trilha vertical de abas mais visualizador reativo**, com o orbe como estado recolhido | É o que a spec original já reservou para esta fase. As alternativas avaliadas com mockup foram "bolinha só preenchida por dentro" (não tem onde caber o screenshot de 1.5) e "app de chat clássico com barra lateral de sessões" (resolve histórico, mas o James deixa de ser HUD sobre a tela e vira mais um app na barra de tarefas). A lista de sessões do app clássico não se perde: vira uma das cinco abas |
| D2 | Stack | **React 18 + Vite + vitest, como hoje, mais Tailwind. Sem shadcn** | A reescrita é de componentes, não de toolchain: `web/embed.go` já embute `dist` e os testes de componente já existem. Tailwind entra porque o layout de D1 (trilha, dois painéis, colapso para uma coluna) escrito à mão reproduz o `App.css` de 414 linhas. shadcn fica de fora por encaixe, não por qualidade: traz Radix junto e empurra o visual para o genérico de painel administrativo, que é o oposto da identidade de HUD |
| D3 | Janela | **Uma janela só, que cresce** | O resize nativo no abrir/fechar não é enfeite: a janela do shell é frameless, transparente e always-on-top, então mantê-la grande o tempo todo engoliria clique de tudo que está embaixo. Duas janelas separadas eliminariam o resize, mas criariam dois clientes WS e estado duplicado — e o backend abre sessão nova por conexão ([`ws.go:36`](../../../internal/api/ws.go)), então duas janelas seriam duas conversas |
| D4 | Streaming | **Fora desta fase** | Ver 2.2. Registrado que o formato do evento de mensagem é justamente o que streaming mudaria, e portanto há retrabalho conhecido se ele entrar depois. Aceito |
| D5 | Sessões | **Arquivo mais retomada** | Só arquivo deixaria o bug de 1.4 meio consertado: a tela mostraria conversas antigas mas continuaria incapaz de continuar uma. "Continue a conversa de ontem" foi registrado pela Fase 4 como dívida explícita para a fase que mexesse na SPA. Custo assumido: é a maior peça de backend desta fase. Ver R1 |
| D6 | Postgres | **Fica, e o R8 da Fase 4 é resolvido por esta fase, não reaberto** | D11 da Fase 4 manteve o banco com prazo de reavaliação "na Fase 6, que é quem reescreve os dois únicos leitores". A reavaliação se resolve pelo próprio desenho: com D5, as abas Conversa e Sessões passam a consumir `sessoes` e `mensagens` de verdade. A condição que sustentava a dúvida — tabelas escritas que ninguém lê — deixa de valer. A análise completa está em 1.4 da spec da Fase 4 e não é refeita aqui |
| D7 | Fonte da aba Memória | **Endpoint novo `GET /api/memoria`** | Das cinco abas, é a única sem fonte. `Vault/Memoria` é lido hoje só dentro do turno, por [`ws.go:134`](../../../internal/api/ws.go), para compor o system prompt. Expor por REST é mais simples que inventar evento WS: a memória muda por escrita do modelo, não por turno, e a aba pode reler quando abre |
| D8 | Evento `token` | **Removido do `ws.go`** | Ele duplica `resposta_final` com o mesmo conteúdo, imediatamente antes. Sem streaming (D4), manter um evento chamado `token` que não carrega token é armadilha para a próxima pessoa. Remover é mais barato agora que a SPA que o ignora está sendo reescrita de qualquer forma |
| D9 | Estado do visualizador | **Derivado do turno, sem estado próprio** | Precedência fixa: aprovação pendente, depois captura recente de `ver_tela`, depois última saída de ferramenta, depois vazio. Estado derivado não dessincroniza. Efeito colateral desejado: mata o card de aprovação flutuante de hoje ([`App.tsx:191`](../../../web/src/App.tsx)), que divide espaço com a conversa e pode rolar para fora da vista |
| D10 | Arraste e resize da janela | **Movidos para módulo próprio, não reescritos** | Os comentários longos em [`App.tsx:35-93`](../../../web/src/App.tsx) documentam bugs reais já pagos — clique rápido deixando `arrastando` preso em true, mouseup chegando antes da resposta de `posicaoJanela()`. Reescrever do zero é recomprar esses bugs. A lógica sai intacta do componente |
| D11 | Local de execução | **Direto em `main`, commit por task** | Mesmo padrão das Fases 3, 4 e 5 |
| D12 | Pasta | **`web/` reescrita no lugar, sem pasta paralela** | Uma SPA de 1.105 linhas somando código e teste não justifica migração incremental com duas árvores vivas. `web/embed.go` e a rota de arquivos estáticos em [`rest.go:66`](../../../internal/api/rest.go) não mudam |

---

## 4. Arquitetura

Duas camadas são tocadas. O grosso do trabalho é em `web/src`; o Go recebe três mudanças pontuais.

### 4.1 `web/src` — a quebra do `App.tsx`

Hoje [`App.tsx`](../../../web/src/App.tsx) tem 230 linhas e faz cinco trabalhos: arraste da janela nativa, ciclo de vida do WebSocket, estado das mensagens, card de aprovação e reprodução de áudio. A divisão alvo:

| Módulo | Responsabilidade | Depende de |
|---|---|---|
| `lib/ws.ts` | conexão, reconexão com backoff, fila de envio enquanto fechado, estado de conexão observável | nada |
| `lib/eventos.ts` | tipos TypeScript dos eventos do backend, um lugar só | nada |
| `lib/api.ts` | chamadas REST: sessões, mensagens, memória | nada |
| `estado/turno.ts` | mensagens, ferramentas em voo, última captura, aprovação pendente | `eventos` |
| `shell/orbe.tsx` | arraste e resize nativo (D10), abrir/fechar | `lib/janela` |
| `abas/conversa.tsx` … `abas/sessoes.tsx` | uma por aba | `estado`, `lib/api` |
| `visualizador.tsx` | painel direito, deriva do turno (D9) | `estado` |

Cada módulo é testável isolado: `ws.ts` com socket falso, `estado/turno.ts` como função pura sobre eventos, abas com `@testing-library/react`.

### 4.2 As cinco abas

| Aba | Conteúdo | Fonte |
|---|---|---|
| Conversa | turno atual, com ferramentas em voo intercaladas | WS |
| Tela | último PNG de `ver_tela`, superfície e escala, lista de elementos | WS |
| Atividade | histórico de ferramentas do turno, com duração e erro; aprovações | WS |
| Memória | notas do `Vault/Memoria`, por assunto | `GET /api/memoria` |
| Sessões | lista de sessões, abrir para ler, retomar para continuar | `GET /api/sessoes` |

### 4.3 `internal/api` — o que muda no Go

1. **`GET /api/memoria`** (D7). Devolve as notas por assunto, no mesmo formato que `Notas.Renderizar` já produz para o system prompt, sem duplicar a lógica de leitura.
2. **Retomada de sessão** (D5). `ws.go` passa a aceitar um id de sessão do cliente e, quando ele vem, carrega `sessaoID` e reconstrói `historico` a partir de `mensagens`. Ver 5.2.
3. **Remoção de `token`** (D8). Uma linha em [`ws.go:101`](../../../internal/api/ws.go).

Nada mais no Go é tocado. `processarTurno`, o gate de permissão, o hub de clientes e o fluxo de aprovação ficam intactos — é o mesmo princípio que a Fase 4 usou para não encostar no fluxo que produziu três bugs na Fase 3.

---

## 5. Protocolo

### 5.1 Eventos consumidos

Todos os seis eventos que sobrevivem a D8 passam a ter tratamento explícito. `ferramenta_iniciada` abre um item em voo no estado; `ferramenta_concluida` o fecha, com erro quando `erro` não é vazio. Evento desconhecido é registrado no console e ignorado, nunca derruba a tela.

O envelope não muda: `{tipo, id, dados}`, o mesmo contrato que [`ws.test.ts`](../../../web/src/lib/ws.test.ts) já fixa. Vale lembrar a razão documentada em [`ws.go:191-199`](../../../internal/api/ws.go): o cliente só repassa `dados` aos manipuladores, e por isso `aprovacao_requerida` duplica o id lá dentro. Esse contrato é preservado.

### 5.2 Retomada de sessão

O cliente manda o id da sessão a retomar. O servidor carrega as mensagens daquela sessão e reconstrói `historico []agente.Mensagem`.

A dificuldade está na reconstrução, e é real: [`persistirNovasMensagens`](../../../internal/api/ws.go) grava, na mesma coluna `chamadas`, **ou** `m.Chamadas` **ou** `m.Resultados`, escolhendo por qual dos dois está preenchido. Ler de volta exige desambiguar pelo papel da mensagem, e uma reconstrução errada produz histórico que o Gemini rejeita ou, pior, aceita com semântica trocada. Ver R1.

Sessão nova continua sendo o caminho padrão: cliente que não manda id se comporta exatamente como hoje.

### 5.3 `GET /api/memoria`

Sem parâmetros. Devolve as notas agrupadas por assunto. Erro de leitura do vault devolve 200 com lista vazia mais um campo de aviso, nunca 500: memória é melhor-esforço em todo o resto do sistema ([`ws.go:137`](../../../internal/api/ws.go)) e a aba não pode ser a única parte que trata vault quebrado como falha fatal.

---

## 6. Segurança e privacidade

O modelo de permissão não muda: loopback é Privilegiado, qualquer outra origem é Restrito, decidido em [`permissao.go:32`](../../../internal/permissao/permissao.go) sem lista de exceção por endereço.

Duas consequências desta fase merecem registro explícito:

- **A aba Tela exibe o conteúdo da tela do desktop para qualquer cliente conectado, inclusive o celular pela VPN.** Isso não é novo — é a consequência já registrada em D5 e R7 da Fase 5, onde `ver_tela` foi definida como `NivelSeguro` por decisão do Arthur. A fase apenas torna visível o que já trafegava. Nenhuma permissão nova é concedida.
- **A aba Memória expõe notas pessoais por REST.** O endpoint fica sob o mesmo servidor e a mesma fronteira de rede do resto; não há autenticação por usuário no projeto, e esta fase não a introduz.

---

## 7. Tratamento de erro

A regra que faltava: **todo estado que não é "tudo certo" precisa aparecer na tela.** Três estados hoje invisíveis passam a existir:

| Estado | Como aparece |
|---|---|
| Reconectando | faixa persistente, com contagem de tentativa |
| Backend inalcançável | faixa de erro, com o envio bloqueado em vez de lançar exceção |
| Turno em andamento | ferramentas em voo com tempo decorrido, para turno longo não parecer travado |

Áudio continua melhor-esforço, como já é: autoplay bloqueado não derruba nada e o arquivo ainda é excluído ([`App.tsx:124-135`](../../../web/src/App.tsx)). Esse comportamento é preservado na migração.

---

## 8. Estratégia de teste

- **vitest** para `estado/turno.ts` (função pura sobre sequência de eventos), `lib/ws.ts` (socket falso: queda, reconexão, fila) e cada aba com `@testing-library/react`.
- **Playwright**, pela skill `webapp-testing`, para o fluxo que nenhum teste unitário cobre e que é a razão da fase: mandar mensagem, ver a resposta renderizar, derrubar o `jamesd` e ver a faixa de reconexão, subir de novo e ver o envio voltar.
- **Go**: teste para `GET /api/memoria` e para a reconstrução de histórico de 5.2, este último com as duas formas da coluna `chamadas`.

O critério de pronto não é suíte verde. É a verificação ao vivo de 10, pelo mesmo padrão da Fase 5.

---

## 9. Restrição conhecida

A tensão registrada em D12 da Fase 5 continua: com o `jamesd` em nuvem, o print da tela local viaja pela internet até o servidor e de lá até o Google. Esta fase não a resolve e não a piora — só torna o conteúdo visível para quem já o recebia. A decisão fica para a fase de nuvem.

---

## 10. Critérios de aceitação

Todos verificados ao vivo, com `jamesd`, device e Postgres reais, e leitura pelo Postgres quando houver dúvida sobre a tela.

| # | Critério |
|---|---|
| CA1 | Mandar uma mensagem pela interface e ver a resposta renderizar na tela, sem consultar o banco para saber se chegou |
| CA2 | Durante um turno com ferramenta, ver o nome da ferramenta em voo e a conclusão dela, com duração |
| CA3 | Derrubar o `jamesd` com a página aberta: a faixa de reconexão aparece. Subir de novo: a conexão volta sozinha e o envio funciona |
| CA4 | Pedir `ver_tela` e ver, na aba Tela, o print e a lista de elementos com id, nome e tipo |
| CA5 | Recarregar a página e continuar a mesma conversa pela aba Sessões, com o modelo demonstrando lembrar do que foi dito antes do reload |
| CA6 | Abrir a aba Memória e ver as notas que o `Vault/Memoria` contém |
| CA7 | Pelo celular, na VPN, disparar ferramenta perigosa e aprovar pelo desktop, com o pedido aparecendo no visualizador e não como card flutuante |
| CA8 | Layout em uma coluna no celular, com a trilha vertical virando barra inferior, sem rolagem horizontal |

---

## 11. Ordem de implementação

Da folha para a raiz, deixando a troca da interface por último — mesmo princípio da Fase 1, para o app ficar quebrado pelo menor tempo possível.

1. `GET /api/memoria` com teste (Go, isolado, não afeta a SPA atual).
2. Reconstrução de histórico e retomada de sessão no `ws.go`, com teste das duas formas da coluna `chamadas`.
3. Remoção do `token`.
4. Tailwind instalado e configurado, com a SPA atual ainda funcionando.
5. `lib/ws.ts` com reconexão e fila, testado com socket falso.
6. `estado/turno.ts` como função pura, testado.
7. `shell/orbe.tsx`: arraste e resize movidos de `App.tsx` sem alteração de lógica.
8. Abas, uma por vez, começando por Conversa.
9. Visualizador reativo.
10. Colapso para uma coluna e verificação no celular.
11. Playwright no fluxo de ponta a ponta.
12. Verificação ao vivo dos CA1 a CA8.

---

## 12. Riscos conhecidos

| # | Risco | Mitigação |
|---|---|---|
| R1 | Reconstruir `historico` da coluna polimórfica `chamadas` produzir histórico semanticamente errado que o Gemini aceita em silêncio | Desambiguar pelo papel da mensagem, nunca por tentativa de decodificar; teste com as duas formas gravadas; CA5 exige o modelo demonstrar lembrança real, não só a tela mostrar texto antigo |
| R2 | A suíte Go fechar verde sem nunca tocar o banco | [`testutil_test.go:16`](../../../internal/armazenamento/postgres/testutil_test.go) faz `t.Skip` quando `DATABASE_URL` está vazia (a spec da Fase 4 cita a linha 15, que é a guarda `if url == ""`). Com a SPA passando a depender de `/api/sessoes` e da retomada, isso deixa de ser detalhe. Rodar a suíte com `DATABASE_URL` definida faz parte do critério de pronto desta fase |
| R3 | Tailwind e o CSS legado convivendo durante a migração e produzindo estilo imprevisível | A migração é por aba (passo 8), e cada aba concluída deixa de depender do `App.css`. O arquivo antigo só é removido quando a última aba sair dele |
| R4 | A janela expandida, sendo transparente e always-on-top, engolir clique do que está embaixo | D3 mantém o resize nativo justamente por isso. Verificar no CA1 que, com o painel fechado, o clique atravessa fora do orbe |
| R5 | Layout de três colunas ficar inutilizável no celular | CA8 é critério explícito. A trilha vira barra inferior e o visualizador vira aba, em vez de comprimir três colunas |
| R6 | A aba Tela tornar rotineira a exposição do desktop a um cliente remoto | Não é permissão nova (seção 6), mas a visibilidade muda o comportamento na prática. Registrado para reavaliação na fase de nuvem, junto com D12 da Fase 5 |
| R7 | Retomar sessão antiga e a memória durável divergirem — nota escrita depois da conversa retomada aparece como se sempre tivesse existido | Aceito. A memória entra pelo system prompt a cada turno, por desenho da Fase 4, e é lida no momento do turno, não no da sessão. Registrado para não ser diagnosticado como bug |
| R8 | O R10 seguir aberto e agora com uma hipótese a menos | 1.6 elimina a chamada ao Gemini como suspeita: ela tem deadline de 30s. Sobram device, hub e a leitura do WS. A ação concreta que continua válida é a reprodução isolada em Python — `elementos.varrer()` seguido imediatamente de `uiautomation.ControlFromPoint()` na mesma thread. Fora desta fase |
