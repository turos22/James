# James — Fase 7A: o James vira aplicativo

**Data:** 2026-08-11
**Autor:** Arthur Orosco (design colaborativo)
**Estado:** aprovado, aguardando revisão do documento escrito
**Projeto:** `C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts`
**Base:** [`2026-08-09-james-fase-6-frontend-novo-design.md`](2026-08-09-james-fase-6-frontend-novo-design.md) (o front-end que esta fase substitui), [`HANDOFF-fase-6-execucao.md`](../HANDOFF-fase-6-execucao.md) (achados abertos, três dos quais esta fase fecha) e [`2026-08-11-permissao-denylist-e-origem.md`](../decisoes/2026-08-11-permissao-denylist-e-origem.md) (o modelo de permissão que torna a UI de aprovação obrigatória).
**Design de referência:** [`referencias/painel-de-controle-james/`](../referencias/painel-de-controle-james/) — projeto Next gerado no v0.dev e fornecido pelo Arthur como alvo visual.

Este documento é autocontido: não é necessário ler os anteriores para implementar a Fase 7A.

---

## 1. Contexto

As Fases 0 a 6 estão fechadas e em `main`. O objetivo declarado — o James como aplicativo de verdade, e depois no celular — deixou de estar bloqueado.

O Arthur trouxe um design pronto (v0.dev) e uma frase que define a fase: **"não teremos mais uma bolinha para abrir e fechar o chat"**. O James deixa de ser um widget flutuante e passa a ser um aplicativo com barra lateral e cinco abas.

Seis levantamentos feitos nesta sessão definem o tamanho real do trabalho. Todos foram lidos no código, nenhum presumido.

### 1.1 Não existe contabilidade de token no caminho de produção

`UsageMetadata` aparece em exatamente um lugar executável do repositório: `scripts/verificar_tokens_imagem/main.go`, uma ferramenta avulsa da Fase 5. Fora dela, só comentários em `internal/memoria/janela.go`.

A tabela `mensagens` (`internal/armazenamento/migracoes/0001_inicial.sql`) tem `papel`, `conteudo`, `chamadas` e `criada_em`. A tabela `sessoes` tem `titulo`, `criada_em` e `atualizada_em`. Nenhuma das duas guarda token, custo, duração ou erro.

Consequência: a aba DashBoard inteira e a coluna "1.240 tokens" do Histórico, ambas presentes no design de referência, não têm de onde ler. Isso não é front-end — é instrumentação nova no agente, migração de banco e agregação.

### 1.2 O Vault não tem links, tags nem frontmatter

Uma nota hoje é um arquivo por assunto com linhas soltas. `Vault/Memoria/carro.md` inteiro é uma linha:

```
- O carro do Arthur é um Fusca 1971 azul.
```

O Graph View do Obsidian é um grafo de `[[wikilinks]]`. Sem modelo de aresta, o grafo desenha pontos sem linha nenhuma.

### 1.3 O design de referência não tem onde pôr três coisas que já funcionam

O v0 entrega cinco abas: James, Mensagens, Memória, Histórico, DashBoard. Some com três superfícies existentes:

- **Card de aprovação.** A denylist instalada em 2026-08-11 (`internal/ferramentas/classificador.go`) confirma toda ferramenta destrutiva. Sem UI de aprovar/negar, cada uma dessas trava até o timeout de 300s do `config.toml`.
- **Trilha de atividade.** Quais ferramentas rodaram no turno.
- **Aba Tela.** A saída do `ver_tela`.

### 1.4 O shell Wails inteiro existe para servir a bolinha

`shell/main.go` abre a janela com `Width: 100, Height: 100`, `Frameless: true`, `AlwaysOnTop: true` e `WebviewIsTransparent: true`. `web/src/lib/janela.ts` cresce e encolhe essa janela por HTTP em `127.0.0.1:17313`, com resize ancorado no canto onde a bolinha está — para o chat não abrir para fora da área visível.

Matar a bolinha invalida esse modelo inteiro. Não é dano colateral: é consequência obrigatória do pedido.

### 1.5 O contexto do modelo descarta o começo da conversa em silêncio

`internal/memoria/janela.go:86`, `Janela.Montar`, percorre o histórico de trás para frente e para quando estoura `OrcamentoTokens` (8000, do `config.toml`). O que sobra é **descartado sem registro**. Conversa longa perde o começo, e nada no sistema sabe disso.

### 1.6 O título da sessão é a primeira mensagem inteira

