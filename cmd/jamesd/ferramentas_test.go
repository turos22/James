package main

import (
	"testing"
	"time"

	"james/internal/config"
)

// TestConstruirComandoSistemaAplicaConfig é o teste que a Task 8 deixou sem
// rede de segurança: prova que Timeout e MaxBytesSaida chegam com o valor do
// config, não zero, sem precisar subir o jamesd inteiro (executar() abre
// Postgres de verdade e bloqueia num canal que só retorna com o servidor
// caindo — não dá pra chamar de teste).
func TestConstruirComandoSistemaAplicaConfig(t *testing.T) {
	cfg := &config.Config{
		TerminalTimeoutSegundos: 42,
		TerminalMaxBytesSaida:   999,
	}

	c := construirComandoSistema(cfg)

	if c.Timeout != 42*time.Second {
		t.Errorf("Timeout = %s, queria %s", c.Timeout, 42*time.Second)
	}
	if c.MaxBytesSaida != 999 {
		t.Errorf("MaxBytesSaida = %d, queria 999", c.MaxBytesSaida)
	}
}

// TestConstruirBuscaNaoDevolveNil é só o ponto de costura por enquanto — a
// Task 11 injeta timeout/max_resultados de verdade em ferramentas.Busca e
// ganha asserção completa aqui.
func TestConstruirBuscaNaoDevolveNil(t *testing.T) {
	cfg := &config.Config{TavilyAPIKey: "chave-de-teste"}

	b := construirBusca(cfg)

	if b == nil {
		t.Fatal("construirBusca devolveu nil")
	}
}

// TestConstruirClaudeCodeNaoDevolveNil é só o ponto de costura por enquanto —
// a Task 15 injeta diretório/timeout de verdade em ferramentas.ClaudeCode e
// ganha asserção completa aqui.
func TestConstruirClaudeCodeNaoDevolveNil(t *testing.T) {
	cfg := &config.Config{}

	c := construirClaudeCode(cfg)

	if c == nil {
		t.Fatal("construirClaudeCode devolveu nil")
	}
}
