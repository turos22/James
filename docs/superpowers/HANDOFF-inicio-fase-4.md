# Handoff — Fase 3 fechada, começando a Fase 4

Escrito em 2026-08-06, no fim da execução da Fase 3 (já mergeada em `main`)
e antes de qualquer trabalho da Fase 4 começar. Uma nova conversa deve ler
este arquivo inteiro antes de fazer qualquer coisa.

## Onde está tudo

Não tem worktree ativo — Fase 3 rodou direto em `main`, e por padrão a Fase
4 também roda, mas **isso é uma decisão a confirmar com o Arthur no
brainstorm**, não presumir (as Fases 1 e 3 usaram padrões diferentes:
worktree isolado vs. `main` direto — os dois já foram usados neste projeto).

- **Handoffs anteriores** (contexto histórico, ainda válidos):
  `docs/superpowers/HANDOFF.md` (Fase 0),
  `docs/superpowers/HANDOFF-fase-1-execucao.md` (execução da Fase 1),
  `docs/superpowers/HANDOFF-inicio-fase-3.md` (início da Fase 3),
  `docs/superpowers/HANDOFF-fase-3-execucao.md` (fechamento da Fase 3 —
  leia este em especial, tem um item ainda em aberto, ver seção própria
  abaixo).
- **Specs**: `docs/superpowers/specs/2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`
  (spec original — é a única fonte que já fala de Fase 4, nas seções
  citadas abaixo) e `docs/superpowers/specs/2026-08-05-james-fase-3-qualidade-das-ferramentas-design.md`
  (spec da Fase 3).
- **Plano da Fase 3**: `docs/superpowers/plans/2026-08-05-james-fase-3-qualidade-das-ferramentas.md`
  (20 tasks, todas fechadas).

## Estado do projeto: Fases 0, 1 e 3 completas

| Fase | Conteúdo | Status |
|---|---|---|
| 0 | Fundação: segredos, git, testes, Docker | ✅ fechada |
| 1 | Núcleo: back-end como serviço | ✅ fechada, mergeada |
| ~~2~~ | *vaga — absorvida pela 1 (Postgres) e pela 4 (pgvector)* | não existe |
| 3 | Qualidade das ferramentas | ✅ fechada, mergeada |
| **4** | **Obsidian como cérebro de contexto (RAG, pgvector)** | 🔶 próxima |
| 5 | Máquina física: controle de tela e voz | pendente |
| 6 | Front-end novo | pendente |

**O que existe hoje:**
- `cmd/jamesd` — back-end Go de serviço: REST + WebSocket, Postgres, loop do
  agente (Gemini), 4 ferramentas com comportamento consertado na Fase 3
  (`executar_comando_sistema` com timeout/truncagem/confirmação para
  escrita, `perguntar_ao_claude_code` headless com sessão persistente e
  confirmação sempre, `pesquisar_internet` com saída formatada e marcada,
  `manipular_interface` ainda com coordenadas adivinhadas — isso é Fase 5).
- `internal/memoria/janela.go` — **é o ponto de encaixe da Fase 4**, ver
  seção própria abaixo.
- `internal/armazenamento/migracoes/0001_inicial.sql` — única migração que
  existe hoje: tabelas `sessoes`, `mensagens`, `aprovacoes`. Nenhuma coluna
  ou extensão de embedding, nenhum `pgvector` instalado ainda — a Fase 4
  começa de uma base limpa nesse quesito.
- `dispositivo/james_device/` — cliente Python na máquina física.
- `web/` — SPA React servida pelo `jamesd`. `shell/` — invólucro Wails.
- `cmd/migrar-historico` — importou o `Athena.db` antigo pro Postgres
  (rodado uma vez, na Fase 1).

## Item aberto da Fase 3, não é bloqueante pra Fase 4 mas fica registrado

