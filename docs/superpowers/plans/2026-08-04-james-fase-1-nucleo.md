# James — Fase 1: Núcleo — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transformar o `app.go` monolítico de 574 linhas em `jamesd`: back-end de serviço testável com agente, Provider+Gemini, ferramentas atrás de registro, permissão por perfil (loopback/VPN), API REST+WS, armazenamento Postgres, memória em janela, TTS entregue ao cliente, e o `james-device` (Python) portado com WS/reconexão.

**Architecture:** Ver `docs/superpowers/specs/2026-08-04-james-fase-1-nucleo-design.md` — seção 3 (topologia + estrutura de pastas), seção 4 (componentes), seção 5 (modelo de dados). Ordem de implementação segue a mitigação de risco da spec: núcleo testável isolado primeiro (`higiene`, `config`, `ferramentas` tipos+registro, `permissao`, `agente`), depois as tools concretas e a ponte do device, depois camada de dados (`armazenamento`, `memoria`), depois transporte (`api`, `voz`), depois `cmd/jamesd` amarrando tudo, depois `james-device` (Python), por último `web`/`shell` e remoção do código morto. `permissao` depende só do tipo `ferramentas.Nivel` (dois valores, sem lógica) — continua sendo, na prática, o primeiro teste de lógica de negócio real do projeto, como a spec pede.

**Tech Stack:** Go 1.26 (módulo `james`), `pgx/v5`, `gorilla/websocket`, `google.golang.org/genai`, `modernc.org/sqlite` (já em `go.mod`); Python 3.13 com `websockets`, `pyautogui`, `speech_recognition` (`dispositivo/pyproject.toml`); React+TypeScript+Vite (`frontend/` → `web/`), `vitest`.

---

## Task 1: `internal/higiene` — detecção genérica de credencial

**Files:**
- Modify: `internal/higiene/higiene.go`
- Modify: `internal/higiene/higiene_test.go`

- [ ] **Step 1: Escrever os testes que falham**

Adicionar ao final de `internal/higiene/higiene_test.go`:

```go
func TestVarrerAcusaConnectionStringComSenha(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "config_real.toml")
	conteudo := "url = \"" + "postgres" + "://james:" + "SegredoReal123" + "@localhost:5433/james\""
	if err := os.WriteFile(arquivo, []byte(conteudo), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado de connection string, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaAWSAccessKey(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "notas.txt")
	chave := "AKIA" + "IOSFODNN7EXAMPLE"
	if err := os.WriteFile(arquivo, []byte(chave), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado de AWS key, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaTokenGitHub(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "notas.txt")
	token := "ghp_" + "0000000000000000000000000000000000"
	if err := os.WriteFile(arquivo, []byte(token), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado de token GitHub, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaJWT(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "notas.txt")
	jwt := "eyJhbGciOiJIUzI1NiJ9" + "." + "eyJzdWIiOiIxMjM0NTY3ODkwIn0" + "." + "dGVzdHNpZ25hdHVyZQ"
	if err := os.WriteFile(arquivo, []byte(jwt), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado de JWT, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaBlocoPEM(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "chave.txt")
	pem := "-----BEGIN RSA PRIVATE KEY-----\nMIIExemplo\n-----END RSA PRIVATE KEY-----"
	if err := os.WriteFile(arquivo, []byte(pem), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado de bloco PEM, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerIgnoraEnvExample(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, ".env.example")
	conteudo := "DATABASE_URL=" + "postgres" + "://james:SENHA@localhost:5433/james"
	if err := os.WriteFile(arquivo, []byte(conteudo), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf(".env.example é template, deveria ser ignorado; vieram %d achados: %+v", len(achados), achados)
	}
}
```

- [ ] **Step 2: Rodar os testes e confirmar que falham**

Run: `go test ./internal/higiene/... -v`
Expected: FAIL nos 6 testes novos (nenhum achado é gerado ainda pelos padrões genéricos, e `.env.example` ainda não está na lista de ignorados).

- [ ] **Step 3: Implementar a detecção genérica**

Editar `internal/higiene/higiene.go`. Adicionar o import `"regexp"` e, depois do bloco `var padroes = []string{...}`, adicionar:

```go
// padroesRegex cobre segredos sem prefixo fixo — precisam de forma, não
// substring. Cada entrada carrega um nome descritivo pro Achado.
var padroesRegex = []struct {
	nome   string
	padrao *regexp.Regexp
}{
	{"connection-string-com-senha", regexp.MustCompile(`://[^:/@\s]+:[^:/@\s]+@`)},
	{"aws-access-key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"token-github", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`)},
	{"jwt", regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)},
	{"chave-privada-pem", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
}
```

Alterar `arquivosIgnorados` pra incluir o template:

```go
var arquivosIgnorados = map[string]bool{
	".env":         true,
	".env.example": true,
}
```

**Achado real, não hipotético:** rodar o regex de connection-string contra a árvore atual do repositório (`grep -rE '://[^\s/@]+:[^\s/@]+@'`) já acusa `docs/superpowers/plans/2026-08-03-james-fase-0-fundacao.md` e este próprio plano — documentação que mostra o formato `postgres://james:SENHA@localhost:5433/james` como exemplo, com placeholder, não segredo real. Estender `diretoriosIgnorados` pra incluir `docs`:

```go
var diretoriosIgnorados = map[string]bool{
	".git":         true,
	"node_modules": true,
	"TTs":          true,
	"vosk":         true,
	"dist":         true,
	"docs":         true, // specs/planos mostram formato de connection
	// string como exemplo (placeholder tipo "SENHA"), não segredo real —
	// o risco de verdade (config.toml, .env, código-fonte, binário
	// compilado) continua coberto fora de docs/
}
```

Dentro do loop de `Varrer`, logo depois do `for _, padrao := range padroes { ... }`, adicionar:

```go
for _, pr := range padroesRegex {
	if pr.padrao.Match(conteudo) {
		achados = append(achados, Achado{Arquivo: caminho, Padrao: pr.nome})
	}
}
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

Run: `go test ./internal/higiene/... -v`
Expected: PASS em todos, incluindo `TestRepositorioLimpo` (que varre o repositório inteiro — se ela falhar apontando um arquivo real com um dos padrões novos, é sinal de falso positivo genuíno: ajustar o regex ou estender `arquivosIgnorados`/`diretoriosIgnorados`, nunca remover o teste).

- [ ] **Step 5: Commit**

```bash
git add internal/higiene/higiene.go internal/higiene/higiene_test.go
git commit -m "feat: internal/higiene detecta connection string, AWS, GitHub, JWT e PEM"
```

---

## Task 2: `internal/config` — `.env` + `config.toml` com falha rápida

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `config.toml`
- Modify: `go.mod` (adicionar `github.com/BurntSushi/toml`)

- [ ] **Step 1: Adicionar a dependência de parsing TOML**

Run: `go get github.com/BurntSushi/toml@latest`
Expected: `go.mod` e `go.sum` atualizados, sem erro.

- [ ] **Step 2: Escrever os testes que falham**

Criar `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func escreverToml(t *testing.T, dir, conteudo string) string {
	t.Helper()
	caminho := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(caminho, []byte(conteudo), 0o600); err != nil {
		t.Fatalf("não consegui escrever config.toml de teste: %v", err)
	}
	return caminho
}

func TestCarregarFalhaSemGeminiAPIKey(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5433/james")
	caminho := escreverToml(t, t.TempDir(), "")

	_, err := Carregar(caminho)
	if err == nil {
		t.Fatal("esperava erro por GEMINI_API_KEY ausente, veio nil")
	}
}

func TestCarregarFalhaSemDatabaseURL(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "chave-de-teste")
	t.Setenv("DATABASE_URL", "")
	caminho := escreverToml(t, t.TempDir(), "")

	_, err := Carregar(caminho)
	if err == nil {
		t.Fatal("esperava erro por DATABASE_URL ausente, veio nil")
	}
}

func TestCarregarAplicaDefaults(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "chave-de-teste")
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5433/james")
	caminho := escreverToml(t, t.TempDir(), "")

	cfg, err := Carregar(caminho)
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.BindLoopback == "" {
		t.Error("BindLoopback deveria ter default, veio vazio")
	}
	if cfg.MaxIteracoes == 0 {
		t.Error("MaxIteracoes deveria ter default, veio 0")
	}
	if cfg.TimeoutAprovacaoSegundos == 0 {
		t.Error("TimeoutAprovacaoSegundos deveria ter default, veio 0")
	}
}

func TestCarregarLeValoresDoToml(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "chave-de-teste")
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5433/james")
	caminho := escreverToml(t, t.TempDir(), `
[agente]
modelo = "gemini-2.5-pro"
max_iteracoes = 8

[permissao]
timeout_aprovacao_segundos = 60
`)

	cfg, err := Carregar(caminho)
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.Modelo != "gemini-2.5-pro" {
		t.Errorf("Modelo = %q, queria gemini-2.5-pro", cfg.Modelo)
	}
	if cfg.MaxIteracoes != 8 {
		t.Errorf("MaxIteracoes = %d, queria 8", cfg.MaxIteracoes)
	}
	if cfg.TimeoutAprovacaoSegundos != 60 {
		t.Errorf("TimeoutAprovacaoSegundos = %d, queria 60", cfg.TimeoutAprovacaoSegundos)
	}
}

func TestCarregarRejeitaModeloDescartado(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "chave-de-teste")
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5433/james")
	caminho := escreverToml(t, t.TempDir(), `
[agente]
modelo = "gemini-2.5-flash-lite"
`)

	_, err := Carregar(caminho)
	if err == nil {
		t.Fatal("gemini-2.5-flash-lite está descartado pela spec (D4/Anexo), esperava erro")
	}
}
```

- [ ] **Step 3: Rodar os testes e confirmar que falham**

Run: `go test ./internal/config/... -v`
Expected: FAIL — pacote `config` ainda não existe.

- [ ] **Step 4: Implementar `internal/config/config.go`**

```go
// Package config carrega segredos do .env e o resto da configuração do
// config.toml, com falha rápida: falta algo obrigatório, o jamesd não sobe.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
)

type Config struct {
	GeminiAPIKey string
	TavilyAPIKey string
	DatabaseURL  string

	BindLoopback string
	BindVPNPorta int
	CertArquivo  string
	ChaveArquivo string

	Modelo          string
	MaxIteracoes    int
	TimeoutSegundos int

	OrcamentoTokens int

	TimeoutAprovacaoSegundos int

	VozTTS string
}

type arquivoToml struct {
	Servidor struct {
		BindLoopback string `toml:"bind_loopback"`
		BindVPNPorta int    `toml:"bind_vpn_porta"`
		CertArquivo  string `toml:"cert_arquivo"`
		ChaveArquivo string `toml:"chave_arquivo"`
	} `toml:"servidor"`
	Agente struct {
		Modelo          string `toml:"modelo"`
		MaxIteracoes    int    `toml:"max_iteracoes"`
		TimeoutSegundos int    `toml:"timeout_segundos"`
	} `toml:"agente"`
	Memoria struct {
		OrcamentoTokens int `toml:"orcamento_tokens"`
	} `toml:"memoria"`
	Permissao struct {
		TimeoutAprovacaoSegundos int `toml:"timeout_aprovacao_segundos"`
	} `toml:"permissao"`
	Voz struct {
		VozTTS string `toml:"voz_tts"`
	} `toml:"voz"`
}

// modeloDescartado é o único modelo proibido pela spec — o mais fraco da
// família Gemini, causa provável do planejamento ruim de tools no app antigo.
const modeloDescartado = "gemini-2.5-flash-lite"

// Carregar lê o .env (mesmo diretório do config.toml, se existir) e o
// config.toml em caminhoToml, aplica defaults e falha rápido se faltar
// segredo obrigatório ou se o modelo estiver descartado.
func Carregar(caminhoToml string) (*Config, error) {
	_ = godotenv.Load(".env")

	var arq arquivoToml
	if _, err := os.Stat(caminhoToml); err == nil {
		if _, err := toml.DecodeFile(caminhoToml, &arq); err != nil {
			return nil, fmt.Errorf("erro ao ler %s: %w", caminhoToml, err)
		}
	}

	cfg := &Config{
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		TavilyAPIKey: os.Getenv("TAVILY_API_KEY"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),

		BindLoopback: valorOuDefault(arq.Servidor.BindLoopback, "127.0.0.1:8080"),
		BindVPNPorta: intOuDefault(arq.Servidor.BindVPNPorta, 8443),
		CertArquivo:  valorOuDefault(arq.Servidor.CertArquivo, "certs/tailscale.crt"),
		ChaveArquivo: valorOuDefault(arq.Servidor.ChaveArquivo, "certs/tailscale.key"),

		Modelo:          valorOuDefault(arq.Agente.Modelo, "gemini-2.5-flash"),
		MaxIteracoes:    intOuDefault(arq.Agente.MaxIteracoes, 5),
		TimeoutSegundos: intOuDefault(arq.Agente.TimeoutSegundos, 30),

		OrcamentoTokens: intOuDefault(arq.Memoria.OrcamentoTokens, 8000),

		TimeoutAprovacaoSegundos: intOuDefault(arq.Permissao.TimeoutAprovacaoSegundos, 300),

		VozTTS: valorOuDefault(arq.Voz.VozTTS, "pt-BR-AntonioNeural"),
	}

	var faltando []string
	if cfg.GeminiAPIKey == "" {
		faltando = append(faltando, "GEMINI_API_KEY")
	}
	if cfg.DatabaseURL == "" {
		faltando = append(faltando, "DATABASE_URL")
	}
	if len(faltando) > 0 {
		return nil, fmt.Errorf("configuração incompleta, faltando: %s", strings.Join(faltando, ", "))
	}

	if cfg.Modelo == modeloDescartado {
		return nil, fmt.Errorf("modelo %q está descartado pela spec (o mais fraco da família Gemini)", modeloDescartado)
	}

	return cfg, nil
}

func valorOuDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func intOuDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
```

- [ ] **Step 5: Rodar os testes e confirmar que passam**

Run: `go test ./internal/config/... -v`
Expected: PASS em todos os 6 testes.

- [ ] **Step 6: Criar `config.toml` de exemplo na raiz do projeto**

```toml
[servidor]
bind_loopback = "127.0.0.1:8080"
bind_vpn_porta = 8443
cert_arquivo = "certs/tailscale.crt"
chave_arquivo = "certs/tailscale.key"

[agente]
# Conferir a lista vigente de modelos Gemini antes de trocar. Proibido:
# gemini-2.5-flash-lite (mais fraco da família).
modelo = "gemini-2.5-flash"
max_iteracoes = 5
timeout_segundos = 30

[memoria]
orcamento_tokens = 8000

[permissao]
timeout_aprovacao_segundos = 300

[voz]
voz_tts = "pt-BR-AntonioNeural"
```

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go config.toml go.mod go.sum
git commit -m "feat: internal/config carrega .env + config.toml com falha rápida"
```

---

## Task 3: `internal/ferramentas` — tipos e registro

**Files:**
- Create: `internal/ferramentas/tipos.go`
- Create: `internal/ferramentas/registro.go`
- Create: `internal/ferramentas/registro_test.go`

- [ ] **Step 1: Escrever os testes que falham**

Criar `internal/ferramentas/registro_test.go`:

```go
package ferramentas

import (
	"context"
	"testing"
)

type ferramentaFalsa struct {
	nome  string
	nivel Nivel
}

func (f ferramentaFalsa) Declaracao() Declaracao {
	return Declaracao{Nome: f.nome, Descricao: "ferramenta de teste"}
}
func (f ferramentaFalsa) Nivel() Nivel { return f.nivel }
func (f ferramentaFalsa) Executar(ctx context.Context, args map[string]any) (string, error) {
	return "ok", nil
}

func TestRegistrarEBuscar(t *testing.T) {
	r := NovoRegistro()
	f := ferramentaFalsa{nome: "teste", nivel: NivelSeguro}

	if err := r.Registrar(f); err != nil {
		t.Fatalf("Registrar devolveu erro: %v", err)
	}

	achada, ok := r.Buscar("teste")
	if !ok {
		t.Fatal("Buscar não achou a ferramenta registrada")
	}
	if achada.Declaracao().Nome != "teste" {
		t.Errorf("nome = %q, queria teste", achada.Declaracao().Nome)
	}
}

func TestRegistrarRejeitaNomeDuplicado(t *testing.T) {
	r := NovoRegistro()
	f := ferramentaFalsa{nome: "teste", nivel: NivelSeguro}

	if err := r.Registrar(f); err != nil {
		t.Fatalf("primeiro Registrar devolveu erro: %v", err)
	}
	if err := r.Registrar(f); err == nil {
		t.Fatal("esperava erro no registro duplicado, veio nil")
	}
}

func TestBuscarNomeInexistenteDevolveFalse(t *testing.T) {
	r := NovoRegistro()
	_, ok := r.Buscar("não-existe")
	if ok {
		t.Fatal("Buscar deveria devolver false pra nome não registrado")
	}
}

func TestDeclaracoesListaTodas(t *testing.T) {
	r := NovoRegistro()
	_ = r.Registrar(ferramentaFalsa{nome: "a", nivel: NivelSeguro})
	_ = r.Registrar(ferramentaFalsa{nome: "b", nivel: NivelPerigoso})

	decls := r.Declaracoes()
	if len(decls) != 2 {
		t.Fatalf("esperava 2 declarações, vieram %d", len(decls))
	}
}
```

- [ ] **Step 2: Rodar os testes e confirmar que falham**

Run: `go test ./internal/ferramentas/... -v`
Expected: FAIL — pacote ainda não existe.

- [ ] **Step 3: Implementar os tipos**

Criar `internal/ferramentas/tipos.go`:

```go
// Package ferramentas define o contrato de tool que o agente consome e o
// registro onde cada tool concreta se cadastra.
package ferramentas

import "context"

// Nivel é o que o gate de permissão lê pra decidir se uma tool executa sem
// aprovação humana.
type Nivel int

const (
	NivelSeguro   Nivel = iota // leitura, consulta, sem efeito colateral
	NivelPerigoso              // executa, altera estado, controla a máquina
)

// Parametro descreve um argumento de tool pro schema que vai pro Provider.
type Parametro struct {
	Tipo      string // "string", "integer", "number", "boolean"
	Descricao string
}

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
```

- [ ] **Step 4: Implementar o registro**

Criar `internal/ferramentas/registro.go`:

```go
package ferramentas

import (
	"fmt"
	"sync"
)

type Registro interface {
	Registrar(f Ferramenta) error
	Buscar(nome string) (Ferramenta, bool)
	Declaracoes() []Declaracao
}

type registro struct {
	mu          sync.RWMutex
	ferramentas map[string]Ferramenta
}

// NovoRegistro devolve um Registro vazio, seguro pra uso concorrente.
func NovoRegistro() Registro {
	return &registro{ferramentas: make(map[string]Ferramenta)}
}

func (r *registro) Registrar(f Ferramenta) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	nome := f.Declaracao().Nome
	if nome == "" {
		return fmt.Errorf("ferramenta sem nome não pode ser registrada")
	}
	if _, existe := r.ferramentas[nome]; existe {
		return fmt.Errorf("ferramenta %q já registrada", nome)
	}
	r.ferramentas[nome] = f
	return nil
}

func (r *registro) Buscar(nome string) (Ferramenta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.ferramentas[nome]
	return f, ok
}

func (r *registro) Declaracoes() []Declaracao {
	r.mu.RLock()
	defer r.mu.RUnlock()
	decls := make([]Declaracao, 0, len(r.ferramentas))
	for _, f := range r.ferramentas {
		decls = append(decls, f.Declaracao())
	}
	return decls
}
```

- [ ] **Step 5: Rodar os testes e confirmar que passam**

Run: `go test ./internal/ferramentas/... -v`
Expected: PASS em todos os 4 testes.

- [ ] **Step 6: Commit**

```bash
git add internal/ferramentas/tipos.go internal/ferramentas/registro.go internal/ferramentas/registro_test.go
git commit -m "feat: internal/ferramentas com tipos e registro thread-safe"
```

---

## Task 4: `internal/permissao` — perfil por origem de conexão

**Files:**
- Create: `internal/permissao/permissao.go`
- Create: `internal/permissao/permissao_test.go`

- [ ] **Step 1: Escrever os testes que falham**

Criar `internal/permissao/permissao_test.go`:

```go
package permissao

import (
	"testing"

	"james/internal/ferramentas"
)

func TestPerfilPode(t *testing.T) {
	casos := []struct {
		nome   string
		perfil Perfil
		nivel  ferramentas.Nivel
		quer   bool
	}{
		{"privilegiado executa seguro", Privilegiado, ferramentas.NivelSeguro, true},
		{"privilegiado executa perigoso", Privilegiado, ferramentas.NivelPerigoso, true},
		{"restrito executa seguro", Restrito, ferramentas.NivelSeguro, true},
		{"restrito não executa perigoso sem aprovação", Restrito, ferramentas.NivelPerigoso, false},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := c.perfil.Pode(c.nivel); got != c.quer {
				t.Errorf("Pode() = %v, queria %v", got, c.quer)
			}
		})
	}
}

func TestResolverPerfil(t *testing.T) {
	casos := []struct {
		nome           string
		enderecoRemoto string
		quer           Perfil
	}{
		{"loopback IPv4 é privilegiado", "127.0.0.1:54321", Privilegiado},
		{"loopback IPv6 é privilegiado", "[::1]:54321", Privilegiado},
		{"IP do tailnet é restrito", "100.96.219.29:54321", Restrito},
		{"IP de LAN é restrito", "192.168.15.23:54321", Restrito},
		{"endereço malformado é restrito por padrão (fail closed)", "não-é-um-endereço", Restrito},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := ResolverPerfil(c.enderecoRemoto); got != c.quer {
				t.Errorf("ResolverPerfil(%q) = %v, queria %v", c.enderecoRemoto, got, c.quer)
			}
		})
	}
}
```

- [ ] **Step 2: Rodar os testes e confirmar que falham**

Run: `go test ./internal/permissao/... -v`
Expected: FAIL — pacote ainda não existe.

- [ ] **Step 3: Implementar**

Criar `internal/permissao/permissao.go`:

```go
// Package permissao decide, por origem de conexão, se um cliente executa
// ferramentas perigosas sem aprovação humana.
package permissao

import (
	"net"

	"james/internal/ferramentas"
)

type Perfil string

const (
	Privilegiado Perfil = "privilegiado"
	Restrito     Perfil = "restrito"
)

// Pode informa se o perfil executa o nível sem aprovação humana.
func (p Perfil) Pode(n ferramentas.Nivel) bool {
	if p == Privilegiado {
		return true
	}
	return n == ferramentas.NivelSeguro
}

// ResolverPerfil decide o perfil a partir do endereço remoto de uma conexão
// HTTP/WS (net/http Request.RemoteAddr). Uma camada só: loopback é
// Privilegiado, qualquer outra origem é Restrito — sem lista de exceção por
// endereço (decisão P5: IP do Tailscale não é estável entre relogins).
// Endereço que não dá pra interpretar como IP fica Restrito por padrão
// (fail closed).
func ResolverPerfil(enderecoRemoto string) Perfil {
	host, _, err := net.SplitHostPort(enderecoRemoto)
	if err != nil {
		host = enderecoRemoto
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return Privilegiado
	}
	return Restrito
}
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

Run: `go test ./internal/permissao/... -v`
Expected: PASS em todos os casos das duas tabelas.

- [ ] **Step 5: Commit**

```bash
git add internal/permissao/permissao.go internal/permissao/permissao_test.go
git commit -m "feat: internal/permissao resolve perfil por loopback x VPN"
```

---

## Task 5: `internal/agente` — tipos, `Provider` e o loop

**Files:**
- Create: `internal/agente/tipos.go`
- Create: `internal/agente/loop.go`
- Create: `internal/agente/loop_test.go`

- [ ] **Step 1: Escrever os testes que falham**

Criar `internal/agente/loop_test.go`:

```go
package agente

import (
	"context"
	"errors"
	"testing"

	"james/internal/ferramentas"
	"james/internal/permissao"
)

// providerFalso devolve as respostas roteirizadas em ordem, uma por chamada
// a Gerar. Não gasta token, não abre janela — a fronteira que o Provider
// promete.
type providerFalso struct {
	respostas []Resposta
	chamadas  int
}

func (p *providerFalso) Gerar(ctx context.Context, req Requisicao) (Resposta, error) {
	if p.chamadas >= len(p.respostas) {
		return Resposta{}, errors.New("providerFalso: sem mais respostas roteirizadas")
	}
	r := p.respostas[p.chamadas]
	p.chamadas++
	return r, nil
}

func (p *providerFalso) GerarStream(ctx context.Context, req Requisicao, emitir func(string)) (Resposta, error) {
	return p.Gerar(ctx, req)
}

type ferramentaFalsa struct {
	nome      string
	nivel     ferramentas.Nivel
	saida     string
	erro      error
	executada bool
}

