# Handoff — Fase 5 fechada (visão e controle de tela)

Escrito em 2026-08-08, ao fim da verificação ao vivo (Task 15) que fecha a
fase. Uma nova conversa deve ler este arquivo inteiro antes de fazer
qualquer coisa.

## Onde está tudo

- **Handoff de início desta fase** (contexto histórico, achados que
  motivaram o escopo): `docs/superpowers/HANDOFF-inicio-fase-5.md`
- **Spec**: `docs/superpowers/specs/2026-08-07-james-fase-5-maquina-fisica-design.md`
  — 15 decisões (D1-D15), 10 riscos (R1-R10), seção 10.1 tem o resultado
  detalhado da verificação ao vivo com evidência de cada critério
- **Plano**: `docs/superpowers/plans/2026-08-07-james-fase-5-maquina-fisica.md`
  — 15 tasks, todas fechadas, subagent-driven-development com dois
  estágios de revisão por task (spec compliance + qualidade)

## Estado: 15 de 15 tasks fechadas, executadas em `main` direto (sem worktree)

Todo o trabalho de código está em `main`, entre os commits `3a9c124`
(Task 1) e `03c2cf3` (correções do CA4, último commit de código antes do
fechamento em `20b419d`). Cada task passou por implementador → revisor de
conformidade → revisor de qualidade, geralmente 2-4 rounds de correção por
task — o processo achou bugs reais em praticamente toda task, não só
nitpick.

**O que existe agora, que não existia no início da fase:**

- `dispositivo/james_device/superficies.py` — geometria de monitor,
  conversão de coordenada local↔global, ativação de DPI awareness
- `dispositivo/james_device/captura.py` — screenshot por superfície, PNG
  em resolução nativa
- `dispositivo/james_device/elementos.py` — varredura da árvore UI
  Automation, formatação da lista de elementos, teto de 80 linhas
- `dispositivo/james_device/tela.py` — ganhou as ações `ver_tela` e
  `clicar_elemento`, e o clique por pixel cru (`click`) agora valida
  limites, converte coordenada e reporta o que atingiu de verdade
- `internal/ferramentas/vertela.go` — sexta ferramenta do registro
- `internal/ferramentas/tipos.go` — interface `FerramentaComImagem`
  (contrato aditivo, mesmo padrão de `ExigeConfirmacao`)
- `internal/dispositivo/hub.go` — `AcaoTela`/`Resultado` carregam
  superfície, elemento e imagem; timeout por ação
- `internal/memoria/janela.go` — custo de imagem correto (ver CA3 abaixo)
- `internal/agente/gemini/mapeamento.go` — imagem de resultado de
  ferramenta não é mais descartada em silêncio
- `scripts/verificar_tokens_imagem/` — ferramenta de reverificação contra
  a API real, fica no repo pra quando o Google mudar o billing de novo
- `prompts/sistema.md` — item 2 nomeia `ver_tela` e a sequência certa

## O que a verificação ao vivo (Task 15) provou — e mudou

Isso não foi formalidade. Rodei `jamesd` + device Python + Postgres reais
contra o desktop físico, com os dois monitores de verdade
(`DISPLAY1` 1920x1080, `DISPLAY7` 1920x1080 @ 150%, origem `(1920,255)`).
Toda leitura de resultado foi pelo Postgres via REST
(`/api/sessoes/{id}/mensagens`), nunca confiando no frontend — ver nota de
metodologia abaixo, é importante.

### CA3 — a fórmula de custo de token estava errada

A tabela de tiling que eu tinha lido da documentação do Gemini
(`ai.google.dev/gemini-api/docs/image-understanding`) não vale pra
`gemini-2.5-flash` nesta chamada. `CountTokens` não serve pra verificar
isso — devolve um valor fixo (~259) pra **qualquer** dimensão de imagem,
testado até 8000x4000, sem aplicar tiling nenhum. Só `GenerateContent`
real, lendo `UsageMetadata.PromptTokensDetails` (que isola a modalidade
"imagem" do resto do prompt), revelou o custo verdadeiro: **258 tokens
fixos**, confirmado em 300x200, 384x384, 385x385, 960x540, 1920x1080,
3840x2160 e 5120x2880 — sempre o mesmo valor.

Corrigido em `ff0b2b1`: `EstimarTokensImagem` virou função trivial
(constante `custoFixoImagem = 258`), a maquinaria de tile morta foi
removida, os testes que assumiam 1032/1548 tokens foram corrigidos pro
valor real.

### CA4 — três bugs reais achados e corrigidos, o mecanismo funciona de ponta a ponta

`ver_tela` real devolveu imagem + lista de elementos reconhecíveis (botões
da Calculadora, abas do Chrome, itens da barra de tarefas) num desktop
real. Corrigido em `03c2cf3`:

1. **`TIPOS_INTERESSANTES` usava nomes sem sufixo** (`"Button"`), mas a
   biblioteca `uiautomation` devolve `ControlTypeName` com sufixo
   (`"ButtonControl"`) — nunca checado contra a biblioteca real desde que
   foi escrito. Efeito: `varrer()` sempre devolvia zero elementos em
   qualquer app, **silenciosamente**, sem erro nenhum.
2. **Sem teto de elementos** — um desktop com poucos apps abertos já
   produz 244 elementos (a maior parte da barra de tarefas, não da janela
   em foco, porque D14 varre todas as janelas visíveis da superfície de
   propósito). Era o risco R2 que a Task 10 tinha deixado registrado e não
   fechado. Teto de 80 linhas, com aviso honesto de quantos ficaram de
   fora.
3. **Nomes de controle com quebra de linha embutida** — relógio, indicador
   de rede e indicador de microfone da bandeja do sistema têm `.Name` com
   `\n`/`\r\n` de verdade (texto de tooltip embutido). Quebrava a promessa
   de "uma linha por elemento". Corrigido colapsando whitespace.

### CA5, CA7, CA8 — passaram sem exigir mudança de código

- **CA5**: clique por nome (`clicar_elemento`) acertou o botão "Um" da
  Calculadora — `cliquei em "Um" (ButtonControl) em (1163,761) de DISPLAY1`.
- **CA7**: clique em coordenada crua devolveu `"que é um PaneControl sem
  nome"` em vez do sucesso incondicional de antes. O ramo "nenhum elemento
  identificável" (quando `ControlFromPoint` devolve `None`) não foi
  exercitado ao vivo — no desktop real, quase sempre existe algum
  controle em qualquer ponto, então esse ramo é mais raro do que o design
  assumia. Continua coberto só por mock (Task 12), não é falha, é limite
  honesto da verificação.
- **CA8**: `ver_tela` sem argumento, com a janela em foco de fato no
  `DISPLAY7`, declarou `"superficie: DISPLAY7 1920x1080 escala 150%"` —
  não `DISPLAY1`. Clique num elemento daquela superfície (`"Iniciar"`)
  converteu a coordenada local `(564,1044)` corretamente e acertou o
  alvo.

### CA6 — o critério mais importante, e o que a fase inteira existe pra fechar

Pedido de clique num elemento de uma varredura antiga, depois de a janela
em foco mudar, foi recusado com `"a tela mudou desde o ver_tela — chame
ver_tela de novo"`. Testado duas vezes:

1. Trocando de janela de propósito (Calculadora → VS Code).
2. **Organicamente** — sem eu forçar nada: o texto de um widget da bandeja
   (previsão do tempo) mudou entre a varredura e o clique, e a checagem
   de identidade de retângulo em `_revalidar` pegou isso sozinha, com a
   mensagem `"outro controle está nesse ponto agora — a tela mudou desde
   o ver_tela"`.

Nenhum clique real aconteceu em nenhum dos dois casos. Isso é a prova de
que o Anexo A #20 original (clique sempre reportava sucesso, mesmo
errado) está morto.

## Achado real, não corrigido — R10 na spec

**Encadear `ver_tela` seguido de uma ação de clique no MESMO turno do
modelo trava o turno indefinidamente** — sem erro, sem nenhum timeout
disparando, CPU zerada nos dois lados (Go e Python). Reproduzido 3 vezes,
com restart completo do `jamesd`/device entre as tentativas (não foi
sessão contaminada).

O que já descartei como causa:
- **Não é `elementos.varrer` isolado** — roda em menos de 1 segundo,
  inclusive numa thread separada simulando o modelo de execução real do
  device.
- **Não é a API do Gemini** — testada em paralelo durante o travamento,
  respondendo normal (`go run ./scripts/verificar_tokens_imagem` completou
  em 13s incluindo compilação).

Em turnos **separados** (uma mensagem só pra `ver_tela`, outra só pra
clicar) sempre funcionou, rápido e consistente — foi assim que todos os
CA4-CA8 foram fechados. É como o item 2 do `prompts/sistema.md` já
orienta o modelo a operar, então isso não bloqueia o uso normal. Mas se o
Gemini decidir encadear as duas chamadas na mesma resposta (o que ele pode
fazer, nada no código impede), o turno trava pra sempre.

Hipótese mais provável, não confirmada: interação COM/UIA (reentrância de
apartamento STA, ou handle expirando) quando duas chamadas UIA acontecem
em sequência apertada na mesma thread do device, sem o intervalo real que
uma resposta do Gemini introduz entre turnos separados.

**Duas coisas concretas pra próxima sessão que pegar isso:**
1. Reproduzir de forma isolada, fora do device real — talvez um script
   Python que chama `elementos.varrer()` e, imediatamente em seguida (sem
   yield, sem sleep), `uiautomation.ControlFromPoint()` na mesma thread,
   pra ver se o travamento aparece sem precisar de todo o pipeline WS.