O bug da aprovação remota (celular pedindo ação perigosa, desktop aprovando)
teve três causas raiz encontradas e corrigidas com teste nesta sessão —
detalhe completo em `docs/superpowers/HANDOFF-fase-3-execucao.md` e no spike
`docs/superpowers/spikes/2026-08-06-bug-aprovacao-remota.md`. O Arthur
confirmou que a conectividade do celular voltou (era desconexão do
Tailscale, sem relação com o código), mas **não foi confirmado ao vivo que o
fluxo de aprovação em si funciona de ponta a ponta pelo celular** depois das
correções — ele decidiu deixar esse reteste pra depois. Não é problema da
Fase 4, só não tratar como "verificado" se o assunto voltar à tona.

## Objetivo final do Arthur — fora de escopo até o fim de TODAS as fases

Em 2026-08-06 o Arthur declarou explicitamente que o objetivo final é
instalar o James como **aplicativo nativo no celular**. Ele foi claro que
isso só entra em pauta **depois que as Fases 4, 5 e 6 fecharem** — não
antecipar essa conversa, não sugerir trabalho nela, mesmo que pareça
relacionado ao que a Fase 4 ou 6 estão fazendo.

## O que a spec original já disse sobre a Fase 4 — ponto de partida real

Do spec de 03/08 (seção "Consequência de D6" e tabela "Não entra"):

> **Consequência de D6:** a Fase 2 (migração para Postgres) deixa de existir
> como fase separada. O `pgvector` e o schema de embeddings migram para a
> Fase 4, junto com o design do RAG — **desenhar granularidade de chunk
> antes de desenhar o RAG seria adivinhação.**

Ou seja: a spec original **deliberadamente não fixou** granularidade de
chunk, schema de embedding, nem estratégia de retrieval — isso é trabalho
de design da própria Fase 4, não uma lacuna a preencher de memória.

Da seção 4.6 (`memoria/`), sobre o ponto de encaixe:

> Hoje `app.go:359` envia **o dia inteiro** em toda requisição, ficando mais
> caro e mais lento a cada mensagem. **A Fase 4 pluga o Obsidian aqui.**

**Verificado nesta sessão:** `internal/memoria/janela.go` hoje é uma janela
de recência pura — corta as últimas N mensagens que cabem num orçamento de
tokens (`Janela{OrcamentoTokens}.Montar(mensagens) []Mensagem`), sem nenhuma
recuperação semântica. É literalmente o ponto onde a Fase 1 corrigiu o "dia
inteiro" (bug #4 do Anexo A original) trocando por um corte de recência —
e é esse mesmo pacote que a Fase 4 estende ou substitui com retrieval
semântico. Ler `internal/memoria/janela.go` e `janela_test.go` antes de
propor design.

Da tabela "Não entra" da Fase 1: `Obsidian, RAG, pgvector | Fase 4` —
confirma que nada disso foi tocado nas Fases 1 ou 3.

## O `Vault/` — intocado, exatamente onde a spec disse que estaria

Verificado nesta sessão, com `ls`, `git status` e leitura direta — não é
suposição:

- `Vault/` existe em `C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts\Vault`,
  fora do `.gitignore` (exceto a subpasta abaixo).
- `Vault/Contexto_Core.md` — **rastreado no git**, é o system prompt da
  antiga "Athena" (pré-rename), com personalidade e diretrizes próprias
  (menciona coordenadas fixas 1920x1080, autonomia total, etc. — conteúdo
  antigo, não necessariamente o que o James deve herdar; a Fase 1 já disse
  que ele "deixa de ser fonte do system prompt", papel que é de
  `prompts/sistema.md` hoje).
- `Vault/Logs_Diarios/` — **gitignorado** (conteúdo pessoal). Contém logs
  diários reais de interação, formato `### 🕒 Interação - HH:MM` seguido de
  `**Arthur:** ...` / `**Athena:** ...`, um arquivo por dia. **O log mais
  recente é de 2026-08-03** — um dia antes da Fase 0 fechar. Confirmado por
  `grep -rl "Vault"` que **nenhum código atual (`internal/`, `dispositivo/`)
  escreve mais nesses arquivos** — é um corpus histórico congelado da era
  pré-rewrite, não algo que o `jamesd` atual está alimentando.