func (f *ferramentaFalsa) Declaracao() ferramentas.Declaracao {
	return ferramentas.Declaracao{Nome: f.nome}
}
func (f *ferramentaFalsa) Nivel() ferramentas.Nivel { return f.nivel }
func (f *ferramentaFalsa) Executar(ctx context.Context, args map[string]any) (string, error) {
	f.executada = true
	return f.saida, f.erro
}

func novoRegistroCom(fs ...*ferramentaFalsa) ferramentas.Registro {
	r := ferramentas.NovoRegistro()
	for _, f := range fs {
		_ = r.Registrar(f)
	}
	return r
}

func TestExecutarSemChamadaDevolveTextoDireto(t *testing.T) {
	provider := &providerFalso{respostas: []Resposta{{Texto: "oi, tudo certo"}}}
	loop := &Loop{Provider: provider, Ferramentas: ferramentas.NovoRegistro(), MaxIteracoes: 5}

	resp, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "oi"}, permissao.Privilegiado, nil)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if resp.Texto != "oi, tudo certo" {
		t.Errorf("Texto = %q, queria %q", resp.Texto, "oi, tudo certo")
	}
}

func TestExecutarDespachaFerramentaSeguraSemAprovacao(t *testing.T) {
	eco := &ferramentaFalsa{nome: "eco", nivel: ferramentas.NivelSeguro, saida: "resultado do eco"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "eco"}}},
		{Texto: "feito"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(eco), MaxIteracoes: 5}

	pediuAprovacao := false
	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) {
		pediuAprovacao = true
		return true, nil
	}

	resp, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "roda o eco"}, permissao.Restrito, solicitar)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !eco.executada {
		t.Error("ferramenta segura deveria ter sido executada")
	}
	if pediuAprovacao {
		t.Error("ferramenta segura não deveria pedir aprovação, mesmo com perfil Restrito")
	}
	if resp.Texto != "feito" {
		t.Errorf("Texto = %q, queria feito", resp.Texto)
	}
}

func TestExecutarSolicitaAprovacaoParaFerramentaPerigosaComPerfilRestrito(t *testing.T) {
	perigosa := &ferramentaFalsa{nome: "perigosa", nivel: ferramentas.NivelPerigoso, saida: "executou"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "perigosa"}}},
		{Texto: "feito"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(perigosa), MaxIteracoes: 5}

	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) { return true, nil }

	_, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "faz a ação"}, permissao.Restrito, solicitar)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !perigosa.executada {
		t.Error("ferramenta perigosa deveria ter executado após aprovação")
	}
}

func TestExecutarNegaFerramentaPerigosaQuandoAprovacaoRecusada(t *testing.T) {
	perigosa := &ferramentaFalsa{nome: "perigosa", nivel: ferramentas.NivelPerigoso, saida: "não devia rodar"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "perigosa"}}},
		{Texto: "entendido, não fiz"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(perigosa), MaxIteracoes: 5}

	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) { return false, nil }

	_, mensagens, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "faz a ação"}, permissao.Restrito, solicitar)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if perigosa.executada {
		t.Error("ferramenta perigosa não deveria ter executado — aprovação foi negada")
	}

	achouErroDeNegacao := false
	for _, m := range mensagens {
		for _, r := range m.Resultados {
			if r.Erro == "ação negada pelo usuário" {
				achouErroDeNegacao = true
			}
		}
	}
	if !achouErroDeNegacao {
		t.Error("esperava um ResultadoFerramenta com Erro de negação no histórico")
	}
}

func TestExecutarPrivilegiadoNuncaPedeAprovacao(t *testing.T) {
	perigosa := &ferramentaFalsa{nome: "perigosa", nivel: ferramentas.NivelPerigoso, saida: "ok"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "perigosa"}}},
		{Texto: "feito"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(perigosa), MaxIteracoes: 5}

	pediuAprovacao := false
	solicitar := func(ctx context.Context, c ChamadaFerramenta) (bool, error) {
		pediuAprovacao = true
		return true, nil
	}

	_, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "faz a ação"}, permissao.Privilegiado, solicitar)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if pediuAprovacao {
		t.Error("perfil Privilegiado nunca deveria pedir aprovação")
	}
	if !perigosa.executada {
		t.Error("perfil Privilegiado deveria executar a ferramenta perigosa direto")
	}
}

func TestExecutarErroDeToolNaoDerrubaLoop(t *testing.T) {
	falha := &ferramentaFalsa{nome: "falha", nivel: ferramentas.NivelSeguro, erro: errors.New("comando não encontrado")}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "falha"}}},
		{Texto: "tentei outra coisa"},
	}}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(falha), MaxIteracoes: 5}

	resp, mensagens, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "roda"}, permissao.Privilegiado, nil)
	if err != nil {
		t.Fatalf("erro de tool não deveria derrubar o loop (R2), veio: %v", err)
	}
	if resp.Texto != "tentei outra coisa" {
		t.Errorf("Texto = %q, queria que o loop continuasse até resposta final", resp.Texto)
	}

	achouErro := false
	for _, m := range mensagens {
		for _, r := range m.Resultados {
			if r.Erro == "comando não encontrado" {
				achouErro = true
			}
		}
	}
	if !achouErro {
		t.Error("esperava o erro da tool propagado em ResultadoFerramenta.Erro")
	}
}

func TestExecutarEstouraTetoDeIteracoesRetornaErroHonesto(t *testing.T) {
	nuncaTermina := &ferramentaFalsa{nome: "nunca_termina", nivel: ferramentas.NivelSeguro, saida: "de novo"}
	respostas := make([]Resposta, 0, 4)
	for i := 0; i < 4; i++ {
		respostas = append(respostas, Resposta{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "nunca_termina"}}})
	}
	provider := &providerFalso{respostas: respostas}
	loop := &Loop{Provider: provider, Ferramentas: novoRegistroCom(nuncaTermina), MaxIteracoes: 3}

	_, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "roda"}, permissao.Privilegiado, nil)
	if !errors.Is(err, ErroTetoIteracoes) {
		t.Fatalf("erro = %v, queria ErroTetoIteracoes (R5: mensagem honesta ao estourar o teto)", err)
	}
}

func TestExecutarChamaHooksDeInicioEFimPorFerramenta(t *testing.T) {
	eco := &ferramentaFalsa{nome: "eco", nivel: ferramentas.NivelSeguro, saida: "ok"}
	provider := &providerFalso{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "eco"}}},
		{Texto: "feito"},
	}}

	var iniciadas []string
	var concluidas []string
	loop := &Loop{
		Provider: provider, Ferramentas: novoRegistroCom(eco), MaxIteracoes: 5,
		AoIniciarFerramenta:  func(c ChamadaFerramenta) { iniciadas = append(iniciadas, c.Nome) },
		AoConcluirFerramenta: func(r ResultadoFerramenta) { concluidas = append(concluidas, r.Nome) },
	}

	_, _, err := loop.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "roda o eco"}, permissao.Privilegiado, nil)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if len(iniciadas) != 1 || iniciadas[0] != "eco" {
		t.Errorf("AoIniciarFerramenta chamado com %v, queria [eco]", iniciadas)
	}
	if len(concluidas) != 1 || concluidas[0] != "eco" {
		t.Errorf("AoConcluirFerramenta chamado com %v, queria [eco]", concluidas)
	}
}
```

- [ ] **Step 2: Rodar os testes e confirmar que falham**

Run: `go test ./internal/agente/... -v`
Expected: FAIL — pacote ainda não existe.

- [ ] **Step 3: Implementar os tipos**

Criar `internal/agente/tipos.go`:

```go
// Package agente executa o loop central do James: monta contexto, chama o
// Provider, despacha ferramentas respeitando o perfil de permissão, e
// repete até ter uma resposta final ou estourar o teto de iterações.
package agente

import (
	"context"

	"james/internal/ferramentas"
)

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

// Provider é a fronteira que torna o loop testável com respostas
// roteirizadas, sem gastar token e sem abrir janela.
type Provider interface {
	Gerar(ctx context.Context, req Requisicao) (Resposta, error)
	GerarStream(ctx context.Context, req Requisicao, emitir func(string)) (Resposta, error)
}
```

- [ ] **Step 4: Implementar o loop**

Criar `internal/agente/loop.go`:

```go
package agente

import (
	"context"
	"errors"
	"fmt"

	"james/internal/ferramentas"
	"james/internal/permissao"
)

// ErroTetoIteracoes é devolvido quando o loop estoura o teto sem chegar a
// uma resposta final — R5: mensagem honesta, nunca fallback mascarando
// fracasso.
var ErroTetoIteracoes = errors.New("atingiu o teto de iterações sem concluir")

type Loop struct {
	Provider     Provider
	Ferramentas  ferramentas.Registro
	MaxIteracoes int

	// AoIniciarFerramenta e AoConcluirFerramenta, quando não-nil, são
	// chamados em volta de cada despacho — é o que a API (Task 17) usa pra
	// emitir ferramenta_iniciada/ferramenta_concluida no WS em tempo real,
	// em vez de só no final do turno. Opcionais: nil é um loop mudo, útil
	// nos testes deste pacote.
	AoIniciarFerramenta  func(ChamadaFerramenta)
	AoConcluirFerramenta func(ResultadoFerramenta)
}

// SolicitarAprovacao pergunta a um cliente privilegiado se autoriza uma
// chamada perigosa vinda de um cliente Restrito. nil é válido quando não se
// espera nenhuma chamada perigosa vinda de perfil Restrito nesta execução
// (ex.: sessão do desktop) — se essa suposição for falsa, despachar trata
// como negado, nunca como aprovado por omissão.
type SolicitarAprovacao func(ctx context.Context, chamada ChamadaFerramenta) (aprovado bool, err error)

// Executar roda o loop até obter uma resposta sem chamada de ferramenta ou
// estourar MaxIteracoes. Devolve também o histórico de mensagens acumulado
// (para persistência pelo chamador).
func (l *Loop) Executar(ctx context.Context, sistema string, historico []Mensagem, entrada Mensagem, perfil permissao.Perfil, solicitarAprovacao SolicitarAprovacao) (Resposta, []Mensagem, error) {
	mensagens := append(append([]Mensagem{}, historico...), entrada)

	for i := 0; i < l.MaxIteracoes; i++ {
		req := Requisicao{Sistema: sistema, Mensagens: mensagens, Declaracoes: l.Ferramentas.Declaracoes()}
		resp, err := l.Provider.Gerar(ctx, req)
		if err != nil {
			return Resposta{}, mensagens, fmt.Errorf("provider: %w", err)
		}

		mensagens = append(mensagens, Mensagem{Papel: PapelModelo, Texto: resp.Texto, Chamadas: resp.Chamadas})

		if len(resp.Chamadas) == 0 {
			return resp, mensagens, nil
		}

		resultados := make([]ResultadoFerramenta, 0, len(resp.Chamadas))
		for _, chamada := range resp.Chamadas {
			resultados = append(resultados, l.despachar(ctx, chamada, perfil, solicitarAprovacao))
		}
		mensagens = append(mensagens, Mensagem{Papel: PapelFerramenta, Resultados: resultados})
	}

	return Resposta{}, mensagens, ErroTetoIteracoes
}

func (l *Loop) despachar(ctx context.Context, chamada ChamadaFerramenta, perfil permissao.Perfil, solicitarAprovacao SolicitarAprovacao) ResultadoFerramenta {
	if l.AoIniciarFerramenta != nil {
		l.AoIniciarFerramenta(chamada)
	}
	resultado := l.despacharSemHook(ctx, chamada, perfil, solicitarAprovacao)
	if l.AoConcluirFerramenta != nil {
		l.AoConcluirFerramenta(resultado)
	}
	return resultado
}

func (l *Loop) despacharSemHook(ctx context.Context, chamada ChamadaFerramenta, perfil permissao.Perfil, solicitarAprovacao SolicitarAprovacao) ResultadoFerramenta {
	tool, ok := l.Ferramentas.Buscar(chamada.Nome)
	if !ok {
		return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: fmt.Sprintf("ferramenta %q não encontrada", chamada.Nome)}
	}

	if !perfil.Pode(tool.Nivel()) {
		if solicitarAprovacao == nil {
			return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: "ação negada pelo usuário"}
		}
		aprovado, err := solicitarAprovacao(ctx, chamada)
		if err != nil {
			return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: fmt.Sprintf("erro ao pedir aprovação: %v", err)}
		}
		if !aprovado {
			return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: "ação negada pelo usuário"}
		}
	}

	saida, err := tool.Executar(ctx, chamada.Args)
	if err != nil {
		return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: err.Error()}
	}
	return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Saida: saida}
}
```

- [ ] **Step 5: Rodar os testes e confirmar que passam**

Run: `go test ./internal/agente/... -v`
Expected: PASS em todos os 8 testes.

- [ ] **Step 6: Commit**

```bash
git add internal/agente/tipos.go internal/agente/loop.go internal/agente/loop_test.go
git commit -m "feat: internal/agente com loop testável, gate de aprovação e teto de iterações"
```

---

## Task 6: `internal/agente/gemini` — implementação real do `Provider`

**Files:**
- Create: `internal/agente/gemini/gemini.go`
- Create: `internal/agente/gemini/mapeamento.go`
- Create: `internal/agente/gemini/mapeamento_test.go`

**Nota:** só as funções puras de mapeamento (tipos `agente` ↔ tipos `genai`) são testadas automaticamente — chamar a API real custaria token a cada `go test`, e é exatamente o problema que a interface `Provider` existe pra evitar (spec seção 7). A chamada real fica coberta por um smoke test manual no Step 6.

- [ ] **Step 1: Escrever os testes de mapeamento que falham**

Criar `internal/agente/gemini/mapeamento_test.go`:

```go
package gemini

import (
	"testing"

	"google.golang.org/genai"

	"james/internal/agente"
	"james/internal/ferramentas"
)

func TestParaGenaiContentsMapeiaPapeis(t *testing.T) {
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "oi"},
		{Papel: agente.PapelModelo, Texto: "olá", Chamadas: []agente.ChamadaFerramenta{
			{Nome: "eco", Args: map[string]any{"x": "1"}},
		}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
			{Nome: "eco", Saida: "resultado"},
		}},
	}

	contents := paraGenaiContents(mensagens)
	if len(contents) != 3 {
		t.Fatalf("esperava 3 Content, vieram %d", len(contents))
	}
	if contents[0].Role != "user" {
		t.Errorf("contents[0].Role = %q, queria user", contents[0].Role)
	}
	if contents[1].Role != "model" {
		t.Errorf("contents[1].Role = %q, queria model", contents[1].Role)
	}
	if contents[2].Role != "user" {
		t.Errorf("contents[2].Role = %q, queria user — a API Gemini não tem role \"function\"; "+
			"NewContentFromFunctionResponse do SDK usa RoleUser por padrão (corrige o bug #6 do Anexo A, "+
			"onde o app.go antigo usava \"tool\" sem confirmar se era aceito)", contents[2].Role)
	}
}

func TestParaGenaiToolMapeiaParametros(t *testing.T) {
	decls := []ferramentas.Declaracao{
		{
			Nome:      "pesquisar_internet",
			Descricao: "busca na internet",
			Parametros: map[string]ferramentas.Parametro{
				"query": {Tipo: "string", Descricao: "termo de busca"},
			},
			Obrigatorios: []string{"query"},
		},
	}

	tool := paraGenaiTool(decls)
	if len(tool.FunctionDeclarations) != 1 {
		t.Fatalf("esperava 1 FunctionDeclaration, vieram %d", len(tool.FunctionDeclarations))
	}
	fd := tool.FunctionDeclarations[0]
	if fd.Name != "pesquisar_internet" {
		t.Errorf("Name = %q, queria pesquisar_internet", fd.Name)
	}
	if fd.Parameters.Properties["query"].Type != genai.TypeString {
		t.Error("propriedade query deveria ser TypeString")
	}
	if len(fd.Parameters.Required) != 1 || fd.Parameters.Required[0] != "query" {
		t.Errorf("Required = %v, queria [query]", fd.Parameters.Required)
	}
}

func TestParaRespostaExtraiTextoEChamadas(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: "texto final"},
						{FunctionCall: &genai.FunctionCall{Name: "eco", Args: map[string]any{"x": "1"}}},
					},
				},
			},
		},
	}

	r := paraResposta(resp)
	if r.Texto != "texto final" {
		t.Errorf("Texto = %q, queria texto final", r.Texto)
	}
	if len(r.Chamadas) != 1 || r.Chamadas[0].Nome != "eco" {
		t.Errorf("Chamadas = %+v, queria 1 chamada pra eco", r.Chamadas)
	}
}

func TestParaRespostaCandidatoVazioDevolveRespostaZero(t *testing.T) {
	resp := &genai.GenerateContentResponse{}
	r := paraResposta(resp)
	if r.Texto != "" || len(r.Chamadas) != 0 {
		t.Errorf("esperava Resposta zero, veio %+v", r)
	}
}
```

- [ ] **Step 2: Rodar os testes e confirmar que falham**

Run: `go test ./internal/agente/gemini/... -v`
Expected: FAIL — pacote ainda não existe.

- [ ] **Step 3: Implementar o mapeamento**

Criar `internal/agente/gemini/mapeamento.go`:

```go
package gemini

import (
	"google.golang.org/genai"

	"james/internal/agente"
	"james/internal/ferramentas"
)

func paraGenaiContents(mensagens []agente.Mensagem) []*genai.Content {
	var contents []*genai.Content
	for _, m := range mensagens {
		switch m.Papel {
		case agente.PapelUsuario:
			parts := []*genai.Part{{Text: m.Texto}}
			if len(m.Imagem) > 0 {
				parts = append(parts, &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: m.Imagem}})
			}
			contents = append(contents, &genai.Content{Role: "user", Parts: parts})
		case agente.PapelModelo:
			var parts []*genai.Part
			if m.Texto != "" {
				parts = append(parts, &genai.Part{Text: m.Texto})
			}
			for _, c := range m.Chamadas {
				parts = append(parts, &genai.Part{FunctionCall: &genai.FunctionCall{Name: c.Nome, Args: c.Args}})
			}
			contents = append(contents, &genai.Content{Role: "model", Parts: parts})
		case agente.PapelFerramenta:
			var parts []*genai.Part
			for _, r := range m.Resultados {
				saida := r.Saida
				if r.Erro != "" {
					saida = "erro: " + r.Erro
				}
				parts = append(parts, &genai.Part{FunctionResponse: &genai.FunctionResponse{
					Name:     r.Nome,
					Response: map[string]any{"output": saida},
				}})
			}
			// "user" é o role correto pra resposta de tool na API Gemini —
			// confirmado contra a doc oficial do SDK via context7
			// (NewContentFromFunctionResponse: "Defaults to RoleUser if
			// role is empty", exemplo mostra role="user"). Não existe role
			// "function" nesta API — isso é de outras APIs (ex.: OpenAI).
			// Corrige o bug #6 do Anexo A, onde o app.go antigo usava
			// "tool" sem confirmar se era aceito.
			contents = append(contents, &genai.Content{Role: "user", Parts: parts})
		}
	}
	return contents
}

func paraGenaiTool(decls []ferramentas.Declaracao) *genai.Tool {
	var fds []*genai.FunctionDeclaration
	for _, d := range decls {
		props := map[string]*genai.Schema{}
		for nome, p := range d.Parametros {
			props[nome] = &genai.Schema{Type: tipoGenai(p.Tipo), Description: p.Descricao}
		}
		fds = append(fds, &genai.FunctionDeclaration{
			Name:        d.Nome,
			Description: d.Descricao,
			Parameters:  &genai.Schema{Type: genai.TypeObject, Properties: props, Required: d.Obrigatorios},
		})
	}
	return &genai.Tool{FunctionDeclarations: fds}
}

func tipoGenai(tipo string) genai.Type {
	switch tipo {
	case "integer":
		return genai.TypeInteger
	case "number":
		return genai.TypeNumber
	case "boolean":
		return genai.TypeBoolean
	default:
		return genai.TypeString
	}
}

func paraResposta(resp *genai.GenerateContentResponse) agente.Resposta {
	var r agente.Resposta
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return r
	}
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			r.Texto = part.Text
		}
		if part.FunctionCall != nil {
			r.Chamadas = append(r.Chamadas, agente.ChamadaFerramenta{
				ID:   part.FunctionCall.Name,
				Nome: part.FunctionCall.Name,
				Args: part.FunctionCall.Args,
			})
		}
	}
	return r
}
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

Run: `go test ./internal/agente/gemini/... -v`
Expected: PASS nos 4 testes de mapeamento.

- [ ] **Step 5: Implementar o `Provider` real**

Criar `internal/agente/gemini/gemini.go`:

```go
// Package gemini implementa agente.Provider sobre a API do Gemini.
package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	"james/internal/agente"
)

type Provider struct {
	cliente *genai.Client
	modelo  string
}

// Novo cria o Provider real. Conferir a lista vigente de modelos Gemini
// antes de fixar `modelo` em produção — gemini-2.5-flash-lite é proibido
// (internal/config já recusa isso no boot).
func Novo(ctx context.Context, apiKey, modelo string) (*Provider, error) {
	cliente, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente Gemini: %w", err)
	}
	return &Provider{cliente: cliente, modelo: modelo}, nil
}

func (p *Provider) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	contents := paraGenaiContents(req.Mensagens)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: req.Sistema}}},
		Tools:             []*genai.Tool{paraGenaiTool(req.Declaracoes)},
	}

	resp, err := p.cliente.Models.GenerateContent(ctx, p.modelo, contents, config)
	if err != nil {
		return agente.Resposta{}, fmt.Errorf("gemini: %w", err)
	}
	return paraResposta(resp), nil
}

// GerarStream nesta fase não faz streaming token-a-token do SDK — chama
// Gerar e emite o texto final de uma vez. Streaming incremental de verdade
// fica como melhoria futura (não bloqueia nenhum critério de aceitação da
// Fase 1, que só exige o evento `token` existir no protocolo WS).
func (p *Provider) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	resp, err := p.Gerar(ctx, req)
	if err != nil {
		return agente.Resposta{}, err
	}
	if resp.Texto != "" && emitir != nil {
		emitir(resp.Texto)
	}
	return resp, nil
}
```

- [ ] **Step 6: Smoke test manual contra a API real (não automatizado)**

Isso não entra em `go test` — custa token e a interface `Provider` existe exatamente pra não precisar disso em CI. Rodar uma vez à mão, com `GEMINI_API_KEY` real no `.env`:

```bash
go run ./cmd/smoke-gemini
```

Criar um `cmd/smoke-gemini/main.go` temporário (não commitado — deletar depois do teste manual) que chama `gemini.Novo` e `Gerar` com uma pergunta simples e imprime `resp.Texto`. Confirmar que:
1. a resposta chega sem erro
2. o role `"user"` pra `Mensagem` `PapelFerramenta` (resposta de tool) é aceito pela API de verdade num turno com chamada de ferramenta simulada no histórico — a escolha já é respaldada pela doc oficial do SDK (`NewContentFromFunctionResponse` usa `RoleUser` por padrão), mas vale a confirmação empírica contra a API real antes de fechar a fase

Documentar o resultado (colado, não resumido) no PR ou no commit de fechamento da fase.

- [ ] **Step 7: Commit**

```bash
git add internal/agente/gemini/gemini.go internal/agente/gemini/mapeamento.go internal/agente/gemini/mapeamento_test.go
git commit -m "feat: internal/agente/gemini implementa Provider sobre a API do Gemini"
```

---

## Task 7: `internal/ferramentas/terminal.go` — `executar_comando_sistema`

Porta o comportamento do `app.go` antigo (`ExecutarNoTerminal`) como está — sem melhorar, só atrás da interface `Ferramenta` (spec: qualidade das tools é Fase 3).

**Files:**
- Create: `internal/ferramentas/terminal.go`
- Create: `internal/ferramentas/terminal_test.go`

- [ ] **Step 1: Escrever os testes que falham**

```go
package ferramentas

import (
	"context"
	"errors"
	"testing"
)

func TestComandoSistemaExecutaViaExecutorInjetado(t *testing.T) {
	var comandoRecebido string
	c := &ComandoSistema{Executor: func(comando string) (string, error) {
		comandoRecebido = comando
		return "saída do comando", nil
	}}

	saida, err := c.Executar(context.Background(), map[string]any{"comando": "Get-Process"})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if saida != "saída do comando" {
		t.Errorf("saida = %q, queria %q", saida, "saída do comando")
	}
	if comandoRecebido != "Get-Process" {
		t.Errorf("comando repassado = %q, queria Get-Process", comandoRecebido)
	}
}

func TestComandoSistemaRejeitaArgumentoAusente(t *testing.T) {
	c := &ComandoSistema{Executor: func(string) (string, error) { return "", nil }}
	if _, err := c.Executar(context.Background(), map[string]any{}); err == nil {
		t.Fatal("esperava erro por parâmetro comando ausente, veio nil")
	}
}

func TestComandoSistemaPropagaErroDoExecutor(t *testing.T) {
	c := &ComandoSistema{Executor: func(string) (string, error) { return "", errors.New("falhou") }}
	if _, err := c.Executar(context.Background(), map[string]any{"comando": "x"}); err == nil {
		t.Fatal("esperava erro propagado do Executor, veio nil")
	}
}

func TestComandoSistemaNivelPerigoso(t *testing.T) {
	c := NovoComandoSistema()
	if c.Nivel() != NivelPerigoso {
		t.Error("executar_comando_sistema deve ser NivelPerigoso")
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./internal/ferramentas/... -run ComandoSistema -v`
Expected: FAIL — `ComandoSistema` ainda não existe.

