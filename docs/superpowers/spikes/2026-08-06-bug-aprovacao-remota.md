# Spike — diagnóstico do bug da aprovação remota

**Data:** 2026-08-06
**Task:** 18 do plano da Fase 3.
**Reprodução real, pelo Arthur:** `jamesd` rodando, celular conectado via VPN Tailscale, chat funcionando normalmente (mensagem simples enviada e respondida). Ao pedir uma ação que exige aprovação, **o card de aprovação não apareceu no James do desktop** (chat estava **aberto**, não só a bolinha), e o processo ficou travado esperando no celular.

## Hipóteses testadas e refutadas

### H-isOpen: card escondido atrás do chat fechado — REFUTADA

Em [`App.tsx:189-197`](../../../web/src/App.tsx), o `CardAprovacao` só renderiza dentro de `{isOpen && (...)}`. Hipótese inicial: chat fechado no desktop, card nunca visível.

**Refutada pelo Arthur:** o chat estava aberto durante o teste. Continua sendo um gap de UX real (nenhum indicador na bolinha fechada quando há aprovação pendente), mas não é a causa do que foi observado.

### H-CSS: texto do card invisível por contraste — REFUTADA

Hipótese: `.card-aprovacao` não tem regra própria em `App.css` (confirmado por busca — zero ocorrências fora do componente), então o texto herdaria cor padrão do navegador (preto) sobre o fundo escuro do `.chat-window`.

**Testado ao vivo:** subi `web/` via `vite dev`, forcei temporariamente `isOpen=true` e um `pedidoAprovacao` de mentira no estado (edição revertida logo em seguida, nunca commitada), e inspecionei o DOM renderizado de verdade.

`read_page` confirmou que os elementos existem na árvore (parágrafo, JSON, botões Aprovar/Negar). Estilos computados via JS:

```json
{
  "chatWindowBg": "rgba(0, 0, 0, 0)",
  "paragrafo": { "color": "rgb(255, 255, 255)", "opacity": "1", "display": "block", "visibility": "visible" },
  "pre":       { "color": "rgb(255, 255, 255)", "opacity": "1", "display": "block", "visibility": "visible" },
  "botao":     { "color": "rgb(0, 0, 0)", "backgroundColor": "rgb(240, 240, 240)", "opacity": "1" }
}
```

Texto branco sobre o gradiente escuro do `.chat-window` (o `backgroundColor` computado vem transparente porque o fundo real é `background-image: linear-gradient`, não `background-color`), botões com o estilo padrão do navegador (cinza claro, texto preto) — visível em qualquer um dos dois casos. **Refutada**: se o card renderizasse, ele seria visualmente legível.

## Dois bugs confirmados por leitura de código (não hipótese — determinísticos)

### Bug 1 — `dados.id` sempre vazio no evento `aprovacao_requerida`

[`ws.ts:14-20`](../../../web/src/lib/ws.ts): `ClienteWS.onmessage` despacha `fn(envelope.dados)` — **só** o campo `dados` do envelope chega ao manipulador registrado via `.on()`. O `id` do envelope nunca é repassado. Isso é o contrato testado e documentado do próprio `ClienteWS` (`ws.test.ts:31-40`: `recebidos` só contém o valor de `dados`).

[`ws.go`, `criarSolicitarAprovacao`]: o broadcast manda

```go
s.Clientes.BroadcastAprovacaoRequerida(aprovacao.ID.String(), map[string]any{
    "ferramenta": chamada.Nome,
    "argumentos": chamada.Args,
})
```

O id vai só no campo `id` do **envelope**, nunca dentro de `dados`. Mas [`App.tsx:115-117`](../../../web/src/App.tsx):

```ts
cliente.on("aprovacao_requerida", (dados: any) => {
  setPedidoAprovacao({ id: dados.id ?? "", ferramenta: dados.ferramenta, argumentos: dados.argumentos ?? {} });
});
```

lê `dados.id`, que **nunca existe** nesse payload — sempre cai no fallback `""`. Consequência: `pedidoAprovacao.id` é **sempre string vazia**, em 100% das chamadas, sem depender de timing ou rede.

Se o usuário clicasse "Aprovar", `decidirAprovacao` mandaria `enviar("aprovar", "", {})`. No backend, `s.AprovacoesPendentes.resolver("", true)` não encontra `""` no mapa de pendências (a chave real é um UUID) — devolve `false` e **não faz nada**. O canal que `criarSolicitarAprovacao` está esperando nunca recebe o sinal. O pedido fica pendente até o timeout — mesmo que o Arthur tivesse clicado em Aprovar.

Este bug quebra a aprovação de ponta a ponta, independente de o card aparecer ou não.

### Bug 2 — `TimeoutTurno` embrulha o turno inteiro, não só a chamada ao Provider

O comentário em [`rest.go:39-41`](../../../internal/api/rest.go) diz: *"TimeoutTurno limita cada chamada ao Provider dentro do loop do agente"*. O código em [`ws.go:109-114`](../../../internal/api/ws.go) faz o oposto: `context.WithTimeout(ctx, s.TimeoutTurno)` embrulha o `loop.Executar` **inteiro**, e a espera pela aprovação humana (até `TimeoutAprovacaoSegundos`, default 300s) acontece **dentro** desse contexto.

`TimeoutSegundos` (que vira `TimeoutTurno`) tem default de **30s** ([`config.go:87`](../../../internal/config/config.go)). Se o modelo já gastou algum tempo em iterações anteriores antes de decidir chamar a ferramenta perigosa, a aprovação pode ter bem menos que 30s de orçamento real — e `loop.go:48` verifica `ctx.Err()` no início de cada iteração, então o loop inteiro aborta com `context deadline exceeded` assim que o `ctxTurno` vence, não só a chamada ao Provider.