- `Vault/Resumo/` — vazio.
- `Vault/metada.json` — rastreado no git, `{"ultimo_resumo": "", "total_interacoes": 0}`
  — sugere que o mecanismo de resumo da Athena antiga mal chegou a rodar.

**Não encontrado nesta sessão:** o spec original (seção "Estrutura de
diretórios") diz que `obsidian.py` do "Gui" antigo "é preservado fora do
repositório como referência para a Fase 4". Uma busca rasa em
`C:\Users\Arthur\SI\Projetos` e no perfil do Windows não achou esse arquivo.
**Perguntar ao Arthur onde ele está** antes de assumir que não existe mais —
pode ter lógica real de chunking/leitura do Vault que valeria ver antes de
desenhar do zero.

**Decisão de design em aberto pro brainstorm, não resolvida aqui:** o corpus
histórico do `Vault/Logs_Diarios/` (até 2026-08-03) entra no RAG junto com
as `mensagens` novas do Postgres, ou o RAG começa só dali pra frente? É uma
pergunta de produto (o Arthur quer que o James "lembre" de conversas da era
Athena?), não uma decisão técnica a tomar sozinho.

## Workflow a seguir (mesmo das Fases 0, 1 e 3)

1. **`brainstorming`** primeiro — explorar com o Arthur o escopo exato da
   Fase 4: granularidade de chunk, o que vira embedding (mensagens? o Vault
   inteiro? os dois?), quando embeddar (síncrono no turno? job separado?),
   como o retrieval se pluga em `internal/memoria` (estende `Janela` ou é
   um mecanismo novo ao lado dela?), e a pergunta do corpus histórico acima.
2. **`writing-plans`** — depois do brainstorm aprovado, plano task-por-task
   no mesmo formato das fases anteriores.
3. **`subagent-driven-development`** — implementador → revisor de spec →
   revisor de qualidade por task, igual às 45 tasks já fechadas neste
   projeto (25 da Fase 1, 20 da Fase 3). Esse processo já achou bugs reais
   que review de código sozinho não pegaria nas duas fases anteriores — não
   pular, mesmo sob pressa.

**Decidir com o Arthur no brainstorm:** worktree isolado (como a Fase 1) ou
direto em `main` (como a Fase 3)? Não presumir.

## Ambiente confirmado nesta sessão (2026-08-06, `main`)

- Postgres real: container `james-postgres`, `127.0.0.1:5433`, saudável.
  Nenhuma extensão `pgvector` instalada ainda — confirmar se precisa ser
  habilitada na imagem/`docker-compose.yml` como parte da Fase 4.
- `docker-compose.yml`, `.env` e `config.toml` já existem e funcionam.
- Tailscale MagicDNS ativo (`desktop-dc358rs.taila95a1d.ts.net`), usado pro
  celular alcançar o `jamesd` — não deve precisar de nenhuma mudança pra
  Fase 4, é infraestrutura já estável.
- `executar.bat` na raiz é o launcher de conveniência do Arthur (sobe
  Postgres, builda a SPA, roda `jamesd`, abre `wails dev`). Manter
  atualizado se o fluxo de teste mudar.

## Regra que vale pra qualquer sessão futura neste projeto

CLAUDE.md do Arthur é claro: nunca declarar algo funcionando/pronto sem ter
rodado o comando e lido a saída **nesta mensagem**. Isso valeu as três fases
anteriores inteiras — a Fase 3 em particular só fechou depois de três bugs
reais serem achados e corrigidos com teste rodado de verdade contra o
`jamesd` real, não por inspeção de código. Manter esse padrão na Fase 4,
que mexe em algo novo (embeddings, retrieval) sem nenhum precedente de teste
já estabelecido neste projeto — vale desenhar a estratégia de teste com
cuidado extra no brainstorm.