Correção de uma suposição que eu ia escrever errado: `ws.go:214` faz `s.Sessoes.Criar(ctx, texto)` passando o texto **integral** da primeira mensagem do usuário como título, sem truncar. Então o Histórico tem preview desde já — o que falta é corte na exibição, não a origem do dado.

E `listarSessoes` (`internal/api/rest.go`) chama `s.Sessoes.Listar(r.Context())` e devolve a tabela inteira, sem limite nem paginação. Isso é o achado nº 2 do handoff da Fase 6, onde a aba chegou a renderizar 1646 botões.

---

## 2. Escopo

O pedido do Arthur — shell novo, cinco abas, grafo de memória, dashboard de custo, tabela de turnos, resumo incremental, wikilinks — **não cabe numa spec só**. Para calibrar: a Fase 6 foi *apenas* o front-end, teve 20 tasks e acumulou 12 correções durante a execução. Este pedido é maior e mexe em banco, agente e memória ao mesmo tempo.

Fatiamento aprovado (D1), na ordem "app usável primeiro":

| Fase | Conteúdo |
|---|---|
| **7A** (esta spec) | Shell, sidebar, aba James, Mensagens, Histórico, ciclo de sessão, resumo rolante, janela nativa |
| 7B | Tabela de turnos, instrumentação de token/custo/duração/erro, custo por ferramenta, DashBoard, imagem do `ver_tela` |
| 7C | Vault com wikilinks, migração das notas existentes, grafo de memória |

### 2.1 Entra

1. Shell de aplicativo com barra lateral colapsável e cinco itens de navegação.
2. Janela nativa repensada: de widget flutuante para janela de aplicativo com controles reais.
3. Aba **James**: orbe reagindo ao áudio real de saída, card de aprovação, trilha de atividade com duração, saída textual do `ver_tela`.
4. Aba **Mensagens**: conversa da sessão corrente, estado de conexão real, auto-scroll condicional, botão de sessão nova.
5. Aba **Histórico**: lista paginada, agrupada por dia, com busca no servidor.
6. Ciclo de vida de sessão por `sessionStorage`, mais o comando WS `nova_sessao`.
7. Resumo rolante da sessão, com migração `0002`.
8. Paginação keyset em `GET /api/sessoes`.
9. Teste de ponta a ponta com Playwright.

### 2.2 Não entra

1. **DashBoard com dados.** A aba existe na sidebar e mostra estado vazio franco. Métrica nenhuma é inventada. (7B)
2. **Instrumentação de token e custo.** Inclui a coluna de tokens do Histórico, que fica vazia. (7B)
3. **Imagem do `ver_tela` no front.** Achado aberto desde a Fase 6; a aba mostra a lista de elementos, como hoje. (7B)
4. **Wikilinks e grafo de memória.** A aba Memória mantém `abas/memoria.tsx` como está. (7C)
5. **Nuvem, celular e app nativo.** Nada aqui muda topologia de rede.
6. **Router e histórico de navegação.** Estado de aba é local. (Dívida consciente: o botão voltar do Android vai pedir isso quando o app for para o celular.)
7. **Perfil no `aprovar`/`negar`.** Achado aberto da Fase 6, não alcançável hoje. Segue aberto.

---

## 3. Decisões desta sessão