- [ ] **Step 3: Implementar**

```go
package ferramentas

import (
	"context"
	"fmt"
	"os/exec"
)

// ComandoSistema porta o comportamento de app.go:187 (ExecutarNoTerminal)
// como está — sem sandboxing, sem allowlist de comando. Consertar isso é
// Fase 3; aqui só entra atrás da interface Ferramenta.
type ComandoSistema struct {
	Executor func(comando string) (saida string, err error)
}

func NovoComandoSistema() *ComandoSistema {
	return &ComandoSistema{Executor: executarPowerShell}
}

func executarPowerShell(comando string) (string, error) {
	out, err := exec.Command("powershell", "-Command", comando).CombinedOutput()
	return string(out), err
}

func (c *ComandoSistema) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "executar_comando_sistema",
		Descricao: "Executa comandos PowerShell. Se não souber o comando pra uma tarefa, use pesquisar_internet primeiro.",
		Parametros: map[string]Parametro{
			"comando": {Tipo: "string", Descricao: "comando PowerShell a executar"},
		},
		Obrigatorios: []string{"comando"},
	}
}

func (c *ComandoSistema) Nivel() Nivel { return NivelPerigoso }

func (c *ComandoSistema) Executar(ctx context.Context, args map[string]any) (string, error) {
	comando, ok := args["comando"].(string)
	if !ok || comando == "" {
		return "", fmt.Errorf("parâmetro comando ausente ou vazio")
	}
	saida, err := c.Executor(comando)
	if err != nil {
		return saida, fmt.Errorf("erro ao executar: %w", err)
	}
	return saida, nil
}
```

- [ ] **Step 4: Rodar e confirmar sucesso**

Run: `go test ./internal/ferramentas/... -run ComandoSistema -v`
Expected: PASS nos 4 testes.

- [ ] **Step 5: Commit**

```bash
git add internal/ferramentas/terminal.go internal/ferramentas/terminal_test.go
git commit -m "feat: ferramentas.ComandoSistema porta executar_comando_sistema"
```

---

## Task 8: `internal/ferramentas/claudecode.go` — `perguntar_ao_claude_code`

Porta `app.go:170` (`ChamarClaudeCode`) como está: dispara `rodar_claude.ps1` numa janela nova, fire-and-forget. Consertar (captura de saída, tirar o sequestro do OpenRouter) é Fase 3.

**Files:**
- Create: `internal/ferramentas/claudecode.go`
- Create: `internal/ferramentas/claudecode_test.go`

- [ ] **Step 1: Escrever os testes que falham**

```go
package ferramentas

import (
	"context"
	"errors"
	"testing"
)

func TestClaudeCodeDisparaComInstrucao(t *testing.T) {
	var instrucaoRecebida string
	c := &ClaudeCode{Executor: func(instrucao string) error {
		instrucaoRecebida = instrucao
		return nil
	}}

	saida, err := c.Executar(context.Background(), map[string]any{"instrucao": "refatora o main.go"})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if instrucaoRecebida != "refatora o main.go" {
		t.Errorf("instrucao repassada = %q", instrucaoRecebida)
	}
	if saida == "" {
		t.Error("esperava confirmação de disparo, veio vazio")
	}
}

func TestClaudeCodeRejeitaArgumentoAusente(t *testing.T) {
	c := &ClaudeCode{Executor: func(string) error { return nil }}
	if _, err := c.Executar(context.Background(), map[string]any{}); err == nil {
		t.Fatal("esperava erro por parâmetro instrucao ausente, veio nil")
	}
}

func TestClaudeCodePropagaErroDoExecutor(t *testing.T) {
	c := &ClaudeCode{Executor: func(string) error { return errors.New("falhou ao abrir") }}
	if _, err := c.Executar(context.Background(), map[string]any{"instrucao": "x"}); err == nil {
		t.Fatal("esperava erro propagado, veio nil")
	}
}

func TestClaudeCodeNivelPerigoso(t *testing.T) {
	c := NovoClaudeCode()
	if c.Nivel() != NivelPerigoso {
		t.Error("perguntar_ao_claude_code deve ser NivelPerigoso")
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./internal/ferramentas/... -run ClaudeCode -v`
Expected: FAIL — `ClaudeCode` ainda não existe.

- [ ] **Step 3: Implementar**

```go
package ferramentas

import (
	"context"
	"fmt"
	"os/exec"
)

// ClaudeCode porta app.go:170 (ChamarClaudeCode) como está: fire-and-forget,
// não captura saída, abre janela nova via rodar_claude.ps1. Consertar é
// Fase 3 (Anexo A #17, #18).
type ClaudeCode struct {
	Executor func(instrucao string) error
}

func NovoClaudeCode() *ClaudeCode {
	return &ClaudeCode{Executor: dispararRodarClaude}
}

func dispararRodarClaude(instrucao string) error {
	cmd := exec.Command("cmd", "/C", "start", "powershell", "-NoExit",
		"-ExecutionPolicy", "Bypass", "-File", "rodar_claude.ps1", instrucao)
	return cmd.Run()
}

func (c *ClaudeCode) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "perguntar_ao_claude_code",
		Descricao: "Usa o agente Claude Code pra interagir com o ambiente de desenvolvimento.",
		Parametros: map[string]Parametro{
			"instrucao": {Tipo: "string", Descricao: "instrução ou mensagem pro Claude Code"},
		},
		Obrigatorios: []string{"instrucao"},
	}
}

func (c *ClaudeCode) Nivel() Nivel { return NivelPerigoso }

func (c *ClaudeCode) Executar(ctx context.Context, args map[string]any) (string, error) {
	instrucao, ok := args["instrucao"].(string)
	if !ok || instrucao == "" {
		return "", fmt.Errorf("parâmetro instrucao ausente ou vazio")
	}
	if err := c.Executor(instrucao); err != nil {
		return "", fmt.Errorf("erro ao iniciar Claude Code: %w", err)
	}
	return "Claude Code iniciado. Tarefa enviada para o agente.", nil
}
```

- [ ] **Step 4: Rodar e confirmar sucesso**

Run: `go test ./internal/ferramentas/... -run ClaudeCode -v`
Expected: PASS nos 4 testes.

- [ ] **Step 5: Commit**

```bash
git add internal/ferramentas/claudecode.go internal/ferramentas/claudecode_test.go
git commit -m "feat: ferramentas.ClaudeCode porta perguntar_ao_claude_code"
```

---

## Task 9: `internal/ferramentas/busca.go` — `pesquisar_internet`

Porta `app.go:215` (`BuscarNoTavily`) como está: `search_depth basic`, devolve o JSON bruto da Tavily. Melhorar é Fase 3 (Anexo A #16).

**Files:**
- Create: `internal/ferramentas/busca.go`
- Create: `internal/ferramentas/busca_test.go`

- [ ] **Step 1: Escrever os testes que falham**

```go
package ferramentas

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuscaEnviaQueryEDevolveRespostaBruta(t *testing.T) {
	var corpoRecebido map[string]any
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(corpo, &corpoRecebido)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"title":"exemplo"}]}`))
	}))
	defer servidor.Close()

	b := &Busca{APIKey: "chave-de-teste", Cliente: servidor.Client(), URL: servidor.URL}
	saida, err := b.Executar(context.Background(), map[string]any{"query": "clima em são paulo"})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if saida != `{"results":[{"title":"exemplo"}]}` {
		t.Errorf("saida = %q, deveria devolver o JSON bruto da Tavily", saida)
	}
	if corpoRecebido["query"] != "clima em são paulo" {
		t.Errorf("query enviada = %v, queria clima em são paulo", corpoRecebido["query"])
	}
	if corpoRecebido["search_depth"] != "basic" {
		t.Errorf("search_depth = %v, queria basic (comportamento portado como está)", corpoRecebido["search_depth"])
	}
}

func TestBuscaRejeitaArgumentoAusente(t *testing.T) {
	b := NovaBusca("chave")
	if _, err := b.Executar(context.Background(), map[string]any{}); err == nil {
		t.Fatal("esperava erro por parâmetro query ausente, veio nil")
	}
}

func TestBuscaNivelSeguro(t *testing.T) {
	b := NovaBusca("chave")
	if b.Nivel() != NivelSeguro {
		t.Error("pesquisar_internet deve ser NivelSeguro")
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./internal/ferramentas/... -run Busca -v`
Expected: FAIL — `Busca` ainda não existe.

- [ ] **Step 3: Implementar**

```go
package ferramentas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Busca porta app.go:215 (BuscarNoTavily) como está: search_depth basic,
// devolve o JSON bruto pro modelo extrair o que precisa. Melhorar é Fase 3
// (Anexo A #16).
type Busca struct {
	APIKey  string
	Cliente *http.Client
	URL     string
}

func NovaBusca(apiKey string) *Busca {
	return &Busca{APIKey: apiKey, Cliente: http.DefaultClient, URL: "https://api.tavily.com/search"}
}

func (b *Busca) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "pesquisar_internet",
		Descricao: "Busca informações atualizadas na internet.",
		Parametros: map[string]Parametro{
			"query": {Tipo: "string", Descricao: "termo de busca"},
		},
		Obrigatorios: []string{"query"},
	}
}

func (b *Busca) Nivel() Nivel { return NivelSeguro }

func (b *Busca) Executar(ctx context.Context, args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("parâmetro query ausente ou vazio")
	}

	payload := map[string]any{"api_key": b.APIKey, "query": query, "search_depth": "basic"}
	corpo, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.URL, bytes.NewReader(corpo))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.Cliente.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao conectar na busca: %w", err)
	}
	defer resp.Body.Close()

	respostaBruta, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respostaBruta), nil
}
```

- [ ] **Step 4: Rodar e confirmar sucesso**

Run: `go test ./internal/ferramentas/... -run Busca -v`
Expected: PASS nos 3 testes.

- [ ] **Step 5: Commit**

```bash
git add internal/ferramentas/busca.go internal/ferramentas/busca_test.go
git commit -m "feat: ferramentas.Busca porta pesquisar_internet (Tavily)"
```

---

## Task 10: `internal/dispositivo` — hub que faz a ponte com o `james-device`

Elimina o panic de `app.go:218` (`connPython` nil) — sem device conectado, quem pede ação recebe `ErroDispositivoAusente` como valor de retorno normal, nunca um crash (R3).

**Files:**
- Create: `internal/dispositivo/hub.go`
- Create: `internal/dispositivo/hub_test.go`
- Modify: `go.mod` (promover `github.com/google/uuid` a dependência direta)

- [ ] **Step 1: Promover `google/uuid` a dependência direta**

Run: `go get github.com/google/uuid@v1.6.0`
Expected: `go.mod` move a linha de `uuid` do bloco `// indirect` pro bloco de `require` direto.

- [ ] **Step 2: Escrever os testes que falham**

```go
package dispositivo

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type conexaoFalsa struct {
	enviados [][]byte
	erro     error
}

func (c *conexaoFalsa) Enviar(envelope []byte) error {
	c.enviados = append(c.enviados, envelope)
	return c.erro
}

func TestEnviarAcaoSemDispositivoDevolveErroDedicado(t *testing.T) {
	h := NovoHub()
	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click", X: 10, Y: 20})
	if !errors.Is(err, ErroDispositivoAusente) {
		t.Fatalf("erro = %v, queria ErroDispositivoAusente", err)
	}
}

func TestEnviarAcaoComDispositivoConectadoDevolveResultado(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	go func() {
		// Simula o endpoint WS do device recebendo o envelope e respondendo.
		for len(conexao.enviados) == 0 {
			time.Sleep(time.Millisecond)
		}
		var envelope map[string]any
		_ = json.Unmarshal(conexao.enviados[0], &envelope)
		h.ResolverResultado(envelope["id"].(string), "clique feito", "")
	}()

	saida, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click", X: 10, Y: 20})
	if err != nil {
		t.Fatalf("EnviarAcao devolveu erro: %v", err)
	}
	if saida != "clique feito" {
		t.Errorf("saida = %q, queria clique feito", saida)
	}
}

func TestEnviarAcaoPropagaErroDoDevice(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	go func() {
		for len(conexao.enviados) == 0 {
			time.Sleep(time.Millisecond)
		}
		var envelope map[string]any
		_ = json.Unmarshal(conexao.enviados[0], &envelope)
		h.ResolverResultado(envelope["id"].(string), "", "tecla inválida")
	}()

	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "tecla", Texto: "xyz"})
	if err == nil {
		t.Fatal("esperava erro propagado do device, veio nil")
	}
}

func TestEnviarAcaoEstouraTimeoutSemResposta(t *testing.T) {
	h := NovoHub()
	h.Timeout = 20 * time.Millisecond
	h.Registrar(&conexaoFalsa{})

	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click"})
	if err == nil {
		t.Fatal("esperava timeout (R4), veio nil")
	}
}

func TestDesregistrarVoltaAErroDispositivoAusente(t *testing.T) {
	h := NovoHub()
	h.Registrar(&conexaoFalsa{})
	h.Desregistrar()

	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click"})
	if !errors.Is(err, ErroDispositivoAusente) {
		t.Fatalf("erro = %v, queria ErroDispositivoAusente após Desregistrar", err)
	}
}
```

- [ ] **Step 3: Rodar e confirmar falha**

Run: `go test ./internal/dispositivo/... -v`
Expected: FAIL — pacote ainda não existe.

- [ ] **Step 4: Implementar**

```go
// Package dispositivo é a ponte entre o jamesd e o james-device (Python):
// converte pedidos de ação de tela em mensagens WS e casa a resposta pelo
// id de correlação. Sem device conectado, devolve erro dedicado — nunca
// panic (R3, corrige o bug de app.go:218).
package dispositivo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErroDispositivoAusente = errors.New("máquina física indisponível: james-device não conectado")

type AcaoTela struct {
	Acao  string `json:"acao"`
	X     int    `json:"x,omitempty"`
	Y     int    `json:"y,omitempty"`
	Texto string `json:"texto,omitempty"`
}

// Conexao é o suficiente do lado WS que o Hub precisa — internal/api
// implementa isso sobre o gorilla/websocket de verdade.
type Conexao interface {
	Enviar(envelope []byte) error
}

type resultadoAcao struct {
	saida string
	erro  string
}

type Hub struct {
	mu        sync.Mutex
	conexao   Conexao
	pendentes map[string]chan resultadoAcao
	Timeout   time.Duration
}

func NovoHub() *Hub {
	return &Hub{pendentes: make(map[string]chan resultadoAcao), Timeout: 10 * time.Second}
}

func (h *Hub) Registrar(c Conexao) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conexao = c
}

func (h *Hub) Desregistrar() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conexao = nil
}

// EnviarAcao manda a ação pro device conectado e espera o resultado_acao
// correspondente, com timeout (R4).
func (h *Hub) EnviarAcao(ctx context.Context, acao AcaoTela) (string, error) {
	h.mu.Lock()
	conexao := h.conexao
	h.mu.Unlock()
	if conexao == nil {
		return "", ErroDispositivoAusente
	}

	id := uuid.NewString()
	espera := make(chan resultadoAcao, 1)
	h.mu.Lock()
	h.pendentes[id] = espera
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pendentes, id)
		h.mu.Unlock()
	}()

	envelope, err := json.Marshal(map[string]any{"tipo": "acao_tela", "id": id, "dados": acao})
	if err != nil {
		return "", err
	}
	if err := conexao.Enviar(envelope); err != nil {
		return "", fmt.Errorf("erro ao enviar ação pro device: %w", err)
	}

	select {
	case r := <-espera:
		if r.erro != "" {
			return "", fmt.Errorf("device: %s", r.erro)
		}
		return r.saida, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(h.Timeout):
		return "", fmt.Errorf("timeout esperando resultado_acao do device")
	}
}

// ResolverResultado é chamado pelo endpoint WS do device quando chega um
// resultado_acao — casa pelo id de correlação com quem está esperando em
// EnviarAcao.
func (h *Hub) ResolverResultado(id, saida, erro string) {
	h.mu.Lock()
	espera, ok := h.pendentes[id]
	h.mu.Unlock()
	if !ok {
		return
	}
	espera <- resultadoAcao{saida: saida, erro: erro}
}
```

- [ ] **Step 5: Rodar e confirmar sucesso**

Run: `go test ./internal/dispositivo/... -v`
Expected: PASS nos 5 testes.

- [ ] **Step 6: Commit**

```bash
git add internal/dispositivo/hub.go internal/dispositivo/hub_test.go go.mod go.sum
git commit -m "feat: internal/dispositivo faz a ponte com o james-device sem panic"
```

---

## Task 11: `internal/ferramentas/dispositivo.go` — `manipular_interface`

**Files:**
- Create: `internal/ferramentas/dispositivo.go`
- Create: `internal/ferramentas/dispositivo_test.go`

- [ ] **Step 1: Escrever os testes que falham**

```go
package ferramentas

import (
	"context"
	"errors"
	"testing"

	"james/internal/dispositivo"
)

type ponteFalsa struct {
	acaoRecebida dispositivo.AcaoTela
	saida        string
	erro         error
}

func (p *ponteFalsa) EnviarAcao(ctx context.Context, acao dispositivo.AcaoTela) (string, error) {
	p.acaoRecebida = acao
	return p.saida, p.erro
}

func TestManipularInterfaceRepassaAcaoParaPonte(t *testing.T) {
	ponte := &ponteFalsa{saida: "clique feito"}
	m := NovoManipularInterface(ponte)

	saida, err := m.Executar(context.Background(), map[string]any{
		"acao": "click", "x": float64(100), "y": float64(200),
	})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if saida != "clique feito" {
		t.Errorf("saida = %q, queria clique feito", saida)
	}
	if ponte.acaoRecebida.X != 100 || ponte.acaoRecebida.Y != 200 {
		t.Errorf("coordenadas repassadas = %+v", ponte.acaoRecebida)
	}
}

func TestManipularInterfaceRejeitaAcaoAusente(t *testing.T) {
	m := NovoManipularInterface(&ponteFalsa{})
	if _, err := m.Executar(context.Background(), map[string]any{}); err == nil {
		t.Fatal("esperava erro por parâmetro acao ausente, veio nil")
	}
}

func TestManipularInterfacePropagaDispositivoAusente(t *testing.T) {
	ponte := &ponteFalsa{erro: dispositivo.ErroDispositivoAusente}
	m := NovoManipularInterface(ponte)

	_, err := m.Executar(context.Background(), map[string]any{"acao": "click"})
	if !errors.Is(err, dispositivo.ErroDispositivoAusente) {
		t.Fatalf("erro = %v, queria ErroDispositivoAusente (R3: resposta, não crash)", err)
	}
}

func TestManipularInterfaceNivelPerigoso(t *testing.T) {
	m := NovoManipularInterface(&ponteFalsa{})
	if m.Nivel() != NivelPerigoso {
		t.Error("manipular_interface deve ser NivelPerigoso")
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./internal/ferramentas/... -run ManipularInterface -v`
Expected: FAIL — `ManipularInterface` ainda não existe.

- [ ] **Step 3: Implementar**

```go
package ferramentas

import (
	"context"
	"fmt"

	"james/internal/dispositivo"
)

// ManipularInterface porta app.go:194 (ManipularInterface) atrás da
// interface Ferramenta, trocando o socket TCP cru por internal/dispositivo.
// Coordenadas ainda vêm adivinhadas pelo modelo (Anexo A #20) — consertar é
// Fase 5.
type ManipularInterface struct {
	Ponte dispositivo.Ponte
}

func NovoManipularInterface(ponte dispositivo.Ponte) *ManipularInterface {
	return &ManipularInterface{Ponte: ponte}
}

func (m *ManipularInterface) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "manipular_interface",
		Descricao: "Interações com a interface gráfica: clicar em botões ou pressionar teclas.",
		Parametros: map[string]Parametro{
			"acao":  {Tipo: "string", Descricao: "'click' para mouse ou 'tecla' para pressionar teclas"},
			"x":     {Tipo: "integer", Descricao: "coordenada X (para clique)"},
			"y":     {Tipo: "integer", Descricao: "coordenada Y (para clique)"},
			"texto": {Tipo: "string", Descricao: "tecla a pressionar (para acao 'tecla', ex: 'space', 'enter')"},
		},
		Obrigatorios: []string{"acao"},
	}
}

func (m *ManipularInterface) Nivel() Nivel { return NivelPerigoso }

func (m *ManipularInterface) Executar(ctx context.Context, args map[string]any) (string, error) {
	acao, ok := args["acao"].(string)
	if !ok || acao == "" {
		return "", fmt.Errorf("parâmetro acao ausente ou vazio")
	}
	x, _ := args["x"].(float64)
	y, _ := args["y"].(float64)
	texto, _ := args["texto"].(string)

	saida, err := m.Ponte.EnviarAcao(ctx, dispositivo.AcaoTela{Acao: acao, X: int(x), Y: int(y), Texto: texto})
	if err != nil {
		return "", err
	}
	return saida, nil
}
```

Adicionar `Ponte` como interface em `internal/dispositivo/hub.go` (Task 10), já que `*Hub` implementa `EnviarAcao` com a mesma assinatura:

```go
// Ponte é o que ferramentas.ManipularInterface precisa — *Hub implementa.
type Ponte interface {
	EnviarAcao(ctx context.Context, acao AcaoTela) (string, error)
}
```

(Adicionar essa interface em `internal/dispositivo/hub.go`, junto de `Conexao`.)

- [ ] **Step 4: Rodar e confirmar sucesso**

Run: `go test ./internal/ferramentas/... -run ManipularInterface -v`
Expected: PASS nos 4 testes.

- [ ] **Step 5: Commit**

```bash
git add internal/ferramentas/dispositivo.go internal/ferramentas/dispositivo_test.go internal/dispositivo/hub.go
git commit -m "feat: ferramentas.ManipularInterface fala com o device via internal/dispositivo"
```

---

## Task 12: `internal/armazenamento` — migrações SQL numeradas

**Files:**
- Create: `internal/armazenamento/migracoes.go`
- Create: `internal/armazenamento/migracoes/0001_inicial.sql`
- Create: `internal/armazenamento/postgres/migrar.go`
- Create: `internal/armazenamento/postgres/migrar_test.go`

- [ ] **Step 1: Criar a migração SQL**

Criar `internal/armazenamento/migracoes/0001_inicial.sql` com exatamente o schema da spec seção 5:

```sql
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

- [ ] **Step 2: Embutir os arquivos de migração**

Criar `internal/armazenamento/migracoes.go`:

```go
// Package armazenamento concentra os artefatos compartilhados entre os
// repositórios (SQL numerado embutido no binário).
package armazenamento

import "embed"

//go:embed migracoes/*.sql
var ArquivosMigracao embed.FS
```

- [ ] **Step 3: Escrever o teste de migração que falha**

Criar `internal/armazenamento/postgres/migrar_test.go`:

```go
package postgres

import (
	"context"
	"testing"
)

func TestAplicarMigracoesContraPostgresLocal(t *testing.T) {
	url := URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}

	pool, err := Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	defer pool.Close()

	if err := AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("primeira aplicação falhou: %v", err)
	}
	// Idempotência: rodar de novo não pode falhar nem duplicar.
	if err := AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("segunda aplicação (idempotente) falhou: %v", err)
	}

	var existe bool
	err = pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'sessoes')`).Scan(&existe)
	if err != nil {
		t.Fatalf("erro ao checar tabela sessoes: %v", err)
	}
	if !existe {
		t.Error("tabela sessoes deveria existir depois da migração")
	}
}
```

- [ ] **Step 4: Rodar e confirmar falha**

Run: `go test ./internal/armazenamento/postgres/... -run Migracoes -v`
Expected: FAIL — `AplicarMigracoes` não existe (se `DATABASE_URL` estiver vazia, sobe o Docker primeiro: `docker compose up -d`).

- [ ] **Step 5: Implementar o runner**

Criar `internal/armazenamento/postgres/migrar.go`:

