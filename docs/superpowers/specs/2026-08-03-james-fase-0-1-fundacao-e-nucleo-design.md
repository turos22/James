# James — Fase 0+1: Fundação e Núcleo

**Data:** 2026-08-03
**Autor:** Arthur Orosco (design colaborativo)
**Estado:** aprovado nas três seções, aguardando revisão do documento
**Projeto atual:** `C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts`

---

## 1. Contexto

O projeto hoje é a "Athena": um app Wails v2 (Go + React/TypeScript) que serve como
orquestrador pessoal. Ele escuta por voz, conversa com o Gemini, e executa quatro
tools — comando PowerShell, Claude Code, controle de tela e busca web.

O estado atual compila (`go build` retorna 0, verificado em 2026-08-03), mas acumula
oito problemas estruturais levantados pelo Arthur e confirmados por leitura do código.
O anexo A mapeia cada bug ao arquivo, à linha e à fase que o resolve.

Este documento cobre **apenas as Fases 0 e 1**. As demais fases têm spec própria.

### Rename: Athena → James

O produto passa a se chamar **James**, com persona de "primo caçula do Jarvis":
competente, mas mais solto e direto que o original — sem a formalidade impecável.

| O que | De | Para |
|---|---|---|
| Binário do back-end | — | `jamesd` |
| Agente de dispositivo | `processador_voz.py` | `james-device` |
| Pasta do projeto | `meu-app-ts` | `james` |
| Banco | `Athena.db` (SQLite) | schema `james` no Postgres |
| Wake word | `["athena","atena","athenas","atenas"]` | `["james","jeimes","jaimes","jeims", …]` |
| Voz do TTS | `pt-BR-FranciscaNeural` | `pt-BR-AntonioNeural` |
| Persona | Engenheira de Sistemas de elite | Primo caçula do Jarvis |

**Nota sobre a wake word:** o STT do Google em português vai transcrever "James"
de várias formas (*jeimes*, *jaimes*, *games*, *dimes*). A lista precisa ser
generosa e calibrada com gravações reais do Arthur. A calibração fica para a Fase 5;
a Fase 1 apenas porta a lista com as variantes óbvias.

---

## 2. Decisões travadas

Cada uma foi escolhida entre 3–4 alternativas com trade-offs apresentados.

| # | Decisão | Escolha | Por quê |
|---|---|---|---|
| D1 | Alcance do back-end | Alcançável pela rede | Arthur quer falar com o James do celular |
| D2 | Como expor na rede | Rede overlay (Tailscale/WireGuard) | Nenhuma porta aberta no roteador; identidade de dispositivo e transporte cifrado vêm da VPN, não de código nosso |
| D3 | Modelo de permissão | Perfil por cliente | Desktop mantém autonomia total; celular fica inofensivo se perdido |
| D4 | Acesso ao modelo | Interface `Provider` + Gemini de topo | Torna o loop testável sem gastar token — pré-requisito do TDD |

**Qual modelo Gemini, exatamente.** A spec não fixa um ID. O modelo é lido do
`config.toml` e a escolha concreta é feita na implementação, conferindo a lista vigente
de modelos do Gemini no momento — fixar um ID aqui envelheceria mal. A única restrição
travada é negativa e vale como critério: **`gemini-2.5-flash-lite` está descartado**,
por ser o mais fraco da família e a causa provável do planejamento ruim de tools.
| D5 | Destino do Python | Vira agente de dispositivo | Bibliotecas de UI Automation em Python são muito superiores às de Go; é o caminho para consertar o clique na Fase 5 |
| D6 | Banco | PostgreSQL já na Fase 1 | A Fase 1 reescreve a persistência de qualquer jeito; SQLite viraria adaptador descartável |

**Consequência de D6:** a Fase 2 (migração para Postgres) deixa de existir como fase
separada. O `pgvector` e o schema de embeddings migram para a Fase 4, junto com o
design do RAG — desenhar granularidade de chunk antes de desenhar o RAG seria adivinhação.

