# Design de referência — painel de controle do James

Projeto Next gerado no v0.dev e fornecido pelo Arthur em 2026-08-11 como **alvo
visual** da Fase 7. Guardado aqui porque a spec aponta para ele e porque
documento em disco é o que sobrevive ao reset de contexto.

## Isto não é código para compilar

Não roda, não é instalado e não entra no `web/`. É Next 16 + React 19 +
shadcn; o James é Vite + React 18 embutido no binário Go por
`//go:embed all:dist`. A decisão de **portar o design em vez de migrar para
Next** está registrada como D2 em
[`../../specs/2026-08-11-james-fase-7a-aplicativo-design.md`](../../specs/2026-08-11-james-fase-7a-aplicativo-design.md).

O que se aproveita daqui: layout, tokens de cor (`app/globals.css`), tipografia,
espaçamento, ícones e o desenho do orbe (`components/panels/james-panel.tsx`).

## Onde o alvo e a implementação divergem de propósito

| Referência | Fase 7A | Motivo |
|---|---|---|
| Orbe com botão Reproduzir e oscilador sintético | Sem botão; reage ao áudio real de saída do TTS | Pedido do Arthur (D5) |
| "online" verde fixo no chat | Estado real da conexão WS | Bolinha fixa mentiria durante uma queda |
| Cinco abas e nada mais | Aprovação, atividade e `ver_tela` moram na aba James | Existem e funcionam; o design não previu (D3) |
| Memória com editor e mini-grafo estático | Aba atual, sem mudança na 7A | Vault não tem links hoje; grafo é 7C |
| DashBoard com dados de exemplo | Estado vazio franco | Nada disso é medido hoje; instrumentação é 7B |
| Três círculos decorativos | Controles reais de janela no Wails; ausentes no navegador | D4 |

Arquivos `placeholder-*` e o lock file do pnpm foram removidos na cópia.