| # | Decisão | Motivo |
|---|---|---|
| **D1** | Fatiar em 7A/7B/7C, app usável primeiro | Plano grande acumula correção tarde; o histórico do projeto mostra 12 correções em 20 tasks |
| **D2** | Portar o design para o Vite existente; não migrar para Next | Next quer runtime Node ou export estático, quebra o `//go:embed all:dist` e o shell Wails, e nenhum recurso de servidor dele é usado — o James é WebSocket ao vivo. Vite também é o melhor caminho para empacotar no celular depois |
| **D3** | A aba James absorve aprovação, atividade e tela | Mantém as cinco abas do design; a aba do orbe vira "o James agora" |
| **D4** | A janela Wails vira janela de aplicativo: `Frameless` mantido, `AlwaysOnTop` e transparência saem | Sem a bolinha, o modelo de widget não tem função. `Frameless` fica para os três círculos do design virarem controles reais |
| **D5** | O orbe reage ao áudio real de saída; sem botão e sem oscilador sintético | Pedido explícito do Arthur. O elemento `Audio` já existe em `App.tsx:79` |
| **D6** | Sessão nova quando a janela fecha, via `sessionStorage` | Sobrevive a F5 e a queda de rede; morre ao fechar. Preserva a correção do `retomar` no `onopen` (`App.tsx:58-61`) e mantém celular e desktop independentes |
| **D7** | `nova_sessao` é comando WS, não limpeza no cliente | `sessaoID` e `historico` são variáveis locais do handler (`ws.go:59`); limpar só no cliente deixa o servidor na conversa velha, sem erro |
| **D8** | Resumo rolante síncrono, disparado por estouro de orçamento | Em background exigiria coordenação com a chamada seguinte, e não há onde pendurar isso hoje. A latência aparece só no turno que transborda |
| **D9** | O resumo tem orçamento próprio (`orcamento_resumo`, 1000) | Sem teto, o resumo cresce sem parar e come a janela que deveria proteger |
| **D10** | Falha do resumidor não derruba o turno | Degrada para a janela truncada, que é o comportamento de hoje |
| **D11** | Paginação keyset em `/api/sessoes`; busca no servidor | Filtro no cliente com paginação mostraria resultado incompleto e pareceria bug |
| **D12** | A coluna de tokens do Histórico fica vazia na 7A | Não preencher com número inventado |
| **D13** | Memória e DashBoard aparecem na sidebar desde a 7A | Sidebar mutilada é pior que aba franca sobre o que ainda não mede |
| **D14** | Preço e câmbio ficam no `config.toml`, com data da cotação visível | Tabela de preço embutida no código envelhece calada e passa a mentir. *(Decidido aqui, aplicado na 7B)* |
| **D15** | Sem router; estado de aba em `useState` no `App` | Janela só, sem URL a preservar |
| **D16** | Fontes como `woff2` local, sem `next/font` e sem CDN | O binário roda offline; o embed não busca recurso externo |

---

## 4. Arquitetura

### 4.1 `web/src` — o que morre e o que nasce

**Morrem:** `shell/orbe.tsx`, `shell/orbe.css`, `trilha.tsx`, `visualizador.tsx`, `abas/atividade.tsx`, `abas/tela.tsx`, `abas/sessoes.tsx`.

**Nascem:** `shell/aplicativo.tsx`, `shell/barraLateral.tsx`, `shell/controlesJanela.tsx`, `abas/james.tsx`, `abas/mensagens.tsx`, `abas/historico.tsx`.

**Sobrevivem sem mudança de papel:** `lib/ws.ts`, `lib/eventos.ts`, `lib/api.ts`, `lib/markdown.ts`, `estado/turno.ts`, `abas/memoria.tsx`.

`App.tsx` **não muda de responsabilidade**: continua dono do `ClienteWS`, do redutor de turno e do estado de conexão, e distribui para as abas. Muda de layout. Isso é deliberado — preserva a reemissão de `retomar` no `onopen` sem tocar nela.

### 4.2 Barra lateral

208px expandida, 64px recolhida, transição de 300ms. Cinco itens, na ordem do design: James, Mensagens, Memória, Histórico, DashBoard. Aba ativa em `--app-blue`.

A aba James exibe **contador quando há aprovação pendente**. Sem isso, estando em outra aba você não veria o James travado esperando um sim, e o timeout de 300s corre igual.

### 4.3 A janela nativa

Em `shell/main.go`: `AlwaysOnTop` e `WebviewIsTransparent` saem, `Frameless` fica, tamanho inicial 1100×760, redimensionável.

Em `shell/controle.go`: entram minimizar, maximizar e fechar, no mesmo servidor de controle em loopback que já existe.

Em `web/src/lib/janela.ts`: `redimensionarJanela` **sai**, junto com o teste dela — maximizar e restaurar passam pelo endpoint novo, e função sem chamador apodrece. `moverJanela` e `posicaoJanela` sobrevivem para arrastar pela barra de título.

Fora do shell — navegador, celular — a porta 17313 não existe, todo `fetch` falha em silêncio, e os três círculos **não são desenhados**. A detecção é por sondagem do servidor de controle, não por user-agent.

### 4.4 `internal/api` — o que muda no Go

- `ws.go`: comando `nova_sessao`; `processarTurno` ganha a etapa de resumo.
- `rest.go`: `GET /api/sessoes` ganha `limite`, `antes` e `busca`.
- `internal/armazenamento/postgres/sessao.go`: `Listar` vira paginado; entra `AtualizarResumo`.
- `internal/armazenamento/postgres/mensagem.go`: entram `IDDeCorte` e `ListarNaoResumidas` (§7.2).
- `internal/memoria/resumo.go` (novo): o resumidor.