**Numeração das fases.** A decomposição acordada foi 0 a 6. Com a absorção da Fase 2,
o número 2 fica vago de propósito: renumerar invalidaria tudo que já foi conversado.
Mapa vigente:

| Fase | Conteúdo |
|---|---|
| 0 | Fundação: segredos, git, testes, Docker |
| 1 | Núcleo: back-end como serviço |
| ~~2~~ | *absorvida pela Fase 1 (Postgres) e pela Fase 4 (pgvector)* |
| 3 | Qualidade das ferramentas |
| 4 | Obsidian como cérebro de contexto |
| 5 | Máquina física: controle de tela e voz |
| 6 | Front-end novo |

**Consequência de D1:** o front-end obrigatoriamente vira SPA web servida pelo back-end.
O shell desktop deixa de ser dono da UI e passa a ser um dos clientes dela.

### Fato relevante: assinatura Pro ≠ API

Assinatura Claude Pro autentica o **Claude Code** via OAuth e consome cota da assinatura.
Chamada direta à Messages API é cobrança separada por token. Portanto:

- "Claude Code como ferramenta do James" → **grátis** dentro da assinatura. É o caminho.
- "Claude como cérebro do James" → conta nova. Fora de escopo.

O comando que restaura o acesso correto ao Claude Code:

```powershell
Remove-Item Env:\ANTHROPIC_BASE_URL, Env:\ANTHROPIC_API_KEY, Env:\ANTHROPIC_AUTH_TOKEN -ErrorAction SilentlyContinue; claude
```

seguido de `/login`. Isso desfaz o sequestro do `rodar_claude.ps1`, que hoje redireciona
o Claude Code para o OpenRouter com um modelo NVIDIA gratuito. **Aplicar de fato é Fase 3**;
aqui fica registrado como entrada de design.

---

## 3. Arquitetura

### 3.1 Topologia — três processos, uma máquina

```
   CLIENTE DESKTOP                                    CLIENTE CELULAR
   shell Wails + WebView2                             navegador
   perfil: privilegiado                               perfil: restrito
          │                                                  │
          │  HTTPS + WebSocket                               │
          └──────────────────┐            ┌──────────────────┘
                             ▼            ▼
                  ┌────────────────────────────────┐
                  │           jamesd (Go)          │
                  │  bind: interface da VPN        │
                  │        + loopback              │
                  │                                │
                  │  agente · ferramentas · api    │
                  │  permissao · memoria · voz     │
                  └───────────────┬────────────────┘
                                  │ WebSocket (loopback)
                                  │ conexão de SAÍDA do device ↑
                  ┌───────────────┴────────────────┐
                  │      james-device (Python)     │
                  │      voz · tela                │
                  └────────────────────────────────┘
                                  │
                  ┌───────────────┴────────────────┐
                  │      PostgreSQL (Docker)       │
                  └────────────────────────────────┘
```

Tudo roda na máquina do Arthur. **Nenhuma VPS.** O Tailscale não é servidor alugado:
é software instalado no PC e no celular que faz os aparelhos se enxergarem como se
estivessem na mesma rede local, de qualquer lugar.

Três invariantes da topologia:

**I1 — Só o `jamesd` abre porta.** O `james-device` conecta para fora, no loopback.
Ele nunca escuta. Isso elimina uma superfície de ataque, dá reconexão natural quando
o back-end reinicia, e faz o `jamesd` saber com precisão se a máquina física está
disponível — o que hoje ele não sabe (causa do panic em `app.go:218`).

**I2 — O `jamesd` nunca faz bind em `0.0.0.0`.** Bind explícito na interface da VPN
mais loopback. Se a VPN cair, o serviço fica inalcançável de fora, em vez de vazar
silenciosamente para a LAN.

**I3 — Uma UI, dois invólucros.** O shell desktop e o navegador do celular consomem
exatamente a mesma SPA servida pelo `jamesd`.

