# Handoff de revisão — auditar a Fase 6 e levar tudo a 100%

Escrito em 2026-08-10, a pedido do Arthur, logo depois de a Fase 6 ser
fechada. **Este documento não registra o que foi feito** — para isso existe
`HANDOFF-fase-6-execucao.md`. Este aqui é uma pauta de auditoria: o que
revisar, o que testar, e o que só o Arthur pode testar.

Leia inteiro antes de agir. Não confie no que está escrito aqui sem conferir:
uma parte considerável dos erros desta fase apareceu justamente porque alguém
foi verificar uma afirmação de documento contra o código.

## Onde está tudo

- **Spec**: `specs/2026-08-09-james-fase-6-frontend-novo-design.md` — decisões
  D1-D12, riscos, e a seção 10.1 com o resultado da verificação ao vivo
- **Plano**: `plans/2026-08-09-james-fase-6-frontend-novo.md` — 20 tasks
- **Handoff de execução**: `HANDOFF-fase-6-execucao.md`
- **Fase anterior**: `HANDOFF-fase-5-execucao.md`

Fase 6 são 35 commits em `main`, de `c064cbf` a `0c2e7aa`.

---

## Parte 1 — o que revisar no código

Prioridade alta, em ordem. Todas são áreas onde um erro seria silencioso.

**1.1 `internal/api/eventos.go`** — nasceu na Task 18 e concentra os payloads
dos eventos e a emissão compartilhada entre o caminho de texto (`ws.go`) e o
de voz (`dispositivo.go`). Antes dela, os dois caminhos duplicavam a montagem
e já divergiram na prática (o evento `token` foi removido de um e sobreviveu
no outro). Confira que `TestFormatoDeEventosIdenticoEntreTextoEVoz` realmente
compara os bytes JSON dos dois caminhos, e não só a existência dos eventos.

**1.2 `reconstruirHistorico` e `mensagensExibiveis`** (`ws.go`, `eventos.go`).
A coluna `chamadas` do Postgres é polimórfica: guarda **ou** `Chamadas` **ou**
`Resultados`, e os dois tipos compartilham `ID` e `Nome`. A desambiguação é
pelo papel da mensagem. Se alguém "simplificar" isso para tentar decodificar
nos dois formatos, o histórico volta com semântica trocada e o Gemini aceita
sem reclamar. Há prova de inversão nos testes — confirme que ela ainda pega.

**1.3 `ListarRecentesPorSessao` x `ListarPorSessao`** (`postgres/mensagem.go`).
Os dois existem de propósito e têm semântica **oposta**: o REST lista do
começo para exibir, a retomada pega o fim para continuar. Já houve um bug aqui
— o plano mandava a retomada usar o método do REST, o que carregaria o começo
da conversa. Existe um teste que afirma explicitamente que os dois divergem;
ele existe para impedir que alguém "conserte" um dos dois.

**1.4 `web/src/lib/ws.ts`** — o `ClienteWS` rastreia `aberto` num campo
privado em vez de ler `socket.readyState`, de propósito: o duplo de teste não
tem `readyState` nem a constante `OPEN`, e a comparação daria `undefined ===
undefined`. Se alguém "melhorar" isso para usar `readyState`, o teste de fila
passa a mentir.

**1.5 `escolherConteudo`** (`visualizador.tsx`) — a precedência entre tela e
ferramenta é por **recência**, não fixa. A versão fixa congelava o painel na
primeira captura da sessão. Há teste específico para isso.

**1.6 O que vale re-revisar por amostragem:** as tasks 6, 7, 8 e 17 não
receberam as duas revisões completas (conformidade + qualidade). Eu pulei
declaradamente por serem configuração ou por a revisão de conformidade já ter
coberto o que eu perguntaria — mas se sobrar orçamento, essas são as
candidatas.

---

## Parte 2 — o que testar sozinho

O ambiente pode estar de pé ou não. Roteiro de subida:

```bash
docker compose up -d
```

Depois `go run ./cmd/jamesd` e, se for testar voz ou tela,
`python -m dispositivo.james_device.main` com o cwd na raiz.