`internal/memoria/janela.go` **não muda.** A primeira versão deste documento previa `Montar` devolvendo duas fatias; a revisão mostrou que é desnecessário — ver §7.2.

---

## 5. Aba James

Quatro regiões numa coluna. O painel de estado abaixo do orbe segue a precedência: **aprovação pendente › ferramenta em curso › última leitura de tela › repouso**.

### 5.1 O orbe

Canvas de 320px com borda ondulante, como o design de referência. Duas diferenças, ambas pedidas pelo Arthur: **sem botão** e **sem oscilador sintético**.

A fonte é o áudio real de saída. `App.tsx:75-84` já cria `new Audio('/api/audio/{id}')` no evento `audio_pronto`. Esse elemento passa a ser roteado por:

```
HTMLAudioElement → createMediaElementSource → AnalyserNode → destination
```

`getByteFrequencyData` alimenta a deformação: os bins graves puxam o raio, os agudos fazem a ondulação fina. Sem áudio tocando, respiração senoidal lenta.

Três armadilhas, todas de falha silenciosa, que a implementação precisa tratar:

1. **`createMediaElementSource` só pode ser chamado uma vez por elemento.** O nó tem que nascer junto com o `Audio`, ou o segundo áudio da sessão estoura.
2. **Roteado pelo grafo, o som só sai pelo `destination`.** Esquecer esse `connect` deixa o orbe animando em silêncio absoluto — e o sintoma parece "o TTS parou de funcionar".
3. **`AudioContext` nasce `suspended` sem gesto do usuário.** Um contexto único no app, com `resume()` no primeiro clique em qualquer lugar. O tratamento de autoplay bloqueado que já existe — apagar o arquivo do disco mesmo sem tocar — continua valendo.

O componente recebe o `AnalyserNode` por injeção, para ser testável sem áudio real.

### 5.2 Aprovação, atividade, tela

- **Aprovação:** ferramenta, argumentos e os botões aprovar/negar. É o conteúdo que hoje vive em `visualizador.tsx`, movido.
- **Atividade:** nome, estado e **duração** de cada ferramenta do turno. `iniciadaEm` já existe no redutor e ninguém consome — isto fecha o achado CA2 aberto na Fase 6.
- **Tela:** a lista de elementos do `ver_tela`, como `abas/tela.tsx` mostra hoje. A imagem fica para a 7B.

---

## 6. Aba Mensagens e ciclo de sessão

### 6.1 A tela

Cabeçalho, balões com scroll vertical, campo no rodapé — o layout do design. Quatro adaptações:

1. O **"online" verde fixo vira o estado real da conexão**: `conectado` / `Reconectando… (tentativa N)` / `desconectado`, que o `ClienteWS` já publica. Bolinha verde permanente seria mentira, e a Fase 6 gastou trabalho para que a queda ficasse visível.
2. O cabeçalho mostra a **sessão corrente** — título truncado, data de criação — e o botão **Nova sessão**.
3. **Auto-scroll condicional:** só rola para o fim se o usuário já estava no fim.
4. **Transcrição de voz** entra como balão do usuário. O evento `transcricao` já existe e já é tratado pelo redutor.

### 6.2 Ciclo de vida

`sessionStorage` guarda o id da sessão corrente:

- **Ao carregar com id guardado:** emite `retomar` com ele.
- **Ao carregar sem id:** não emite nada. A sessão nasce no servidor no primeiro turno — é o que `processarTurno` já faz quando `sessaoID` é `uuid.Nil`.
- **Queda de rede:** `sessionStorage` sobrevive, o `retomar` do `onopen` continua valendo, a conversa é a mesma.
- **Janela fechada e reaberta:** `sessionStorage` morreu, sessão nova.
- **Botão Nova sessão:** limpa `sessionStorage`, zera o redutor e emite `nova_sessao`.

### 6.3 O comando `nova_sessao`

`sessaoID` e `historico` são **variáveis locais do handler** (`ws.go:59`). Limpar só no cliente deixaria o servidor continuando a conversa velha — cliente e servidor discordando sobre qual é a sessão, sem erro nenhum. É a mesma classe de bug que a auditoria da Fase 6 pegou no `retomar`, e por isso exige teste de regressão dedicado.

Protocolo: cliente envia `nova_sessao` sem dados; servidor zera `sessaoID` e `historico` e responde `sessao_retomada` com `sessao_id` vazio e `mensagens: []`. Reusar o evento de resposta evita um evento novo para dizer a mesma coisa.