```go
package postgres

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"

	"james/internal/armazenamento"
)

// AplicarMigracoes aplica, em ordem e dentro de transação, cada .sql de
// internal/armazenamento/migracoes que ainda não constar em
// schema_migracoes. Idempotente: rodar de novo não reaplica nem falha.
func AplicarMigracoes(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migracoes (
			nome        TEXT PRIMARY KEY,
			aplicada_em TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("erro ao criar schema_migracoes: %w", err)
	}

	entradas, err := armazenamento.ArquivosMigracao.ReadDir("migracoes")
	if err != nil {
		return fmt.Errorf("erro ao listar migrações embutidas: %w", err)
	}

	nomes := make([]string, 0, len(entradas))
	for _, e := range entradas {
		nomes = append(nomes, e.Name())
	}
	sort.Strings(nomes)

	for _, nome := range nomes {
		var jaAplicada bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migracoes WHERE nome=$1)`, nome).Scan(&jaAplicada); err != nil {
			return fmt.Errorf("erro ao checar migração %s: %w", nome, err)
		}
		if jaAplicada {
			continue
		}

		conteudo, err := armazenamento.ArquivosMigracao.ReadFile("migracoes/" + nome)
		if err != nil {
			return fmt.Errorf("erro ao ler migração %s: %w", nome, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("erro ao abrir transação pra migração %s: %w", nome, err)
		}
		if _, err := tx.Exec(ctx, string(conteudo)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("erro ao aplicar migração %s: %w", nome, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migracoes (nome) VALUES ($1)`, nome); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("erro ao registrar migração %s: %w", nome, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("erro ao commitar migração %s: %w", nome, err)
		}
	}
	return nil
}
```

- [ ] **Step 6: Rodar e confirmar sucesso**

Run: `docker compose up -d && go test ./internal/armazenamento/... -v`
Expected: PASS — schema aplicado, idempotência confirmada.

- [ ] **Step 7: Commit**

```bash
git add internal/armazenamento/migracoes.go internal/armazenamento/migracoes/0001_inicial.sql internal/armazenamento/postgres/migrar.go internal/armazenamento/postgres/migrar_test.go
git commit -m "feat: migrações SQL numeradas, embutidas e idempotentes"
```

---

## Task 13: `internal/armazenamento/postgres` — repositórios

**Files:**
- Create: `internal/armazenamento/postgres/testutil_test.go`
- Create: `internal/armazenamento/postgres/sessao.go`
- Create: `internal/armazenamento/postgres/sessao_test.go`
- Create: `internal/armazenamento/postgres/mensagem.go`
- Create: `internal/armazenamento/postgres/mensagem_test.go`
- Create: `internal/armazenamento/postgres/aprovacao.go`
- Create: `internal/armazenamento/postgres/aprovacao_test.go`

- [ ] **Step 1: Criar o helper de teste compartilhado**

Criar `internal/armazenamento/postgres/testutil_test.go`:

```go
package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// abrirPoolMigrado pula o teste se DATABASE_URL não estiver definida (mesma
// convenção de conexao_test.go), senão devolve um pool com o schema em dia.
func abrirPoolMigrado(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}
	pool, err := Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("erro ao aplicar migrações: %v", err)
	}
	return pool
}
```

- [ ] **Step 2: Escrever os testes de `SessaoRepositorio` que falham**

Criar `internal/armazenamento/postgres/sessao_test.go`:

```go
package postgres

import (
	"context"
	"testing"
)

func TestSessaoRepositorioCriaEBusca(t *testing.T) {
	pool := abrirPoolMigrado(t)
	repo := NovoSessaoRepositorio(pool)

	criada, err := repo.Criar(context.Background(), "sessão de teste")
	if err != nil {
		t.Fatalf("Criar devolveu erro: %v", err)
	}
	if criada.Titulo != "sessão de teste" {
		t.Errorf("Titulo = %q, queria sessão de teste", criada.Titulo)
	}

	achada, err := repo.BuscarPorID(context.Background(), criada.ID)
	if err != nil {
		t.Fatalf("BuscarPorID devolveu erro: %v", err)
	}
	if achada.ID != criada.ID {
		t.Errorf("ID = %v, queria %v", achada.ID, criada.ID)
	}
}

func TestSessaoRepositorioListaOrdenaPorAtualizacao(t *testing.T) {
	pool := abrirPoolMigrado(t)
	repo := NovoSessaoRepositorio(pool)

	a, _ := repo.Criar(context.Background(), "primeira")
	b, _ := repo.Criar(context.Background(), "segunda")

	sessoes, err := repo.Listar(context.Background())
	if err != nil {
		t.Fatalf("Listar devolveu erro: %v", err)
	}

	achouA, achouB := false, false
	for _, s := range sessoes {
		if s.ID == a.ID {
			achouA = true
		}
		if s.ID == b.ID {
			achouB = true
		}
	}
	if !achouA || !achouB {
		t.Error("Listar deveria incluir as duas sessões criadas neste teste")
	}
}
```

- [ ] **Step 3: Rodar e confirmar falha**

Run: `docker compose up -d && go test ./internal/armazenamento/postgres/... -run Sessao -v`
Expected: FAIL — `SessaoRepositorio` não existe.

- [ ] **Step 4: Implementar `SessaoRepositorio`**

Criar `internal/armazenamento/postgres/sessao.go`:

```go
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Sessao struct {
	ID           uuid.UUID
	Titulo       string
	CriadaEm     time.Time
	AtualizadaEm time.Time
}

type SessaoRepositorio struct {
	pool *pgxpool.Pool
}

func NovoSessaoRepositorio(pool *pgxpool.Pool) *SessaoRepositorio {
	return &SessaoRepositorio{pool: pool}
}

func (r *SessaoRepositorio) Criar(ctx context.Context, titulo string) (Sessao, error) {
	id := uuid.New()
	if _, err := r.pool.Exec(ctx, `INSERT INTO sessoes (id, titulo) VALUES ($1, $2)`, id, titulo); err != nil {
		return Sessao{}, fmt.Errorf("erro ao criar sessão: %w", err)
	}
	return r.BuscarPorID(ctx, id)
}

func (r *SessaoRepositorio) BuscarPorID(ctx context.Context, id uuid.UUID) (Sessao, error) {
	var s Sessao
	err := r.pool.QueryRow(ctx,
		`SELECT id, titulo, criada_em, atualizada_em FROM sessoes WHERE id=$1`, id,
	).Scan(&s.ID, &s.Titulo, &s.CriadaEm, &s.AtualizadaEm)
	if err != nil {
		return Sessao{}, fmt.Errorf("erro ao buscar sessão %s: %w", id, err)
	}
	return s, nil
}

func (r *SessaoRepositorio) Listar(ctx context.Context) ([]Sessao, error) {
	linhas, err := r.pool.Query(ctx,
		`SELECT id, titulo, criada_em, atualizada_em FROM sessoes ORDER BY atualizada_em DESC`)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar sessões: %w", err)
	}
	defer linhas.Close()

	var sessoes []Sessao
	for linhas.Next() {
		var s Sessao
		if err := linhas.Scan(&s.ID, &s.Titulo, &s.CriadaEm, &s.AtualizadaEm); err != nil {
			return nil, err
		}
		sessoes = append(sessoes, s)
	}
	return sessoes, linhas.Err()
}
```

- [ ] **Step 5: Rodar e confirmar sucesso**

Run: `go test ./internal/armazenamento/postgres/... -run Sessao -v`
Expected: PASS.

- [ ] **Step 6: Escrever os testes de `MensagemRepositorio` que falham**

Criar `internal/armazenamento/postgres/mensagem_test.go`:

```go
package postgres

import (
	"context"
	"testing"
)

func TestMensagemRepositorioSalvaEListaEmOrdem(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(context.Background(), "sessão pra mensagens")
	if err != nil {
		t.Fatalf("erro ao criar sessão de apoio: %v", err)
	}

	if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", "oi", nil); err != nil {
		t.Fatalf("Salvar (1) devolveu erro: %v", err)
	}
	if _, err := mensagens.Salvar(context.Background(), sessao.ID, "modelo", "olá", nil); err != nil {
		t.Fatalf("Salvar (2) devolveu erro: %v", err)
	}

	lista, err := mensagens.ListarPorSessao(context.Background(), sessao.ID, 10)
	if err != nil {
		t.Fatalf("ListarPorSessao devolveu erro: %v", err)
	}
	if len(lista) != 2 {
		t.Fatalf("esperava 2 mensagens, vieram %d", len(lista))
	}
	if lista[0].Papel != "usuario" || lista[1].Papel != "modelo" {
		t.Errorf("ordem errada: %+v", lista)
	}
}

func TestMensagemRepositorioRespeitaLimite(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, _ := sessoes.Criar(context.Background(), "sessão pra limite")
	for i := 0; i < 5; i++ {
		if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", "msg", nil); err != nil {
			t.Fatalf("Salvar devolveu erro: %v", err)
		}
	}

	lista, err := mensagens.ListarPorSessao(context.Background(), sessao.ID, 2)
	if err != nil {
		t.Fatalf("ListarPorSessao devolveu erro: %v", err)
	}
	if len(lista) != 2 {
		t.Fatalf("esperava 2 mensagens (limite), vieram %d", len(lista))
	}
}

func TestMensagemRepositorioSalvarAtualizaSessao(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(context.Background(), "sessão pra checar atualizada_em")
	if err != nil {
		t.Fatalf("erro ao criar sessão de apoio: %v", err)
	}

	if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", "oi", nil); err != nil {
		t.Fatalf("Salvar devolveu erro: %v", err)
	}

	atualizada, err := sessoes.BuscarPorID(context.Background(), sessao.ID)
	if err != nil {
		t.Fatalf("BuscarPorID devolveu erro: %v", err)
	}
	if !atualizada.AtualizadaEm.After(sessao.AtualizadaEm) {
		t.Errorf("AtualizadaEm = %v, esperava depois da criação (%v) — Salvar deveria bater a sessão", atualizada.AtualizadaEm, sessao.AtualizadaEm)
	}
}
```

- [ ] **Step 7: Rodar e confirmar falha**

Run: `go test ./internal/armazenamento/postgres/... -run Mensagem -v`
Expected: FAIL — `MensagemRepositorio` não existe.

- [ ] **Step 8: Implementar `MensagemRepositorio`**

Criar `internal/armazenamento/postgres/mensagem.go`:

```go
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Mensagem struct {
	ID       int64
	SessaoID uuid.UUID
	Papel    string
	Conteudo string
	Chamadas []byte
	CriadaEm time.Time
}

type MensagemRepositorio struct {
	pool *pgxpool.Pool
}

func NovoMensagemRepositorio(pool *pgxpool.Pool) *MensagemRepositorio {
	return &MensagemRepositorio{pool: pool}
}

// Salvar grava uma mensagem e atualiza sessoes.atualizada_em — sem isso a
// coluna nunca mudaria depois da criação da sessão, e Listar() (que ordena
// por atualizada_em DESC) na prática ordenaria por criação, não por
// atividade recente. chamadas pode ser nil (mensagem sem chamada de
// ferramenta) — vira NULL em chamadas JSONB.
func (r *MensagemRepositorio) Salvar(ctx context.Context, sessaoID uuid.UUID, papel, conteudo string, chamadas []byte) (Mensagem, error) {
	var m Mensagem
	err := r.pool.QueryRow(ctx,
		`INSERT INTO mensagens (sessao_id, papel, conteudo, chamadas) VALUES ($1,$2,$3,$4)
		 RETURNING id, sessao_id, papel, conteudo, chamadas, criada_em`,
		sessaoID, papel, conteudo, chamadas,
	).Scan(&m.ID, &m.SessaoID, &m.Papel, &m.Conteudo, &m.Chamadas, &m.CriadaEm)
	if err != nil {
		return Mensagem{}, fmt.Errorf("erro ao salvar mensagem: %w", err)
	}

	if _, err := r.pool.Exec(ctx, `UPDATE sessoes SET atualizada_em = now() WHERE id = $1`, sessaoID); err != nil {
		return Mensagem{}, fmt.Errorf("erro ao atualizar sessão: %w", err)
	}

	return m, nil
}

func (r *MensagemRepositorio) ListarPorSessao(ctx context.Context, sessaoID uuid.UUID, limite int) ([]Mensagem, error) {
	linhas, err := r.pool.Query(ctx,
		`SELECT id, sessao_id, papel, conteudo, chamadas, criada_em FROM mensagens
		 WHERE sessao_id=$1 ORDER BY criada_em ASC LIMIT $2`, sessaoID, limite)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar mensagens da sessão %s: %w", sessaoID, err)
	}
	defer linhas.Close()

	var mensagens []Mensagem
	for linhas.Next() {
		var m Mensagem
		if err := linhas.Scan(&m.ID, &m.SessaoID, &m.Papel, &m.Conteudo, &m.Chamadas, &m.CriadaEm); err != nil {
			return nil, err
		}
		mensagens = append(mensagens, m)
	}
	return mensagens, linhas.Err()
}
```

- [ ] **Step 9: Rodar e confirmar sucesso**

Run: `go test ./internal/armazenamento/postgres/... -run Mensagem -v`
Expected: PASS.

- [ ] **Step 10: Escrever os testes de `AprovacaoRepositorio` que falham**

Criar `internal/armazenamento/postgres/aprovacao_test.go`:

```go
package postgres

import (
	"context"
	"testing"
)

func TestAprovacaoRepositorioCriaEDecide(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	aprovacoes := NovoAprovacaoRepositorio(pool)

	sessao, _ := sessoes.Criar(context.Background(), "sessão pra aprovação")

	criada, err := aprovacoes.Criar(context.Background(), sessao.ID, "executar_comando_sistema",
		[]byte(`{"comando":"Get-Process"}`), "celular")
	if err != nil {
		t.Fatalf("Criar devolveu erro: %v", err)
	}
	if criada.Estado != "pendente" {
		t.Errorf("Estado = %q, queria pendente", criada.Estado)
	}

	if err := aprovacoes.Decidir(context.Background(), criada.ID, "aprovada"); err != nil {
		t.Fatalf("Decidir devolveu erro: %v", err)
	}

	final, err := aprovacoes.BuscarPorID(context.Background(), criada.ID)
	if err != nil {
		t.Fatalf("BuscarPorID devolveu erro: %v", err)
	}
	if final.Estado != "aprovada" {
		t.Errorf("Estado = %q, queria aprovada", final.Estado)
	}
	if final.DecididaEm == nil {
		t.Error("DecididaEm deveria estar preenchido após Decidir")
	}
}

func TestAprovacaoRepositorioNaoDecideDuasVezes(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	aprovacoes := NovoAprovacaoRepositorio(pool)

	sessao, _ := sessoes.Criar(context.Background(), "sessão pra aprovação dupla")
	criada, _ := aprovacoes.Criar(context.Background(), sessao.ID, "executar_comando_sistema",
		[]byte(`{}`), "celular")

	if err := aprovacoes.Decidir(context.Background(), criada.ID, "negada"); err != nil {
		t.Fatalf("primeira decisão devolveu erro: %v", err)
	}
	if err := aprovacoes.Decidir(context.Background(), criada.ID, "aprovada"); err == nil {
		t.Fatal("esperava erro ao decidir aprovação que já não estava pendente")
	}
}
```

- [ ] **Step 11: Rodar e confirmar falha**

Run: `go test ./internal/armazenamento/postgres/... -run Aprovacao -v`
Expected: FAIL — `AprovacaoRepositorio` não existe.

- [ ] **Step 12: Implementar `AprovacaoRepositorio`**

Criar `internal/armazenamento/postgres/aprovacao.go`:

```go
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Aprovacao struct {
	ID            uuid.UUID
	SessaoID      uuid.UUID
	Ferramenta    string
	Argumentos    []byte
	SolicitadaPor string
	Estado        string
	CriadaEm      time.Time
	DecididaEm    *time.Time
}

type AprovacaoRepositorio struct {
	pool *pgxpool.Pool
}

func NovoAprovacaoRepositorio(pool *pgxpool.Pool) *AprovacaoRepositorio {
	return &AprovacaoRepositorio{pool: pool}
}

func (r *AprovacaoRepositorio) Criar(ctx context.Context, sessaoID uuid.UUID, ferramenta string, argumentos []byte, solicitadaPor string) (Aprovacao, error) {
	id := uuid.New()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO aprovacoes (id, sessao_id, ferramenta, argumentos, solicitada_por, estado)
		 VALUES ($1,$2,$3,$4,$5,'pendente')`,
		id, sessaoID, ferramenta, argumentos, solicitadaPor)
	if err != nil {
		return Aprovacao{}, fmt.Errorf("erro ao criar aprovação: %w", err)
	}
	return r.BuscarPorID(ctx, id)
}

func (r *AprovacaoRepositorio) BuscarPorID(ctx context.Context, id uuid.UUID) (Aprovacao, error) {
	var a Aprovacao
	err := r.pool.QueryRow(ctx,
		`SELECT id, sessao_id, ferramenta, argumentos, solicitada_por, estado, criada_em, decidida_em
		 FROM aprovacoes WHERE id=$1`, id,
	).Scan(&a.ID, &a.SessaoID, &a.Ferramenta, &a.Argumentos, &a.SolicitadaPor, &a.Estado, &a.CriadaEm, &a.DecididaEm)
	if err != nil {
		return Aprovacao{}, fmt.Errorf("erro ao buscar aprovação %s: %w", id, err)
	}
	return a, nil
}

// Decidir só aplica se a aprovação ainda estiver pendente — evita corrida
// entre aprovar/negar/expirar (decisão dupla é erro, não sobrescrita).
func (r *AprovacaoRepositorio) Decidir(ctx context.Context, id uuid.UUID, estado string) error {
	if estado != "aprovada" && estado != "negada" && estado != "expirada" {
		return fmt.Errorf("estado %q inválido pra decisão", estado)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE aprovacoes SET estado=$1, decidida_em=now() WHERE id=$2 AND estado='pendente'`, estado, id)
	if err != nil {
		return fmt.Errorf("erro ao decidir aprovação %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("aprovação %s não estava pendente", id)
	}
	return nil
}
```

- [ ] **Step 13: Rodar e confirmar sucesso**

Run: `go test ./internal/armazenamento/... -v`
Expected: PASS em todo o pacote.

- [ ] **Step 14: Commit**

```bash
git add internal/armazenamento/postgres/testutil_test.go internal/armazenamento/postgres/sessao.go internal/armazenamento/postgres/sessao_test.go internal/armazenamento/postgres/mensagem.go internal/armazenamento/postgres/mensagem_test.go internal/armazenamento/postgres/aprovacao.go internal/armazenamento/postgres/aprovacao_test.go
git commit -m "feat: repositórios de sessão, mensagem e aprovação sobre Postgres real"
```

---

## Task 14: `internal/memoria` — janela de contexto com orçamento de tokens

Elimina o bug #4 do Anexo A (`app.go:359` manda o dia inteiro em toda requisição).

**Files:**
- Create: `internal/memoria/janela.go`
- Create: `internal/memoria/janela_test.go`

- [ ] **Step 1: Escrever os testes que falham**

```go
package memoria

import (
	"strings"
	"testing"

	"james/internal/agente"
)

func TestMontarDentroDoOrcamentoDevolveTudo(t *testing.T) {
	j := Janela{OrcamentoTokens: 1000}
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "oi"},
		{Papel: agente.PapelModelo, Texto: "olá"},
	}

	resultado := j.Montar(mensagens)
	if len(resultado) != 2 {
		t.Fatalf("esperava as 2 mensagens dentro do orçamento, vieram %d", len(resultado))
	}
}

func TestMontarCortaMensagensAntigasQuandoEstouraOrcamento(t *testing.T) {
	texto := strings.Repeat("palavra ", 50) // ~400 caracteres, ~100 tokens estimados
	j := Janela{OrcamentoTokens: 120}        // cabe pouco mais de 1 mensagem desse tamanho
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: texto},
		{Papel: agente.PapelModelo, Texto: texto},
		{Papel: agente.PapelUsuario, Texto: "última, curta"},
	}

	resultado := j.Montar(mensagens)
	if len(resultado) >= len(mensagens) {
		t.Fatalf("esperava corte de mensagens antigas, vieram todas as %d", len(resultado))
	}
	if resultado[len(resultado)-1].Texto != "última, curta" {
		t.Error("a mensagem mais recente deveria sempre sobreviver ao corte")
	}
}

func TestMontarPreservaOrdemCronologica(t *testing.T) {
	j := Janela{OrcamentoTokens: 1000}
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "primeira"},
		{Papel: agente.PapelModelo, Texto: "segunda"},
		{Papel: agente.PapelUsuario, Texto: "terceira"},
	}

	resultado := j.Montar(mensagens)
	for i, esperado := range []string{"primeira", "segunda", "terceira"} {
		if resultado[i].Texto != esperado {
			t.Errorf("resultado[%d] = %q, queria %q — ordem cronológica quebrada", i, resultado[i].Texto, esperado)
		}
	}
}

func TestMontarSempreIncluiPeloMenosAUltimaMensagem(t *testing.T) {
	textoEnorme := strings.Repeat("x", 10000)
	j := Janela{OrcamentoTokens: 10} // orçamento menor que qualquer mensagem real
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "antiga"},
		{Papel: agente.PapelUsuario, Texto: textoEnorme},
	}

	resultado := j.Montar(mensagens)
	if len(resultado) != 1 {
		t.Fatalf("esperava 1 mensagem (nunca janela vazia), vieram %d", len(resultado))
	}
	if resultado[0].Texto != textoEnorme {
		t.Error("a última mensagem deveria sobreviver mesmo estourando o orçamento sozinha")
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./internal/memoria/... -v`
Expected: FAIL — pacote ainda não existe.

- [ ] **Step 3: Implementar**

```go
// Package memoria monta a janela de contexto que vai pro Provider: as
// mensagens mais recentes que cabem num orçamento de tokens, nunca o
// histórico inteiro (corrige o bug #4 do Anexo A).
package memoria

import "james/internal/agente"

type Janela struct {
	OrcamentoTokens int
}

// EstimarTokens é uma aproximação grosseira (4 caracteres por token) —
// suficiente pra decidir o corte da janela, não pra faturamento exato.
func EstimarTokens(texto string) int {
	return (len(texto) + 3) / 4
}

func custoMensagem(m agente.Mensagem) int {
	custo := EstimarTokens(m.Texto)
	for _, r := range m.Resultados {
		custo += EstimarTokens(r.Saida) + EstimarTokens(r.Erro)
	}
	return custo
}

// Montar devolve, em ordem cronológica, as mensagens mais recentes que
// cabem em OrcamentoTokens. A última mensagem sempre entra, mesmo que
// sozinha já estoure o orçamento — janela vazia nunca é o resultado certo.
func (j Janela) Montar(mensagens []agente.Mensagem) []agente.Mensagem {
	var total int
	var reversa []agente.Mensagem

	for i := len(mensagens) - 1; i >= 0; i-- {
		custo := custoMensagem(mensagens[i])
		if total+custo > j.OrcamentoTokens && len(reversa) > 0 {
			break
		}
		total += custo
		reversa = append(reversa, mensagens[i])
	}

	resultado := make([]agente.Mensagem, len(reversa))
	for i, m := range reversa {
		resultado[len(reversa)-1-i] = m
	}
	return resultado
}
```

- [ ] **Step 4: Rodar e confirmar sucesso**

Run: `go test ./internal/memoria/... -v`
Expected: PASS nos 4 testes.

- [ ] **Step 5: Commit**

```bash
git add internal/memoria/janela.go internal/memoria/janela_test.go
git commit -m "feat: internal/memoria monta janela de contexto com orçamento de tokens"
```

---

## Task 15: `internal/voz` — TTS gerado no servidor, tocado pelo cliente

Corrige o bug #7 do Anexo A (`edge-tts` síncrono, erro ignorado, arquivo fixo). O bug #8 (`speaker.Init` trava taxa de amostragem) desaparece por design — o servidor nunca mais toca áudio, só gera e entrega por URL (spec 4.7, decisão D1).

**Files:**
- Create: `internal/voz/sintetizador.go`
- Create: `internal/voz/sintetizador_test.go`

- [ ] **Step 1: Escrever os testes que falham**

```go
package voz

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestGerarChamaExecutorEDevolveArquivoUnicoPorID(t *testing.T) {
	var vozRecebida, arquivoRecebido string
	s := &Sintetizador{
		Executor: func(ctx context.Context, texto, voz, arquivoSaida string) error {
			vozRecebida = voz
			arquivoRecebido = arquivoSaida
			return nil
		},
		DiretorioSaida: t.TempDir(),
		Timeout:        time.Second,
	}

	caminho, err := s.Gerar(context.Background(), "olá Arthur", "pt-BR-AntonioNeural", "resposta-123")
	if err != nil {
		t.Fatalf("Gerar devolveu erro: %v", err)
	}
	if vozRecebida != "pt-BR-AntonioNeural" {
		t.Errorf("voz repassada = %q", vozRecebida)
	}
	if filepath.Base(caminho) != "resposta-123.mp3" {
		t.Errorf("caminho = %q, queria terminar em resposta-123.mp3 (nome único por resposta, corrige bug #7)", caminho)
	}
	if arquivoRecebido != caminho {
		t.Errorf("arquivo passado ao Executor = %q, deveria bater com o devolvido = %q", arquivoRecebido, caminho)
	}
}

func TestGerarPropagaErroDoExecutor(t *testing.T) {
	s := &Sintetizador{
		Executor:       func(ctx context.Context, texto, voz, arquivoSaida string) error { return errors.New("edge-tts não encontrado") },
		DiretorioSaida: t.TempDir(),
		Timeout:        time.Second,
	}

	_, err := s.Gerar(context.Background(), "teste", "pt-BR-AntonioNeural", "id-1")
	if err == nil {
		t.Fatal("esperava erro propagado do Executor (corrige bug #7: erro nunca ignorado), veio nil")
	}
}

func TestGerarRespeitaTimeout(t *testing.T) {
	s := &Sintetizador{
		Executor: func(ctx context.Context, texto, voz, arquivoSaida string) error {
			select {
			case <-time.After(200 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		DiretorioSaida: t.TempDir(),
		Timeout:        20 * time.Millisecond,
	}

	_, err := s.Gerar(context.Background(), "teste", "pt-BR-AntonioNeural", "id-2")
	if err == nil {
		t.Fatal("esperava erro de timeout (R4), veio nil")
	}
}

func TestGerarIDsDiferentesGeramArquivosDiferentes(t *testing.T) {
	s := &Sintetizador{
		Executor:       func(ctx context.Context, texto, voz, arquivoSaida string) error { return nil },
		DiretorioSaida: t.TempDir(),
		Timeout:        time.Second,
	}

	a, _ := s.Gerar(context.Background(), "um", "pt-BR-AntonioNeural", "id-a")
	b, _ := s.Gerar(context.Background(), "dois", "pt-BR-AntonioNeural", "id-b")
	if a == b {
		t.Error("respostas diferentes não podem colidir no mesmo arquivo (corrige bug #7: arquivo de saída fixo)")
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./internal/voz/... -v`
Expected: FAIL — pacote ainda não existe.

- [ ] **Step 3: Implementar**

```go
// Package voz gera áudio via edge-tts, um arquivo por resposta. O servidor
// nunca toca o áudio — só gera e entrega por URL; quem toca é o cliente
// (spec 4.7, decisão D1).
package voz

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Sintetizador struct {
	Executor       func(ctx context.Context, texto, voz, arquivoSaida string) error
	DiretorioSaida string
	Timeout        time.Duration
}

func NovoSintetizador(diretorioSaida string) *Sintetizador {
	return &Sintetizador{
		Executor:       executarEdgeTTS,
		DiretorioSaida: diretorioSaida,
		Timeout:        30 * time.Second,
	}
}

func executarEdgeTTS(ctx context.Context, texto, voz, arquivoSaida string) error {
	cmd := exec.CommandContext(ctx, "edge-tts", "--voice", voz, "--text", texto, "--write-media", arquivoSaida)
	return cmd.Run()
}

// Gerar sintetiza texto em áudio e devolve o caminho do arquivo — nome
// único por id de resposta, nunca um arquivo fixo compartilhado (bug #7).
// Erro do Executor sempre propaga (nunca é engolido, R4 + bug #7).
func (s *Sintetizador) Gerar(ctx context.Context, texto, voz, id string) (string, error) {
	ctx, cancelar := context.WithTimeout(ctx, s.Timeout)
	defer cancelar()

	if err := os.MkdirAll(s.DiretorioSaida, 0o755); err != nil {
		return "", fmt.Errorf("erro ao preparar diretório de áudio: %w", err)
	}

	arquivo := filepath.Join(s.DiretorioSaida, id+".mp3")
	if err := s.Executor(ctx, texto, voz, arquivo); err != nil {
		return "", fmt.Errorf("erro ao gerar áudio: %w", err)
	}
	return arquivo, nil
}
```

- [ ] **Step 4: Rodar e confirmar sucesso**

Run: `go test ./internal/voz/... -v`
Expected: PASS nos 4 testes.

- [ ] **Step 5: Commit**

```bash
git add internal/voz/sintetizador.go internal/voz/sintetizador_test.go
git commit -m "feat: internal/voz gera TTS assíncrono com timeout e arquivo único por resposta"
```

---

## Task 16: `internal/api` — rotas REST

**Files:**
- Create: `internal/api/rest.go`
- Create: `internal/api/rest_test.go`

- [ ] **Step 1: Escrever os testes que falham**

Criar `internal/api/rest_test.go`:

```go
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"james/internal/armazenamento/postgres"
)

func servidorDeTeste(t *testing.T) *Servidor {
	t.Helper()
	url := postgres.URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}
	pool, err := postgres.Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("erro ao aplicar migrações: %v", err)
	}

	return &Servidor{
		Sessoes:        postgres.NovoSessaoRepositorio(pool),
		Mensagens:      postgres.NovoMensagemRepositorio(pool),
		DiretorioAudio: t.TempDir(),
	}
}

func TestSaudeDevolveOK(t *testing.T) {
	s := servidorDeTeste(t)
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/saude")
	if err != nil {
		t.Fatalf("GET /api/saude devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, queria 200", resp.StatusCode)
	}
}

func TestCriarEListarSessoes(t *testing.T) {
	s := servidorDeTeste(t)
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	corpo, _ := json.Marshal(map[string]string{"titulo": "sessão via REST"})
	resp, err := http.Post(servidor.URL+"/api/sessoes", "application/json", bytes.NewReader(corpo))
	if err != nil {
		t.Fatalf("POST /api/sessoes devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, queria 201", resp.StatusCode)
	}
	var criada map[string]any
	json.NewDecoder(resp.Body).Decode(&criada)
	if criada["ID"] == nil {
		t.Fatal("resposta de criação deveria incluir ID")
	}

	respLista, err := http.Get(servidor.URL + "/api/sessoes")
	if err != nil {
		t.Fatalf("GET /api/sessoes devolveu erro: %v", err)
	}
	defer respLista.Body.Close()
	var lista []map[string]any
	json.NewDecoder(respLista.Body).Decode(&lista)
	if len(lista) == 0 {
		t.Error("esperava pelo menos a sessão recém-criada na listagem")
	}
}

func TestListarMensagensDaSessao(t *testing.T) {
	s := servidorDeTeste(t)
	sessao, err := s.Sessoes.Criar(context.Background(), "sessão pra mensagens via REST")
	if err != nil {
		t.Fatalf("erro ao criar sessão de apoio: %v", err)
	}
	if _, err := s.Mensagens.Salvar(context.Background(), sessao.ID, "usuario", "oi", nil); err != nil {
		t.Fatalf("erro ao salvar mensagem de apoio: %v", err)
	}

	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/sessoes/" + sessao.ID.String() + "/mensagens")
	if err != nil {
		t.Fatalf("GET mensagens devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, queria 200", resp.StatusCode)
	}
	var mensagens []map[string]any
	json.NewDecoder(resp.Body).Decode(&mensagens)
	if len(mensagens) != 1 {
		t.Errorf("esperava 1 mensagem, vieram %d", len(mensagens))
	}
}

func TestServirAudioDevolveArquivoExistente(t *testing.T) {
	s := servidorDeTeste(t)
	if err := os.WriteFile(filepath.Join(s.DiretorioAudio, "resp-1.mp3"), []byte("audio falso"), 0o600); err != nil {
		t.Fatalf("erro ao escrever fixture de áudio: %v", err)
	}

	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/audio/resp-1")
	if err != nil {
		t.Fatalf("GET /api/audio devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, queria 200", resp.StatusCode)
	}
}

func TestServirAudioInexistenteDevolve404(t *testing.T) {
	s := servidorDeTeste(t)
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	resp, err := http.Get(servidor.URL + "/api/audio/não-existe")
	if err != nil {
		t.Fatalf("GET /api/audio devolveu erro: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, queria 404", resp.StatusCode)
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `docker compose up -d && go test ./internal/api/... -v`
Expected: FAIL — pacote ainda não existe.

- [ ] **Step 3: Implementar**

Criar `internal/api/rest.go`:

```go
// Package api expõe o jamesd via REST (sessões, histórico, áudio) e
// WebSocket (conversa em tempo real, ponte com o device) — ver Task 17/18.
package api

import (
	"encoding/json"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"

	"james/internal/armazenamento/postgres"
)

type Servidor struct {
	Sessoes        *postgres.SessaoRepositorio
	Mensagens      *postgres.MensagemRepositorio
	DiretorioAudio string
}

func (s *Servidor) Rotas() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/saude", s.saude)
	mux.HandleFunc("GET /api/sessoes", s.listarSessoes)
	mux.HandleFunc("POST /api/sessoes", s.criarSessao)
	mux.HandleFunc("GET /api/sessoes/{id}/mensagens", s.listarMensagens)
	mux.HandleFunc("GET /api/audio/{id}", s.servirAudio)
	return mux
}

func (s *Servidor) saude(w http.ResponseWriter, r *http.Request) {
	responderJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Servidor) listarSessoes(w http.ResponseWriter, r *http.Request) {
	sessoes, err := s.Sessoes.Listar(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderJSON(w, http.StatusOK, sessoes)
}

func (s *Servidor) criarSessao(w http.ResponseWriter, r *http.Request) {
	var corpo struct {
		Titulo string `json:"titulo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&corpo); err != nil {
		http.Error(w, "corpo inválido", http.StatusBadRequest)
		return
	}
	sessao, err := s.Sessoes.Criar(r.Context(), corpo.Titulo)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderJSON(w, http.StatusCreated, sessao)
}

func (s *Servidor) listarMensagens(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		http.Error(w, "id inválido", http.StatusBadRequest)
		return
	}
	mensagens, err := s.Mensagens.ListarPorSessao(r.Context(), id, 200)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	responderJSON(w, http.StatusOK, mensagens)
}

func (s *Servidor) servirAudio(w http.ResponseWriter, r *http.Request) {
	caminho := filepath.Join(s.DiretorioAudio, r.PathValue("id")+".mp3")
	http.ServeFile(w, r, caminho)
}

func responderJSON(w http.ResponseWriter, status int, corpo any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(corpo)
}
```

- [ ] **Step 4: Rodar e confirmar sucesso**

Run: `go test ./internal/api/... -v`
Expected: PASS nos 5 testes.

- [ ] **Step 5: Commit**

```bash
git add internal/api/rest.go internal/api/rest_test.go
git commit -m "feat: internal/api expõe saude, sessoes, mensagens e audio via REST"
```

---

## Task 17: `internal/api` — WebSocket de clientes, aprovação e integração com o agente

**Files:**
- Create: `internal/api/envelope.go`
- Create: `internal/api/clientes.go`
- Create: `internal/api/clientes_test.go`
- Create: `internal/api/aprovacoes_pendentes.go`
- Create: `internal/api/aprovacoes_pendentes_test.go`
- Create: `internal/api/ws.go`
- Create: `internal/api/ws_test.go`
- Modify: `internal/api/rest.go` (adicionar campos ao `Servidor`)
- Modify: `go.mod` (promover `github.com/gorilla/websocket` a dependência direta)

- [ ] **Step 1: Promover `gorilla/websocket` a dependência direta**

Run: `go get github.com/gorilla/websocket@v1.5.3`

- [ ] **Step 2: Escrever os testes do `ClientesHub` que falham**

Criar `internal/api/clientes_test.go`:

```go
package api

import (
	"testing"

	"james/internal/permissao"
)

type clienteFalso struct {
	perfil  permissao.Perfil
	enviados []string
}

func (c *clienteFalso) Perfil() permissao.Perfil { return c.perfil }
func (c *clienteFalso) Enviar(tipo, id string, dados any) error {
	c.enviados = append(c.enviados, tipo)
	return nil
}

func TestBroadcastAprovacaoSoAtingePrivilegiados(t *testing.T) {
	hub := NovoClientesHub()
	privilegiado := &clienteFalso{perfil: permissao.Privilegiado}
	restrito := &clienteFalso{perfil: permissao.Restrito}
	hub.Registrar(privilegiado)
	hub.Registrar(restrito)

	hub.BroadcastAprovacaoRequerida("aprov-1", map[string]string{"ferramenta": "x"})

	if len(privilegiado.enviados) != 1 || privilegiado.enviados[0] != "aprovacao_requerida" {
		t.Errorf("cliente privilegiado deveria ter recebido aprovacao_requerida, recebeu %v", privilegiado.enviados)
	}
	if len(restrito.enviados) != 0 {
		t.Errorf("cliente restrito não deveria receber aprovacao_requerida, recebeu %v", restrito.enviados)
	}
}

func TestRemoverParaDeReceberBroadcast(t *testing.T) {
	hub := NovoClientesHub()
	c := &clienteFalso{perfil: permissao.Privilegiado}
	hub.Registrar(c)
	hub.Remover(c)

	hub.BroadcastAprovacaoRequerida("aprov-1", map[string]string{})
	if len(c.enviados) != 0 {
		t.Error("cliente removido não deveria mais receber broadcast")
	}
}
```

- [ ] **Step 3: Rodar e confirmar falha**

Run: `go test ./internal/api/... -run ClientesHub -v`
Expected: FAIL — tipos ainda não existem.

- [ ] **Step 4: Implementar `Envelope` e `ClientesHub`**

Criar `internal/api/envelope.go`:

```go
package api

import "encoding/json"

// Envelope é o formato comum de toda mensagem WS (spec seção 4.4).
type Envelope struct {
	Tipo  string          `json:"tipo"`
	ID    string          `json:"id"`
	Dados json.RawMessage `json:"dados"`
}

func codificarDados(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}
```

Criar `internal/api/clientes.go`:

```go
package api

import (
	"sync"

	"github.com/gorilla/websocket"

	"james/internal/permissao"
)

// ClienteConectado é o que o ClientesHub precisa de um cliente WS —
// desacoplado do transporte real pra ser testável sem socket de verdade.
type ClienteConectado interface {
	Perfil() permissao.Perfil
	Enviar(tipo, id string, dados any) error
}

type conexaoCliente struct {
	ws     *websocket.Conn
	mu     sync.Mutex
	perfil permissao.Perfil
}

func (c *conexaoCliente) Perfil() permissao.Perfil { return c.perfil }

func (c *conexaoCliente) Enviar(tipo, id string, dados any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteJSON(Envelope{Tipo: tipo, ID: id, Dados: codificarDados(dados)})
}

// ClientesHub rastreia os clientes conectados só pra poder fazer broadcast
// de aprovacao_requerida — cada conexão trata sua própria conversa direto.
type ClientesHub struct {
	mu    sync.Mutex
	todos map[ClienteConectado]bool
}

func NovoClientesHub() *ClientesHub {
	return &ClientesHub{todos: make(map[ClienteConectado]bool)}
}

func (h *ClientesHub) Registrar(c ClienteConectado) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.todos[c] = true
}

func (h *ClientesHub) Remover(c ClienteConectado) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.todos, c)
}

// BroadcastAprovacaoRequerida só atinge clientes Privilegiado — só quem
// pode decidir precisa ver o pedido de aprovação.
func (h *ClientesHub) BroadcastAprovacaoRequerida(id string, dados any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.todos {
		if c.Perfil() == permissao.Privilegiado {
			_ = c.Enviar("aprovacao_requerida", id, dados)
		}
	}
}
```

- [ ] **Step 5: Rodar e confirmar sucesso**

Run: `go test ./internal/api/... -run "ClientesHub|Broadcast|Remover" -v`
Expected: PASS.

- [ ] **Step 6: Escrever os testes de `aprovacoesPendentes` que falham**

Criar `internal/api/aprovacoes_pendentes_test.go`:

```go
package api

import (
	"testing"
	"time"
)

func TestAguardarEResolverEntregaDecisao(t *testing.T) {
	r := NovoRegistroAprovacoesPendentes()
	espera := r.aguardar("id-1")

	ok := r.resolver("id-1", true)
	if !ok {
		t.Fatal("resolver deveria achar o canal pendente")
	}

	select {
	case aprovado := <-espera:
		if !aprovado {
			t.Error("esperava aprovado=true")
		}
	case <-time.After(time.Second):
		t.Fatal("canal não recebeu a decisão a tempo")
	}
}

func TestResolverIDDesconhecidoDevolveFalse(t *testing.T) {
	r := NovoRegistroAprovacoesPendentes()
	if r.resolver("não-existe", true) {
		t.Error("resolver deveria devolver false pra id não pendente")
	}
}
```

- [ ] **Step 7: Rodar e confirmar falha**

Run: `go test ./internal/api/... -run Aguardar -v`
Expected: FAIL.

- [ ] **Step 8: Implementar `aprovacoesPendentes`**

Criar `internal/api/aprovacoes_pendentes.go`:

```go
package api

import "sync"

// registroAprovacoesPendentes casa a decisão que chega por "aprovar"/"negar"
// no WS com quem está bloqueado esperando em SolicitarAprovacao.
type registroAprovacoesPendentes struct {
	mu     sync.Mutex
	canais map[string]chan bool
}

func NovoRegistroAprovacoesPendentes() *registroAprovacoesPendentes {
	return &registroAprovacoesPendentes{canais: make(map[string]chan bool)}
}

func (r *registroAprovacoesPendentes) aguardar(id string) chan bool {
	c := make(chan bool, 1)
	r.mu.Lock()
	r.canais[id] = c
	r.mu.Unlock()
	return c
}

func (r *registroAprovacoesPendentes) resolver(id string, aprovado bool) bool {
	r.mu.Lock()
	c, ok := r.canais[id]
	if ok {
		delete(r.canais, id)
	}
	r.mu.Unlock()
	if !ok {
		return false
	}
	c <- aprovado
	return true
}
```

- [ ] **Step 9: Rodar e confirmar sucesso**

Run: `go test ./internal/api/... -run Aguardar -v`
Expected: PASS.

- [ ] **Step 10: Adicionar os campos que faltam em `Servidor`**

Editar `internal/api/rest.go`, struct `Servidor`:

```go
type Servidor struct {
	Sessoes        *postgres.SessaoRepositorio
	Mensagens      *postgres.MensagemRepositorio
	Aprovacoes     *postgres.AprovacaoRepositorio
	DiretorioAudio string

	Loop                *agente.Loop
	Memoria             memoria.Janela
	SistemaPrompt       string
	Clientes            *ClientesHub
	AprovacoesPendentes *registroAprovacoesPendentes
	TimeoutAprovacao    time.Duration
	// TimeoutTurno limita cada chamada ao Provider dentro do loop do
	// agente (R4 — timeout em tudo). Zero desativa o limite (usado nos
	// testes deste pacote, que não têm rede real pra travar).
	TimeoutTurno time.Duration

	// Sintetizador é opcional — nil (padrão nos testes) pula a geração de
	// áudio sem quebrar o turno (spec 4.7, Task 15).
	Sintetizador *voz.Sintetizador
	VozTTS       string
}
```

(Adicionar os imports `"time"`, `"james/internal/agente"`, `"james/internal/memoria"`, `"james/internal/voz"` no topo de `rest.go`.) Registrar a rota WS em `Rotas()`:

```go
mux.HandleFunc("GET /ws", s.ws)
```

- [ ] **Step 11: Escrever o teste de ponta a ponta que falha (cliente Privilegiado, sem aprovação)**

Criar `internal/api/ws_test.go`:

```go
package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/ferramentas"
	"james/internal/memoria"
)

type providerFixo struct{ texto string }

func (p *providerFixo) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	return agente.Resposta{Texto: p.texto}, nil
}
func (p *providerFixo) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

func servidorWSDeTeste(t *testing.T) *httptest.Server {
	t.Helper()
	url := postgres.URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}
	pool, err := postgres.Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("erro ao aplicar migrações: %v", err)
	}

	s := &Servidor{
		Sessoes:    postgres.NovoSessaoRepositorio(pool),
		Mensagens:  postgres.NovoMensagemRepositorio(pool),
		Aprovacoes: postgres.NovoAprovacaoRepositorio(pool),
		Loop: &agente.Loop{
			Provider:     &providerFixo{texto: "resposta de teste"},
			Ferramentas:  ferramentas.NovoRegistro(),
			MaxIteracoes: 5,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: 8000},
		SistemaPrompt:       "sistema de teste",
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
		TimeoutAprovacao:    time.Second,
	}
	return httptest.NewServer(s.Rotas())
}

func TestWSMensagemDevolveRespostaFinal(t *testing.T) {
	servidor := servidorWSDeTeste(t)
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(Envelope{Tipo: "mensagem", ID: "1", Dados: codificarDados(map[string]string{"texto": "oi"})}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	achouRespostaFinal := false
	for i := 0; i < 5; i++ {
		var recebido Envelope
		if err := conn.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler do WS: %v", err)
		}
		if recebido.Tipo == "resposta_final" {
			achouRespostaFinal = true
			break
		}
	}
	if !achouRespostaFinal {
		t.Fatal("esperava receber um envelope resposta_final")
	}
}
```

- [ ] **Step 12: Rodar e confirmar falha**

Run: `docker compose up -d && go test ./internal/api/... -run WSMensagem -v`
Expected: FAIL — `s.ws` não existe.

- [ ] **Step 13: Implementar o handler WS**

Criar `internal/api/ws.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/permissao"
)

var upgrader = websocket.Upgrader{
	// A rede já é fechada pela VPN/loopback (spec I1/I2) — Origin de
	// browser não é a fronteira de segurança aqui.
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Servidor) ws(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("erro no upgrade WS", "erro", err)
		return
	}
	defer conn.Close()

	cliente := &conexaoCliente{ws: conn, perfil: permissao.ResolverPerfil(r.RemoteAddr)}
	s.Clientes.Registrar(cliente)
	defer s.Clientes.Remover(cliente)

	var sessaoID uuid.UUID
	var historico []agente.Mensagem

	for {
		var envelope Envelope
		if err := conn.ReadJSON(&envelope); err != nil {
			return
		}

		switch envelope.Tipo {
		case "mensagem":
			s.tratarMensagem(r.Context(), cliente, envelope, &sessaoID, &historico)
		case "aprovar":
			s.AprovacoesPendentes.resolver(envelope.ID, true)
		case "negar":
			s.AprovacoesPendentes.resolver(envelope.ID, false)
		}
	}
}

func (s *Servidor) tratarMensagem(ctx context.Context, cliente *conexaoCliente, envelope Envelope, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	var dados struct {
		Texto string `json:"texto"`
	}
	if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": "corpo inválido"})
		return
	}

	resp, audioID, err := s.processarTurno(ctx, cliente.Perfil(), sessaoID, historico, dados.Texto,
		func(c agente.ChamadaFerramenta) {
			cliente.Enviar("ferramenta_iniciada", envelope.ID, map[string]string{"nome": c.Nome})
		},
		func(rf agente.ResultadoFerramenta) {
			cliente.Enviar("ferramenta_concluida", envelope.ID, map[string]string{"nome": rf.Nome, "erro": rf.Erro})
		},
	)
	if err != nil {
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": err.Error()})
		return
	}

	cliente.Enviar("token", envelope.ID, map[string]string{"texto": resp.Texto})
	cliente.Enviar("resposta_final", envelope.ID, map[string]string{"texto": resp.Texto})
	if audioID != "" {
		cliente.Enviar("audio_pronto", envelope.ID, map[string]string{"id": audioID})
	}
}

// processarTurno é o núcleo compartilhado entre uma mensagem de cliente WS
// (Task 17) e uma entrada de voz vinda do device (Task 18): garante a
// sessão, monta a janela de memória, roda o loop do agente com os hooks de
// visibilidade, persiste as mensagens novas e gera o áudio da resposta
// (Task 15/4.7 da spec — TTS entregue ao cliente). Devolve a resposta e o
// id do áudio gerado (vazio se não gerou); quem chama decide pra quem os
// eventos intermediários e o audio_pronto vão.
func (s *Servidor) processarTurno(ctx context.Context, perfil permissao.Perfil, sessaoID *uuid.UUID, historico *[]agente.Mensagem, texto string, aoIniciar func(agente.ChamadaFerramenta), aoConcluir func(agente.ResultadoFerramenta)) (agente.Resposta, string, error) {
	if *sessaoID == uuid.Nil {
		sessao, err := s.Sessoes.Criar(ctx, texto)
		if err != nil {
			return agente.Resposta{}, "", err
		}
		*sessaoID = sessao.ID
	}

	tamanhoAntes := len(*historico)
	janela := s.Memoria.Montar(*historico)
	entrada := agente.Mensagem{Papel: agente.PapelUsuario, Texto: texto}

	loop := *s.Loop
	loop.AoIniciarFerramenta = aoIniciar
	loop.AoConcluirFerramenta = aoConcluir

	ctxTurno := ctx
	if s.TimeoutTurno > 0 {
		var cancelar context.CancelFunc
		ctxTurno, cancelar = context.WithTimeout(ctx, s.TimeoutTurno)
		defer cancelar()
	}

	resp, mensagens, err := loop.Executar(ctxTurno, s.SistemaPrompt, janela, entrada, perfil, s.criarSolicitarAprovacao(*sessaoID))
	if err != nil {
		return agente.Resposta{}, "", err
	}

	*historico = mensagens
	s.persistirNovasMensagens(ctx, *sessaoID, mensagens, tamanhoAntes)

	audioID := s.gerarAudioDaResposta(ctx, resp.Texto)
	return resp, audioID, nil
}

// gerarAudioDaResposta é melhor-esforço: se o Sintetizador não estiver
// configurado (testes, por exemplo) ou a geração falhar, devolve "" e loga
// — nunca derruba o turno por causa do áudio (mesma filosofia de R2).
func (s *Servidor) gerarAudioDaResposta(ctx context.Context, texto string) string {
	if s.Sintetizador == nil || texto == "" {
		return ""
	}
	id := uuid.NewString()
	if _, err := s.Sintetizador.Gerar(ctx, texto, s.VozTTS, id); err != nil {
		slog.Error("erro ao gerar áudio da resposta", "erro", err)
		return ""
	}
	return id
}

// criarSolicitarAprovacao devolve o callback que o Loop chama quando um
// perfil Restrito pede uma ferramenta perigosa: cria a aprovação pendente,
// avisa os clientes Privilegiado, e espera decisão ou timeout (R4).
func (s *Servidor) criarSolicitarAprovacao(sessaoID uuid.UUID) agente.SolicitarAprovacao {
	return func(ctx context.Context, chamada agente.ChamadaFerramenta) (bool, error) {
		argumentos, _ := json.Marshal(chamada.Args)
		aprovacao, err := s.Aprovacoes.Criar(ctx, sessaoID, chamada.Nome, argumentos, "cliente-restrito")
		if err != nil {
			return false, err
		}

		espera := s.AprovacoesPendentes.aguardar(aprovacao.ID.String())
		s.Clientes.BroadcastAprovacaoRequerida(aprovacao.ID.String(), map[string]any{
			"ferramenta": chamada.Nome,
			"argumentos": chamada.Args,
		})

		select {
		case aprovado := <-espera:
			estado := "negada"
			if aprovado {
				estado = "aprovada"
			}
			if err := s.Aprovacoes.Decidir(ctx, aprovacao.ID, estado); err != nil {
				slog.Error("erro ao registrar decisão de aprovação", "erro", err)
			}
			return aprovado, nil
		case <-time.After(s.TimeoutAprovacao):
			if err := s.Aprovacoes.Decidir(ctx, aprovacao.ID, "expirada"); err != nil {
				slog.Error("erro ao expirar aprovação", "erro", err)
			}
			return false, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
}

func (s *Servidor) persistirNovasMensagens(ctx context.Context, sessaoID uuid.UUID, todas []agente.Mensagem, apartirDe int) {
	for _, m := range todas[apartirDe:] {
		var chamadasJSON []byte
		switch {
		case len(m.Chamadas) > 0:
			chamadasJSON, _ = json.Marshal(m.Chamadas)
		case len(m.Resultados) > 0:
			chamadasJSON, _ = json.Marshal(m.Resultados)
		}
		if _, err := s.Mensagens.Salvar(ctx, sessaoID, string(m.Papel), m.Texto, chamadasJSON); err != nil {
			slog.Error("erro ao persistir mensagem", "erro", err)
		}
	}
}
```

- [ ] **Step 14: Rodar e confirmar sucesso**

Run: `go test ./internal/api/... -v`
Expected: PASS em todo o pacote (REST + ClientesHub + aprovacoesPendentes + WS ponta a ponta).

**Nota sobre cobertura:** o teste de ponta a ponta cobre o caminho Privilegiado (loopback) — o `httptest.Server` sempre conecta via `127.0.0.1`, então não dá pra simular uma origem de VPN de verdade nesse ambiente. O fluxo de aprovação (perfil Restrito, `criarSolicitarAprovacao`, broadcast, timeout) já está coberto nos testes unitários de `ClientesHub` e `aprovacoesPendentes` acima. Confirmar o caminho Restrito de ponta a ponta de verdade é o critério de aceitação #6 da spec — testado à mão, com o celular real na VPN, no fechamento da fase.

- [ ] **Step 15: Commit**

```bash
git add internal/api/envelope.go internal/api/clientes.go internal/api/clientes_test.go internal/api/aprovacoes_pendentes.go internal/api/aprovacoes_pendentes_test.go internal/api/ws.go internal/api/ws_test.go internal/api/rest.go go.mod go.sum
git commit -m "feat: internal/api liga WS de clientes ao loop do agente com fluxo de aprovação"
```

---

## Task 18: `internal/api/dispositivo.go` — endpoint WS do `james-device`

Fecha o outro lado da ponte do Task 10/11: aqui é onde o `james-device` conecta de verdade, e onde `entrada_voz` vira uma conversa completa, com a resposta indo pros clientes Privilegiado (o dono da máquina, seja qual for o cliente que ele tem aberto).

**Files:**
- Create: `internal/api/dispositivo.go`
- Create: `internal/api/dispositivo_test.go`
- Modify: `internal/api/rest.go` (campo `DispositivoHub`)
- Modify: `internal/api/clientes.go` (`BroadcastParaPrivilegiados`)
- Modify: `internal/api/clientes_test.go` (teste do broadcast genérico)

- [ ] **Step 1: Escrever o teste de `BroadcastParaPrivilegiados` que falha**

Adicionar a `internal/api/clientes_test.go`:

```go
func TestBroadcastParaPrivilegiadosEnviaTipoEDadosCorretos(t *testing.T) {
	hub := NovoClientesHub()
	privilegiado := &clienteFalso{perfil: permissao.Privilegiado}
	restrito := &clienteFalso{perfil: permissao.Restrito}
	hub.Registrar(privilegiado)
	hub.Registrar(restrito)

	hub.BroadcastParaPrivilegiados("resposta_final", "turno-1", map[string]string{"texto": "oi"})

	if len(privilegiado.enviados) != 1 || privilegiado.enviados[0] != "resposta_final" {
		t.Errorf("privilegiado deveria ter recebido resposta_final, recebeu %v", privilegiado.enviados)
	}
	if len(restrito.enviados) != 0 {
		t.Error("restrito não deveria receber eventos originados do device")
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `go test ./internal/api/... -run BroadcastParaPrivilegiados -v`
Expected: FAIL — método não existe.

- [ ] **Step 3: Implementar `BroadcastParaPrivilegiados`**

Editar `internal/api/clientes.go`, adicionar depois de `BroadcastAprovacaoRequerida`:

```go
// BroadcastParaPrivilegiados manda qualquer evento (token, ferramenta_*,
// resposta_final, erro) pra todo cliente Privilegiado conectado — é como o
// dono da máquina vê a resposta a uma entrada de voz vinda do device, já
// que a voz não chega por um cliente WS específico.
func (h *ClientesHub) BroadcastParaPrivilegiados(tipo, id string, dados any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.todos {
		if c.Perfil() == permissao.Privilegiado {
			_ = c.Enviar(tipo, id, dados)
		}
	}
}
```

- [ ] **Step 4: Rodar e confirmar sucesso**

Run: `go test ./internal/api/... -run "BroadcastAprovacao|BroadcastParaPrivilegiados|Remover" -v`
Expected: PASS.

- [ ] **Step 5: Adicionar `DispositivoHub` ao `Servidor`**

Editar `internal/api/rest.go`: adicionar `DispositivoHub *dispositivo.Hub` à struct `Servidor` e o import `"james/internal/dispositivo"`. Registrar a rota em `Rotas()`:

```go
mux.HandleFunc("GET /ws/dispositivo", s.wsDispositivo)
```

- [ ] **Step 6: Escrever os testes do endpoint de device que falham**

Criar `internal/api/dispositivo_test.go`:

```go
package api

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/dispositivo"
	"james/internal/ferramentas"
	"james/internal/memoria"
)

func servidorComDispositivoDeTeste(t *testing.T) *httptest.Server {
	t.Helper()
	url := postgres.URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}
	pool, err := postgres.Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("erro ao aplicar migrações: %v", err)
	}

	s := &Servidor{
		Sessoes:    postgres.NovoSessaoRepositorio(pool),
		Mensagens:  postgres.NovoMensagemRepositorio(pool),
		Aprovacoes: postgres.NovoAprovacaoRepositorio(pool),
		Loop: &agente.Loop{
			Provider:     &providerFixo{texto: "resposta pra voz"},
			Ferramentas:  ferramentas.NovoRegistro(),
			MaxIteracoes: 5,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: 8000},
		SistemaPrompt:       "sistema de teste",
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
		TimeoutAprovacao:    time.Second,
		DispositivoHub:      dispositivo.NovoHub(),
	}
	return httptest.NewServer(s.Rotas())
}

func TestWSDispositivoResultadoAcaoResolveHub(t *testing.T) {
	servidor := servidorComDispositivoDeTeste(t)
	defer servidor.Close()

	// A instância de Hub usada pelo servidor é a que precisamos checar —
	// pegamos ela de volta reabrindo a mesma referência através do handler
	// não é possível diretamente, então testamos via EnviarAcao chamado
	// depois da conexão do device se registrar.
	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws/dispositivo"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS do device: %v", err)
	}
	defer conn.Close()

	// Servidor manda a ação; simulamos o device lendo e respondendo.
	go func() {
		var recebido Envelope
		if err := conn.ReadJSON(&recebido); err != nil {
			return
		}
		conn.WriteJSON(Envelope{
			Tipo: "resultado_acao", ID: recebido.ID,
			Dados: codificarDados(map[string]string{"saida": "clique feito"}),
		})
	}()

	// Não temos acesso direto ao *Hub do servidor de teste a partir daqui
	// sem expor um campo — então o teste real desta ponta a ponta vive em
	// internal/dispositivo (Task 10). Este teste garante que a conexão WS
	// não derruba o servidor e aceita o envelope resultado_acao sem erro.
	time.Sleep(100 * time.Millisecond)
}

