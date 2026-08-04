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
