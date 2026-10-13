package config

import (
	"os"
	"path/filepath"
	"strings"
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
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5433/james")
	caminho := escreverToml(t, t.TempDir(), "[agente]\nprovedor = \"gemini\"\nmodelo = \"gemini-2.5-flash\"\n")

	_, err := Carregar(caminho)
	if err == nil || !strings.Contains(err.Error(), "GEMINI_API_KEY") {
		t.Fatalf("esperava erro citando GEMINI_API_KEY, veio %v", err)
	}
}

func TestCarregarFalhaSemDatabaseURL(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "chave-de-teste")
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")
	t.Setenv("DATABASE_URL", "")
	caminho := escreverToml(t, t.TempDir(), "")

	_, err := Carregar(caminho)
	if err == nil {
		t.Fatal("esperava erro por DATABASE_URL ausente, veio nil")
	}
}

func TestCarregarAplicaDefaults(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "chave-de-teste")
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")
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
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5433/james")
	caminho := escreverToml(t, t.TempDir(), `
[agente]
provedor = "gemini"
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
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5433/james")
	caminho := escreverToml(t, t.TempDir(), `
[agente]
provedor = "gemini"
modelo = "gemini-2.5-flash-lite"
`)

	_, err := Carregar(caminho)
	if err == nil {
		t.Fatal("gemini-2.5-flash-lite está descartado pela spec (D4/Anexo), esperava erro")
	}
}

func TestCarregarAplicaDefaultsDeFerramentas(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(caminho, []byte(""), 0o600); err != nil {
		t.Fatalf("erro ao escrever config: %v", err)
	}
	t.Setenv("GEMINI_API_KEY", "chave")
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")
	t.Setenv("DATABASE_URL", "postgres://x")

	cfg, err := Carregar(caminho)
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}

	if cfg.TerminalTimeoutSegundos != 60 {
		t.Errorf("TerminalTimeoutSegundos = %d, queria 60", cfg.TerminalTimeoutSegundos)
	}
	if cfg.TerminalMaxBytesSaida != 65536 {
		t.Errorf("TerminalMaxBytesSaida = %d, queria 65536", cfg.TerminalMaxBytesSaida)
	}
	if cfg.ClaudeCodeTimeoutSegundos != 600 {
		t.Errorf("ClaudeCodeTimeoutSegundos = %d, queria 600", cfg.ClaudeCodeTimeoutSegundos)
	}
	if cfg.BuscaTimeoutSegundos != 20 {
		t.Errorf("BuscaTimeoutSegundos = %d, queria 20", cfg.BuscaTimeoutSegundos)
	}
	if cfg.BuscaMaxResultados != 5 {
		t.Errorf("BuscaMaxResultados = %d, queria 5", cfg.BuscaMaxResultados)
	}
	// ClaudeCodeDiretorio é o único sem default: o James sobe sem ele, e a
	// ferramenta é que falha na hora da chamada.
	if cfg.ClaudeCodeDiretorio != "" {
		t.Errorf("ClaudeCodeDiretorio = %q, queria vazio por não ter default", cfg.ClaudeCodeDiretorio)
	}
}

func TestCarregarLeValoresDeFerramentasDoToml(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "config.toml")
	conteudo := `
[ferramentas.terminal]
timeout_segundos = 15
max_bytes_saida = 2048

[ferramentas.claude_code]
diretorio = "C:\\projetos\\alvo"
timeout_segundos = 120

[ferramentas.busca]
timeout_segundos = 7
max_resultados = 3
`
	if err := os.WriteFile(caminho, []byte(conteudo), 0o600); err != nil {
		t.Fatalf("erro ao escrever config: %v", err)
	}
	t.Setenv("GEMINI_API_KEY", "chave")
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")
	t.Setenv("DATABASE_URL", "postgres://x")

	cfg, err := Carregar(caminho)
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}

	if cfg.TerminalTimeoutSegundos != 15 {
		t.Errorf("TerminalTimeoutSegundos = %d, queria 15", cfg.TerminalTimeoutSegundos)
	}
	if cfg.TerminalMaxBytesSaida != 2048 {
		t.Errorf("TerminalMaxBytesSaida = %d, queria 2048", cfg.TerminalMaxBytesSaida)
	}
	if cfg.ClaudeCodeDiretorio != `C:\projetos\alvo` {
		t.Errorf("ClaudeCodeDiretorio = %q, queria C:\\projetos\\alvo", cfg.ClaudeCodeDiretorio)
	}
	if cfg.ClaudeCodeTimeoutSegundos != 120 {
		t.Errorf("ClaudeCodeTimeoutSegundos = %d, queria 120", cfg.ClaudeCodeTimeoutSegundos)
	}
	if cfg.BuscaTimeoutSegundos != 7 {
		t.Errorf("BuscaTimeoutSegundos = %d, queria 7", cfg.BuscaTimeoutSegundos)
	}
	if cfg.BuscaMaxResultados != 3 {
		t.Errorf("BuscaMaxResultados = %d, queria 3", cfg.BuscaMaxResultados)
	}
}

func TestCarregarAplicaDefaultsDaMemoriaDuravel(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "chave-de-teste")
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5433/james")

	cfg, err := Carregar(filepath.Join(t.TempDir(), "nao-existe.toml"))
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.OrcamentoNotas != 2000 {
		t.Errorf("OrcamentoNotas = %d, queria 2000", cfg.OrcamentoNotas)
	}
	if cfg.MemoriaDiretorio != "Vault/Memoria" {
		t.Errorf("MemoriaDiretorio = %q, queria %q", cfg.MemoriaDiretorio, "Vault/Memoria")
	}
}

// O config.toml é versionado e compartilhado pelo Windows e pelo Fedora; o
// caminho do workspace não é. A env vence o toml (spec 2026-10-09 D6).
func TestCarregarEnvDoClaudeCodeVenceOToml(t *testing.T) {
	caminho := escreverToml(t, t.TempDir(), "[ferramentas.claude_code]\ndiretorio = \"C:\\\\no\\\\windows\"\n")
	t.Setenv("GEMINI_API_KEY", "chave")
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JAMES_CLAUDE_CODE_DIRETORIO", "/home/arthur/workspace")

	cfg, err := Carregar(caminho)
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.ClaudeCodeDiretorio != "/home/arthur/workspace" {
		t.Errorf("ClaudeCodeDiretorio = %q, queria o da env", cfg.ClaudeCodeDiretorio)
	}
}

func TestCarregarEnvVaziaDoClaudeCodeNaoApagaOToml(t *testing.T) {
	caminho := escreverToml(t, t.TempDir(), "[ferramentas.claude_code]\ndiretorio = \"/do/toml\"\n")
	t.Setenv("GEMINI_API_KEY", "chave")
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JAMES_CLAUDE_CODE_DIRETORIO", "")

	cfg, err := Carregar(caminho)
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.ClaudeCodeDiretorio != "/do/toml" {
		t.Errorf("ClaudeCodeDiretorio = %q, queria o do toml", cfg.ClaudeCodeDiretorio)
	}
}

// --- provedor do agente e motor de voz (spec 2026-10-10 voz em tempo real) ---

func ambienteBase(t *testing.T) {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("CLAUDE_API_KEY", "")
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5434/james")
}

func TestCarregarPadraoEhAnthropicHaiku(t *testing.T) {
	ambienteBase(t)
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")

	cfg, err := Carregar(escreverToml(t, t.TempDir(), ""))
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.Provedor != "anthropic" {
		t.Errorf("Provedor = %q, queria anthropic", cfg.Provedor)
	}
	if cfg.Modelo != "claude-haiku-5-5" {
		t.Errorf("Modelo = %q, queria claude-haiku-5-5", cfg.Modelo)
	}
	if cfg.ClaudeAPIKey != "sk-ant-teste" {
		t.Errorf("ClaudeAPIKey não chegou da env")
	}
}

func TestCarregarAnthropicSemChaveFalhaCitandoAVariavel(t *testing.T) {
	ambienteBase(t)
	t.Setenv("GEMINI_API_KEY", "chave-do-gemini") // não substitui a da Anthropic

	_, err := Carregar(escreverToml(t, t.TempDir(), ""))
	if err == nil || !strings.Contains(err.Error(), "CLAUDE_API_KEY") {
		t.Fatalf("esperava erro citando CLAUDE_API_KEY, veio %v", err)
	}
}

func TestCarregarAnthropicNaoExigeChaveDoGemini(t *testing.T) {
	ambienteBase(t)
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")

	if _, err := Carregar(escreverToml(t, t.TempDir(), "")); err != nil {
		t.Fatalf("GEMINI_API_KEY não deveria ser obrigatória com provedor anthropic: %v", err)
	}
}

func TestCarregarGeminiAindaFunciona(t *testing.T) {
	ambienteBase(t)
	t.Setenv("GEMINI_API_KEY", "chave-do-gemini")

	cfg, err := Carregar(escreverToml(t, t.TempDir(), "[agente]\nprovedor = \"gemini\"\nmodelo = \"gemini-2.5-flash\"\n"))
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.Provedor != "gemini" || cfg.Modelo != "gemini-2.5-flash" {
		t.Errorf("provedor/modelo = %q/%q", cfg.Provedor, cfg.Modelo)
	}
}

func TestCarregarRejeitaProvedorDesconhecido(t *testing.T) {
	ambienteBase(t)
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")

	_, err := Carregar(escreverToml(t, t.TempDir(), "[agente]\nprovedor = \"openai\"\n"))
	if err == nil || !strings.Contains(err.Error(), "openai") {
		t.Fatalf("esperava erro nomeando o provedor inválido, veio %v", err)
	}
}

func TestCarregarModeloDoGeminiEmProvedorAnthropicNaoVale(t *testing.T) {
	ambienteBase(t)
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")

	// A regra do flash-lite é do Gemini; com anthropic o modelo é livre.
	cfg, err := Carregar(escreverToml(t, t.TempDir(), "[agente]\nmodelo = \"claude-sonnet-5-5\"\n"))
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.Modelo != "claude-sonnet-5-5" {
		t.Errorf("Modelo = %q", cfg.Modelo)
	}
}

func TestCarregarVozPadraoEhPiperLocal(t *testing.T) {
	ambienteBase(t)
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")

	cfg, err := Carregar(escreverToml(t, t.TempDir(), ""))
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.MotorVoz != "piper" {
		t.Errorf("MotorVoz = %q, queria piper", cfg.MotorVoz)
	}
	if cfg.TTSURL != "http://127.0.0.1:8180" {
		t.Errorf("TTSURL = %q, queria http://127.0.0.1:8180", cfg.TTSURL)
	}
}

func TestCarregarLeMotorEUrlDeVozDoToml(t *testing.T) {
	ambienteBase(t)
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")

	cfg, err := Carregar(escreverToml(t, t.TempDir(), "[voz]\nmotor = \"edge\"\ntts_url = \"http://127.0.0.1:9999\"\n"))
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.MotorVoz != "edge" || cfg.TTSURL != "http://127.0.0.1:9999" {
		t.Errorf("motor/url = %q/%q", cfg.MotorVoz, cfg.TTSURL)
	}
}

func TestCarregarRejeitaMotorDeVozDesconhecido(t *testing.T) {
	ambienteBase(t)
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")

	_, err := Carregar(escreverToml(t, t.TempDir(), "[voz]\nmotor = \"festival\"\n"))
	if err == nil || !strings.Contains(err.Error(), "festival") {
		t.Fatalf("esperava erro nomeando o motor inválido, veio %v", err)
	}
}

func TestCarregarTTSURLSoAceitaLoopback(t *testing.T) {
	ambienteBase(t)
	t.Setenv("CLAUDE_API_KEY", "sk-ant-teste")

	// O servidor de TTS é local e sem autenticação: apontar pra fora mandaria
	// o texto das respostas (que podem ter dado pessoal) pra rede.
	for _, url := range []string{"http://192.168.0.5:8180", "http://exemplo.invalid:8180", "https://127.0.0.1.evil.invalid"} {
		_, err := Carregar(escreverToml(t, t.TempDir(), "[voz]\ntts_url = \""+url+"\"\n"))
		if err == nil {
			t.Errorf("tts_url %q deveria ser recusada (não é loopback)", url)
		}
	}
	for _, url := range []string{"http://127.0.0.1:8180", "http://localhost:8180", "http://[::1]:8180"} {
		if _, err := Carregar(escreverToml(t, t.TempDir(), "[voz]\ntts_url = \""+url+"\"\n")); err != nil {
			t.Errorf("tts_url %q deveria ser aceita: %v", url, err)
		}
	}
}