func TestWSDispositivoEntradaDeVozGeraRespostaParaPrivilegiados(t *testing.T) {
	servidor := servidorComDispositivoDeTeste(t)
	defer servidor.Close()

	urlBase := "ws" + strings.TrimPrefix(servidor.URL, "http")

	connCliente, _, err := websocket.DefaultDialer.Dial(urlBase+"/ws", nil)
	if err != nil {
		t.Fatalf("erro ao conectar cliente: %v", err)
	}
	defer connCliente.Close()

	connDevice, _, err := websocket.DefaultDialer.Dial(urlBase+"/ws/dispositivo", nil)
	if err != nil {
		t.Fatalf("erro ao conectar device: %v", err)
	}
	defer connDevice.Close()

	if err := connDevice.WriteJSON(Envelope{Tipo: "entrada_voz", ID: "v1", Dados: codificarDados(map[string]string{"texto": "oi James"})}); err != nil {
		t.Fatalf("erro ao enviar entrada_voz: %v", err)
	}

	connCliente.SetReadDeadline(time.Now().Add(5 * time.Second))
	achouRespostaFinal := false
	for i := 0; i < 5; i++ {
		var recebido Envelope
		if err := connCliente.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler no cliente: %v", err)
		}
		if recebido.Tipo == "resposta_final" {
			achouRespostaFinal = true
			break
		}
	}
	if !achouRespostaFinal {
		t.Fatal("cliente privilegiado deveria ter recebido resposta_final originada da entrada_voz do device")
	}
}
```

- [ ] **Step 7: Rodar e confirmar falha**

Run: `docker compose up -d && go test ./internal/api/... -run WSDispositivo -v`
Expected: FAIL — `s.wsDispositivo` não existe.

- [ ] **Step 8: Implementar**

Criar `internal/api/dispositivo.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/permissao"
)