**ARMADILHA que já enganou nesta sessão:** `web/embed.go` usa
`//go:embed all:dist`. A SPA é compilada **dentro do binário**. Mudou o front?
`npm run build --prefix web` **e reinicie o `jamesd`**, senão você testa a
interface velha achando que testa a nova.

**Suítes, primeiro:**

```bash
go build ./... && go vet ./... && go test ./... -count=1
```

```bash
npm test --prefix web && npm run build --prefix web
```

Estado no fechamento: Go inteiro verde contra Postgres real (sem SKIP, porque
o `TestMain` carrega o `.env` via `godotenv`), front 71 testes em 13 arquivos.

**O que ainda não foi exercitado ao vivo, em ordem de risco:**

**2.1 Fluxo de aprovação, nem no desktop.** Nenhuma ferramenta perigosa foi
disparada nesta sessão. O card de aprovação vive agora **só** no visualizador
(o `CardAprovacao` foi apagado) e tem cobertura só de teste unitário. Peça
algo que exija confirmação — um comando de escrita no `executar_comando_sistema`
serve — e confirme que o pedido aparece no visualizador, que "Aprovar" e
"Negar" fazem o que dizem, e que o turno segue corretamente nos dois casos.
Isto é o mais importante desta lista: é o mecanismo que impede o James de
executar coisa perigosa sem consentimento, e a Fase 3 já produziu três bugs
nele.

**2.2 Turno por voz de ponta a ponta.** O `tratarEntradaDeVoz` foi refatorado
na Task 18 e perdeu o evento `token`. Os testes Go passam, mas **nenhum turno
falado foi exercitado depois da refatoração**. Precisa do device rodando e de
alguém falando a wake word. A wake word aceita `james`, `jame`, `jeimes`,
`jaimes`, `jeims`, `geimes` — e o comando só é enviado quando você fala
**"enviar"** no fim.

**2.3 Reprodução de áudio (TTS).** O handler de `audio_pronto` foi movido para
o `App.tsx` novo e nunca foi ouvido. Confirme que toca, e que o arquivo é
apagado depois (`DELETE /api/audio/{id}`).

**2.4 Reconexão com turno em andamento.** O CA3 foi testado com a conexão
ociosa. Derrubar o `jamesd` **no meio de um turno** é outro caminho: a fila de
envio do `ClienteWS` deveria segurar a mensagem e mandar depois. Vale exercitar.

**2.5 Evento `erro` na tela.** O redutor transforma erro em mensagem visível
com prefixo `Erro:`. Nunca foi visto ao vivo.

**2.6 Conversa com mais de 200 mensagens.** É o limite de
`limiteMensagensSessao`. Nenhuma sessão real chegou perto, então o
truncamento da retomada nunca foi exercitado com dado verdadeiro.

**Como testar a interface por automação, sem cair na armadilha:** input
controlado do React só atualiza estado quando o `onChange` sintético dispara.
`type` e `fill` de ferramenta de automação **não** disparam — o texto aparece
no DOM, o estado continua vazio, o `Enter` sai pelo early-return, e a
conclusão errada é "o app está quebrado". Use:

```js
const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value').set;
setter.call(campo, 'texto');
campo.dispatchEvent(new Event('input', { bubbles: true }));
campo.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
```

E leia o DOM numa chamada **separada** da que dispara o evento: o React aplica
a atualização depois do handler, não dentro do mesmo bloco síncrono.

---

## Parte 3 — o que só o Arthur pode testar

Não tente concluir nada sobre estes itens sem ele. Peça, espere, e registre o
que ele responder.

**3.1 CA7 — aprovar do celular, pela VPN.** O único critério de aceitação que
ficou sem verificação. Exige o celular no tailnet. O que ele deve fazer: abrir
o James pelo celular, pedir algo que exija ferramenta perigosa, e confirmar
que o pedido aparece **no desktop** para aprovação. O celular é perfil
`Restrito` (qualquer origem que não seja loopback), então toda ferramenta
perigosa dele vira pedido de aprovação. Nenhum cliente na VPN foi testado
nesta fase — nem esse caminho, nem o layout no aparelho real.

