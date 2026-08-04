# James — Fase 1: Núcleo

**Data:** 2026-08-04
**Autor:** Arthur Orosco (design colaborativo)
**Estado:** aprovado, aguardando revisão do documento escrito
**Projeto atual:** `C:\Users\Arthur\SI\Projetos\Athena(Go)\meu-app-ts` (pasta raiz permanece `meu-app-ts/` durante a Fase 1 — ver Decisão P6)
**Base:** [`2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`](2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md), seções 3–11, com os ajustes registrados abaixo. Este documento é autocontido: não é necessário ler o anterior para implementar a Fase 1.

---

## 1. Contexto

A Fase 0 (fundação: higiene de credenciais, harness de teste nas três linguagens, Postgres local, spike de rede) fechou em 2026-08-04, 12 commits, branch `main`, working tree limpo. O spike do Tailscale (`docs/superpowers/spikes/2026-08-03-tailscale.md`) provou bind exclusivo na interface da VPN, emissão de certificado TLS válido via `tailscale cert`, e aceitação desse HTTPS por um motor Chromium — o maior risco de design da Fase 1 está derrubado.

Este documento cobre a Fase 1: transformar o `app.go` monolítico de 574 linhas e sete responsabilidades no back-end de serviço `jamesd`, com agente testável, ferramentas atrás de um registro, permissão por perfil, API REST+WS, armazenamento Postgres, memória em janela, TTS entregue ao cliente, e o rename Athena → James completo no código.

**O que muda em relação à spec de 03/08:** seis decisões, tomadas nesta sessão de brainstorming, descritas na seção 2. O resto da arquitetura, componentes, modelo de dados, tratamento de erro e estratégia de teste da spec anterior segue valendo e está reproduzido aqui.

---

## 2. Decisões desta sessão

| # | Decisão | Escolha | Por quê |
|---|---|---|---|
| P1 | Empacotamento da Fase 1 | Um spec, um plano | O risco de big-bang já é mitigado pela ordem do plano (núcleo testável primeiro, transporte da UI por último); dividir em 1a/1b criaria dois fechamentos sem necessidade — os componentes são interdependentes demais pra fechar um sem o outro rodando de ponta a ponta |
| P2 | Guarda de credenciais genérica | Entra na Fase 1 | É exatamente quando `DATABASE_URL` real com senha do Postgres passa a circular em `config.toml`/`.env` e em logs estruturados novos — o backlog da Fase 0 vira código agora, não depois |
| P3 | Gatilho da migração de dados | Comando avulso manual | `cmd/migrar-historico`, rodado uma vez à mão depois que o `jamesd` já está de pé. Erro aparece no terminal na hora; não mistura responsabilidade de boot do serviço principal com ETL único |
| P4 | UI de aprovação no desktop | Elemento mínimo funcional | Perfil Restrito é conceito novo da Fase 1 — a UI pra aprovar/negar não existe hoje e o critério de aceitação #6 exige ela. Card simples plugado no WS existente, sem redesenho ao redor |
| P5 | Allowlist de dispositivo por IP na VPN | Removida do escopo | O HANDOFF já registra que IP do Tailscale muda a cada relogin e nunca deve ser fixado — uma lista `[dispositivos]` por endereço ficaria obsoleta sozinha. A identidade de peer do Tailscale (que resolveria isso de verdade) está marcada como "a verificar" na spec anterior, não verificada ainda. Fase 1 simplifica: loopback = Privilegiado, VPN = Restrito, sem exceção |
| P6 | Rename físico da pasta `meu-app-ts` → `james` | Manual, pós-fechamento, fora do plano automatizado | É o diretório de trabalho da sessão de implementação — renomear a pasta raiz no meio da execução (via agente ou subagente) quebra paths abertos e a própria sessão. O módulo Go já é `james` (resolve nomeação de import); o rename físico do diretório fica pra você fazer depois que a Fase 1 fechar |

---

## 3. Arquitetura