---

## 7. Resumo rolante

### 7.1 Banco — migração `0002`

```sql
ALTER TABLE sessoes ADD COLUMN resumo TEXT NOT NULL DEFAULT '';
ALTER TABLE sessoes ADD COLUMN resumido_ate BIGINT;
```

`resumido_ate` guarda o `mensagens.id` da última linha coberta pelo resumo (`NULL` = nada resumido ainda). Sem ele não há como saber o que já foi resumido, e o resumo duplica ou se perde. O porquê de ser id — e não contagem — está na §7.2.

Índice novo não é necessário: `idx_mensagens_sessao` já cobre o filtro por `sessao_id`, e ordenar algumas centenas de linhas por `id` depois disso é irrelevante. Se a medição na execução disser o contrário, `(sessao_id, id)` é o índice a acrescentar.

### 7.2 O resumidor lê do banco, não do que `Montar` descartou

Esta é a decisão mais fácil de errar da fase, e a primeira versão deste documento a errou. Três fatos, todos verificados no código:

1. `persistirNovasMensagens` grava **uma linha por `agente.Mensagem`**, e `reconstruirHistorico` faz o inverso 1:1. Memória e banco têm a mesma granularidade.
2. `agente.Mensagem` (`internal/agente/tipos.go:39`) **não tem id**. O que `Montar` descarta não sabe dizer a que linha do Postgres corresponde.
3. `tratarRetomar` carrega `ListarRecentesPorSessao(ctx, sessao, 200)` (`ws.go:178`). Depois de retomar, `historico` é a **cauda** da sessão, não a sessão inteira. Qualquer contagem sobre `len(historico)` mente numa conversa com mais de 200 mensagens — e mente em silêncio.

Portanto o resumo se ancora em **id de linha do Postgres**, e o resumidor busca as mensagens no banco.

Consequência boa: **`Montar` não muda de assinatura.** Basta `len(janela) < len(*historico)` como gatilho de transbordo — `Montar` devolve um sufixo, então encolher significa que descartou. `internal/memoria/janela.go` fica intacto, nenhum chamador é tocado e o risco de esquecer um deles desaparece.

Dois métodos novos em `internal/armazenamento/postgres/mensagem.go`:

```go
// IDDeCorte devolve o id da mensagem mais antiga que ainda está na janela,
// dado o tamanho dela. Tudo com id menor está fora e é candidato a resumo.
func (r *MensagemRepositorio) IDDeCorte(ctx, sessaoID uuid.UUID, tamanhoJanela int) (int64, error)

// ListarNaoResumidas devolve as mensagens da sessão com id < corte e
// id > resumidoAte, em ordem crescente de id.
func (r *MensagemRepositorio) ListarNaoResumidas(ctx, sessaoID uuid.UUID, corte, resumidoAte int64) ([]Mensagem, error)
```

Ordenação por `id`, não por `criada_em`: `id` é `BIGSERIAL`, monotônico e sem empate. `criada_em` tem `DEFAULT now()` e duas mensagens do mesmo turno podem empatar — ordem instável ali produziria resumo com buraco ou repetição. As consultas existentes ficam como estão; só as novas usam `id`.

### 7.3 O fluxo

Em `processarTurno` — o funil único por onde passam voz e texto, confirmado em `ws.go:120` e `dispositivo.go:175`:

1. `janela := s.Memoria.Montar(*historico)`, como hoje.
2. Se `len(janela) == len(*historico)`, não transbordou: nada a fazer.
3. Transbordou: `IDDeCorte(sessao, len(janela))` dá o limite, `ListarNaoResumidas(sessao, corte, resumido_ate)` dá o que falta resumir.
4. Nada a resumir (já coberto): segue.
5. Há o que resumir: o resumidor recebe **resumo anterior + essas mensagens** e devolve o resumo atualizado. Grava `resumo` e `resumido_ate` = maior id do conjunto.
6. O contexto enviado ao modelo vira **notas do Vault + resumo da sessão + cauda literal**.

O modelo funciona igual numa conexão nova, depois de `retomar` e depois de reconexão, porque nada nele depende de índice em memória.

O resumo entra pelo system prompt, junto do bloco do Vault e depois dele — mesma justificativa já registrada em `processarTurno`: resumo não é conversa e não pode competir com recência por orçamento.

### 7.4 Configuração

`config.toml`, seção `[memoria]`: `orcamento_resumo = 1000`.