**Limitação aceita:** o PC precisa estar ligado para o celular funcionar. O James
existe para controlar *aquela* máquina; hospedá-lo fora dela removeria o propósito.

### 3.2 Estrutura de pastas

Monorepo com três artefatos independentes:

```
james/
├── cmd/jamesd/main.go             binário do back-end
├── internal/
│   ├── agente/
│   │   ├── agente.go              o loop
│   │   ├── provider.go            interface Provider + tipos
│   │   └── gemini/                implementação Gemini
│   ├── ferramentas/
│   │   ├── registro.go            registry
│   │   ├── terminal.go
│   │   ├── claudecode.go
│   │   ├── busca.go
│   │   └── dispositivo.go         ponte para o james-device
│   ├── permissao/                 perfis + gate de execução
│   ├── api/
│   │   ├── rest.go
│   │   ├── ws.go
│   │   ├── dispositivo.go         endpoint WS do device
│   │   └── identidade.go          resolução de perfil
│   ├── armazenamento/
│   │   ├── postgres/              repositórios
│   │   └── migracoes/             SQL numerado
│   ├── memoria/                   janela de contexto
│   ├── voz/                       TTS
│   └── config/                    .env + config.toml
├── dispositivo/
│   ├── james_device/
│   │   ├── conexao.py             cliente WS, reconexão, heartbeat
│   │   ├── voz.py                 wake word + STT
│   │   └── tela.py                cliques e teclado
│   ├── tests/
│   └── pyproject.toml
├── web/                           SPA React
├── shell/                         invólucro Wails mínimo
├── prompts/sistema.md             system prompt, fonte única
├── docs/superpowers/specs/
├── docker-compose.yml             Postgres local
├── config.toml                    configuração não-secreta
├── .env.example
└── .gitignore
```

**Saem do repositório:** DLLs do Vosk (~70 MB, nenhum código as usa), `TTs/espeak-ng-data/`,
`vosk/`, `meu-app-ts.exe`, `Athena.db` e os WAL, `arquivos do Gui/`. O `obsidian.py`
do Gui é preservado fora do repositório como referência para a Fase 4.

**Áudio — duas categorias distintas.** Os arquivos gerados pelo TTS são efêmeros e
ficam no `.gitignore`. Os arquivos de *asset* que o agente de dispositivo precisa para
funcionar (`entrada.mp3`, `faaah.wav`, `um_minuto.mp3`) são versionados em
`dispositivo/assets/`. Ignorar `audios/` inteiro quebraria o `james-device`.

**Destino do `Vault/`.** Permanece no disco, intocado, fora do escopo da Fase 1 — é a
entrada da Fase 4. A única mudança é que `Vault/Contexto_Core.md` **deixa de ser fonte
do system prompt**, papel que passa a `prompts/sistema.md`.

**Nomenclatura:** pacotes em português conforme o CLAUDE.md do Arthur. Termos consagrados
do domínio LLM (`Provider`, `tool`, `token`) permanecem em inglês dentro do vocabulário,
mas o pacote chama `ferramentas`.

O `app.go` de 574 linhas e sete responsabilidades deixa de existir.

---

## 4. Componentes

### 4.1 `agente/`

Executa o loop: monta contexto → chama o `Provider` → despacha tools → devolve
resultados ao modelo → repete até texto final ou teto de iterações.

```go
package agente

type Papel string

const (
    PapelUsuario    Papel = "usuario"
    PapelModelo     Papel = "modelo"
    PapelFerramenta Papel = "ferramenta"
)

type ChamadaFerramenta struct {
    ID   string
    Nome string
    Args map[string]any
}

type ResultadoFerramenta struct {
    ID    string
    Nome  string
    Saida string
    Erro  string          // preenchido quando a tool falhou
}

type Mensagem struct {
    Papel      Papel
    Texto      string
    Imagem     []byte                // screenshot, quando houver
    Chamadas   []ChamadaFerramenta
    Resultados []ResultadoFerramenta
}

type Requisicao struct {
    Sistema     string
    Mensagens   []Mensagem
    Declaracoes []ferramentas.Declaracao
}

type Resposta struct {
    Texto    string
    Chamadas []ChamadaFerramenta
}

type Provider interface {
    Gerar(ctx context.Context, req Requisicao) (Resposta, error)
    GerarStream(ctx context.Context, req Requisicao, emitir func(string)) (Resposta, error)
}
```

