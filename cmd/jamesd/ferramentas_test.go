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

// TestConstruirBuscaAplicaConfig prova que Cliente.Timeout e MaxResultados
// chegam com o valor do config, não zero — mesma falha silenciosa que
// motivou a extração em TestConstruirComandoSistemaAplicaConfig: a ferramenta
// funciona do mesmo jeito, só que sem timeout e sem limite, e compilar não
// acusa nada.
func TestConstruirBuscaAplicaConfig(t *testing.T) {
	cfg := &config.Config{
		TavilyAPIKey:         "chave-de-teste",
		BuscaTimeoutSegundos: 15,
		BuscaMaxResultados:   3,
	}

	b := construirBusca(cfg)

	if b == nil {
		t.Fatal("construirBusca devolveu nil")
	}
	if b.Cliente.Timeout != 15*time.Second {
		t.Errorf("Cliente.Timeout = %s, queria %s", b.Cliente.Timeout, 15*time.Second)
	}
	if b.MaxResultados != 3 {
		t.Errorf("MaxResultados = %d, queria 3", b.MaxResultados)
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