### 3.1 Topologia — três processos, uma máquina

```
   CLIENTE DESKTOP                                    CLIENTE CELULAR
   shell Wails + WebView2                             navegador
   perfil: privilegiado (loopback)                    perfil: restrito (VPN)
          │                                                  │
          │  HTTPS + WebSocket                               │
          └──────────────────┐            ┌──────────────────┘
                             ▼            ▼
                  ┌────────────────────────────────┐
                  │           jamesd (Go)          │
                  │  bind: interface da VPN         │
                  │        + loopback               │
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

Tudo roda na máquina do Arthur. Nenhuma VPS. Três invariantes:

**I1 — Só o `jamesd` abre porta.** O `james-device` conecta para fora, no loopback. Nunca escuta.

**I2 — O `jamesd` nunca faz bind em `0.0.0.0`.** Bind explícito na interface da VPN, resolvida em runtime (nunca IP fixo em `config.toml` — o spike confirmou que o IP muda a cada relogin), mais loopback.

**I3 — Uma UI, dois invólucros.** Shell desktop e navegador do celular consomem a mesma SPA servida pelo `jamesd`.

**Confirmado pelo spike de 04/08:** bind exclusivo funciona (alcançável do tailnet, recusado em loopback e LAN), `tailscale cert` emite certificado Let's Encrypt válido (exige "HTTPS Certificates" ligado no admin console — já feito), motor Chromium aceita o HTTPS sem aviso. Ressalva do spike: confirmação definitiva com o WebView2 real fica para quando o shell existir — sem motivo técnico pra esperar divergência.

### 3.2 Estrutura de pastas

```
meu-app-ts/                        (nome físico da pasta durante a Fase 1 — ver P6)
├── cmd/
│   ├── jamesd/main.go              binário do back-end
│   └── migrar-historico/main.go    ETL único Athena.db → Postgres
├── internal/
│   ├── agente/
│   │   ├── agente.go               o loop
│   │   ├── provider.go             interface Provider + tipos
│   │   └── gemini/                 implementação Gemini
│   ├── ferramentas/
│   │   ├── registro.go             registry
│   │   ├── terminal.go
│   │   ├── claudecode.go
│   │   ├── busca.go
│   │   └── dispositivo.go          ponte para o james-device
│   ├── permissao/                  perfil (loopback/VPN) + gate de execução
│   ├── api/
│   │   ├── rest.go
│   │   ├── ws.go
│   │   ├── dispositivo.go          endpoint WS do device
│   │   └── identidade.go           resolução de perfil por origem da conexão
│   ├── armazenamento/
│   │   ├── postgres/               repositórios (já existe da Fase 0)
│   │   └── migracoes/              SQL numerado
│   ├── memoria/                    janela de contexto
│   ├── voz/                        TTS
│   ├── higiene/                    guarda de credenciais (já existe, ganha detecção genérica)
│   └── config/                     .env + config.toml
├── dispositivo/
│   ├── james_device/
│   │   ├── conexao.py              cliente WS, reconexão, heartbeat
│   │   ├── wake.py                 wake word + STT (já existe da Fase 0)
│   │   └── tela.py                 cliques e teclado
│   ├── tests/
│   └── pyproject.toml
├── web/                            SPA React (renomeado de frontend/)
├── shell/                          invólucro Wails mínimo
├── prompts/sistema.md              system prompt, fonte única
├── docs/superpowers/specs/
├── docker-compose.yml              Postgres local (já existe)
├── config.toml                     configuração não-secreta
├── .env.example
└── .gitignore
```

**Saem do repositório:** DLLs do Vosk, `TTs/espeak-ng-data/`, `vosk/`, `meu-app-ts.exe`, `Athena.db` e os WAL, `arquivos do Gui/`. `Vault/` permanece intocado no disco, fora do escopo — entrada da Fase 4.

**Áudio:** arquivos gerados por TTS são efêmeros (`.gitignore`); assets que o `james-device` precisa (`entrada.mp3`, `faaah.wav`, `um_minuto.mp3`) são versionados em `dispositivo/assets/`.

**Nomenclatura:** pacotes em português. Termos consagrados do domínio LLM (`Provider`, `tool`, `token`) ficam em inglês no vocabulário; o pacote chama `ferramentas`.

O `app.go` deixa de existir.

---

## 4. Componentes

### 4.1 `agente/`

Executa o loop: monta contexto → chama o `Provider` → despacha tools → devolve resultados ao modelo → repete até texto final ou teto de iterações.

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
    Erro  string
}

type Mensagem struct {
    Papel      Papel
    Texto      string
    Imagem     []byte
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

O `Provider` é a fronteira que torna o loop testável com respostas roteirizadas, sem gastar token. Elimina: prompt duplicado no histórico, histórico sem limite, resposta de tool malformada, fallback `"Ação concluída com sucesso."` mascarando fracasso.

**Modelo Gemini:** lido do `config.toml`, escolha concreta feita na implementação conferindo a lista vigente de modelos. Única restrição travada: `gemini-2.5-flash-lite` está descartado.

### 4.2 `ferramentas/`

Um registro; cada tool num arquivo. Toda tool declara o próprio nível de risco.

```go
package ferramentas