O `Provider` é a fronteira que torna o loop testável com respostas roteirizadas,
sem gastar token e sem abrir janela.

**Bugs que este pacote elimina:** prompt duplicado no histórico (hoje `app.go:466`
salva e `app.go:483` reanexa), histórico sem limite, resposta de tool malformada,
e o fallback `"Ação concluída com sucesso."` mascarando fracasso.

### 4.2 `ferramentas/`

Um registro; cada tool num arquivo. Toda tool declara o próprio **nível de risco**,
que é o que o gate de permissão lê.

```go
package ferramentas

type Nivel int

const (
    NivelSeguro   Nivel = iota // leitura, consulta, sem efeito colateral
    NivelPerigoso              // executa, altera estado, controla a máquina
)

type Declaracao struct {
    Nome         string
    Descricao    string
    Parametros   map[string]Parametro
    Obrigatorios []string
}

type Ferramenta interface {
    Declaracao() Declaracao
    Nivel() Nivel
    Executar(ctx context.Context, args map[string]any) (string, error)
}

type Registro interface {
    Registrar(f Ferramenta) error
    Buscar(nome string) (Ferramenta, bool)
    Declaracoes() []Declaracao
}
```

Níveis das quatro tools atuais:

| Tool | Nível |
|---|---|
| `executar_comando_sistema` | perigoso |
| `perguntar_ao_claude_code` | perigoso |
| `manipular_interface` | perigoso |
| `pesquisar_internet` | seguro |

**Importante:** na Fase 1 as quatro entram atrás desta interface **com o comportamento
ruim que têm hoje**. A busca continua fraca e o Claude Code continua abrindo janela.
Consertar é a Fase 3.

### 4.3 `permissao/`

```go
package permissao

type Perfil string

const (
    Privilegiado Perfil = "privilegiado"
    Restrito     Perfil = "restrito"
)

// Pode informa se o perfil executa o nível sem aprovação humana.
func (p Perfil) Pode(n ferramentas.Nivel) bool
```

- `Privilegiado` → executa qualquer nível, sem perguntar.
- `Restrito` → executa apenas `NivelSeguro`. Nível perigoso cria aprovação pendente
  e emite `aprovacao_requerida` aos clientes privilegiados. A execução aguarda decisão
  ou expira por timeout.

**Resolução de perfil, em duas camadas:**

1. Conexão originada do **loopback** → é o shell na própria máquina → `Privilegiado`.
   Não é escalada de privilégio: qualquer processo já rodando naquele PC podia abrir
   PowerShell por conta própria.
2. Conexão originada da **interface da VPN** → `Restrito` por padrão, salvo se o
   endereço constar na lista `[dispositivos]` do `config.toml`.

**Refinamento a verificar:** o Tailscale expõe identidade do peer que conectou, o que
substituiria a lista por endereço. Não foi verificado. A regra de duas camadas acima
não depende disso e é o que a Fase 1 implementa.

### 4.4 `api/`

**REST** — sessões, histórico, health, e o áudio gerado.

| Método | Rota | Descrição |
|---|---|---|
| `GET` | `/api/saude` | status do serviço, do banco e do device |
| `GET` | `/api/sessoes` | lista sessões |
| `POST` | `/api/sessoes` | cria sessão |
| `GET` | `/api/sessoes/{id}/mensagens` | histórico paginado |
| `GET` | `/api/audio/{id}` | áudio TTS de uma resposta |

