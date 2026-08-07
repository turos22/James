# James — Fase 5: Máquina física (visão e controle de tela)

**Data:** 2026-08-07
**Autor:** Arthur Orosco (design colaborativo)
**Estado:** aprovado, aguardando revisão do documento escrito
**Projeto:** `C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts`
**Base:** [`2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`](2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md) (Anexo A #19 e #20, e a decisão D5 sobre o destino do Python) e [`HANDOFF-inicio-fase-5.md`](../HANDOFF-inicio-fase-5.md). Este documento é autocontido: não é necessário ler os anteriores para implementar a Fase 5.

**Esta fase não é aditiva.** Diferente da Fase 4, ela altera assinaturas existentes em quatro pacotes Go e reescreve um teste que hoje passa. As alterações estão enumeradas em 4 e a justificativa de cada uma está em 3.

---

## 1. Contexto

As Fases 0, 1, 3 e 4 fecharam e estão mergeadas em `main`. A spec original reservou para a Fase 5 o tema "máquina física", com dois itens do Anexo A: #19 (screenshot só enviado quando o texto contém a palavra "tela") e #20 (coordenadas X/Y adivinhadas pelo modelo, 1920x1080 fixo, PyAutoGUI sem DPI awareness). A calibração da wake word com gravações reais também foi adiada para cá.

Quatro verificações feitas nesta sessão redefinem o que a fase precisa ser. Elas estão registradas porque as decisões centrais dependem delas, e porque duas contradizem o enquadramento original.

### 1.1 A visão nunca existiu — o encanamento está pronto e nada o alimenta

O Anexo A #19 descreve o screenshot como algo que existe e é enviado sob condição errada. Isso deixou de ser verdade em algum ponto entre a Athena antiga e hoje. Verificado por leitura de código:

O lado Go está pronto e correto. `agente.Mensagem.Imagem` é campo real ([`tipos.go:36`](../../../internal/agente/tipos.go)); [`janela.go:30`](../../../internal/memoria/janela.go) cobra o custo dele no orçamento, com teste; [`mapeamento.go:16`](../../../internal/agente/gemini/mapeamento.go) monta o `genai.Part` com `InlineData` quando o campo não é vazio. Um `[]byte` de PNG que chegasse ali seria entregue ao Gemini corretamente.

Nada o produz. Buscas por `screenshot|imagem|print|captura|ImageGrab|mss` em `dispositivo/james_device/` não retornam nenhuma captura de tela. Do lado Go, o único ponto de entrada de mensagem de usuário em produção é [`ws.go:126`](../../../internal/api/ws.go), que monta `agente.Mensagem{Papel, Texto}` sem `Imagem` — e a assinatura de `processarTurno` ([`ws.go:115`](../../../internal/api/ws.go)) recebe `texto string`, de modo que **não existe parâmetro por onde uma imagem passaria**.

Consequência: o item 2 do [`prompts/sistema.md:9`](../../../prompts/sistema.md) — "VISÃO PROATIVA: se houver um print da tela, analise antes de agir" — instrui o modelo a usar um recurso que nenhum caminho de código aciona.

Consequência para o desenho: #19 não é "consertar um filtro". É construir o caminho inteiro, do device até o prompt. A fase é maior do que o Anexo A sugeria.

### 1.2 O clique é cego, e o retorno mente sucesso

[`tela.py`](../../../dispositivo/james_device/tela.py) tem 35 linhas. Não chama DPI awareness, não consulta resolução real, não valida limites: executa `pyautogui.moveTo(x, y)` seguido de `click()` no par de coordenadas que chegar.

E [`tela.py:26`](../../../dispositivo/james_device/tela.py) devolve `{"saida": f"clique em ({x},{y})"}` **incondicionalmente** — mesmo quando o clique cai em área vazia. O comentário no topo do arquivo confirma que as coordenadas continuam adivinhadas pelo modelo e que consertar é Fase 5.

Isso reenquadra o #20. O problema não é apenas que o modelo adivinha a coordenada; é que ele **nunca recebe sinal de que errou**. Um loop de agente sem sinal de erro não tem como corrigir a própria trajetória, o que é a explicação mecânica para o padrão de 10 respostas fallback em 11 registrado em `Vault/Logs_Diarios/2026-07-08.md` e citado na spec original.

Portanto a fase tem duas metades obrigatórias, e ambas atacam o mesmo bug: dar visão ao modelo, e fazer o device dizer a verdade sobre o que aconteceu.

### 1.3 A wake word depende de rede a cada três segundos

[`main.py:60-63`](../../../dispositivo/james_device/main.py) é um `while True` que grava 3 segundos de áudio e chama `recognize_google(audio, language="pt-BR")`. Só depois de a transcrição voltar da rede é que [`wake.py`](../../../dispositivo/james_device/wake.py) compara a string contra as cinco variantes de `VARIANTES_PADRAO` ([`wake.py:14`](../../../dispositivo/james_device/wake.py)).

Três consequências, nenhuma delas endereçada por "calibrar a lista": o James só ativa com internet; tudo que é falado perto do microfone sobe para o Google continuamente; e cada ativação paga latência de rede.

A lista de variantes é o remendo. O mecanismo é o problema. Ver D10 para o que esta fase faz a respeito e o que deliberadamente não faz.

### 1.4 A conta de token de imagem erra por cerca de 80x

[`janela.go:30`](../../../internal/memoria/janela.go) cobra `(len(m.Imagem) + 3) / 4` — a mesma régua de 4 caracteres por token usada para texto, aplicada a bytes binários. O comentário nas linhas 23 a 29 é explícito de que isso superestima e que superestimar é o lado seguro para um orçamento.

Verificado contra a documentação oficial do Gemini (`ai.google.dev/gemini-api/docs/image-understanding`), o custo real é por área, não por byte:

- ambas as dimensões ≤ 384 px: **258 tokens**;
- acima disso: `crop = floor(min(largura, altura) / 1.5)`, número de tiles = `ceil(largura/crop) × ceil(altura/crop)`, **258 tokens por tile**.

O exemplo da própria documentação ancora o arredondamento: 960x540 tem `crop = 360`, e `960/360 = 2,67` com `540/360 = 1,5` resultam em **6 tiles (3 × 2)**, cerca de 1.548 tokens.

Um screenshot de 1920x1080 tem a mesma proporção 16:9 e portanto cai nos mesmos 6 tiles, ~1.548 tokens. A fórmula atual cobraria cerca de 125.000 pelo mesmo PNG de 500 KB.

O efeito prático da fórmula errada não é acadêmico. Com o orçamento padrão de 8000 ([`config.go:120`](../../../internal/config/config.go)) e a regra de [`janela.go:43`](../../../internal/memoria/janela.go) de que a última mensagem sempre entra mesmo estourando sozinha, um screenshot entraria sozinho na janela e **despejaria a conversa inteira**. Corrigida a conta, uma tela cheia cabe folgada e sobra orçamento para a conversa.

Modelo em uso confirmado: `gemini-2.5-flash` ([`config.go:116`](../../../internal/config/config.go)).

### 1.5 Item aberto herdado da Fase 3

O bug da aprovação remota (celular pede ação perigosa, desktop aprova) foi corrigido com teste na Fase 3, mas o fluxo de ponta a ponta pelo celular real nunca foi revalidado ao vivo depois das correções. Detalhe em [`HANDOFF-fase-3-execucao.md`](../HANDOFF-fase-3-execucao.md) e no spike [`2026-08-06-bug-aprovacao-remota.md`](../spikes/2026-08-06-bug-aprovacao-remota.md). Continua aberto; não é escopo desta fase, mas D5 encosta nele e por isso está registrado aqui.

---

## 2. Escopo

### 2.1 Entra

1. Captura de tela no device e o caminho completo da imagem até o Gemini.
2. Ferramenta `ver_tela`, sexta do registro, devolvendo imagem **e** lista de elementos clicáveis de uma superfície.
3. Enumeração de monitores como superfícies, com conversão local↔global de coordenada (D14, D15).
3. Clique por elemento via UI Automation, com revalidação antes de agir.
4. DPI awareness no device, e validação de limites no caminho de pixel cru.
5. Retorno honesto do device: erro real quando o clique não pôde ser feito, e identificação do elemento atingido quando foi.
6. Correção da estimativa de token de imagem em `memoria`, de bytes para tiles.
7. Correção do descarte silencioso de imagem no ramo `PapelFerramenta` de `mapeamento.go`.
8. Log passivo de quase-acertos da wake word.
9. Ajuste do item 2 do `prompts/sistema.md` para instruir a sequência real (`ver_tela` antes de `manipular_interface`).

### 2.2 Não entra

- **Troca do mecanismo de wake word por detector local** (Porcupine, openWakeWord). É o conserto certo do problema descrito em 1.3, e está adiado, não cancelado. Justificativa em D10.
- **Marcação numerada sobre o screenshot (Set-of-Mark).** Avaliada no brainstorm e descartada para esta fase. A lista de elementos com id curto entrega o mesmo endereçamento preciso sem o risco de uma tela densa virar duzentas caixas ilegíveis. Registrado para que uma sessão futura não redescubra a opção do zero.
- **Resolver a tensão entre device local e `jamesd` em nuvem.** Registrada em D12 e na seção 9 como restrição conhecida. Decidir aqui seria antecipar a Fase 6.
- **Revalidação ao vivo do fluxo de aprovação pelo celular** (1.5). Continua aberto, fora desta fase.
- **Ampliar o `.gitignore` para a pasta `audios/` inteira.** Gap conhecido, decisão registrada do Arthur de deixar passar. O arquivo novo desta fase entra no `.gitignore` normalmente; o gap antigo não é tocado.

---

## 3. Decisões desta sessão

| # | Decisão | Escolha | Por quê |
|---|---|---|---|
| D1 | Gatilho da visão | Ferramenta `ver_tela`, chamada quando o modelo decide | Reusa o registro de ferramentas endurecido na Fase 3 e paga o custo da imagem só quando ela serve. Mata o #19 na raiz: o gate deixa de ser uma regex sobre o texto do usuário e passa a ser decisão do modelo. As alternativas avaliadas foram print automático a cada turno (paga imagem em 100% das falas, inclusive "que horas são", e manda a tela para a nuvem do Google continuamente) e captura acoplada ao clique (inútil por construção: quando `manipular_interface` foi emitida, a coordenada **já** foi adivinhada) |
| D2 | Mecânica do clique | Clique por elemento via UI Automation; pixel cru mantido como escape, agora com DPI correto e limites validados | É a direção que a spec original já apontou em D5 ("bibliotecas de UI Automation em Python são muito superiores às de Go; é o caminho para consertar o clique na Fase 5"). A coordenada passa a vir do sistema operacional, nunca do palpite do modelo. Cobre app nativo e navegador, que é onde o Arthur declarou que usa. Pixel cru sobrevive porque canvas e jogo não expõem árvore de acessibilidade |
| D3 | Como o modelo referencia um elemento | Aceita **id curto** (`e7`) ou **nome**, e o device revalida antes de clicar | Só id quebra quando a varredura envelhece; só nome quebra em dois casos comuns — elemento sem nome (botão de ícone) e três botões "OK" na mesma tela. O argumento decisivo não é completude: a revalidação é o **mesmo mecanismo** que produz o erro real exigido por 1.2. Escolher A ou B deixaria o buraco do retorno mentiroso aberto |
| D4 | Biblioteca de UI Automation | [`uiautomation`](https://github.com/yinkaisheng/Python-UIAutomation-for-Windows) | Wrapper fino da API COM de UIAutomation, com suporte declarado a MFC, WinForms, WPF, Qt, Firefox, Chrome e Electron. `pywinauto` também tem backend `uia`, mas traz camada de localizar-janela-por-título que não é usada aqui: depois de D14 o alvo é uma superfície inteira, e a enumeração parte da raiz descendo pelas janelas visíveis, não de uma busca por título. **Escolha por encaixe, não por medição: não foi encontrado benchmark comparativo entre as duas.** O device esconde a biblioteca atrás de uma função, então a troca é localizada se ela falhar em app real |
| D5 | Nível de permissão do `ver_tela` | `NivelSeguro`, sem aprovação, disponível a qualquer perfil | Decisão do Arthur, tomada depois de a consequência ser apresentada. **Consequência registrada: o celular enxerga a tela do desktop sem que nada seja aprovado.** A ferramenta não altera estado, o que justifica `NivelSeguro` pela régua de [`tipos.go:12`](../../../internal/ferramentas/tipos.go); o que ela faz é ler dado sensível e enviá-lo à nuvem. Ver R7 |
| D6 | Transporte da imagem | Base64 dentro do envelope JSON que já existe | O protocolo device↔Go já é `{tipo, id, dados}` com `dados` livre ([`conexao.py:115`](../../../dispositivo/james_device/conexao.py)), e `resultado_acao` já é o canal de volta. Não é preciso inventar transporte. Verificado que o limite não bloqueia: `conn.go:924` do `gorilla/websocket` v1.5.3, no cache de módulos, só aplica corte `if c.readLimit > 0`, e o James nunca chama `SetReadLimit` — logo o valor é zero e não há limite. Ver R8 |
| D7 | Caminho da imagem até o Gemini | Interface opcional `FerramentaComImagem` no pacote `ferramentas`, mais `InlineData` no ramo `PapelFerramenta` do mapeamento | O contrato aditivo já é padrão estabelecido no projeto: `ExigeConfirmacao` ([`tipos.go:45`](../../../internal/ferramentas/tipos.go)) existe exatamente para isso, e seu comentário registra a filosofia — "ferramenta que não implementa se comporta exatamente como antes". As outras cinco ferramentas não mudam uma linha. A alternativa (trocar a assinatura de `Ferramenta.Executar` para devolver struct) forçaria mudança em todas elas sem ganho |
| D8 | Estimativa de token de imagem | Por tiles, conforme 1.4, com `len/4` preservado como fallback | A fórmula atual erra por ~80x e cria um teto fantasma que tornaria a visão inviável. O fallback não é preguiça: o comentário original em [`janela.go:26`](../../../internal/memoria/janela.go) argumenta que superestimar é o lado seguro para um orçamento, e isso continua correto para imagem cujo header não decodifica. A intenção do autor sobrevive onde ainda faz sentido, deixando de ser o caminho principal |
| D9 | Invalidação do mapa de elementos | Por mudança da janela em foco, da superfície alvo, ou da configuração de monitores — nunca por relógio | Os três são eventos determinísticos; um timer produziria janela de erro arbitrária, grande demais ou pequena demais conforme o app. A janela em foco entra mesmo com a varredura sendo por monitor (D14), porque trocar de janela reordena o que está visível naquele monitor |
| D10 | Wake word | Log passivo dos quase-acertos, em arquivo próprio do device | A sessão de calibração guiada, que é a leitura direta do que a spec original prometeu, tem um furo lógico: o Arthur falando de propósito no microfone é exatamente a condição em que a detecção **já** funciona. Os erros acontecem longe do microfone, com ruído, na fala rápida — e só o log passivo os coleta. Custo baixo, cabe na fase sem competir com o trabalho de tela. A troca por detector local (1.3) fica para uma fase própria, com o dado deste log em mãos |
| D11 | Local de execução | Direto em `main`, commit por task | Escolha do Arthur, mesmo padrão das Fases 3 e 4 |
| D12 | Tensão com a nuvem | Registrada, não resolvida | O Arthur declarou o objetivo de rodar o `jamesd` em nuvem, com desktop e celular no mesmo serviço. `ver_tela` empurra na direção oposta: o print da tela local passa a viajar pela internet até a nuvem e de lá até o Google. UI Automation não sofre (roda no device; só o resultado trafega). Efeito no desenho desta fase: **compressão vira requisito de rede, não só de orçamento de token**. Ver seção 9 |
| D13 | Formato e resolução da imagem | **PNG, na resolução nativa da tela** | Três razões convergem. Primeira: o custo de token é por área e não por byte (1.4), então comprimir mais **não economiza token nenhum** — economiza rede, que hoje é loopback. Segunda: JPEG borra texto pequeno, que é exatamente o que precisa ficar legível numa interface; PNG mantém nítido e comprime bem em UI, que é feita de áreas de cor sólida. Terceira, e é a que evita bug: [`mapeamento.go:17`](../../../internal/agente/gemini/mapeamento.go) já **fixa** `MIMEType: "image/png"`. Um device que mandasse JPEG sob um MIME que diz PNG seria erro silencioso. Redimensionar fica reservado para quando a nuvem de D12 sair do papel, e aí o MIME deixa de poder ser constante. Registrado o piso: derrubar ambos os lados abaixo de 384 px levaria o custo aos 258 tokens fixos, mas a essa altura a interface está ilegível e a ferramenta perde o propósito |
| D14 | Unidade de captura | **Superfície = um monitor.** `ver_tela` sem argumento captura a superfície que contém a janela em foco; com `superficie`, captura a pedida. Os elementos vêm de **todas as janelas visíveis daquela superfície**, não só da janela em foco | Levantado pelo Arthur depois da primeira versão desta spec, e ele estava certo: a versão anterior dizia "janela em foco", o que é mais restrito que uma tela e deixaria o James cego para metade do que o Arthur enxerga. Medição da máquina, **corrigida durante a implementação da Task 1** (a medição original, via `.NET Screen.AllScreens`, não era DPI-aware e reportava a resolução escalada — o mesmo fenômeno que D2/tela.py descreve): dois monitores, `DISPLAY1` 1920x1080 @ 100% em `(0,0)` e `DISPLAY7`, fisicamente **1920x1080 @ 150%**, em `(1920,255)` — não `1280x720 @ 100%` como a primeira versão desta linha registrou. A origem `(1920,255)` estava certa; só a resolução do segundo monitor mudou. Uma imagem por monitor, e não uma do desktop virtual, porque o desalinhamento vertical de 255 px faria o retângulo que engloba os dois conter áreas pretas que não pertencem a monitor nenhum. Custo pela fórmula de D8, a confirmar em CA3: cada monitor a 1920x1080 dá 6 tiles (~1.548 tokens) — o cálculo não muda com a correção, porque `EstimarTokensImagem` usa a dimensão real capturada em runtime, nunca um valor fixado em design. A abstração vale além do desktop: o `jamesd` na nuvem e o app de celular são superfícies também |
| D15 | Espaço de coordenadas | **Local à superfície** na lista e no protocolo; o device converte para global antes de agir | O modelo enxerga uma imagem de um monitor; pedir que ele raciocine no espaço do desktop virtual, onde a segunda tela começa em x=1920, é convite a erro. A conversão fica no device, que é quem conhece a origem de cada monitor. Consequência registrada em 5.3: esquecer a soma da origem faz o clique cair no monitor errado com feedback plausível |

---

## 4. Arquitetura

Seis camadas são tocadas. As três primeiras concentram o trabalho novo; as três últimas são correções pontuais em código existente.

### 4.1 Device — `dispositivo/james_device/`

**`tela.py`** (existente, alterado). Passa a chamar DPI awareness na carga do módulo. Sem isso o resto é enfeite: numa tela a 150%, `pyautogui` reporta 1280x720 para uma tela que é 1920x1080, e todo clique cai deslocado. Ganha as ações `ver_tela` e `clicar_elemento`. As ações `click` e `tecla` continuam existindo, agora com validação de limites contra a resolução real e com retorno honesto.

**`superficies.py`** (novo). Enumera os monitores e devolve, para cada um, id, resolução, escala DPI e origem no espaço global. Resolve também qual superfície contém a janela em foco, que é o padrão de D14, e faz a conversão local↔global de D15. É o único lugar do device que conhece geometria de monitor — `captura.py` e `elementos.py` perguntam a ele em vez de calcular por conta.

**`elementos.py`** (novo). Esconde `uiautomation` atrás de duas funções:

- `varrer(superficie)` — devolve os elementos clicáveis de todas as janelas visíveis daquela superfície, cada um com id curto, nome, tipo de controle e retângulo já em coordenada local;
- `resolver(elemento_id | nome, tipo)` — reencontra o elemento e o revalida, devolvendo motivo nomeado quando falha.

Guarda o mapa da última varredura, com a superfície e o handle da janela em foco que a originaram (D9).

**`captura.py`** (novo). Tira o print de uma superfície e devolve bytes PNG na resolução nativa (D13). Isolado de `elementos.py` porque as duas coisas falham por motivos diferentes e uma deve poder funcionar sem a outra: uma tela sem árvore de acessibilidade ainda produz imagem útil.

**`wake.py`** (existente, alterado). Ganha a medição de distância e a escrita do log de quase-acertos (D10).

### 4.2 Ponte — `internal/dispositivo/hub.go`

`AcaoTela` ([`hub.go:23`](../../../internal/dispositivo/hub.go)) ganha os campos de elemento. `resultadoAcao` ([`hub.go:41`](../../../internal/dispositivo/hub.go)) e `ResolverResultado` ([`hub.go:135`](../../../internal/dispositivo/hub.go)) passam a carregar imagem. `Ponte.EnviarAcao` ([`hub.go:38`](../../../internal/dispositivo/hub.go)) passa a devolver struct em vez de `string`.

O `Timeout` fixo de 10 s ([`hub.go:62`](../../../internal/dispositivo/hub.go)) vira por-ação. Varrer a árvore UIA de uma tela cheia pode passar de 10 s; um clique não deveria ter o mesmo teto de uma varredura. Ver R1.

### 4.3 Ferramentas — `internal/ferramentas/`

`ver_tela` nova, `NivelSeguro` (D5). `manipular_interface` ([`dispositivo.go`](../../../internal/ferramentas/dispositivo.go)) ganha as ações novas e continua `NivelPerigoso`.

Interface nova, no mesmo formato aditivo de `ExigeConfirmacao`:

```go
// FerramentaComImagem é implementada pela ferramenta cujo resultado inclui
// uma imagem — hoje só ver_tela. Contrato aditivo, igual ExigeConfirmacao:
// ferramenta que não implementa se comporta exatamente como antes.
type FerramentaComImagem interface {
    ExecutarComImagem(ctx context.Context, args map[string]any) (string, []byte, error)
}
```

### 4.4 Loop — `internal/agente/`

`ResultadoFerramenta` ([`tipos.go:26`](../../../internal/agente/tipos.go)) ganha `Imagem []byte`. `despacharSemHook` faz a type assertion para a interface nova no mesmo ponto e no mesmo formato em que já a faz para `ExigeConfirmacao` ([`loop.go:116`](../../../internal/agente/loop.go)), antes da chamada de [`loop.go:133`](../../../internal/agente/loop.go).

### 4.5 Mapeamento — `internal/agente/gemini/mapeamento.go`

O ramo `PapelFerramenta` ([`mapeamento.go:29`](../../../internal/agente/gemini/mapeamento.go)) hoje monta apenas `FunctionResponse` com `output` string, e nunca lê `m.Imagem` — o que faz a imagem ser **descartada em silêncio**. Passa a anexar um `Part` com `InlineData` no mesmo `Content` quando o resultado tem imagem.

O `MIMEType: "image/png"` constante da [linha 17](../../../internal/agente/gemini/mapeamento.go) é mantido como constante, e D13 existe para que ele continue verdadeiro. No dia em que o device passar a mandar outro formato, o MIME deixa de poder ser fixo — e o sintoma de esquecer isso é silencioso, não um erro de compilação.

### 4.6 Memória — `internal/memoria/janela.go`

`custoMensagem` passa a estimar imagem por tiles (D8), com dimensão obtida de `image.DecodeConfig`, que lê apenas o header.

---

## 5. Protocolo

Envelope inalterado: `{tipo, id, dados}`. Nenhum tipo de mensagem novo é introduzido — `ver_tela` e `clicar_elemento` são ações dentro de `acao_tela`, o que evita mexer no despacho de [`conexao.py:112`](../../../dispositivo/james_device/conexao.py), que ignora silenciosamente qualquer `tipo` desconhecido.

### 5.1 `ver_tela`

Pedido: `{"acao": "ver_tela"}` ou `{"acao": "ver_tela", "superficie": "DISPLAY7"}`. Sem o parâmetro, vale o padrão de D14: a superfície que contém a janela em foco.

Resposta em `dados`: `saida` (texto, que vira o `output` do `FunctionResponse`) e `imagem_b64`.

A `saida` abre declarando a superfície capturada e segue com a lista de elementos, uma linha por elemento:

```
superficie: DISPLAY1 (primaria) 1920x1080 escala 100%
e1  Button  "Salvar"           (820,455)
e2  Edit    "Nome do arquivo"  (640,410)
e3  Button  ""                 (1180,60)
```

O cabeçalho não é enfeite: sem ele o modelo não tem como saber se `(820,455)` é grande ou pequeno em relação à tela, nem qual das duas telas está olhando. Com a resolução declarada, ele deriva proporção sozinho quando precisar — que é o motivo de não existir uma coluna separada de coordenada normalizada.

Só o **centro** do elemento vai para o modelo, não o retângulo completo. O centro basta para casar a lista com a imagem; o retângulo custaria quatro vezes mais token sem ganho, e quem precisa dele é o device, que o guarda. O `e3` sem nome é o caso que justifica o id: botão de ícone continua endereçável.

As coordenadas da lista são **locais à superfície** (D15): `(0,0)` é o canto superior esquerdo daquele monitor, não do desktop virtual. É o que casa com a imagem que o modelo está olhando.

### 5.2 `clicar_elemento`

Pedido: `{"acao": "clicar_elemento", "elemento_id": "e7"}` ou `{"acao": "clicar_elemento", "nome": "Salvar", "tipo": "Button"}`.

Antes de clicar, o device revalida nesta ordem, parando no primeiro que falhar:

1. existe mapa de varredura, e ele pertence à superfície e à janela em foco atuais (se não, erro pedindo `ver_tela`);
2. o elemento ainda existe na árvore **agora**, não no snapshot;
3. o retângulo atual bate com o guardado;
4. está visível e habilitado.

Cada falha vira erro nomeado — "a tela mudou desde o `ver_tela`, chame de novo" — e nunca um clique no vazio. Sucesso devolve o elemento atingido, com a superfície: `cliquei em "Salvar" (Button) em (820,455) de DISPLAY1`.

### 5.3 `click` — o caminho de pixel

Continua aceitando `x`/`y`, agora interpretados como locais à superfície (D15), com `superficie` opcional seguindo o mesmo padrão de `ver_tela`. O device converte para coordenada global antes de acionar o `pyautogui`, somando a origem do monitor.

**Essa conversão é obrigatória e é onde um bug silencioso mora.** No setup atual do Arthur, `DISPLAY7` tem origem `(1920, 255)`: um clique em `(100, 100)` daquele monitor é `(2020, 355)` em coordenada global. Esquecer a soma faz o clique cair no monitor errado, e o feedback de 5.3 reportaria com honestidade o elemento errado — erro plausível e difícil de rastrear.

Passa também a validar contra a resolução real da superfície alvo, obtida com DPI awareness ativo; fora dos limites vira erro, não clique.

E ganha feedback que hoje não tem: depois de clicar, o device consulta a UIA sobre qual controle está naquele ponto e reporta. `cliquei em (820,455), que é o Button "Salvar"`, ou `cliquei em (820,455), nenhum elemento identificável nesse ponto` — o que já é informação honesta, e é o que permite ao modelo perceber que errou em canvas e jogo, onde a lista de elementos não ajuda.

---

## 6. Segurança e privacidade

`ver_tela` é `NivelSeguro` e disponível a qualquer perfil, por decisão registrada em D5. A consequência aceita é que o celular enxerga a tela do desktop sem aprovação. Isso está aqui para ser encontrado por quem revisar o modelo de permissão depois, não para ser relitigado nesta fase.

O log de wake word grava apenas os quase-acertos, com timestamp e transcrição. Não grava áudio, e não grava as transcrições que não chegaram perto da palavra de ativação. Vai para arquivo próprio do device, **não** para o `Vault/`, que é lido pelo modelo a cada turno e não deve receber esse ruído. Entra no `.gitignore`.

O `jamesd` não persiste screenshot. A imagem existe na memória do processo durante o turno e vai para o Gemini; não é gravada em disco nem no Postgres. Isso é consequência de a imagem viver em `ResultadoFerramenta`, que não é persistido como binário — e é deliberado.

---

## 7. Tratamento de erro

O device mantém a filosofia que já tem em [`tela.py:17`](../../../dispositivo/james_device/tela.py): nunca levanta, erro vira valor de retorno, e o `jamesd` decide o que fazer. O hub mantém `ErroDispositivoAusente` e o timeout. O loop mantém erro de ferramenta virando `ResultadoFerramenta.Erro`, que o modelo lê e pode corrigir.

O que muda não é o mecanismo, é o conteúdo: os erros passam a existir. Hoje o caminho de clique tem exatamente um erro possível (ação desconhecida) e um sucesso incondicional. Depois desta fase ele tem erro para elemento inexistente, elemento movido, elemento invisível, elemento desabilitado, coordenada fora da tela e varredura ausente.

Falha de captura e falha de varredura são independentes (4.1): `ver_tela` numa janela sem árvore de acessibilidade ainda devolve imagem, com a lista vazia e a razão declarada na `saida`.

---

## 8. Estratégia de teste

**Testável em pytest, sem hardware:** a conversão local↔global e a escolha de superfície em `superficies.py`, com a lista de monitores injetada — inclusive o caso real de `DISPLAY7` em `(1920,255)`, que é o que prova D15; a medição de distância e a escrita do log em `wake.py`; o formatador da lista de elementos; a lógica de revalidação de `elementos.py` contra uma árvore falsa injetada no lugar de `uiautomation`.

Vale destacar que a peça mais perigosa desta fase — a aritmética de coordenada, cujo erro produz clique no monitor errado com feedback plausível — é também a **mais barata de testar**: são funções puras sobre geometria conhecida. Isso deve estar coberto antes de qualquer verificação ao vivo.

**Testável em Go:** o hub, com `Conexao` falsa — padrão já estabelecido no repositório; o mapeamento, por tabela, incluindo o caso que hoje descarta imagem em silêncio; `janela`, com PNGs reais gerados em memória; o loop, com ferramenta falsa implementando `FerramentaComImagem`.

**Reescrita obrigatória:** [`janela_test.go:74`](../../../internal/memoria/janela_test.go) `TestMontarContaImagemComoCusto` usa `make([]byte, 10000)` com o comentário "screenshot de mentira — só o tamanho importa aqui". Com D8 o tamanho deixa de importar e o header passa a importar, e 10.000 bytes zerados não têm header. O teste passa a gerar PNG real em memória. **Isto não é um teste sendo afrouxado para acomodar a mudança — o comportamento que ele trava é o que está sendo corrigido**, e a versão nova deve continuar provando que imagem custa caro o suficiente para cortar a janela.

**Não testável automatizado, e reconhecido como tal:** a varredura UIA real e a captura real dependem de tela. São fechadas por verificação ao vivo (10).

**Validação da fórmula de tiles.** A aritmética de D8 foi derivada da documentação e do exemplo 960x540, não medida. Na implementação ela deve ser conferida contra o `count_tokens` real da API para pelo menos duas resoluções, uma abaixo e uma acima do corte de 384 px. Se divergir, a fórmula é ajustada ao que a API responder.

---

## 9. Restrição conhecida: nuvem

O objetivo declarado de rodar o `jamesd` em nuvem (D13 da spec da Fase 4) tem tensão real com esta fase. Registrado, não resolvido:

- `ver_tela` faz o print da máquina local atravessar a internet até a nuvem, e de lá até o Google. Hoje, com tudo em loopback, o único trecho de rede é o segundo.
- Isso transforma compressão em requisito de rede, não apenas de orçamento de token. O tamanho da imagem passa a ter dois motivos independentes para importar.
- UI Automation não sofre: roda no device, e só o resultado textual trafega. Se a nuvem acontecer, o caminho de elemento envelhece melhor que o caminho de imagem — o que é mais um argumento a favor de D2, encontrado depois da decisão.

Reavaliar quando a implantação em nuvem sair do papel.

---

## 10. Critérios de aceitação

Cada item precisa ser demonstrado com comando rodado e saída lida, conforme a regra de verificação do projeto.

1. **CA1** — `go test ./...` verde, incluindo o `TestMontarContaImagemComoCusto` reescrito e os testes novos de mapeamento, hub e loop.
2. **CA2** — `pytest` verde no device, incluindo revalidação com árvore falsa e log de wake word.
3. **CA3** — a fórmula de tiles bate com o `count_tokens` real da API em duas resoluções, uma abaixo e uma acima de 384 px, com a saída registrada.
4. **CA4 (ao vivo)** — com o `jamesd` rodando e o device conectado, um `ver_tela` real devolve imagem e lista de elementos de uma janela aberta de verdade, e a lista contém um elemento reconhecível. O cabeçalho declara a superfície correta.
5. **CA5 (ao vivo)** — o James clica num botão **pelo nome**, em app nativo aberto de verdade, e o efeito é observado na tela.
6. **CA6 (ao vivo)** — o mesmo clique, pedido depois de a tela ter mudado, devolve erro nomeado em vez de clicar no lugar errado. Este é o critério que prova 1.2 corrigido, e é o mais importante da lista.
7. **CA7 (ao vivo)** — um clique por pixel em área vazia devolve "nenhum elemento identificável nesse ponto", e não o sucesso incondicional de hoje.
8. **CA8 (ao vivo)** — com a janela em foco no `DISPLAY7`, `ver_tela` sem argumento captura o `DISPLAY7` e não o primário, e um clique por elemento naquela tela acerta o alvo. É o critério que prova a conversão de coordenada de D15: se a soma da origem `(1920,255)` estiver errada, o clique cai no monitor primário e este teste falha.

---

## 11. Ordem de implementação

A ordem é ditada por uma restrição: nada do lado Go pode ser verificado de ponta a ponta antes de o device produzir imagem de verdade, e nada do device adianta se a imagem for descartada no mapeamento. Por isso o caminho de imagem é construído inteiro antes do caminho de elemento.

1. `superficies.py`: enumeração de monitores, escala DPI e conversão local↔global. Vem antes de tudo porque `tela.py`, `captura.py` e `elementos.py` dependem dela para saber o que "a tela" significa — e porque é a única peça nova inteiramente testável sem hardware: a geometria dos monitores é dado de entrada, e a conversão é aritmética.
2. DPI awareness e validação de limites em `tela.py`, com o retorno honesto do caminho de pixel. Já corrige parte de 1.2 sozinho.
3. Correção da estimativa de token (D8) e reescrita do teste. Não depende de nada e nada depende dela para compilar — mas precisa vir antes do passo 7, porque sem ela a primeira imagem real que chegar despeja a conversa inteira da janela e o comportamento observado em CA4 seria enganoso.
4. `mapeamento.go`: `InlineData` no ramo `PapelFerramenta`, com teste que hoje falharia.
5. `FerramentaComImagem`, `ResultadoFerramenta.Imagem` e a type assertion no loop.
6. `hub.go`: campos de elemento e de superfície, imagem no resultado, timeout por ação.
7. `captura.py` e a ação `ver_tela` no device; ferramenta `ver_tela`. **Aqui fecha CA4.**
8. `elementos.py`: varredura por superfície, lista e mapa.
9. `clicar_elemento` com a revalidação completa. **Aqui fecham CA5, CA6, CA7 e CA8.**
10. Log passivo de wake word.
11. Ajuste do item 2 do `prompts/sistema.md`.

---

## 12. Riscos conhecidos

| # | Risco | Mitigação |
|---|---|---|
| R1 | Varredura UIA estoura o `Timeout` de 10 s do hub. **D14 agravou este risco**: varrer todas as janelas visíveis de um monitor é mais caro que varrer só a janela em foco, que era o desenho anterior | Timeout por ação (4.2). Se ainda estourar, filtrar por janela visível e não minimizada antes de descer a árvore, e cortar por profundidade. Medir antes de otimizar |
| R2 | Tela densa gera lista de elementos grande o bastante para pesar no token | A lista é magra por construção (5.1). Se pesar, cortar por profundidade e por área mínima do elemento. Medir antes de otimizar |
| R3 | Corrida entre a revalidação e a mudança de tela — a tela pode mudar entre o passo 4 e o clique | Não é eliminável, apenas estreitável: a janela de erro cai de "desde o `ver_tela`" para "desde a revalidação". Aceito conscientemente; é estritamente melhor que o comportamento atual |
| R4 | Apps que não expõem árvore de acessibilidade útil (canvas, jogo, Electron mal comportado) | É o motivo de o caminho de pixel sobreviver (D2), agora com o feedback de 5.3 |
| R5 | Múltiplos monitores com escalas de DPI diferentes | **Coberto por D14**: cada superfície declara a própria escala, então escala mista deixa de ser caso não tratado. O que sobra é a mudança de configuração em runtime (monitor conectado ou desconectado com o James rodando) — tratada só pela invalidação de D9, sem reenumeração automática |
| R6 | Janela arrastada de forma a ficar metade em cada monitor | Regra: o elemento pertence à superfície que contém o **centro** do seu retângulo. Simples e determinística, mas erra para uma janela cortada quase ao meio — nesse caso alguns elementos aparecem numa superfície e o resto na outra, e nenhuma das duas listas está completa. Caso conhecido, não tratado. Sair disso exigiria varredura no espaço do desktop virtual, que D14 descartou por causa das áreas mortas |
| R7 | `ver_tela` sem aprovação, de qualquer perfil, envia a tela do desktop para quem pedir (D5) | Nenhuma. Decisão consciente do Arthur, com a consequência declarada |
| R8 | Sem `SetReadLimit`, o `/ws/dispositivo` aceita mensagem de qualquer tamanho (D6) | Não é regressão — já é assim hoje. Mas com imagem trafegando passa a existir expectativa de tamanho, e um limite explícito e folgado deve ser definido junto com a captura |
| R9 | A verificação ao vivo move o mouse e digita na máquina em que o trabalho está sendo feito | Executar CA5 a CA8 com um app de teste em foco, nunca com o editor ou o terminal do projeto em foco. CA8 é o mais delicado, porque envolve mover o foco para o segundo monitor |
