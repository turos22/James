# Handoff — Fase 6 fechada (front-end novo)

Escrito em 2026-08-10, ao fim da verificação ao vivo. Uma nova conversa deve
ler este arquivo inteiro antes de fazer qualquer coisa.

## Onde está tudo

- **Spec**: `docs/superpowers/specs/2026-08-09-james-fase-6-frontend-novo-design.md`
  — 12 decisões (D1-D12), riscos, e a seção **10.1** com o resultado da
  verificação ao vivo critério por critério, inclusive o que não passou
- **Plano**: `docs/superpowers/plans/2026-08-09-james-fase-6-frontend-novo.md`
  — 20 tasks; 19 executadas, mais uma task 21 que nasceu da verificação
- **Handoff da fase anterior**: `docs/superpowers/HANDOFF-fase-5-execucao.md`

## Estado: 35 commits em `main`, de `c064cbf` a `41e310b`

Executado com `subagent-driven-development`: implementador → revisor de
conformidade → revisor de qualidade por task, direto em `main` (D11).

**O que existe agora, que não existia:**

- `internal/api/eventos.go` — payloads dos eventos como structs nomeadas, e a
  emissão compartilhada entre o caminho de texto (`ws.go`) e o de voz
  (`dispositivo.go`), via uma função `emitir` injetada
- `internal/memoria/vault.go` — `Listar()` devolve as notas estruturadas
- `internal/api/rest.go` — `GET /api/memoria`
- `internal/api/ws.go` — `reconstruirHistorico`, `aplicarRetomada`,
  `tratarRetomar`, e o comando WS `retomar`
- `internal/armazenamento/postgres/mensagem.go` — `ListarRecentesPorSessao`
- `web/src/` reescrito: `lib/eventos.ts`, `lib/ws.ts` (reconexão, fila,
  estado), `estado/turno.ts` (redutor puro), `shell/orbe.tsx` + `orbe.css`,
  `trilha.tsx`, `abas/` (conversa, tela, atividade, memória, sessões),
  `visualizador.tsx`, e `App.tsx` como composição
- Tailwind 4.3.3. `App.css` e `CardAprovacao` morreram

Front: 1.048 linhas de código e 962 de teste, 71 testes verdes. Go: suíte
inteira verde contra Postgres real.

## O que a verificação ao vivo provou

Detalhe por critério está em 10.1 da spec. Resumo: **CA1 a CA6 e CA8 passam**,
verificados contra `jamesd` real, device Python conectado, Postgres no Docker
e chamadas reais ao Gemini. **CA7 não foi exercitado** — exige o celular do
Arthur no tailnet, e nada foi verificado ali.

O que mais importa: derrubar o `jamesd` com a página aberta agora produz
`Reconectando ao James... (tentativa 3)` e trava o campo, em vez de a tela
ficar muda. E `ver_tela` chega até a interface — a aba Tela mostrou 80
elementos reais da máquina mais a linha de truncagem.

## Achados abertos, não corrigidos

1. **Duração de ferramenta não é exibida.** O CA2 pedia; `iniciadaEm` existe
   no estado e ninguém consome.
2. **1646 sessões na aba Sessões.** `GET /api/sessoes` não tem limite nem
   paginação, e a suíte Go escreve no banco de trabalho sem limpar. Ver R9.
3. **Playwright não foi escrito.** Item de escopo da seção 2.1 não entregue.
   A verificação ao vivo cobriu o mesmo terreno, mas à mão — não se repete
   sozinha, então a próxima fase que mexer no front não tem rede.
4. **R10 da Fase 5 continua aberto** e intocado: encadear `ver_tela` com um
   clique no mesmo turno trava. Durante esta verificação eu pedi de propósito
   só a leitura da tela, nunca leitura mais clique.

## Correções de fato em documentos anteriores

- **O timeout do Gemini existe.** O handoff da Fase 5 afirma que não há
  deadline nas chamadas ao provider. Há: `loop.go:88` aplica
  `context.WithTimeout` e `cmd/jamesd/main.go:115` alimenta com 30s do
  `config.toml`. Isso elimina a chamada ao Gemini como suspeita do R10.
- **O R2 desta spec estava errado** e foi marcado como descartado. Eu presumi
  que a suíte pulava o banco por falta de `DATABASE_URL`; o `TestMain` carrega
  o `.env` via `godotenv`, e os testes rodam contra o Postgres real sem SKIP.
- **Metade da nota de metodologia da Fase 5 era artefato de teste.** Aquele
  handoff registrou que `Enter` não disparava o envio no SPA antigo. O mesmo
  sintoma se reproduz no SPA novo: input controlado do React só atualiza
  estado quando o `onChange` sintético dispara, e `type`/`fill` de ferramenta
  de automação não o disparam. A outra metade da nota — respostas que
  completavam no backend e não apareciam — era real, e é o que esta fase
  consertou.

## Lição de processo, com números

O plano tinha 20 tasks e acumulou **doze correções** durante a execução. As
que teriam custado mais caro:

- `ListarPorSessao` é `ORDER BY criada_em ASC`: a retomada carregaria o
  **começo** da conversa, e a janela de memória montaria a cauda desse pedaço
  velho. O modelo continuaria a conversa errada, sem erro nenhum.
- O evento `token` foi removido de `ws.go` e sobreviveu em `dispositivo.go`.
- Map nil em Go serializa como `null`, não `{}` — e o tipo TypeScript
  prometia objeto, então o card de aprovação estouraria em runtime.
- A precedência do visualizador congelaria o painel na primeira captura de
  tela da sessão.
- Apagar o `App.css`, como o plano mandava, deixaria o orbe sem estilo — e o
  orbe é a única coisa clicável com o painel fechado.

**Nenhuma dessas apareceu relendo o documento. Todas apareceram lendo o
código que o documento descrevia, ou rodando o sistema.** O formato da saída
do `ver_tela`, por exemplo, usa espaço duplo entre campos — o plano assumiu
espaço simples, e só um implementador que foi conferir no Python real pegou.

Vale também registrar um erro de relato: um implementador afirmou ter escrito
cobertura de teste que não existia no arquivo. O revisor de conformidade
pegou por grep. Relato errado é mais caro que código errado, porque o
coordenador decide o próximo passo em cima dele.

## Ambiente confirmado (2026-08-09/10)

- Docker Desktop e `james-postgres` de pé sem incidente (diferente da Fase 5).
- `jamesd` sobe com `go run ./cmd/jamesd`; serve loopback `127.0.0.1:8080` e
  VPN `100.96.219.29:8443`.
- **`web/embed.go` usa `//go:embed all:dist`**: a SPA é compilada dentro do
  binário. Mudou o front? `npm run build --prefix web` **e reinicie o
  `jamesd`**, senão você testa a interface velha sem perceber.
- O device Python conecta em `ws://127.0.0.1:8080/ws/dispositivo` e loga
  `escuta ativa iniciada`.
- A cota de uso estourou duas vezes durante a execução. Em nenhuma delas ficou
  trabalho pela metade: cada correção entra por commit próprio do
  implementador, e o working tree foi conferido antes de retomar.

## O que vem depois

As Fases 0 a 6 estão fechadas. O objetivo declarado do Arthur — app nativo no
celular e implantação em nuvem — deixa de estar bloqueado.

Antes disso, vale decidir sobre os quatro achados abertos acima. O mais barato
com maior retorno é o Playwright: sem ele, toda verificação de front vira
trabalho manual de novo. O mais arriscado de ignorar é o R10, que continua
sem causa raiz desde a Fase 5.