2. Adicionar `context.WithTimeout` explícito nas chamadas ao Gemini em
   `internal/agente/gemini/gemini.go` — hoje não tem deadline nenhum. Não
   foi a causa raiz medida aqui, mas é uma falta de rede de segurança real
   que este achado expôs: se qualquer chamada de rede travar por qualquer
   motivo, hoje nada força o timeout.

## Nota de metodologia — o frontend não é confiável pra verificação

O SPA atual (`web/dist`) não renderizou nenhuma resposta de forma
confiável durante toda a sessão de verificação: `Enter` simples no campo
de texto não disparava o handler de envio (precisou de um
`KeyboardEvent('keydown')` disparado via JS), e as respostas do assistente
não apareciam na tela mesmo depois de completarem no backend (confirmado
lendo o Postgres — a mensagem existia, só não subia pra tela). Isso é uma
lacuna real do frontend atual, fora do escopo desta fase (front-end novo é
Fase 6), mas registro porque quase me fez concluir "está travado" quando
na verdade tinha terminado rápido e só não apareceu na tela.

**Regra pra próxima sessão de verificação ao vivo, aqui ou em qualquer
outra fase**: não presumir que o SPA funciona só porque o backend
funciona. Ler o Postgres direto (`GET /api/sessoes`,
`GET /api/sessoes/{id}/mensagens`, com `Chamadas` em base64) é mais lento
mas é a fonte da verdade.

## Ambiente confirmado nesta sessão (2026-08-08)

- **Docker Desktop travou 3 vezes seguidas** com o mesmo tipo de erro
  (socket WSL2 obsoleto/corrompido bloqueando um serviço interno
  diferente a cada vez — primeiro "Inference manager", depois "Secrets
  Engine"). `Remove-Item` do PowerShell não consegue apagar esses
  arquivos (são sockets Unix criados pelo WSL2, o Windows não sabe lidar
  com eles diretamente). O que funcionou:
  ```
  wsl -d Ubuntu -e find /mnt/c/Users/<usuario>/AppData/Local -maxdepth 3 \
    \( -iname '*docker*sock*' -o -iname 'dockerInference' -o -iname 'engine.sock' \) \
    -delete
  ```
  seguido de matar os processos do Docker Desktop e reabrir. Se isso
  acontecer nas próximas sessões, esse é o conserto — não perder tempo
  tentando `Remove-Item` de novo.
- `james-postgres` (Docker) já existia de sessões anteriores, subiu
  saudável em segundos assim que o Docker Desktop ficou de pé.
- `.env` com `GEMINI_API_KEY` presente e testado ao vivo (chamadas reais
  ao Gemini, incluindo `GenerateContent` com imagem).
- `go build ./...`, `go vet ./...`, `go test ./...` limpos. Suíte Python
  em `dispositivo/`: 84 testes, todos passando.
- A sessão anterior foi interrompida no meio da verificação ao vivo (o
  processo do Claude Code caiu, derrubando `jamesd`/device em background)
  — precisou resubir tudo do zero. Se isso acontecer de novo, o roteiro
  de subida é: Docker Desktop → `docker ps` confirmar `james-postgres`
  saudável → `go run ./cmd/jamesd` em background → 
  `python -m dispositivo.james_device.main` em background (cwd na raiz do
  repo) → confirmar "conectado ao jamesd" no log do device.

## Regra que vale pra qualquer sessão futura neste projeto

CLAUDE.md do Arthur é claro: nunca declarar algo funcionando/pronto sem
ter rodado o comando e lido a saída **nesta mensagem**. Essa regra foi o
que puxou o fio do CA3 (a fórmula "parecia certa" porque vinha da doc
oficial, e só a medição real mostrou que estava errada por completo) e do
CA4 (o código "parecia certo" porque compilava e os testes com mock
passavam — só a varredura contra uma árvore UIA real revelou que
`TIPOS_INTERESSANTES` nunca batia com nada). As duas correções mais
importantes desta fase só existem porque alguém rodou o negócio de
verdade em vez de confiar na leitura do código.

## O que vem depois

A spec original do James lista **Fase 6 — front-end novo** como a
próxima, e o objetivo final do Arthur (app nativo no celular, implantação
em nuvem) fica fora de pauta até Fases 5 e 6 fecharem — Fase 5 está
fechada agora, falta só a 6.

Antes de começar a Fase 6, vale decidir com o Arthur:
1. **R10 é bloqueante pra Fase 6, ou investiga em paralelo?** A Fase 6
   reescreve o frontend, que é justamente a peça que escondeu o problema
   de renderização nesta sessão — pode fazer sentido resolver os dois
   juntos.
2. A nota de metodologia sobre o frontend não renderizar respostas é
   *exatamente* o tipo de bug que motivaria a Fase 6 a existir — vale
   levar essa evidência concreta pro brainstorm dela.