O resumidor usa o mesmo modelo e o mesmo timeout de 30s do agente. **Se falhar, o turno não falha**: registra `slog.Warn` e segue com a janela truncada — exatamente o comportamento de hoje.

---

## 8. Aba Histórico

Fonte: `GET /api/sessoes`, com paginação keyset.

```
GET /api/sessoes?limite=50&antes=<timestamp>&busca=<texto>
```

Ordenação por `atualizada_em DESC`. `antes` é o cursor. Busca é `ILIKE` sobre `titulo`, no servidor — buscar dentro do conteúdo das mensagens fica para 7B/7C.

Agrupamento por dia (Hoje / Ontem / data), como no design. Preview é o `titulo` truncado na exibição — a origem já existe (§1.6). Scroll infinito. A coluna de tokens fica vazia (D12).

---

## 9. Design visual

Tokens de cor do design de referência entram em `web/src/style.css`, no bloco `@theme` do Tailwind: `--app-bg`, `--app-panel`, `--app-sidebar`, `--app-blue`, `--app-blue-foreground`, `--app-icon`, `--app-hover`, `--app-line`, `--app-text`, `--app-muted`. As duas pontas já estão em Tailwind 4.3.3, então o mecanismo é idêntico.

Fontes: **Geist Sans** no corpo, **Source Serif 4** em títulos e rótulos, como `woff2` local em `web/src/assets/fonts/` — do jeito que o Nunito já é servido (D16).

`lucide-react` é a **única dependência nova**. A versão é fixada na implementação, conferindo a que o projeto de referência declara.

O tema é escuro fixo, como o design. Claro não entra.

---

## 10. Tratamento de erro

| Situação | Comportamento |
|---|---|
| WS cai | `ClienteWS` reconecta, campo trava, cabeçalho de Mensagens mostra a tentativa. Já existe |
| Resumidor falha | `slog.Warn`, turno segue com janela truncada (D10) |
| Migração 0002 falha | `jamesd` não sobe. Migração é pré-condição, não melhor-esforço |
| Servidor de controle ausente | Círculos não desenhados, `fetch` falha em silêncio. Já existe |
| `AudioContext` bloqueado | Orbe em repouso, áudio não toca, arquivo é apagado assim mesmo. Já existe |
| `/api/sessoes` falha | Histórico mostra erro e botão de tentar de novo; não trava as outras abas |
| Erro do backend no WS | Mensagem já é sanitizada desde a auditoria da Fase 6 — **não regredir** isso ao mexer em `tratarMensagem` |

---

## 11. Estratégia de teste

**Go** — `go vet ./... && go test ./...` contra o Postgres real:

- Migração `0002` aplicada em banco com dados preexistentes.
- `IDDeCorte` e `ListarNaoResumidas`: janela cobrindo a sessão inteira, janela parcial, e o caso que mais importa — **sessão retomada com mais de 200 mensagens**, onde `historico` é só a cauda e a ancoragem por id é o que salva.
- Resumidor com provider mock: primeiro resumo, resumo incremental, nada a resumir, e falha degradando sem derrubar o turno.
- `nova_sessao`: **o servidor precisa esquecer a sessão**. Teste de regressão explícito de que o turno seguinte não grava na sessão velha.
- Paginação keyset: página, cursor, borda de página cheia, busca.

**Front** — `vitest`:

- Redutor com `nova_sessao`.
- Barra lateral: seleção, colapso, contador de aprovação.
- Orbe com `AnalyserNode` injetado — sem áudio real.
- Auto-scroll condicional: rola no fim, não rola no meio.
- Histórico: paginação e busca.

**Ponta a ponta** — Playwright, que **entra nesta fase**. Foi prometido na Fase 6 e não foi escrito; sem ele toda verificação de front volta a ser manual. Cobertura mínima: abrir o app, trocar de aba, enviar mensagem e ver a resposta, abrir sessão nova, retomar do histórico.

> **Armadilha obrigatória no teste de ponta a ponta.** Input controlado do React só atualiza estado quando o `onChange` sintético dispara. `type` e `fill` de ferramenta de automação **não** disparam. Use setter nativo mais `dispatchEvent(new Event('input', {bubbles:true}))`. Sem esse aviso, o primeiro teste "prova" que o app está quebrado quando não está — já custou tempo duas vezes neste projeto.

**Verificação ao vivo:** `npm run build --prefix web` **e reiniciar o `jamesd`**. O `//go:embed all:dist` compila a SPA dentro do binário e já enganou antes.