**WebSocket** — `/ws` para clientes, `/ws/dispositivo` para o `james-device`.
Envelope comum:

```json
{ "tipo": "<nome>", "id": "<correlação>", "dados": { } }
```

| Direção | Tipos |
|---|---|
| Cliente → servidor | `mensagem`, `aprovar`, `negar`, `cancelar` |
| Servidor → cliente | `token`, `ferramenta_iniciada`, `ferramenta_concluida`, `aprovacao_requerida`, `resposta_final`, `audio_pronto`, `status_dispositivo`, `erro` |
| Device → servidor | `registro`, `entrada_voz`, `resultado_acao`, `heartbeat` |
| Servidor → device | `acao_tela`, `acao_teclado`, `ping` |

O evento `token` entrega a resposta saindo aos poucos, em vez de aparecer pronta.
O `ferramenta_iniciada` resolve a cegueira atual: o Arthur passa a ver *o que* o James
está fazendo, em vez de esperar no escuro.

### 4.5 `armazenamento/`

PostgreSQL via `pgx`. Repositórios por agregado, migrações em SQL numerado aplicadas
na subida. Sem ORM.

### 4.6 `memoria/`

Monta a janela de contexto: últimas N mensagens da sessão dentro de um orçamento de
tokens configurável. Hoje `app.go:359` envia **o dia inteiro** em toda requisição,
ficando mais caro e mais lento a cada mensagem. A Fase 4 pluga o Obsidian aqui.

### 4.7 `voz/`

TTS via `edge-tts`, assíncrono, com timeout, arquivo por resposta e erro propagado.

**Mudança de comportamento derivada de D1:** hoje o Go toca o áudio no alto-falante
do PC (`app.go:567`). Mantido assim, falar com o James pelo celular na rua faria o
computador de casa responder em voz alta, sozinho. Portanto o back-end **gera** o áudio
e o entrega por URL — **quem toca é o cliente**.

Isso também elimina a corrida no arquivo fixo `audios/audio_jarvis.mp3` e o
`speaker.Init` que trava a taxa de amostragem no primeiro áudio tocado.

### 4.8 `config/`

`.env` guarda **somente segredos**. `config.toml` guarda o resto (bind, perfis de
dispositivo, modelo, orçamento de tokens, timeouts). Validação na subida com **falha
rápida**: falta chave, o `jamesd` não sobe e diz qual faltou.

O system prompt sai do código Go e vira `prompts/sistema.md`, fonte única. Hoje ele
está duplicado entre a constante em `app.go:151` e `Vault/Contexto_Core.md`, e as
duas cópias já divergiram.

### 4.9 `web/` e `shell/`

`shell/` vira um invólucro Wails mínimo: janela frameless, transparente, always-on-top,
com o WebView2 apontando para a SPA. Todos os bindings `wailsjs/go/main/App` morrem.

`web/` na Fase 1 é a **UI atual, sem redesenho**, com as chamadas de binding trocadas
por um cliente HTTP/WS. A Fase 1 não pode deixar o Arthur sem app funcionando.
Abas verticais e visualizador reativo são a Fase 6.

---

## 5. Modelo de dados

```sql
-- migracoes/0001_inicial.sql

CREATE TABLE sessoes (
    id            UUID PRIMARY KEY,
    titulo        TEXT,
    criada_em     TIMESTAMPTZ NOT NULL DEFAULT now(),
    atualizada_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE mensagens (
    id        BIGSERIAL PRIMARY KEY,
    sessao_id UUID NOT NULL REFERENCES sessoes(id) ON DELETE CASCADE,
    papel     TEXT NOT NULL CHECK (papel IN ('usuario','modelo','ferramenta')),
    conteudo  TEXT NOT NULL,
    chamadas  JSONB,
    criada_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_mensagens_sessao ON mensagens (sessao_id, criada_em);

CREATE TABLE aprovacoes (
    id             UUID PRIMARY KEY,
    sessao_id      UUID NOT NULL REFERENCES sessoes(id) ON DELETE CASCADE,
    ferramenta     TEXT NOT NULL,
    argumentos     JSONB NOT NULL,
    solicitada_por TEXT NOT NULL,
    estado         TEXT NOT NULL CHECK (estado IN ('pendente','aprovada','negada','expirada')),
    criada_em      TIMESTAMPTZ NOT NULL DEFAULT now(),
    decidida_em    TIMESTAMPTZ
);

CREATE INDEX idx_aprovacoes_pendentes ON aprovacoes (estado, criada_em)
    WHERE estado = 'pendente';
```