Isso não explica sozinho "card nunca aparece" (o broadcast acontece antes do `select` observar o `ctx.Done()`), mas encurta drasticamente a janela real de aprovação em relação ao que a configuração promete, e o comentário do código está objetivamente errado sobre o que o código faz.

### Bug 3 — descoberto na verificação ao vivo da Task 17, não no teste do Arthur: a mesma conexão trava tentando aprovar a si própria

Ao verificar `perguntar_ao_claude_code` ao vivo (Task 17), sob a decisão D10 (Claude Code exige confirmação **sempre**, inclusive do Privilegiado), toda chamada dessa ferramenta pelo próprio desktop dispara um pedido de aprovação que só o próprio desktop pode responder — e o desktop usa uma única conexão WS pra mandar mensagem e pra aprovar.

`s.ws()` processa cada conexão com um único laço `for { ReadJSON(); switch tipo { case "mensagem": s.tratarMensagem(...) } }`, e `tratarMensagem` roda de forma síncrona, bloqueando o laço até o turno inteiro terminar — inclusive a espera pela aprovação humana. Isso é inofensivo quando quem aprova é **outra** conexão (celular pede, desktop aprova). É uma trava certa quando é a **mesma** conexão: o "aprovar" fica atrás do "mensagem" bloqueado na fila de leitura, e só é lido depois que `tratarMensagem` retorna — o que só acontece depois do timeout de aprovação estourar.

Confirmado ao vivo contra o `jamesd` real: **exatos 5 minutos** (`time` no teste), batendo com o default de 300s. Reproduzido em teste automatizado (`TestWSMesmaConexaoConsegueAprovarSuaPropriaAcao`, timeout curto de 2s pra não travar a suíte) antes de corrigir.

**Corrigido** desacoplando a leitura da conexão em duas partes: um goroutine dedicado lê cada frame do socket continuamente e resolve `aprovar`/`negar` na hora; só `mensagem` é repassada por canal pro processamento sequencial (que continua rodando uma de cada vez, sem introduzir corrida em `historico`/`sessaoID`). Depois do fix, o mesmo cenário resolve em ~40ms.

## O que continua em aberto

Nenhuma das duas hipóteses testadas (chat fechado; contraste CSS) explica "o card nunca apareceu" isoladamente no teste original do Arthur (celular pedindo, desktop devendo aprovar — duas conexões diferentes, não afetadas pelo Bug 3). Bug 1 e Bug 2 quebravam a **ação** e a **janela** de aprovação nesse cenário; nenhum dos três bugs encontrados explica a **visibilidade** do card em si, que teoricamente deveria ter aparecido.

Candidato não testado, por falta de acesso ao vivo: `ClienteWS` ([`ws.ts`](../../../web/src/lib/ws.ts)) **não tem lógica de reconexão nenhuma**. Se o WebSocket do desktop cair por qualquer motivo (rede, suspensão da máquina, restart do `jamesd`) depois que a página carregou, a conexão fica morta permanentemente, sem nenhum indicador visual de desconexão — o `Clientes.Remover` já tira o cliente do hub no lado do servidor (via `defer` em `ws()`), então nenhum broadcast futuro alcançaria esse desktop, e o usuário não teria como saber. Não foi possível confirmar se foi isso que aconteceu no teste do Arthur — fica registrado para descartar ou confirmar no reteste ao vivo.

## Correção aplicada nesta sessão

Bug 1, Bug 2 e Bug 3 corrigidos com teste (commits `43419e5` e `35ac155`). A causa de "card nunca aparece" no cenário celular→desktop continua sem confirmação — **reteste ao vivo necessário** (celular + desktop) pra ver se os bugs já resolvem o sintoma observado ou se sobra algo pra investigar (nesse caso, o próximo passo é instrumentar `BroadcastAprovacaoRequerida`/`Enviar` com log, já que hoje o erro de escrita no socket é descartado em silêncio: `_ = c.Enviar(...)`).

## Task 17 — verificação ao vivo do Claude Code, feita nesta sessão

Sem acesso à UI do desktop, a verificação foi feita conectando direto no WS do `jamesd` (`ws://127.0.0.1:8080/ws`) via um programa Go descartável (nunca commitado), fazendo o papel do cliente Privilegiado: manda a mensagem, aprova automaticamente qualquer `aprovacao_requerida` que aparecer — o mesmo papel que o clique em "Aprovar" no desktop faria.

**Item 1 — autenticação pela assinatura Pro, mesmo com ambiente sequestrado de propósito.** `jamesd` subiu com `ANTHROPIC_BASE_URL=https://exemplo-invalido.test`, `ANTHROPIC_API_KEY` e `ANTHROPIC_AUTH_TOKEN` falsos no próprio ambiente do processo. Chamada a `perguntar_ao_claude_code` completou em 9,5s e devolveu `"OK"` — prova que `AmbienteSemAnthropic` limpou as variáveis antes de invocar o `claude`, e a sessão autenticou pela assinatura real via OAuth, não pelo endpoint falso (que teria falhado ao conectar).

**Item 2 — saída real capturada.** A resposta não é mais a string fixa de disparo; é o texto real devolvido pelo Claude Code (`"OK"`, e depois `"- README.md"` — ver item 3).

**Item 3 — sessão persistente entre chamadas.** Duas mensagens na mesma conexão (mesma sessão do James): a primeira pediu pro Claude Code listar os arquivos de topo do diretório configurado, resposta `"- README.md"`; a segunda perguntou qual foi o primeiro item listado na resposta anterior, resposta `"README.md"` — o Claude Code reteve o contexto entre as duas chamadas via `--resume` no mesmo id de sessão, confirmado ao vivo.

Os três critérios de aceitação da Task 17 estão verificados com evidência real.
