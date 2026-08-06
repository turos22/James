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

## O que continua em aberto

Nenhuma das hipóteses testadas explica "o card nunca apareceu" isoladamente — os dois bugs confirmados quebram a **ação** de aprovar (Bug 1) e encurtam a **janela** de aprovação (Bug 2), mas não a **visibilidade** do card em si, que teoricamente deveria ter aparecido.

Candidato não testado, por falta de acesso ao vivo: `ClienteWS` ([`ws.ts`](../../../web/src/lib/ws.ts)) **não tem lógica de reconexão nenhuma**. Se o WebSocket do desktop cair por qualquer motivo (rede, suspensão da máquina, restart do `jamesd`) depois que a página carregou, a conexão fica morta permanentemente, sem nenhum indicador visual de desconexão — o `Clientes.Remover` já tira o cliente do hub no lado do servidor (via `defer` em `ws()`), então nenhum broadcast futuro alcançaria esse desktop, e o usuário não teria como saber. Não foi possível confirmar se foi isso que aconteceu no teste do Arthur — fica registrado para descartar ou confirmar no reteste ao vivo.

## Correção aplicada nesta sessão

Bug 1 e Bug 2 corrigidos com teste (ver commit associado a esta task). A causa de "card nunca aparece" continua sem confirmação — **reteste ao vivo necessário** (celular + desktop) depois da correção, pra ver se os dois bugs já resolvem o sintoma observado ou se sobra algo pra investigar (nesse caso, o próximo passo é instrumentar `BroadcastAprovacaoRequerida`/`Enviar` com log, já que hoje o erro de escrita no socket é descartado em silêncio: `_ = c.Enviar(...)`).