type Nivel int

const (
    NivelSeguro   Nivel = iota
    NivelPerigoso
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

| Tool | Nível |
|---|---|
| `executar_comando_sistema` | perigoso |
| `perguntar_ao_claude_code` | perigoso |
| `manipular_interface` | perigoso |
| `pesquisar_internet` | seguro |

**Importante:** as quatro entram atrás desta interface com o comportamento que têm hoje. Consertar comportamento é Fase 3.

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
- `Restrito` → executa apenas `NivelSeguro`. Nível perigoso cria aprovação pendente e emite `aprovacao_requerida` aos clientes privilegiados. Espera decisão ou expira por timeout (`config.toml`, default 300s).

**Resolução de perfil — uma camada só (Decisão P5):**

1. Conexão originada do loopback → `Privilegiado`. Não é escalada de privilégio: qualquer processo já rodando naquele PC podia abrir PowerShell por conta própria.
2. Conexão originada da interface da VPN → `Restrito`, sem exceção.

Sem lista de endereços. A escalação por dispositivo específico via identidade de peer do Tailscale fica registrada como item de backlog (seção 8 deste documento).

### 4.4 `api/`

**REST**

| Método | Rota | Descrição |
|---|---|---|
| `GET` | `/api/saude` | status do serviço, do banco e do device |
| `GET` | `/api/sessoes` | lista sessões |
| `POST` | `/api/sessoes` | cria sessão |
| `GET` | `/api/sessoes/{id}/mensagens` | histórico paginado |
| `GET` | `/api/audio/{id}` | áudio TTS de uma resposta |

**WebSocket** — `/ws` para clientes, `/ws/dispositivo` para o `james-device`. Envelope comum:

```json
{ "tipo": "<nome>", "id": "<correlação>", "dados": { } }
```

| Direção | Tipos |
|---|---|
| Cliente → servidor | `mensagem`, `aprovar`, `negar`, `cancelar` |
| Servidor → cliente | `token`, `ferramenta_iniciada`, `ferramenta_concluida`, `aprovacao_requerida`, `resposta_final`, `audio_pronto`, `status_dispositivo`, `erro` |
| Device → servidor | `registro`, `entrada_voz`, `resultado_acao`, `heartbeat` |
| Servidor → device | `acao_tela`, `acao_teclado`, `ping` |

### 4.5 `armazenamento/`

PostgreSQL via `pgx` (camada de conexão já entregue na Fase 0). Repositórios por agregado, migrações em SQL numerado aplicadas na subida. Sem ORM.

### 4.6 `memoria/`

Monta a janela de contexto: últimas N mensagens da sessão dentro de um orçamento de tokens configurável. Elimina o envio do dia inteiro em toda requisição. A Fase 4 pluga o Obsidian aqui.

### 4.7 `voz/`

TTS via `edge-tts`, assíncrono, com timeout, arquivo por resposta, erro propagado. O back-end **gera** o áudio e entrega por URL (`/api/audio/{id}`) — **quem toca é o cliente**, não o servidor. Evita o computador de casa falar sozinho quando o Arthur usa o celular na rua, e elimina a corrida no arquivo fixo antigo.

### 4.8 `config/`

`.env` guarda somente segredos. `config.toml` guarda o resto (bind, modelo, orçamento de tokens, timeouts, timeout de aprovação). Sem seção de perfil por dispositivo — a resolução loopback×VPN (P5) é lógica de código, não configuração. Validação na subida com falha rápida: falta chave, o `jamesd` não sobe e diz qual faltou.

System prompt sai do código Go e vira `prompts/sistema.md`, fonte única.

### 4.9 `higiene/` (atualizado — Decisão P2)

Além dos 4 prefixos literais já cobertos (Gemini, Tavily, NVIDIA, OpenRouter), ganha detecção por regex para segredos sem prefixo fixo:

- connection string com senha embutida: `://[^:/@]+:[^:/@]+@`
- AWS access key: `AKIA[0-9A-Z]{16}`
- token GitHub: `gh[pousr]_[A-Za-z0-9]{36,}`
- JWT: três segmentos base64url separados por `.`, começando em `eyJ`
- bloco de chave privada PEM: `-----BEGIN [A-Z ]*PRIVATE KEY-----`

Mesma filosofia da Fase 0: roda dentro de `go test ./...`, mesmas exclusões (`.env` por design, DLLs de terceiros, diretórios de dependência).

### 4.10 `web/` e `shell/`

`shell/` vira invólucro Wails mínimo: janela frameless, transparente, always-on-top, WebView2 apontando pra SPA. Bindings `wailsjs/go/main/App` morrem.

`web/` (renomeado de `frontend/`) é a UI atual, sem redesenho, com bindings trocados por cliente HTTP/WS — **mais o card de aprovação mínimo** (Decisão P4): componente que escuta `aprovacao_requerida`, mostra a ação pedida e dois botões (aprovar/negar), envia a decisão de volta pelo mesmo WS. Sem estilo elaborado — funcional, não é a Fase 6.

### 4.11 `cmd/migrar-historico/` (novo — Decisão P3)

Comando avulso, não parte do boot do `jamesd`. Lê `Athena.db` (path via flag, default `./Athena.db`), cria sessão sintética `"Histórico Athena"`, mapeia `role`→`papel` (`user`→`usuario`, `assistant`→`modelo`), insere em `mensagens`. Idempotente: rodar duas vezes não duplica (verifica se a sessão sintética já existe antes de inserir). Roda uma vez, à mão, depois que o `jamesd` já está de pé com as migrações aplicadas.

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

Sem alteração em relação à spec de 03/08 — `aprovacoes.estado` já contempla `'expirada'`, usado pelo timeout do card de aprovação mínimo (P4).

---

## 6. Tratamento de erro

**R1 — Falha rápida na subida.** Chave ausente, Postgres fora do ar ou migração pendente impedem o `jamesd` de subir, com mensagem dizendo o que faltou.

**R2 — Erro de tool não derruba o loop.** Volta pro modelo como `ResultadoFerramenta.Erro`.

**R3 — Dispositivo ausente é resposta, não crash.** Sem `james-device` conectado, a tool de tela retorna "máquina física indisponível".

**R4 — Timeout em tudo.** Chamada ao modelo, execução de tool, espera por aprovação, geração de TTS.

**R5 — Teto de iterações com mensagem honesta.** Ao estourar, o James informa que não concluiu e o que travou.

**R6 — Log estruturado com `log/slog`, e nunca segredo em log.**

Princípio: o modelo aprende do erro, o usuário enxerga o erro, o processo sobrevive ao erro.

---

## 7. Estratégia de teste

| Camada | Como | Contra o quê |
|---|---|---|
| `agente` | provider falso, respostas roteirizadas | loop, dispatch, teto de iterações |
| `permissao` | tabela de casos, lógica pura | loopback × VPN → perfil × nível (duas linhas, simplificado por P5) |
| `ferramentas` | fakes para exec e HTTP | contrato de cada tool |
| `higiene` | fixtures com cada padrão genérico novo | detecção de connection string, AWS, GitHub, JWT, PEM |
| `armazenamento` | Postgres real em Docker | SQL de verdade, não mock |
| `api` | `httptest` + cliente WS real | contrato REST e do socket, incluindo fluxo de aprovação |
| `james-device` | `pytest` + servidor WS falso | conexão, reconexão, heartbeat |
| `web` | `vitest` | cliente de API, componente de aprovação |
| `cmd/migrar-historico` | fixture de `Athena.db` de teste | mapeamento de papel, idempotência |

Harness externo: `go vet ./... && go test ./...`, `pytest`, `vitest` mais navegador real via `webapp-testing`, e `/code-review` + `/security-review` no diff antes de fechar.

**Primeiro teste do projeto:** `permissao` — núcleo de segurança, lógica pura, não depende de mais nada existir.

---

## 8. Escopo

### Entra

- Reestruturação de pastas dentro de `meu-app-ts/` e rename Athena → James no código (módulo, binário `jamesd`, agente `james-device`, banco, wake word, voz `pt-BR-AntonioNeural`, persona "primo caçula do Jarvis") — **exceto** o rename físico da pasta raiz (P6, manual)
- `jamesd`: agente, `Provider`+Gemini, registro de ferramentas, permissão (loopback/VPN, sem allowlist por IP), API REST+WS, armazenamento Postgres, memória com janela, TTS entregue ao cliente, config
- `internal/higiene` com detecção genérica além dos 4 prefixos atuais
- `james-device`: Python com WS, reconexão, heartbeat; voz (wake word já portada da Fase 0) e tela portadas como estão
- `shell` Wails mínimo; `web` repontado pra API, com card de aprovação mínimo
- `cmd/migrar-historico`: comando avulso, idempotente
- Bind na interface da VPN, resolvido em runtime
- Remoção do código morto (`chamadaResumo`, `toolBusca` não usada, `Resumir` vazio)

### Não entra

| Fora | Fase |
|---|---|
| Qualidade das tools: Claude Code headless, busca melhor, tool de Spotify | 3 |
| Obsidian, RAG, `pgvector` | 4 |
| UI Automation, conserto do clique, calibração real da wake word | 5 |
| Abas verticais, visualizador reativo, redesenho da UI, polimento visual do card de aprovação | 6 |
| Allowlist de dispositivo por identidade de peer Tailscale | backlog (seção 10) |
| Rename físico da pasta raiz `meu-app-ts` → `james` | manual, pós-Fase 1 (P6) |
| Roteamento multi-modelo por custo | — |
| Serviço do Windows, auto-start | — |

**Explicitamente:** a Fase 1 não melhora o comportamento das tools. A busca continua fraca, o Claude Code continua abrindo janela, o clique continua errando.

---

## 9. Critérios de aceitação

Dez verificações, todas com saída colável:

1. `go vet ./... && go test ./...` passa, incluindo os testes novos de `higiene` genérica e da tabela simplificada de `permissao`
2. `pytest` passa
3. `npm test` passa
4. `jamesd` sobe, conecta no Postgres e aplica as migrações; falha rápido e com mensagem clara se faltar config ou segredo
5. O shell abre e a conversa funciona, com as mesmas capacidades de hoje
6. O celular na VPN abre a mesma UI e conversa em modo Restrito; ação perigosa dispara o card de aprovação no desktop, e aprovar/negar funciona fim a fim
7. Matar o `james-device` **não** derruba o `jamesd`; a tool de tela retorna erro claro
8. `cmd/migrar-historico` importa as mensagens do `Athena.db` pra sessão `"Histórico Athena"`; rodar duas vezes não duplica
9. `git log -p` sem nenhuma credencial em nenhum commit
10. Nenhuma chave em texto puro no código

---

## 10. Riscos

| Risco | Mitigação |
|---|---|
| Migração big-bang — a Fase 1 mexe em tudo de uma vez | Plano começa pelo testável isolado (`permissao`, `provider`, `memoria`), troca do transporte da UI por último; o app fica quebrado pelo menor tempo possível |
| Tailscale — bind, TLS, identidade de peer | **Resolvido pelo spike de 04/08** para bind e TLS. Identidade de peer segue não verificada, mas a Fase 1 não depende dela (permissão é só loopback × VPN, P5) |
| Card de aprovação mínimo pode não ser suficiente pro uso real no dia a dia (sem notificação sonora/persistente) | Aceito como mínimo funcional pro critério de aceitação #6; polimento é Fase 6 |
| Rename físico da pasta raiz fica manual, fora do plano | Módulo Go e binários já usam `james` internamente; código e docs referenciam `james/` como nome-alvo mesmo a pasta ainda sendo `meu-app-ts/` até o rename manual — sem bloqueio técnico, só desalinhamento cosmético temporário |
| Wake word "James" em português | Lista de 5 variantes já portada e testada na Fase 0 (`dispositivo/james_device/wake.py`); calibração com gravações reais fica pra Fase 5 |

---

## 11. Backlog aberto (não bloqueia)

1. Allowlist de dispositivo específico via identidade de peer do Tailscale — volta quando essa identidade for verificada num spike futuro (removida do escopo da Fase 1 por P5)
2. WebView2 real ainda não testado contra o HTTPS do tailnet dentro do shell Wails de fato — o spike usou um Chromium genérico. Confirmar quando o shell da Fase 1 existir
3. Critério formal pra quando uma correção descoberta em execução vira seção documentada no plano vs. só um commit solto (herdado do backlog da Fase 0)

---

## Anexo A — Bugs desta fase

Subconjunto do Anexo A da spec de 03/08, filtrado pros itens que a Fase 1 resolve.

| # | Problema | Local | 
|---|---|---|
| 3 | Prompt duplicado no histórico a cada chamada | `app.go:466` + `app.go:483` |
| 4 | Histórico sem limite: envia o dia inteiro em toda requisição | `app.go:359` |
| 5 | Panic por `connPython` nil quando o Python não está conectado | `app.go:218` |
| 6 | Resposta de tool com `Role: "tool"` — comportamento a verificar | `app.go:550` |
| 7 | `edge-tts` síncrono, erro ignorado, arquivo de saída fixo | `app.go:566` |
| 8 | `speaker.Init` trava a taxa de amostragem no primeiro áudio | `app.go:266` |
| 9 | Fallback `"Ação concluída com sucesso."` mascarando fracasso | `app.go:559` |
| 10 | `chamadaResumo` (classificador WEB/LOCAL/CHAT) é código morto | `app.go:382` |
| 11 | `Resumir()` é stub vazio | `app.go:61` |
| 12 | Socket TCP cru, sem framing nem reconexão | `app.go:277`; `processador_voz.py` |
| 13 | System prompt duplicado e já divergente | `app.go:151` vs `Vault/Contexto_Core.md` |
| 14 | Modelo `gemini-2.5-flash-lite` — o mais fraco da família | `app.go:494` |
| 19 | Screenshot só é enviado se o texto contiver a palavra "tela" (move nesta fase, conserta na 5) | `frontend/src/App.tsx:59` |

**Evidência do impacto:** em `Vault/Logs_Diarios/2026-07-08.md`, 10 das 11 respostas foram o fallback `"Ação concluída com sucesso."`.