**3.2 O shell Wails.** Tudo que foi verificado rodou em navegador. No
navegador, `lib/janela.ts` falha silenciosamente (a porta 17313 do servidor de
controle não existe) e vira no-op — foi observado como `ERR_CONNECTION_REFUSED`
no console, e é o comportamento documentado. Consequência: **o arraste do orbe
e o redimensionamento da janela nativa nunca foram exercitados nesta fase.**
E há uma mudança concreta: a janela expandida passou de 800 para **1100** de
largura (`LARGURA_EXPANDIDA` em `shell/orbe.tsx`), para caber o layout de três
colunas. Ninguém viu isso numa janela de verdade. O Arthur deve rodar
`executar.bat` ou `wails dev` e conferir: o orbe arrasta, o painel abre no
tamanho certo, e ao fechar a janela volta para 100x100 sem sair da tela.

**3.3 A aparência.** O CSS do orbe foi extraído de `App.css` para
`shell/orbe.css` byte a byte, e o resto do CSS antigo foi apagado. A intenção
era não mudar nada visualmente, mas isso não foi comparado contra um "antes".
Só o Arthur sabe se está como era.

**3.4 CA8 no aparelho real.** Foi verificado com viewport de 375px no
navegador — trilha em `row`, painel em `column-reverse`, zero rolagem
horizontal. Não é o mesmo que o celular dele.

---

## Parte 4 — achados abertos, com severidade

| # | Achado | Severidade | Onde |
|---|---|---|---|
| 1 | **R10 da Fase 5 sem causa raiz.** Encadear `ver_tela` com um clique no mesmo turno trava indefinidamente, sem erro e sem timeout | Alta | Fase 5, R10 |
| 2 | **Playwright não escrito.** Item de escopo da seção 2.1 não entregue; toda verificação de front volta a ser manual | Alta | spec 10.1 |
| 3 | **`GET /api/sessoes` sem limite nem paginação.** A aba chegou a renderizar 1646 botões | Média | spec R9 |
| 4 | **A suíte Go escreve no banco de trabalho e nunca limpa.** É a origem do lixo do item 3 | Média | spec R9 |
| 5 | **Duração de ferramenta não é exibida.** O CA2 pedia; `iniciadaEm` existe no estado e ninguém consome | Baixa | spec 10.1 |

Sobre o item 1, o que já foi descartado como causa: `elementos.varrer()`
isolado (roda em menos de 1s), a API do Gemini (respondendo normal durante o
travamento), e — descoberto nesta fase — a chamada ao provider, que **tem**
deadline de 30s (`loop.go:88` mais `main.go:115`, alimentado pelo
`config.toml`). O handoff da Fase 5 afirma que não há timeout; está errado.
Sobram device, hub e a leitura do WS. A ação concreta sugerida continua sendo
a reprodução isolada em Python: `elementos.varrer()` seguido imediatamente de
`uiautomation.ControlFromPoint()` na mesma thread.

---

## Armadilhas de ambiente

- **Docker Desktop trava** com sockets WSL2 obsoletos. `Remove-Item` não
  resolve. O conserto está no handoff da Fase 5, seção de ambiente.
- **O race detector não funciona nesta máquina** — o TSan falha ao alocar.
  Reproduzido em teste alheio ao código sob revisão, então é limitação do
  ambiente. Se for falar de concorrência, diga que é por inspeção.
- **`gofmt` acusa arquivos por CRLF** por causa do `core.autocrlf` local. Os
  blobs commitados são LF e passam limpo. Não é achado.

---

## Como reportar ao Arthur

Ele é engenheiro sênior e prefere caminho recomendado a leque de opções.
A regra do `CLAUDE.md` dele vale integralmente: **não declare que algo
funciona sem ter rodado o comando e lido a saída nesta mensagem.** Se não deu
para verificar, diga que não verificou — não diga "deve funcionar".

Separe com clareza, no relatório final: o que você conferiu e passou, o que
você conferiu e falhou (com a saída real), e o que depende dele. O objetivo
que ele declarou é "deixar tudo 100%" — para isso ele precisa saber
exatamente qual é a lista de coisas que faltam, não uma sensação de que está
tudo bem.