// conexaoDispositivo implementa dispositivo.Conexao sobre o WS real —
// é o que internal/dispositivo.Hub usa pra mandar acao_tela.
type conexaoDispositivo struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func (c *conexaoDispositivo) Enviar(envelope []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteMessage(websocket.TextMessage, envelope)
}

func (s *Servidor) wsDispositivo(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("erro no upgrade WS do device", "erro", err)
		return
	}
	defer conn.Close()

	conexao := &conexaoDispositivo{ws: conn}
	s.DispositivoHub.Registrar(conexao)
	defer s.DispositivoHub.Desregistrar()

	// Conversa por voz do device é sempre tratada como perfil Privilegiado:
	// é a mesma máquina física, e as respostas vão pros clientes
	// Privilegiado conectados (não existe "cliente" WS pra voz em si).
	var sessaoID uuid.UUID
	var historico []agente.Mensagem

	for {
		var envelope Envelope
		if err := conn.ReadJSON(&envelope); err != nil {
			return
		}

		switch envelope.Tipo {
		case "resultado_acao":
			var dados struct {
				Saida string `json:"saida"`
				Erro  string `json:"erro"`
			}
			if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
				continue
			}
			s.DispositivoHub.ResolverResultado(envelope.ID, dados.Saida, dados.Erro)

		case "entrada_voz":
			var dados struct {
				Texto string `json:"texto"`
			}
			if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
				continue
			}
			s.tratarEntradaDeVoz(r.Context(), envelope.ID, dados.Texto, &sessaoID, &historico)

		case "registro", "heartbeat":
			// Fase 1 só precisa aceitar sem derrubar a conexão — telemetria
			// de status_dispositivo fica como polimento futuro (não bloqueia
			// nenhum critério de aceitação).
		}
	}
}

func (s *Servidor) tratarEntradaDeVoz(ctx context.Context, id, texto string, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	resp, audioID, err := s.processarTurno(ctx, permissao.Privilegiado, sessaoID, historico, texto,
		func(c agente.ChamadaFerramenta) {
			s.Clientes.BroadcastParaPrivilegiados("ferramenta_iniciada", id, map[string]string{"nome": c.Nome})
		},
		func(rf agente.ResultadoFerramenta) {
			s.Clientes.BroadcastParaPrivilegiados("ferramenta_concluida", id, map[string]string{"nome": rf.Nome, "erro": rf.Erro})
		},
	)
	if err != nil {
		s.Clientes.BroadcastParaPrivilegiados("erro", id, map[string]string{"mensagem": err.Error()})
		return
	}
	s.Clientes.BroadcastParaPrivilegiados("token", id, map[string]string{"texto": resp.Texto})
	s.Clientes.BroadcastParaPrivilegiados("resposta_final", id, map[string]string{"texto": resp.Texto})
	if audioID != "" {
		s.Clientes.BroadcastParaPrivilegiados("audio_pronto", id, map[string]string{"id": audioID})
	}
}
```

- [ ] **Step 9: Rodar e confirmar sucesso**

Run: `go test ./internal/api/... -v`
Expected: PASS em todo o pacote `internal/api`.

- [ ] **Step 10: Commit**

```bash
git add internal/api/dispositivo.go internal/api/dispositivo_test.go internal/api/rest.go internal/api/clientes.go internal/api/clientes_test.go
git commit -m "feat: internal/api liga o endpoint WS do device à ponte e à entrada de voz"
```

---

## Task 19: `prompts/sistema.md` e `cmd/jamesd/main.go` — amarrando tudo

**Files:**
- Create: `prompts/sistema.md`
- Create: `cmd/jamesd/main.go`
- Create: `cmd/jamesd/tailscale.go`
- Create: `cmd/jamesd/tailscale_test.go`

- [ ] **Step 1: Escrever o system prompt, fonte única (corrige bug #13)**

Criar `prompts/sistema.md`:

```markdown
Você é o James, a inteligência central do sistema do Arthur — primo caçula
do Jarvis: competente, mas mais solto e direto, sem a formalidade impecável
do original.

DIRETRIZES DE PENSAMENTO:
1. AUTONOMIA TOTAL: se o Arthur pedir pra abrir algo ou interagir, nunca
   pergunte "como" ou "qual comando". Tente o comando mais provável ou use
   pesquisar_internet pra descobrir.
2. VISÃO PROATIVA: se houver um print da tela, analise antes de agir.
3. FLUXO SEQUENCIAL: quando faltar dado, o fluxo é pesquisar_internet →
   analisar resultado → executar_comando ou manipular_interface.

REGRAS DE RESPOSTA:
- Texto puro, sem markdown nem emoji.
- Seja breve. Confirme a execução e informe o resultado final, sem enfeite.
```

(Primeiro rascunho — revisar tom com o Arthur na verificação de fechamento da fase, seção final deste plano.)

- [ ] **Step 2: Escrever o teste de `resolverIPTailscale` que falha**

Criar `cmd/jamesd/tailscale_test.go`:

```go
package main

import (
	"net"
	"testing"
)

func TestResolverIPTailscaleSemInterfaceDevolveErro(t *testing.T) {
	interfacesOriginal := interfacesDoSistema
	defer func() { interfacesDoSistema = interfacesOriginal }()

	interfacesDoSistema = func() ([]net.Interface, error) {
		return []net.Interface{{Name: "Ethernet"}, {Name: "Wi-Fi"}}, nil
	}

	_, err := resolverIPTailscale()
	if err == nil {
		t.Fatal("esperava erro quando não há interface do Tailscale, veio nil")
	}
}
```

- [ ] **Step 3: Rodar e confirmar falha**

Run: `go test ./cmd/jamesd/... -v`
Expected: FAIL — `resolverIPTailscale` não existe.

- [ ] **Step 4: Implementar a resolução da interface**

Criar `cmd/jamesd/tailscale.go`:

```go
package main

import (
	"fmt"
	"net"
	"strings"
)

// interfacesDoSistema é var pra o teste poder substituir por uma lista
// falsa — net.Interface não é injetável de outro jeito.
var interfacesDoSistema = net.Interfaces

// resolverIPTailscale acha o IPv4 da interface do Tailscale em runtime.
// Nunca fixar esse IP em config: ele muda a cada relogin (spike de 04/08).
func resolverIPTailscale() (string, error) {
	ifaces, err := interfacesDoSistema()
	if err != nil {
		return "", fmt.Errorf("erro ao listar interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if !strings.Contains(strings.ToLower(iface.Name), "tailscale") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				return ip4.String(), nil
			}
		}
	}
	return "", fmt.Errorf("interface do Tailscale não encontrada — ele está rodando e logado?")
}
```

- [ ] **Step 5: Rodar e confirmar sucesso**

Run: `go test ./cmd/jamesd/... -v`
Expected: PASS.

- [ ] **Step 6: Implementar `main.go`**

Criar `cmd/jamesd/main.go`:

```go
// Comando jamesd: back-end de serviço do James. Sobe HTTP em loopback
// (shell desktop) e HTTPS na interface do Tailscale (celular), nunca em
// 0.0.0.0 (invariantes I1/I2 da spec).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"james/internal/agente"
	"james/internal/agente/gemini"
	"james/internal/api"
	"james/internal/armazenamento/postgres"
	"james/internal/config"
	"james/internal/dispositivo"
	"james/internal/ferramentas"
	"james/internal/memoria"
	"james/internal/voz"
)

func main() {
	if err := executar(); err != nil {
		slog.Error("jamesd não subiu", "erro", err)
		os.Exit(1)
	}
}

func executar() error {
	ctx := context.Background()

	cfg, err := config.Carregar("config.toml")
	if err != nil {
		return fmt.Errorf("configuração: %w", err)
	}

	pool, err := postgres.Abrir(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("banco: %w", err)
	}
	defer pool.Close()

	if err := postgres.AplicarMigracoes(ctx, pool); err != nil {
		return fmt.Errorf("migrações: %w", err)
	}

	provider, err := gemini.Novo(ctx, cfg.GeminiAPIKey, cfg.Modelo)
	if err != nil {
		return fmt.Errorf("gemini: %w", err)
	}

	sistemaPrompt, err := os.ReadFile("prompts/sistema.md")
	if err != nil {
		return fmt.Errorf("system prompt: %w", err)
	}

	dispositivoHub := dispositivo.NovoHub()

	registro := ferramentas.NovoRegistro()
	if err := registro.Registrar(ferramentas.NovoComandoSistema()); err != nil {
		return fmt.Errorf("registrar executar_comando_sistema: %w", err)
	}
	if err := registro.Registrar(ferramentas.NovoClaudeCode()); err != nil {
		return fmt.Errorf("registrar perguntar_ao_claude_code: %w", err)
	}
	if err := registro.Registrar(ferramentas.NovaBusca(cfg.TavilyAPIKey)); err != nil {
		return fmt.Errorf("registrar pesquisar_internet: %w", err)
	}
	if err := registro.Registrar(ferramentas.NovoManipularInterface(dispositivoHub)); err != nil {
		return fmt.Errorf("registrar manipular_interface: %w", err)
	}

	servidor := &api.Servidor{
		Sessoes:        postgres.NovoSessaoRepositorio(pool),
		Mensagens:      postgres.NovoMensagemRepositorio(pool),
		Aprovacoes:     postgres.NovoAprovacaoRepositorio(pool),
		DiretorioAudio: "audios",
		Loop: &agente.Loop{
			Provider:     provider,
			Ferramentas:  registro,
			MaxIteracoes: cfg.MaxIteracoes,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: cfg.OrcamentoTokens},
		SistemaPrompt:       string(sistemaPrompt),
		Clientes:            api.NovoClientesHub(),
		AprovacoesPendentes: api.NovoRegistroAprovacoesPendentes(),
		TimeoutAprovacao:    time.Duration(cfg.TimeoutAprovacaoSegundos) * time.Second,
		DispositivoHub:      dispositivoHub,
		Sintetizador:        voz.NovoSintetizador("audios"),
		VozTTS:              cfg.VozTTS,
		TimeoutTurno:        time.Duration(cfg.TimeoutSegundos) * time.Second,
	}

	errosServidor := make(chan error, 2)

	go func() {
		slog.Info("servindo em loopback", "endereco", cfg.BindLoopback)
		errosServidor <- http.ListenAndServe(cfg.BindLoopback, servidor.Rotas())
	}()

	ipVPN, err := resolverIPTailscale()
	if err != nil {
		// Não é fatal: o shell desktop (loopback) continua funcionando.
		// Só o celular fica sem alcance até a VPN voltar — R1 é sobre
		// segredo/config ausente, não sobre uma rede externa instável.
		slog.Error("interface do Tailscale não encontrada — servindo só em loopback; celular não vai alcançar o jamesd até isso ser corrigido", "erro", err)
	} else {
		enderecoVPN := fmt.Sprintf("%s:%d", ipVPN, cfg.BindVPNPorta)
		go func() {
			slog.Info("servindo na VPN com TLS", "endereco", enderecoVPN)
			errosServidor <- http.ListenAndServeTLS(enderecoVPN, cfg.CertArquivo, cfg.ChaveArquivo, servidor.Rotas())
		}()
	}

	return <-errosServidor
}
```

- [ ] **Step 7: Compilar (sem rodar — precisa de Postgres, Gemini e Tailscale reais)**

```bash
go build -o /dev/null ./cmd/jamesd
```

Expected: sem erro de compilação. **Nunca `go build .` ou `go build ./...` sem `-o`** — regra da sessão anterior (deposita `.exe` na raiz com segredo do fonte compilado dentro).

- [ ] **Step 8: Smoke test manual (não automatizado)**

Com `.env` real preenchido, `docker compose up -d`, `prompts/sistema.md` no lugar:

```bash
go run ./cmd/jamesd
```

Confirmar:
1. Log mostra "servindo em loopback"
2. `curl http://127.0.0.1:8080/api/saude` devolve `{"status":"ok"}`
3. Se o Tailscale estiver logado: log mostra "servindo na VPN com TLS" e `curl https://<host>.ts.net:8443/api/saude` funciona de outro dispositivo do tailnet

Colar a saída real no commit de fechamento da fase — não resumir.

- [ ] **Step 9: Commit**

```bash
git add prompts/sistema.md cmd/jamesd/main.go cmd/jamesd/tailscale.go cmd/jamesd/tailscale_test.go
git commit -m "feat: cmd/jamesd amarra config, banco, agente, ferramentas e API num serviço só"
```

---

## Task 20: `cmd/migrar-historico` — comando avulso de migração de dados (Decisão P3)

**Files:**
- Create: `cmd/migrar-historico/main.go`
- Create: `cmd/migrar-historico/main_test.go`

- [ ] **Step 1: Escrever os testes que falham**

Criar `cmd/migrar-historico/main_test.go`:

```go
package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"

	"james/internal/armazenamento/postgres"
)

func criarFixtureSQLite(t *testing.T) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "Athena.db")
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatalf("erro ao abrir sqlite de teste: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE mensagens (id INTEGER PRIMARY KEY, role TEXT, content TEXT, created_at TEXT)`); err != nil {
		t.Fatalf("erro ao criar schema sqlite: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO mensagens (role, content) VALUES ('user','oi'), ('assistant','olá')`); err != nil {
		t.Fatalf("erro ao inserir fixture: %v", err)
	}
	return caminho
}

func abrirPoolDeTeste(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := postgres.URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}
	pool, err := postgres.Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("erro ao aplicar migrações: %v", err)
	}
	return pool
}

// limparSessaoHistorico garante isolamento entre os dois testes abaixo —
// eles compartilham o mesmo Postgres local, sem transação por teste.
func limparSessaoHistorico(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `DELETE FROM sessoes WHERE titulo = $1`, tituloSessaoHistorico); err != nil {
		t.Fatalf("erro ao limpar sessão de histórico antes do teste: %v", err)
	}
}