**Migração de dados:** as mensagens de `Athena.db` (tabela `mensagens`, colunas
`id, role, content, created_at`) são importadas para uma sessão sintética
`"Histórico Athena"`, com `role` mapeado: `user` → `usuario`, `assistant` → `modelo`.
Script único, idempotente, rodado uma vez.

---

## 6. Tratamento de erro

O código atual engole quase tudo: `exec.Command(...).Run()` sem checar retorno,
`fmt.Println` no lugar de log, e `connPython` nil derrubando o processo.
Seis regras substituem isso:

**R1 — Falha rápida na subida.** Chave ausente, Postgres fora do ar ou migração
pendente impedem o `jamesd` de subir, com mensagem dizendo o que faltou. Melhor não
subir do que morrer no meio de uma conversa.

**R2 — Erro de tool não derruba o loop.** A falha volta *para o modelo* como
`ResultadoFerramenta.Erro`. Ele decide: tenta outro caminho ou avisa o usuário.

**R3 — Dispositivo ausente é resposta, não crash.** Sem `james-device` conectado, a
tool de tela retorna "máquina física indisponível". Elimina o panic de `app.go:218`.

**R4 — Timeout em tudo.** Chamada ao modelo, execução de tool, espera por aprovação
e geração de TTS. Nada bloqueia para sempre.

**R5 — Teto de iterações com mensagem honesta.** Ao estourar, o James informa que não
concluiu e o que travou. Nunca mais um fallback mascarando fracasso.

**R6 — Log estruturado com `log/slog`, e nunca segredo em log.**

Princípio: **o modelo aprende do erro, o usuário enxerga o erro, o processo sobrevive
ao erro.** Hoje nenhum dos três acontece.

---

## 7. Estratégia de teste

Modo trabalho exige teste falhando antes do código e verificação com evidência fresca.
O maior ganho vem de graça da interface `Provider`.

| Camada | Como | Contra o quê |
|---|---|---|
| `agente` | provider falso, respostas roteirizadas | loop, dispatch, teto de iterações |
| `permissao` | tabela de casos, lógica pura | perfil × nível de risco |
| `ferramentas` | fakes para exec e HTTP | contrato de cada tool |
| `armazenamento` | Postgres real em Docker | SQL de verdade, não mock |
| `api` | `httptest` + cliente WS real | contrato REST e do socket |
| `james-device` | `pytest` + servidor WS falso | conexão, reconexão, heartbeat |
| `web` | `vitest` | cliente de API |

O banco é testado contra Postgres real: bug de SQL só aparece contra engine de verdade.

Voz e tela dependem de hardware. A camada de **protocolo** é testada; a camada que
toca o hardware fica fina e é verificada à mão. Não se finge cobertura inexistente.

**Harness externo:** `go vet ./... && go test ./...`, `pytest`, `vitest` mais navegador
real via `webapp-testing`, e `/code-review` + `/security-review` no diff antes de fechar.

**Primeiro teste do projeto:** o de `permissao`. É o núcleo de segurança e é lógica
pura, então não depende de nada mais existir.

---

## 8. Escopo

### Entra

**Fase 0 — Fundação**
- Revogar as quatro credenciais expostas; rotacionar as duas que seguem em uso
- `.env` + `.env.example` + `config.toml`
- `.gitignore` cobrindo binários, banco, áudio, DLLs, dados de TTS e `.env`
- `git init` e primeiro commit **após** a limpeza
- Infra de teste nas três linguagens
- `docker-compose.yml` com Postgres
- Correção do `//go:embed` ausente em `main.go:14`

