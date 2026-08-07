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

func TestCarregarAplicaDefaultsDeFerramentas(t *testing.T) {
	dir := t.TempDir()
	caminho := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(caminho, []byte(""), 0o600); err != nil {
		t.Fatalf("erro ao escrever config: %v", err)
	}
	t.Setenv("GEMINI_API_KEY", "chave")
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
	t.Setenv("DATABASE_URL", "postgres://x:y@localhost:5433/james")

	cfg, err := Carregar(filepath.Join(t.TempDir(), "nao-existe.toml"))
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.OrcamentoNotas != 2000 {
		t.Errorf("OrcamentoNotas = %d, queria 2000", cfg.OrcamentoNotas)
	}
	if cfg.DiretorioMemoria != "Vault/Memoria" {
		t.Errorf("DiretorioMemoria = %q, queria %q", cfg.DiretorioMemoria, "Vault/Memoria")
	}
}