O orbe reagindo ao áudio **não é verificável por teste automatizado com confiança**. Vai por verificação ao vivo: falar com o James e conferir que a deformação acompanha a fala, e não um oscilador.

---

## 12. Critérios de aceitação

| # | Critério |
|---|---|
| **CA1** | O app abre direto no shell, sem bolinha. A barra lateral mostra cinco abas e colapsa |
| **CA2** | No shell Wails, os três círculos minimizam, maximizam e fecham a janela de verdade; a janela é redimensionável e não fica sobre as outras. No navegador, os círculos não aparecem |
| **CA3** | A aba James mostra o orbe deformando **conforme a fala do James**, sem botão de reproduzir, e voltando ao repouso quando o áudio acaba |
| **CA4** | Uma ferramenta destrutiva pedida na aba Mensagens faz o card de aprovação aparecer na aba James, com contador na barra lateral; aprovar e negar funcionam |
| **CA5** | A trilha de atividade mostra nome, estado e **duração** de cada ferramenta do turno |
| **CA6** | A aba Mensagens mostra a conversa da sessão corrente com scroll; derrubar o `jamesd` troca o "online" pelo estado real de reconexão |
| **CA7** | Auto-scroll só acontece quando o usuário já está no fim da conversa |
| **CA8** | Fechar a janela e reabrir começa sessão nova; recarregar a página (F5) e derrubar a rede **retomam a mesma** |
| **CA9** | O botão Nova sessão zera a tela **e o servidor**: o turno seguinte grava numa sessão nova, verificado no Postgres |
| **CA10** | Conversa que ultrapassa o orçamento de 8000 tokens produz resumo em `sessoes.resumo`, e o James continua sabendo do começo da conversa |
| **CA11** | Falha do resumidor não derruba o turno |
| **CA12** | A aba Histórico carrega em páginas, agrupa por dia e busca no servidor; com 1600+ sessões no banco, não renderiza tudo de uma vez |
| **CA13** | As abas Memória e DashBoard existem na barra lateral: Memória lista as notas como hoje; DashBoard mostra um texto dizendo que a medição de custo e uso chega na Fase 7B — **sem gráfico de exemplo e sem número de mentira** |
| **CA14** | Suíte Go verde contra Postgres real; suíte vitest verde; Playwright verde |

### 12.1 Resultado da verificação ao vivo

Feita em 2026-08-14 contra `jamesd` real (`npm run build --prefix web` +
`go run ./cmd/jamesd`), Postgres real no Docker, chamadas reais ao Gemini e
TTS real. Sem device Python nem shell Wails abertos — ver limitações.
Detalhe completo, com os comandos e valores reais observados, está no
handoff: `docs/superpowers/HANDOFF-fase-7a-execucao.md`.

| # | Resultado |
|---|---|
| CA1 | **Passa, parcial.** Shell sem bolinha, cinco abas, recolher/expandir do menu confirmados no navegador. Não reabri o executável Wails nesta rodada |
| CA2 | **Parcial.** Confirmado que os círculos não aparecem fora do Wails. Os círculos de verdade (minimizar/maximizar/fechar/redimensionar) não foram clicados nesta verificação — exige mão humana, sem ferramenta de automação de GUI do Windows neste ambiente |
| CA3 | **Parcial.** Áudio real tocou de ponta a ponta (TTS real, `206 Partial Content`), provando a cadeia até o `AnalyserNode`. A deformação visual do orbe não foi vista (captura de tela indisponível neste ambiente) |
| CA4 | **Passa.** Aprovação real de `executar_comando_sistema`, arquivo apagado do disco de verdade, contador zerou depois |
| CA5 | **Passa.** Nome, estado (✓) e duração real (20,8s) registrados na mesma chamada do CA4 |
| CA6 | **Passa.** `online` → `Reconectando… (tentativa 2)` → `online`, processo do `jamesd` morto e religado de verdade |
| CA7 | **Não exercitado ao vivo.** Coberto por teste de unidade (Task 4); precisa de evento concorrente chegando com o usuário rolado pra cima, cenário não montado nesta rodada |
| CA8 | **Passa**, as três pernas (F5, queda de rede, janela nova), cada uma com `sessao_id` do Postgres conferido |
| CA9 | **Passa.** `sessao_id` novo confirmado por `SELECT` direto no Postgres, não só na tela |
| CA10 | **Não exercitado no orçamento de produção.** Mecanismo provado pela suíte de integração (Task 18) com orçamento artificial de 40 tokens; uma conversa real de 8000 tokens não foi tentada (custaria dezenas de chamadas reais ao Gemini) |
| CA11 | **Coberto por teste automatizado**, não forçado ao vivo (não há como fazer o Gemini falhar sob demanda sem mock); nada quebrou nos vários turnos reais desta verificação |
| CA12 | **Passa.** Só 50 de N+ sessões renderizadas (`document.querySelectorAll('main ul li button').length === 50`), agrupamento "Hoje", busca no servidor filtrou corretamente |
| CA13 | **Passa.** Memória listou nota real do vault; DashBoard sem número inventado |
| CA14 | **Passa.** `go vet`/`go test ./...` limpos contra Postgres real; vitest 106/106; Playwright 5/5 |