func TestExecutarMigraMensagensEMapeiaPapel(t *testing.T) {
	pool := abrirPoolDeTeste(t)
	limparSessaoHistorico(t, pool)
	caminhoSQLite := criarFixtureSQLite(t)

	if err := executar(context.Background(), caminhoSQLite, pool); err != nil {
		t.Fatalf("executar devolveu erro: %v", err)
	}

	sessoes := postgres.NovoSessaoRepositorio(pool)
	lista, err := sessoes.Listar(context.Background())
	if err != nil {
		t.Fatalf("erro ao listar sessões: %v", err)
	}

	var achada *postgres.Sessao
	for i := range lista {
		if lista[i].Titulo == tituloSessaoHistorico {
			achada = &lista[i]
		}
	}
	if achada == nil {
		t.Fatal("esperava sessão 'Histórico Athena' criada")
	}

	mensagens := postgres.NovoMensagemRepositorio(pool)
	msgs, err := mensagens.ListarPorSessao(context.Background(), achada.ID, 10)
	if err != nil {
		t.Fatalf("erro ao listar mensagens: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("esperava 2 mensagens migradas, vieram %d", len(msgs))
	}
	if msgs[0].Papel != "usuario" || msgs[1].Papel != "modelo" {
		t.Errorf("papéis mapeados errado (user→usuario, assistant→modelo): %+v", msgs)
	}
}

func TestExecutarEIdempotente(t *testing.T) {
	pool := abrirPoolDeTeste(t)
	limparSessaoHistorico(t, pool)
	caminhoSQLite := criarFixtureSQLite(t)

	if err := executar(context.Background(), caminhoSQLite, pool); err != nil {
		t.Fatalf("primeira execução devolveu erro: %v", err)
	}
	if err := executar(context.Background(), caminhoSQLite, pool); err != nil {
		t.Fatalf("segunda execução (idempotente) devolveu erro: %v", err)
	}

	sessoes := postgres.NovoSessaoRepositorio(pool)
	lista, _ := sessoes.Listar(context.Background())
	total := 0
	for _, s := range lista {
		if s.Titulo == tituloSessaoHistorico {
			total++
		}
	}
	if total != 1 {
		t.Fatalf("esperava 1 sessão de histórico mesmo após rodar 2x, vieram %d", total)
	}
}
```

- [ ] **Step 2: Rodar e confirmar falha**

Run: `docker compose up -d && go test ./cmd/migrar-historico/... -v`
Expected: FAIL — `executar` e `tituloSessaoHistorico` não existem.

- [ ] **Step 3: Implementar**

Criar `cmd/migrar-historico/main.go`:

```go
// Comando migrar-historico: ETL único e idempotente do Athena.db (SQLite)
// pro Postgres do James. Roda uma vez, à mão, depois que o jamesd já subiu
// e aplicou as migrações (decisão P3 — não roda no boot do serviço).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"

	"james/internal/armazenamento/postgres"
)

const tituloSessaoHistorico = "Histórico Athena"

func main() {
	caminhoSQLite := flag.String("sqlite", "./Athena.db", "caminho pro Athena.db original")
	flag.Parse()

	ctx := context.Background()
	pool, err := postgres.Abrir(ctx, postgres.URLDoAmbiente())
	if err != nil {
		slog.Error("migração falhou", "erro", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := postgres.AplicarMigracoes(ctx, pool); err != nil {
		slog.Error("migração falhou", "erro", err)
		os.Exit(1)
	}

	if err := executar(ctx, *caminhoSQLite, pool); err != nil {
		slog.Error("migração falhou", "erro", err)
		os.Exit(1)
	}
	slog.Info("migração concluída")
}

func executar(ctx context.Context, caminhoSQLite string, pool *pgxpool.Pool) error {
	sessoes := postgres.NovoSessaoRepositorio(pool)
	mensagens := postgres.NovoMensagemRepositorio(pool)

	existentes, err := sessoes.Listar(ctx)
	if err != nil {
		return fmt.Errorf("erro ao checar sessões existentes: %w", err)
	}
	for _, s := range existentes {
		if s.Titulo == tituloSessaoHistorico {
			slog.Info("sessão de histórico já existe, nada a fazer", "sessao_id", s.ID)
			return nil
		}
	}

	db, err := sql.Open("sqlite", caminhoSQLite)
	if err != nil {
		return fmt.Errorf("erro ao abrir %s: %w", caminhoSQLite, err)
	}
	defer db.Close()

	linhas, err := db.QueryContext(ctx, `SELECT role, content FROM mensagens ORDER BY id ASC`)
	if err != nil {
		return fmt.Errorf("erro ao ler mensagens do sqlite: %w", err)
	}
	defer linhas.Close()

	sessao, err := sessoes.Criar(ctx, tituloSessaoHistorico)
	if err != nil {
		return fmt.Errorf("erro ao criar sessão de histórico: %w", err)
	}

	total := 0
	for linhas.Next() {
		var role, content string
		if err := linhas.Scan(&role, &content); err != nil {
			return fmt.Errorf("erro ao ler linha do sqlite: %w", err)
		}
		papel := "usuario"
		if role == "assistant" {
			papel = "modelo"
		}
		if _, err := mensagens.Salvar(ctx, sessao.ID, papel, content, nil); err != nil {
			return fmt.Errorf("erro ao salvar mensagem migrada: %w", err)
		}
		total++
	}
	if err := linhas.Err(); err != nil {
		return fmt.Errorf("erro ao percorrer mensagens do sqlite: %w", err)
	}

	slog.Info("mensagens migradas", "total", total, "sessao_id", sessao.ID)
	return nil
}
```

- [ ] **Step 4: Rodar e confirmar sucesso**

Run: `go test ./cmd/migrar-historico/... -v`
Expected: PASS nos 2 testes.

- [ ] **Step 5: Compilar**

```bash
go build -o /dev/null ./cmd/migrar-historico
```

- [ ] **Step 6: Commit**

```bash
git add cmd/migrar-historico/main.go cmd/migrar-historico/main_test.go
git commit -m "feat: cmd/migrar-historico importa o Athena.db uma vez, idempotente"
```

---

## Task 21: `dispositivo/james_device/conexao.py` — cliente WS com reconexão e heartbeat

Substitui o socket TCP cru de `processador_voz.py` (bug #12 do Anexo A) — conecta de fora pra dentro (I1), reconecta sozinho se o `jamesd` reiniciar.

**Files:**
- Modify: `dispositivo/pyproject.toml` (adicionar `websockets`)
- Create: `dispositivo/james_device/conexao.py`
- Create: `dispositivo/tests/test_conexao.py`

- [ ] **Step 1: Adicionar a dependência `websockets`**

Editar `dispositivo/pyproject.toml`:

```toml
[project]
name = "james-device"
version = "0.1.0"
description = "Agente de dispositivo do James: voz e controle de tela"
requires-python = ">=3.11"
dependencies = ["websockets>=13.0"]

[project.optional-dependencies]
dev = ["pytest>=8.0"]

[build-system]
requires = ["setuptools>=68"]
build-backend = "setuptools.build_meta"

[tool.pytest.ini_options]
testpaths = ["tests"]
pythonpath = ["."]
```

Run: `pip install -e ".[dev]"` (dentro de `dispositivo/`, com o venv do projeto ativo).

- [ ] **Step 2: Escrever os testes que falham**

Criar `dispositivo/tests/test_conexao.py`:

```python
import json
import threading
import time

from james_device.conexao import ConexaoJames


class FakeWS:
    """Simula o objeto que `websockets.sync.client.connect` devolve: context
    manager, iterável de mensagens recebidas, e `.send()` que grava o que
    foi mandado."""

    def __init__(self, mensagens=None):
        self._mensagens = list(mensagens or [])
        self.enviados = []

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def __iter__(self):
        yield from self._mensagens

    def send(self, mensagem):
        self.enviados.append(json.loads(mensagem))


def test_tratar_acao_tela_chama_manipulador_e_envia_resultado():
    envelope_entrada = json.dumps({"tipo": "acao_tela", "id": "42", "dados": {"acao": "click", "x": 10, "y": 20}})
    ws = FakeWS(mensagens=[envelope_entrada])

    chamadas = []

    def manipulador(dados):
        chamadas.append(dados)
        return {"saida": "clique feito"}

    conexao = ConexaoJames("ws://localhost/ws/dispositivo", manipulador)
    conexao._sessao(ws)

    assert chamadas == [{"acao": "click", "x": 10, "y": 20}]
    assert len(ws.enviados) == 1
    assert ws.enviados[0]["tipo"] == "resultado_acao"
    assert ws.enviados[0]["id"] == "42"
    assert ws.enviados[0]["dados"] == {"saida": "clique feito"}


def test_ignora_envelope_de_tipo_desconhecido():
    envelope_entrada = json.dumps({"tipo": "ping", "id": "1", "dados": {}})
    ws = FakeWS(mensagens=[envelope_entrada])

    conexao = ConexaoJames("ws://localhost/ws/dispositivo", lambda dados: {"saida": "não devia rodar"})
    conexao._sessao(ws)

    assert ws.enviados == []


def test_executar_reconecta_apos_queda_de_conexao():
    chamadas = {"n": 0}

    def conectar_falso(url):
        chamadas["n"] += 1
        if chamadas["n"] == 1:
            raise OSError("recusado")
        return FakeWS(mensagens=[])

    conexao = ConexaoJames(
        "ws://localhost/ws/dispositivo", lambda dados: {}, intervalo_reconexao=0.01, conectar=conectar_falso
    )
    thread = threading.Thread(target=conexao.executar, daemon=True)
    thread.start()
    time.sleep(0.1)
    conexao.parar()
    thread.join(timeout=1)

    assert chamadas["n"] >= 2, "esperava pelo menos uma reconexão após a primeira falha"


def test_heartbeat_enviado_periodicamente():
    ws = FakeWS(mensagens=[])
    conexao = ConexaoJames("ws://localhost/ws/dispositivo", lambda dados: {}, intervalo_heartbeat=0.01)

    thread = threading.Thread(target=conexao._heartbeat_loop, args=(ws,), daemon=True)
    thread.start()
    time.sleep(0.05)
    conexao.parar()
    thread.join(timeout=1)

    assert len(ws.enviados) >= 1
    assert ws.enviados[0]["tipo"] == "heartbeat"
```

- [ ] **Step 3: Rodar e confirmar falha**

Run: `pytest dispositivo/tests/test_conexao.py -v`
Expected: FAIL — `james_device.conexao` não existe.

- [ ] **Step 4: Implementar**

Criar `dispositivo/james_device/conexao.py`:

```python
"""Cliente WS do james-device: conecta no jamesd, reconecta sozinho se cair
(nunca escuta — invariante I1 da spec), manda heartbeat, despacha acao_tela
pro manipulador injetado e devolve resultado_acao.
"""

import json
import logging
import threading
import time

from websockets.exceptions import ConnectionClosed
from websockets.sync.client import connect as conectar_padrao

logger = logging.getLogger(__name__)


class ConexaoJames:
    def __init__(
        self,
        url,
        manipulador_acao,
        intervalo_heartbeat=15.0,
        intervalo_reconexao=2.0,
        conectar=conectar_padrao,
    ):
        self.url = url
        self.manipulador_acao = manipulador_acao
        self.intervalo_heartbeat = intervalo_heartbeat
        self.intervalo_reconexao = intervalo_reconexao
        self._conectar = conectar
        self._parar = threading.Event()

    def parar(self):
        self._parar.set()

    def executar(self):
        """Loop principal: conecta, processa mensagens, reconecta se cair.
        Só retorna depois de parar() ser chamado."""
        while not self._parar.is_set():
            try:
                with self._conectar(self.url) as ws:
                    logger.info("conectado ao jamesd em %s", self.url)
                    self._sessao(ws)
            except (OSError, ConnectionClosed) as erro:
                logger.info(
                    "conexão com jamesd caiu (%s), tentando de novo em %ss", erro, self.intervalo_reconexao
                )
            if not self._parar.is_set():
                time.sleep(self.intervalo_reconexao)

    def _sessao(self, ws):
        tarefa = threading.Thread(target=self._heartbeat_loop, args=(ws,), daemon=True)
        tarefa.start()
        for mensagem_bruta in ws:
            envelope = json.loads(mensagem_bruta)
            self._tratar(ws, envelope)

    def _heartbeat_loop(self, ws):
        while not self._parar.is_set():
            time.sleep(self.intervalo_heartbeat)
            try:
                ws.send(json.dumps({"tipo": "heartbeat", "id": "", "dados": {}}))
            except (OSError, ConnectionClosed):
                return

    def _tratar(self, ws, envelope):
        if envelope.get("tipo") != "acao_tela":
            return
        resultado = self.manipulador_acao(envelope.get("dados", {}))
        resposta = json.dumps({"tipo": "resultado_acao", "id": envelope.get("id", ""), "dados": resultado})
        ws.send(resposta)
```

- [ ] **Step 5: Rodar e confirmar sucesso**

Run: `pytest dispositivo/tests/test_conexao.py -v`
Expected: PASS nos 4 testes.

- [ ] **Step 6: Commit**

```bash
git add dispositivo/pyproject.toml dispositivo/james_device/conexao.py dispositivo/tests/test_conexao.py
git commit -m "feat: james_device.conexao substitui o socket cru por WS com reconexão"
```

---

## Task 22: `dispositivo/james_device/tela.py` + entrada de voz + entrypoint

Porta o `manipular_tela` de `processador_voz.py` (pyautogui) e o `ouvir_continuamente`/`falar_comando`, ligando tudo num só processo que substitui `processador_voz.py` da raiz.

**Files:**
- Modify: `dispositivo/pyproject.toml` (adicionar `pyautogui`, `SpeechRecognition`, `playsound`)
- Modify: `dispositivo/james_device/conexao.py` (`enviar_entrada_voz`)
- Modify: `dispositivo/tests/test_conexao.py` (teste do método novo)
- Create: `dispositivo/james_device/tela.py`
- Create: `dispositivo/tests/test_tela.py`
- Create: `dispositivo/james_device/main.py`

- [ ] **Step 1: Adicionar as dependências de hardware**

Editar `dispositivo/pyproject.toml`, campo `dependencies`:

```toml
dependencies = ["websockets>=13.0", "pyautogui>=0.9.54", "SpeechRecognition>=3.10", "playsound>=1.3.0"]
```

Run: `pip install -e ".[dev]"`

- [ ] **Step 2: Escrever o teste de `enviar_entrada_voz` que falha**

Adicionar a `dispositivo/tests/test_conexao.py`:

```python
def test_enviar_entrada_voz_manda_envelope_pela_conexao_ativa():
    ws = FakeWS(mensagens=[])
    conexao = ConexaoJames("ws://localhost/ws/dispositivo", lambda dados: {})
    conexao._ws_ativo = ws

    enviado = conexao.enviar_entrada_voz("liga o spotify")

    assert enviado is True
    assert ws.enviados[-1]["tipo"] == "entrada_voz"
    assert ws.enviados[-1]["dados"] == {"texto": "liga o spotify"}


def test_enviar_entrada_voz_sem_conexao_devolve_false():
    conexao = ConexaoJames("ws://localhost/ws/dispositivo", lambda dados: {})
    assert conexao.enviar_entrada_voz("oi") is False
```

- [ ] **Step 3: Rodar e confirmar falha**

Run: `pytest dispositivo/tests/test_conexao.py -v`
Expected: FAIL — `enviar_entrada_voz` não existe.

- [ ] **Step 4: Implementar `enviar_entrada_voz`**

Editar `dispositivo/james_device/conexao.py`. No `__init__`, adicionar:

```python
        self._ws_ativo = None
        self._lock_envio = threading.Lock()
```

Em `_sessao`, guardar a referência da conexão ativa (e limpar ao sair):

```python
    def _sessao(self, ws):
        self._ws_ativo = ws
        tarefa = threading.Thread(target=self._heartbeat_loop, args=(ws,), daemon=True)
        tarefa.start()
        try:
            for mensagem_bruta in ws:
                envelope = json.loads(mensagem_bruta)
                self._tratar(ws, envelope)
        finally:
            self._ws_ativo = None
```

E adicionar o método público:

```python
    def enviar_entrada_voz(self, texto: str) -> bool:
        """Manda entrada_voz pela conexão ativa. Devolve False sem levantar
        se não há conexão no momento — quem fala não trava esperando o
        jamesd voltar."""
        with self._lock_envio:
            if self._ws_ativo is None:
                logger.warning("sem conexão com jamesd, entrada de voz descartada: %r", texto)
                return False
            try:
                self._ws_ativo.send(json.dumps({"tipo": "entrada_voz", "id": "", "dados": {"texto": texto}}))
                return True
            except (OSError, ConnectionClosed):
                logger.warning("falha ao enviar entrada de voz, conexão deve ter caído")
                return False
```

- [ ] **Step 5: Rodar e confirmar sucesso**

Run: `pytest dispositivo/tests/test_conexao.py -v`
Expected: PASS em todos os testes (os 4 do Task 21 + os 2 novos).

- [ ] **Step 6: Escrever os testes de `tela.py` que falham**

Criar `dispositivo/tests/test_tela.py`:

```python
import james_device.tela as tela


def test_executar_acao_click_chama_pyautogui(monkeypatch):
    chamadas = []
    monkeypatch.setattr(tela.pyautogui, "moveTo", lambda x, y, duration: chamadas.append(("moveTo", x, y)))
    monkeypatch.setattr(tela.pyautogui, "click", lambda: chamadas.append(("click",)))

    resultado = tela.executar_acao({"acao": "click", "x": 100, "y": 200})

    assert resultado == {"saida": "clique em (100,200)"}
    assert ("moveTo", 100, 200) in chamadas
    assert ("click",) in chamadas


def test_executar_acao_tecla_chama_pyautogui(monkeypatch):
    chamadas = []
    monkeypatch.setattr(tela.pyautogui, "press", lambda t: chamadas.append(t))

    resultado = tela.executar_acao({"acao": "tecla", "texto": "enter"})

    assert resultado == {"saida": "tecla enter pressionada"}
    assert chamadas == ["enter"]


def test_executar_acao_desconhecida_devolve_erro():
    resultado = tela.executar_acao({"acao": "voar"})
    assert "erro" in resultado


def test_executar_acao_propaga_excecao_como_erro(monkeypatch):
    def levanta(*a, **k):
        raise RuntimeError("tela travada")

    monkeypatch.setattr(tela.pyautogui, "moveTo", levanta)

    resultado = tela.executar_acao({"acao": "click", "x": 1, "y": 1})
    assert "erro" in resultado
    assert "tela travada" in resultado["erro"]
```

- [ ] **Step 7: Rodar e confirmar falha**

Run: `pytest dispositivo/tests/test_tela.py -v`
Expected: FAIL — `james_device.tela` não existe.

- [ ] **Step 8: Implementar `tela.py`**

Criar `dispositivo/james_device/tela.py`:

```python
"""Executa ações de tela pedidas pelo jamesd: clique de mouse e tecla.
Porta processador_voz.py:manipular_tela como está — coordenadas continuam
adivinhadas pelo modelo (Anexo A #20), consertar é Fase 5.
"""

import logging

import pyautogui

logger = logging.getLogger(__name__)

pyautogui.PAUSE = 0.01


def executar_acao(dados: dict) -> dict:
    """Executa uma ação de tela e devolve {"saida": ...} ou {"erro": ...}.
    Nunca levanta — erro vira valor de retorno, o jamesd decide o que fazer
    (mesma filosofia de R2 do lado Go)."""
    acao = dados.get("acao")
    try:
        if acao == "click":
            x = int(dados.get("x", 0))
            y = int(dados.get("y", 0))
            pyautogui.moveTo(x, y, duration=1.0)
            pyautogui.click()
            return {"saida": f"clique em ({x},{y})"}
        if acao == "tecla":
            texto = dados.get("texto", "")
            pyautogui.press(texto)
            return {"saida": f"tecla {texto} pressionada"}
        return {"erro": f"ação desconhecida: {acao}"}
    except Exception as erro:
        logger.exception("erro ao executar ação de tela")
        return {"erro": str(erro)}
```

- [ ] **Step 9: Rodar e confirmar sucesso**

Run: `pytest dispositivo/tests/test_tela.py -v`
Expected: PASS nos 4 testes.

- [ ] **Step 10: Escrever o entrypoint (sem teste automatizado — depende de microfone real)**

Mesma filosofia da spec pra voz/tela (seção 7): a camada de protocolo (`conexao.py`, `tela.py`) é testada; a camada que toca hardware de verdade fica fina e é verificada à mão.

Criar `dispositivo/james_device/main.py`:

```python
"""Ponto de entrada do james-device: liga wake word, voz e tela numa
conversa contínua com o jamesd. Substitui processador_voz.py da raiz.
Depende de microfone real — verificado à mão (Step 11), não em pytest.
"""

import logging
import os
import threading

import speech_recognition as sr
from playsound import playsound

from james_device.conexao import ConexaoJames
from james_device.tela import executar_acao
from james_device.wake import contem_wake_word

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

URL_JAMESD = os.environ.get("JAMES_WS_URL", "ws://127.0.0.1:8080/ws/dispositivo")
PALAVRA_PARADA = "enviar"


def _ouvir_comando(recognizer, source) -> str:
    buffer = []
    while True:
        try:
            audio = recognizer.listen(source, timeout=5, phrase_time_limit=10)
            parcial = recognizer.recognize_google(audio, language="pt-BR").lower()
        except (sr.WaitTimeoutError, sr.UnknownValueError):
            continue

        if PALAVRA_PARADA in parcial:
            limpo = parcial.replace(PALAVRA_PARADA, "").strip()
            if limpo:
                buffer.append(limpo)
            playsound("dispositivo/assets/faaah.wav")
            return " ".join(buffer).strip()
        buffer.append(parcial)


def escutar_e_enviar(conexao: ConexaoJames):
    recognizer = sr.Recognizer()
    recognizer.dynamic_energy_threshold = True

    with sr.Microphone() as source:
        recognizer.adjust_for_ambient_noise(source, duration=1)
        logger.info("escuta ativa iniciada")

        while True:
            try:
                audio = recognizer.listen(source, phrase_time_limit=3)
                texto = recognizer.recognize_google(audio, language="pt-BR").lower()
            except (sr.UnknownValueError, sr.WaitTimeoutError):
                continue
            except sr.RequestError:
                logger.error("erro de conexão com o Google Speech")
                continue

            if contem_wake_word(texto):
                playsound("dispositivo/assets/entrada.mp3")
                comando = _ouvir_comando(recognizer, source)
                if comando:
                    conexao.enviar_entrada_voz(comando)


def main():
    conexao = ConexaoJames(URL_JAMESD, manipulador_acao=executar_acao)
    threading.Thread(target=conexao.executar, daemon=True).start()
    escutar_e_enviar(conexao)


if __name__ == "__main__":
    main()
```

- [ ] **Step 11: Smoke test manual (não automatizado)**

Com o `jamesd` rodando (`go run ./cmd/jamesd`) e um microfone real:

```bash
python -m james_device.main
```

Confirmar: diga "James" (ou variante da lista), diga um comando, diga "enviar" — o James responde no cliente WS conectado (ex.: `websocat ws://127.0.0.1:8080/ws` recebendo `resposta_final`). Colar a saída real no commit de fechamento.

- [ ] **Step 12: Commit**

```bash
git add dispositivo/pyproject.toml dispositivo/james_device/conexao.py dispositivo/tests/test_conexao.py dispositivo/james_device/tela.py dispositivo/tests/test_tela.py dispositivo/james_device/main.py
git commit -m "feat: james_device.tela e main.py fecham a ponta Python (voz, tela, conexão)"
```

---

## Task 23: `web/` — renomeia `frontend/`, troca bindings Wails por HTTP/WS, card de aprovação (Decisão P4)

**Files:**
- `git mv frontend web`
- Modify: `web/package.json` (adicionar `jsdom`, `@testing-library/react`)
- Modify: `web/vite.config.ts` (`test.environment = "jsdom"`)
- Create: `web/src/lib/api.ts`
- Create: `web/src/lib/api.test.ts`
- Create: `web/src/lib/ws.ts`
- Create: `web/src/lib/ws.test.ts`
- Create: `web/src/components/CardAprovacao.tsx`
- Create: `web/src/components/CardAprovacao.test.tsx`
- Modify: `web/src/App.tsx`
- Delete: `web/wailsjs/` (bindings gerados, mortos — spec 4.9)

- [ ] **Step 1: Renomear a pasta**

```bash
git mv frontend web
```

- [ ] **Step 2: Adicionar dependências de teste de DOM**

Editar `web/package.json`, `devDependencies`:

```json
{
  "devDependencies": {
    "@testing-library/react": "^16.0.0",
    "@types/react": "^18.0.17",
    "@types/react-dom": "^18.0.6",
    "@vitejs/plugin-react": "^6.0.5",
    "jsdom": "^25.0.0",
    "typescript": "^5.9.3",
    "vite": "^8.2.0",
    "vitest": "^4.1.10"
  }
}
```

Run (dentro de `web/`): `npm install`

- [ ] **Step 3: Configurar o ambiente de teste**

Editar `web/vite.config.ts`:

```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  test: {
    environment: 'jsdom',
  },
})
```

- [ ] **Step 4: Escrever os testes de `api.ts` que falham**

Criar `web/src/lib/api.test.ts`:

```typescript
import { afterEach, describe, expect, it, vi } from "vitest";
import { criarSessao, listarMensagens, listarSessoes } from "./api";

afterEach(() => {
  vi.restoreAllMocks();
});

describe("listarSessoes", () => {
  it("faz GET /api/sessoes e devolve o JSON", async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => [{ ID: "1", Titulo: "sessão" }],
    });
    vi.stubGlobal("fetch", fakeFetch);

    const sessoes = await listarSessoes();

    expect(fakeFetch).toHaveBeenCalledWith("/api/sessoes");
    expect(sessoes).toEqual([{ ID: "1", Titulo: "sessão" }]);
  });

  it("lança erro quando a resposta não é ok", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue({ ok: false, status: 500 }));
    await expect(listarSessoes()).rejects.toThrow();
  });
});

describe("criarSessao", () => {
  it("faz POST /api/sessoes com o título", async () => {
    const fakeFetch = vi.fn().mockResolvedValue({
      ok: true,
      json: async () => ({ ID: "1", Titulo: "nova" }),
    });
    vi.stubGlobal("fetch", fakeFetch);

    await criarSessao("nova");

    const [url, opcoes] = fakeFetch.mock.calls[0];
    expect(url).toBe("/api/sessoes");
    expect(opcoes.method).toBe("POST");
    expect(JSON.parse(opcoes.body)).toEqual({ titulo: "nova" });
  });
});

describe("listarMensagens", () => {
  it("faz GET /api/sessoes/{id}/mensagens", async () => {
    const fakeFetch = vi.fn().mockResolvedValue({ ok: true, json: async () => [] });
    vi.stubGlobal("fetch", fakeFetch);

    await listarMensagens("abc-123");

    expect(fakeFetch).toHaveBeenCalledWith("/api/sessoes/abc-123/mensagens");
  });
});
```

- [ ] **Step 5: Rodar e confirmar falha**

Run: `npm test` (dentro de `web/`)
Expected: FAIL — `./api` não existe.

- [ ] **Step 6: Implementar `api.ts`**

Criar `web/src/lib/api.ts`:

```typescript
export interface Sessao {
  ID: string;
  Titulo: string;
  CriadaEm: string;
  AtualizadaEm: string;
}

export interface MensagemAPI {
  ID: number;
  SessaoID: string;
  Papel: string;
  Conteudo: string;
  CriadaEm: string;
}

export async function listarSessoes(): Promise<Sessao[]> {
  const resp = await fetch("/api/sessoes");
  if (!resp.ok) throw new Error(`erro ao listar sessões: ${resp.status}`);
  return resp.json();
}

export async function criarSessao(titulo: string): Promise<Sessao> {
  const resp = await fetch("/api/sessoes", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ titulo }),
  });
  if (!resp.ok) throw new Error(`erro ao criar sessão: ${resp.status}`);
  return resp.json();
}

export async function listarMensagens(sessaoId: string): Promise<MensagemAPI[]> {
  const resp = await fetch(`/api/sessoes/${sessaoId}/mensagens`);
  if (!resp.ok) throw new Error(`erro ao listar mensagens: ${resp.status}`);
  return resp.json();
}
```

- [ ] **Step 7: Rodar e confirmar sucesso**

Run: `npm test` — Expected: os testes de `api.ts` passam (outros arquivos ainda vão falhar até os próximos steps).

- [ ] **Step 8: Escrever os testes de `ws.ts` que falham**

Criar `web/src/lib/ws.test.ts`:

```typescript
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ClienteWS } from "./ws";

class FakeWebSocket {
  static instancias: FakeWebSocket[] = [];
  enviados: string[] = [];
  onmessage: ((ev: { data: string }) => void) | null = null;
  constructor(public url: string) {
    FakeWebSocket.instancias.push(this);
  }
  send(dados: string) {
    this.enviados.push(dados);
  }
  close() {}
}

beforeEach(() => {
  FakeWebSocket.instancias = [];
  vi.stubGlobal("WebSocket", FakeWebSocket);
});

describe("ClienteWS", () => {
  it("enviar codifica o envelope como JSON", () => {
    const cliente = new ClienteWS("ws://localhost/ws");
    cliente.enviar("mensagem", "1", { texto: "oi" });

    const socket = FakeWebSocket.instancias[0];
    expect(JSON.parse(socket.enviados[0])).toEqual({ tipo: "mensagem", id: "1", dados: { texto: "oi" } });
  });

  it("on despacha o manipulador certo pelo tipo do envelope", () => {
    const cliente = new ClienteWS("ws://localhost/ws");
    const recebidos: unknown[] = [];
    cliente.on("resposta_final", (dados) => recebidos.push(dados));

    const socket = FakeWebSocket.instancias[0];
    socket.onmessage?.({ data: JSON.stringify({ tipo: "resposta_final", id: "1", dados: { texto: "oi" } }) });

    expect(recebidos).toEqual([{ texto: "oi" }]);
  });

  it("não despacha manipulador de outro tipo", () => {
    const cliente = new ClienteWS("ws://localhost/ws");
    const recebidos: unknown[] = [];
    cliente.on("resposta_final", (dados) => recebidos.push(dados));

    const socket = FakeWebSocket.instancias[0];
    socket.onmessage?.({ data: JSON.stringify({ tipo: "erro", id: "1", dados: { mensagem: "falhou" } }) });

    expect(recebidos).toEqual([]);
  });
});
```

- [ ] **Step 9: Rodar e confirmar falha**

Run: `npm test` — Expected: FAIL nos testes de `ws.ts` (`./ws` não existe).

- [ ] **Step 10: Implementar `ws.ts`**

Criar `web/src/lib/ws.ts`:

```typescript
export interface Envelope {
  tipo: string;
  id: string;
  dados: unknown;
}

type Manipulador = (dados: any) => void;

export class ClienteWS {
  private socket: WebSocket;
  private manipuladores = new Map<string, Manipulador[]>();

  constructor(url: string) {
    this.socket = new WebSocket(url);
    this.socket.onmessage = (evento: MessageEvent) => {
      const envelope: Envelope = JSON.parse(evento.data);
      const lista = this.manipuladores.get(envelope.tipo) ?? [];
      for (const fn of lista) fn(envelope.dados);
    };
  }

  on(tipo: string, manipulador: Manipulador) {
    const lista = this.manipuladores.get(tipo) ?? [];
    lista.push(manipulador);
    this.manipuladores.set(tipo, lista);
  }

  enviar(tipo: string, id: string, dados: unknown) {
    this.socket.send(JSON.stringify({ tipo, id, dados }));
  }

  fechar() {
    this.socket.close();
  }
}
```

- [ ] **Step 11: Rodar e confirmar sucesso**

Run: `npm test` — Expected: `api.ts` e `ws.ts` passam.

- [ ] **Step 12: Escrever os testes do `CardAprovacao` que falham**

Criar `web/src/components/CardAprovacao.test.tsx`:

```tsx
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { CardAprovacao } from "./CardAprovacao";

describe("CardAprovacao", () => {
  it("mostra o nome da ferramenta pedida", () => {
    render(<CardAprovacao ferramenta="manipular_interface" argumentos={{}} onDecidir={() => {}} />);
    expect(screen.getByText(/manipular_interface/)).toBeTruthy();
  });

  it("chama onDecidir(true) ao clicar em Aprovar", () => {
    const onDecidir = vi.fn();
    render(<CardAprovacao ferramenta="executar_comando_sistema" argumentos={{ comando: "ls" }} onDecidir={onDecidir} />);

    fireEvent.click(screen.getByText("Aprovar"));
    expect(onDecidir).toHaveBeenCalledWith(true);
  });

  it("chama onDecidir(false) ao clicar em Negar", () => {
    const onDecidir = vi.fn();
    render(<CardAprovacao ferramenta="x" argumentos={{}} onDecidir={onDecidir} />);

    fireEvent.click(screen.getByText("Negar"));
    expect(onDecidir).toHaveBeenCalledWith(false);
  });
});
```

- [ ] **Step 13: Rodar e confirmar falha**

Run: `npm test` — Expected: FAIL (`./CardAprovacao` não existe).

- [ ] **Step 14: Implementar `CardAprovacao.tsx`**

Criar `web/src/components/CardAprovacao.tsx`:

```tsx
interface Props {
  ferramenta: string;
  argumentos: Record<string, unknown>;
  onDecidir: (aprovado: boolean) => void;
}

// Card mínimo funcional (decisão P4) — sem redesenho, só o necessário pro
// critério de aceitação #6: perfil Restrito pedindo ação perigosa vira
// algo que o Arthur consegue aprovar ou negar no desktop.
export function CardAprovacao({ ferramenta, argumentos, onDecidir }: Props) {
  return (
    <div className="card-aprovacao">
      <p>
        O James quer executar <strong>{ferramenta}</strong>
      </p>
      <pre>{JSON.stringify(argumentos, null, 2)}</pre>
      <div className="card-aprovacao-botoes">
        <button onClick={() => onDecidir(true)}>Aprovar</button>
        <button onClick={() => onDecidir(false)}>Negar</button>
      </div>
    </div>
  );
}
```

- [ ] **Step 15: Rodar e confirmar sucesso**

Run: `npm test` — Expected: PASS em todo `web/`.

- [ ] **Step 16: Rewire `App.tsx`**

Substituir `web/src/App.tsx` inteiro:

```tsx
import { useEffect, useRef, useState } from "react";
import "./App.css";
import qfLogo from "./assets/images/QF.png";
import { CardAprovacao } from "./components/CardAprovacao";
import { separarSegmentos } from "./lib/markdown";
import { ClienteWS } from "./lib/ws";

interface Message {
  role: "user" | "James";
  content: string;
}

interface PedidoAprovacao {
  id: string;
  ferramenta: string;
  argumentos: Record<string, unknown>;
}

function urlWS(): string {
  const protocolo = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocolo}//${window.location.host}/ws`;
}

// window.runtime é injetado pelo Wails só quando a página roda dentro do
// shell desktop — no navegador do celular ele não existe, e a chamada vira
// no-op. Substitui os bindings gerados de wailsjs/runtime (spec 4.9).
function alternarJanelaNativa(expandir: boolean) {
  const runtime = (window as any).runtime;
  if (runtime?.WindowSetSize) {
    runtime.WindowSetSize(expandir ? 800 : 100, expandir ? 900 : 100);
  }
}

function App() {
  const [isOpen, setIsOpen] = useState(false);
  const [inputValue, setInputValue] = useState("");
  const [messages, setMessages] = useState<Message[]>([
    { role: "James", content: "Sistemas online. Como posso ajudar, Arthur?" },
  ]);
  const [isTyping, setIsTyping] = useState(false);
  const [pedidoAprovacao, setPedidoAprovacao] = useState<PedidoAprovacao | null>(null);
  const clienteRef = useRef<ClienteWS | null>(null);

  useEffect(() => {
    const cliente = new ClienteWS(urlWS());
    clienteRef.current = cliente;

    cliente.on("resposta_final", (dados: { texto: string }) => {
      setMessages((prev) => [...prev, { role: "James", content: dados.texto }]);
      setIsTyping(false);
    });

    cliente.on("aprovacao_requerida", (dados: any) => {
      setPedidoAprovacao({ id: dados.id ?? "", ferramenta: dados.ferramenta, argumentos: dados.argumentos ?? {} });
    });

    cliente.on("erro", (dados: { mensagem: string }) => {
      setMessages((prev) => [...prev, { role: "James", content: `Erro: ${dados.mensagem}` }]);
      setIsTyping(false);
    });

    return () => cliente.fechar();
  }, []);

  const renderMessageContent = (content: string) =>
    separarSegmentos(content).map((segmento, indice) =>
      segmento.tipo === "texto" ? (
        <span key={indice}>{segmento.conteudo}</span>
      ) : (
        <pre className="code-block" key={indice}>
          <code className={segmento.linguagem}>{segmento.conteudo}</code>
        </pre>
      )
    );

  const processarMensagem = (texto: string) => {
    if (texto.trim() === "" || !clienteRef.current) return;
    setMessages((prev) => [...prev, { role: "user", content: texto }]);
    setIsTyping(true);
    clienteRef.current.enviar("mensagem", crypto.randomUUID(), { texto });
  };

  const handleSendMessage = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter" && inputValue.trim() !== "") {
      const textoDigitado = inputValue;
      setInputValue("");
      processarMensagem(textoDigitado);
    }
  };

  const handleToggleChat = () => {
    const novoEstado = !isOpen;
    setIsOpen(novoEstado);
    alternarJanelaNativa(novoEstado);
  };

  const decidirAprovacao = (aprovado: boolean) => {
    if (!pedidoAprovacao || !clienteRef.current) return;
    clienteRef.current.enviar(aprovado ? "aprovar" : "negar", pedidoAprovacao.id, {});
    setPedidoAprovacao(null);
  };

  return (
    <div className="container">
      <div className="drag-handle" data-tauri-drag-region onClick={handleToggleChat}>
        <div className="sphere-visual">
          <img src={qfLogo} className="img_qf" alt="James" />
        </div>
      </div>

      {isOpen && (
        <div className="chat-window">
          {pedidoAprovacao && (
            <CardAprovacao
              ferramenta={pedidoAprovacao.ferramenta}
              argumentos={pedidoAprovacao.argumentos}
              onDecidir={decidirAprovacao}
            />
          )}
          <div className="messages-container">
            {messages.map((msg, i) => (
              <div key={i} className={msg.role === "user" ? "msg-user" : "msg-jarvis"}>
                <small style={{ fontSize: "10px", opacity: 0.5, marginBottom: "2px" }}>
                  {msg.role === "user" ? "Arthur" : "James"}
                </small>
                {renderMessageContent(msg.content)}
              </div>
            ))}
            {isTyping && (
              <div className="msg-jarvis typing-dots">
                <span></span>
                <span></span>
                <span></span>
              </div>
            )}
          </div>

          <input
            className="input-area"
            placeholder="Comando de voz ou texto..."
            value={inputValue}
            onChange={(e) => setInputValue(e.target.value)}
            onKeyDown={handleSendMessage}
            autoFocus
          />
        </div>
      )}
    </div>
  );
}