**Fase 1 — Núcleo**
- Reestruturação de pastas e rename Athena → James
- `jamesd`: agente, `Provider`, registro de ferramentas, permissão, API REST+WS,
  armazenamento Postgres, memória com janela, TTS entregue ao cliente, config
- `james-device`: Python com WS, reconexão e heartbeat; voz e tela **portadas como estão**
- `shell` Wails mínimo; `web` repontada para a API
- Migração dos dados do `Athena.db`
- Bind na interface da VPN e perfis de permissão
- Remoção do código morto (`chamadaResumo`, `toolBusca` não usada, `Resumir` vazio)

### Não entra

| Fora | Fase |
|---|---|
| Qualidade das tools: Claude Code headless, busca melhor, tool de Spotify | 3 |
| Obsidian, RAG, `pgvector` | 4 |
| UI Automation, conserto do clique, calibração da wake word | 5 |
| Abas verticais, visualizador reativo, redesenho da UI | 6 |
| Roteamento multi-modelo por custo | — |
| Serviço do Windows, auto-start | — |

**Explicitamente:** a Fase 1 **não melhora o comportamento** das tools. A busca continua
fraca, o Claude Code continua abrindo janela e o clique continua errando. O que muda é
a estrutura que torna esses consertos possíveis e testáveis.

---

## 9. Critérios de aceitação

Nove verificações, todas com saída colável:

1. `go vet ./... && go test ./...` passa
2. `pytest` passa
3. `npm test` passa
4. `jamesd` sobe, conecta no Postgres e aplica as migrações
5. O shell abre e a conversa funciona, com as mesmas capacidades de hoje
6. O celular na VPN abre a mesma UI e conversa; ação perigosa gera card de confirmação no desktop
7. Matar o `james-device` **não** derruba o `jamesd`; a tool de tela retorna erro claro
8. `git log -p` sem nenhuma credencial em nenhum commit
9. Nenhuma chave em texto puro no código

---

## 10. Riscos

| Risco | Mitigação |
|---|---|
| **Migração big-bang** — a Fase 1 mexe em tudo de uma vez | O plano começa pelo testável isolado (`permissao`, `provider`, `memoria`) e deixa a troca do transporte da UI por último; o app fica quebrado pelo menor tempo possível |
| **Tailscale não verificado** — bind na interface, TLS e identidade de peer | Primeira tarefa técnica do plano é um spike curto provando bind + HTTPS + WebView2 apontando para o endereço remoto, antes de qualquer código de produto |
| **Postgres em Docker** — daemon parado hoje | Pré-requisito do Arthur; `docker-compose up` na Fase 0 |
| **Wake word "James" em português** | Lista generosa na Fase 1; calibração real na fase de voz |

---

## 11. Pré-requisitos do Arthur

Bloqueiam a Fase 0 e não podem ser feitos por Claude:

1. **Revogar as quatro credenciais expostas.** Elas estão em texto puro no código e
   compiladas dentro de `meu-app-ts.exe`. Apagar do fonte não basta.

   | Chave | Onde está | Ação |
   |---|---|---|
   | Gemini | `app.go:458` | rotacionar (Google AI Studio) — segue em uso |
   | Tavily | `app.go:229` | rotacionar (app.tavily.com) — segue em uso |
   | NVIDIA | `app.go:385` | **revogar sem repor** — o `chamadaResumo` é código morto e será removido |
   | OpenRouter | `rodar_claude.ps1:6` | **revogar sem repor** — o Claude Code passa a usar a assinatura Pro |

2. **Subir o Docker Desktop.** Instalado (29.1.2), daemon parado em 2026-08-03.

3. **Instalar o Tailscale** no PC e no celular, ambos na mesma conta.