---

## 13. Ordem de implementação

1. Tokens de cor, fontes locais e `lucide-react`.
2. `shell/barraLateral.tsx` e `shell/aplicativo.tsx`, com as abas antigas ligadas provisoriamente.
3. `App.tsx` recomposto; morte de `orbe`, `trilha` e `visualizador`.
4. `shell/controle.go`: minimizar, maximizar, fechar.
5. `shell/main.go` e `controlesJanela.tsx`: a janela de aplicativo.
6. `abas/mensagens.tsx` com estado real de conexão e auto-scroll condicional.
7. `abas/james.tsx`: layout e precedência, sem áudio ainda.
8. O orbe com `AnalyserNode` injetado.
9. Ligação do áudio real em `App.tsx`.
10. Migração `0002`.
11. `sessao.go`: `Listar` paginado, `AtualizarResumo`, busca.
12. `rest.go`: `limite`, `antes`, `busca`.
13. `abas/historico.tsx`.
14. `sessionStorage` e o ciclo de sessão no cliente.
15. `nova_sessao` no `ws.go`, com o teste de regressão.
16. `IDDeCorte` e `ListarNaoResumidas` em `mensagem.go`.
17. `internal/memoria/resumo.go`.
18. Integração do resumo em `processarTurno`.
19. DashBoard vazio e revisão da aba Memória no shell novo.
20. Playwright.
21. Verificação ao vivo e handoff.

---

## 14. Riscos conhecidos

| # | Risco | Mitigação |
|---|---|---|
| **R1** | `createMediaElementSource` chamado duas vezes no mesmo elemento estoura | Criar o nó junto com o `Audio`; teste do caminho de dois áudios seguidos |
| **R2** | Esquecer `connect(destination)` deixa o áudio mudo, e o sintoma parece "o TTS quebrou" | CA3 exige ouvir a fala, não só ver o orbe mexer |
| **R3** | `nova_sessao` dessincronizado entre cliente e servidor — a classe de bug mais cara deste projeto, porque não gera erro | CA9 verifica **no Postgres**, não na tela |
| **R4** | Ancorar o resumo em contagem de mensagens em memória. Numa sessão retomada com mais de 200 mensagens, `historico` é só a cauda, e a contagem resume o trecho errado **sem erro nenhum** | §7.2 fixa a ancoragem por `mensagens.id`; a §11 exige teste exatamente desse cenário |
| **R5** | Resumo cresce sem teto e come a janela | `orcamento_resumo` (D9), com teste do resumo que estoura |
| **R6** | Latência do resumo síncrono no turno que transborda | Aceita (D8). Se incomodar ao vivo, vira item da 7B |
| **R7** | O banco de trabalho tem 1600+ sessões porque a suíte Go escreve e não limpa | CA12 usa isso como cenário de teste em vez de fingir que não existe. Limpeza da suíte é item separado, não desta fase |
| **R8** | Testar a interface velha por causa do `//go:embed` | Escrito na §11 e no handoff |
| **R9** | Playwright diagnosticando errado por causa do input controlado do React | Aviso obrigatório na §11 |
| **R10** | Migração `0002` num banco com dados reais | `DEFAULT ''` em `resumo` e `NULL` permitido em `resumido_ate`; teste de migração sobre banco populado |
| **R11** | A denylist de 2026-08-11 confirma mais coisas que a allowlist antiga; se a UI de aprovação regredir, o James trava por 300s a cada ferramenta destrutiva | CA4 é bloqueante |
| **R12** | Mensagem de erro do backend voltando a vazar host/banco/usuário ao mexer em `tratarMensagem` | Escrito na §10; a sanitização veio da auditoria da Fase 6 e não pode regredir |