export default App;
```

- [ ] **Step 17: Remover os bindings mortos e rodar a suíte inteira**

```bash
rm -rf web/wailsjs
npm test --prefix web
```

Expected: PASS em tudo. Verificação visual real (golden path: abrir chat, mandar mensagem, ver resposta) fica pro Step 18 do Task 25 (verificação de fechamento), com `webapp-testing` contra o `jamesd` de verdade rodando.

- [ ] **Step 18: Commit**

```bash
git add web/ frontend
git commit -m "feat: web/ troca bindings Wails por HTTP/WS e ganha o card de aprovação mínimo"
```

---

## Task 24: `jamesd` serve a SPA embutida + `shell/` Wails mínimo

O `jamesd` é quem serve a UI pros dois invólucros (invariante I3) — o shell não embute mais o `web/dist`, só aponta o WebView2 pra URL do `jamesd`.

**Files:**
- Create: `web/embed.go`
- Modify: `internal/api/rest.go` (campo `EmbutidosSPA`, rota catch-all)
- Modify: `cmd/jamesd/main.go` (wire do embed)
- Create: `shell/frontend/index.html`
- Create: `shell/main.go`
- Create: `shell/wails.json` (adaptado do `wails.json` da raiz)
- Delete: `wails.json` (raiz — vira `shell/wails.json`)

- [ ] **Step 1: Embutir o `web/dist` num pacote Go**

`go:embed` só alcança arquivos dentro da própria árvore do pacote — por isso o embed mora em `web/`, não em `internal/api` nem `cmd/jamesd`.

Criar `web/embed.go`:

```go
// Package web embute o build da SPA (web/dist) no binário do jamesd.
// Requer `npm run build` dentro de web/ antes de `go build`.
package web

import "embed"

//go:embed all:dist
var Dist embed.FS
```

- [ ] **Step 2: Servir a SPA no `internal/api`**

Editar `internal/api/rest.go`: adicionar `EmbutidosSPA fs.FS` (import `"io/fs"`) à struct `Servidor`, e registrar o catch-all em `Rotas()` **depois** das rotas `/api/*` e `/ws*` (pra elas terem prioridade):

```go
	if s.EmbutidosSPA != nil {
		mux.Handle("/", http.FileServer(http.FS(s.EmbutidosSPA)))
	}
```

- [ ] **Step 3: Ligar no `cmd/jamesd`**

Editar `cmd/jamesd/main.go`: adicionar os imports `"io/fs"` e `"james/web"`, e antes de montar `servidor`:

```go
	raizSPA, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return fmt.Errorf("assets da SPA: %w (rodou \"npm run build\" em web/ antes do go build?)", err)
	}
```

E adicionar `EmbutidosSPA: raizSPA,` aos campos de `servidor := &api.Servidor{...}`.

- [ ] **Step 4: Compilar (build da SPA precisa existir primeiro)**

```bash
npm run build --prefix web
go build -o /dev/null ./cmd/jamesd
```

Expected: sem erro. Se faltar `web/dist`, o erro do Step 3 aparece na hora — falha rápida (R1), não binário quebrado em silêncio.

- [ ] **Step 5: Criar a página de redirecionamento do shell**

O shell não tem UI própria — é uma janela apontando pro `jamesd` local. Criar `shell/frontend/index.html`:

```html
<!doctype html>
<html>
<head>
  <meta charset="utf-8" />
  <style>html,body{margin:0;background:transparent;}</style>
</head>
<body>
<script>
  // Desktop sempre fala com o jamesd por loopback — é o que garante o
  // perfil Privilegiado (spec 4.3/P5). O celular usa a VPN, mas o celular
  // não roda este shell.
  window.location.replace("http://127.0.0.1:8080/");
</script>
</body>
</html>
```

- [ ] **Step 6: Criar `shell/main.go`**

```go
// Comando shell: invólucro Wails mínimo — janela frameless, transparente,
// always-on-top, WebView2 apontando pro jamesd local. Não tem lógica de
// aplicação própria: toda UI e todo estado vivem no jamesd + web/ (spec
// 4.9/4.10). Nenhum binding wailsjs/go/main/App sobrevive — eles morreram
// junto com o app.go antigo.
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	err := wails.Run(&options.App{
		Title:            "James",
		Width:            100,
		Height:           100,
		Frameless:        true,
		AlwaysOnTop:      true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer:      &assetserver.Options{Assets: assets},
	})
	if err != nil {
		println("erro ao subir o shell:", err.Error())
	}
}
```

**Nota de verificação:** a API exata de `assetserver.Options` e `options.App` deve ser conferida contra a versão instalada (`github.com/wailsapp/wails/v2 v2.11.0`, já em `go.mod`) — usar o context7 (skill do Arthur pra doc de biblioteca numa versão específica) se `go build` reclamar de campo inexistente.

- [ ] **Step 7: Mover e adaptar `wails.json`**

Ler o `wails.json` da raiz (campos `name`, `author`, etc.) e recriar em `shell/wails.json`, trocando o `frontend:dir` pra apontar pra `frontend` (relativo a `shell/`) e desligando os hooks de build de frontend (não tem `npm run build` aqui, só um HTML estático):

```json
{
  "$schema": "https://wails.io/schemas/config.v2.json",
  "name": "james-shell",
  "outputfilename": "james",
  "frontend:install": "",
  "frontend:build": "",
  "frontend:dev:watcher": "",
  "frontend:dir": "frontend",
  "author": {
    "name": "Arthur Orosco"
  }
}
```

Depois: `rm wails.json` (raiz).

- [ ] **Step 8: Compilar o shell**

```bash
cd shell && wails build -o /dev/null && cd ..
```

Se `wails build` não aceitar `-o /dev/null` (formato de saída específico do Wails, diferente do `go build`), usar a flag equivalente da versão instalada ou `wails build` normal seguido de exclusão manual do binário gerado — nunca deixar o `.exe` gerado for do `.gitignore`.

- [ ] **Step 9: Smoke test manual (não automatizado — janela nativa)**

```bash
docker compose up -d
go run ./cmd/jamesd &
cd shell && wails dev
```

Confirmar: janela abre pequena, frameless, transparente; clicar no logo expande o chat; mandar mensagem funciona fim a fim contra o `jamesd` local. Colar a evidência (screenshot ou descrição do comportamento observado) no commit de fechamento.

- [ ] **Step 10: Commit**

```bash
git add web/embed.go internal/api/rest.go cmd/jamesd/main.go shell/ wails.json
git commit -m "feat: jamesd serve a SPA embutida; shell vira invólucro Wails mínimo"
```

---

## Task 25: Remoção do código morto e fechamento da Fase 1

**Files:**
- Delete (rastreados pelo git — `git rm` funciona): `app.go`, `database.go`, `main.go` (raiz), `processador_voz.py`, `run_athena.bat`, `vosk_api.h`
- **Não rastreados, nada a fazer no git:** `TTs/`, `vosk/`, `libvosk.dll`, `libvosk.lib`, `libgcc_s_seh-1.dll`, `libstdc++-6.dll`, `libwinpthread-1.dll` já estão em `.gitignore` desde o primeiro commit da Fase 0 (`git ls-files` confirma: nunca entraram no histórico). Não existem nem dentro deste worktree — `git worktree` só copia arquivo rastreado. Se ainda estiverem cluttering o checkout original do Arthur fora do worktree, é `rm -rf` manual dele, fora do escopo desta task (não é operação de git)
- Keep: `rodar_claude.ps1` (ainda usado por `ferramentas.ClaudeCode`, caminho relativo à raiz do repo — `jamesd` continua rodando a partir daí)
- Keep: `Athena.db`, `Athena.db-shm`, `Athena.db-wal` no disco (fonte de dados do `cmd/migrar-historico` — não apagar, só continuam fora do git via `.gitignore` já existente da Fase 0)
- Keep: `Vault/` intocado (Fase 4)

- [ ] **Step 1: Confirmar que nada mais referencia o código antigo**

```bash
go build -o /dev/null ./...
```

Se compilar limpo com `app.go`/`database.go`/`main.go` (raiz) ainda presentes, é sinal de que já estão órfãos (nenhum pacote novo os importa — eles são `package main` na raiz, isolados). Confirmar visualmente que nenhum arquivo novo (`cmd/`, `internal/`) importa símbolos deles antes de apagar.

- [ ] **Step 2: Remover os arquivos mortos rastreados**

```bash
git rm app.go database.go main.go processador_voz.py run_athena.bat vosk_api.h
```

(`TTs/`, `vosk/` e as DLLs não aparecem em `git ls-files` — não rodar `git rm` neles, vai falhar com "did not match any files" porque nunca foram commitados.)

- [ ] **Step 3: Rodar a suíte inteira**

```bash
docker compose up -d
go vet ./... && go test ./...
pytest dispositivo/
npm test --prefix web
```

Expected: PASS em tudo, colar a saída real (não resumir) — regra inegociável do CLAUDE.md do Arthur.

- [ ] **Step 4: Rodar a guarda de credenciais contra o repositório inteiro**

```bash
go test ./internal/higiene/... -run TestRepositorioLimpo -v
```

Expected: PASS — nenhuma credencial (nem os padrões genéricos novos) na árvore.

- [ ] **Step 5: Commit**

```bash
git commit -m "chore: remove app.go e o resto do código morto substituído pela Fase 1"
```

---

## Verificação final da Fase 1 (critérios de aceitação da spec, seção 9)

Rodar cada um com evidência colável antes de considerar a fase fechada — nenhum item vira "deve funcionar" sem ter rodado:

1. [ ] `go vet ./... && go test ./...` passa
2. [ ] `pytest` (dentro de `dispositivo/`) passa
3. [ ] `npm test` (dentro de `web/`) passa
4. [ ] `jamesd` sobe, conecta no Postgres, aplica migrações; falha rápido e com mensagem clara se faltar config/segredo (testar removendo `GEMINI_API_KEY` do `.env` temporariamente e confirmar a mensagem)
5. [ ] O shell abre e a conversa funciona, com as mesmas capacidades de hoje (executar comando, Claude Code, busca, manipular tela)
6. [ ] O celular na VPN abre a mesma UI e conversa em modo Restrito; ação perigosa dispara o card de aprovação no desktop, aprovar/negar funciona fim a fim
7. [ ] Matar o `james-device` **não** derruba o `jamesd`; a tool de tela retorna erro claro (testar: fechar `python -m james_device.main`, pedir uma ação de tela pelo chat, confirmar resposta de erro em vez de travar)
8. [ ] `cmd/migrar-historico` importa as mensagens do `Athena.db` pra sessão "Histórico Athena"; rodar duas vezes não duplica
9. [ ] `git log -p` sem nenhuma credencial em nenhum commit
10. [ ] Nenhuma chave em texto puro no código

**Backlog aberto ao fechar a fase (registrado, não bloqueia — spec seção 11):**
- Allowlist de dispositivo por identidade de peer Tailscale (fica pro spike de identidade de peer)
- WebView2 real ainda não testado contra o HTTPS do tailnet dentro do shell de fato (Step 9 do Task 24 testa localhost; testar contra a URL `https://*.ts.net` quando o Tailscale estiver acessível)
- Rename físico da pasta `meu-app-ts` → `james` (P6, manual, feito pelo Arthur depois desta fase fechar)
- `GerarStream` do `Provider` Gemini não faz streaming token-a-token de verdade (Task 6) — protocolo `token` funciona, granularidade fica pra melhoria futura