### Ambiente verificado em 2026-08-03

| Ferramenta | Estado |
|---|---|
| Go | 1.26.1 ✓ |
| Node / npm | 24.12.0 / 11.6.2 ✓ |
| Python | 3.13.9 ✓ |
| git | 2.51.2 ✓ |
| edge-tts | 7.2.8 ✓ |
| Docker | 29.1.2 instalado, **daemon parado** |
| Tailscale | **ausente** |
| psql | ausente do PATH (irrelevante — Postgres vem do Docker) |

---

## Anexo A — Bugs mapeados

Levantados por leitura do código em 2026-08-03. A coluna Fase indica onde cada um morre.

| # | Problema | Local | Fase |
|---|---|---|---|
| 1 | Quatro credenciais em texto puro, também compiladas no `.exe` | `app.go:229,385,458`; `rodar_claude.ps1:6` | 0 |
| 2 | `//go:embed` ausente — `wails build` gera binário com tela em branco | `main.go:14` | 0 |
| 3 | Prompt duplicado no histórico a cada chamada | `app.go:466` + `app.go:483` | 1 |
| 4 | Histórico sem limite: envia o dia inteiro em toda requisição | `app.go:359` | 1 |
| 5 | Panic por `connPython` nil quando o Python não está conectado | `app.go:218` | 1 |
| 6 | Resposta de tool com `Role: "tool"` — comportamento a verificar | `app.go:550` | 1 |
| 7 | `edge-tts` síncrono, erro ignorado, arquivo de saída fixo | `app.go:566` | 1 |
| 8 | `speaker.Init` trava a taxa de amostragem no primeiro áudio | `app.go:266` | 1 |
| 9 | Fallback `"Ação concluída com sucesso."` mascarando fracasso | `app.go:559` | 1 |
| 10 | `chamadaResumo` (classificador WEB/LOCAL/CHAT) é código morto | `app.go:382` | 1 |
| 11 | `Resumir()` é stub vazio | `app.go:61` | 1 |
| 12 | Socket TCP cru, sem framing nem reconexão | `app.go:277`; `processador_voz.py` | 1 |
| 13 | System prompt duplicado e já divergente | `app.go:151` vs `Vault/Contexto_Core.md` | 1 |
| 14 | Modelo `gemini-2.5-flash-lite` — o mais fraco da família | `app.go:494` | 1 |
| 15 | `toolBusca` (Google Search grounding) declarada e nunca usada | `app.go:135` vs `app.go:485` | 3 |
| 16 | Tavily em `basic`, sem `include_answer`, devolve JSON bruto | `app.go:226` | 3 |
| 17 | Claude Code sequestrado para o OpenRouter | `rodar_claude.ps1:5-7` | 3 |
| 18 | Claude Code fire-and-forget: abre janela, não captura saída | `app.go:183` | 3 |
| 19 | Screenshot só é enviado se o texto contiver a palavra "tela" | `frontend/src/App.tsx:59` | 1 move / 5 conserta |
| 20 | Coordenadas X/Y adivinhadas pelo modelo; 1920x1080 fixo; PyAutoGUI sem DPI awareness | `app.go:156`, `app.go:323` | 5 |
| 21 | CSS duplicado — regras finais sobrescrevem as anteriores | `frontend/src/App.css` | 6 |
| 22 | Sem auto-scroll na lista de mensagens | `frontend/src/App.tsx` | 6 |
| 23 | Janela 800x900 vs `.chat-window` fixa em 560x720 | `app.go:314` vs `App.css:125` | 6 |

**Evidência do impacto:** em `Vault/Logs_Diarios/2026-07-08.md`, 10 das 11 respostas
foram o fallback `"Ação concluída com sucesso."` — o dia inteiro de tentativas de
controle de tela sem retorno útil. Em `2026-07-07.md`, duas tentativas de abrir o
Spotify com música específica falharam com
*"Não foi possível abrir o Spotify com a música solicitada."*
