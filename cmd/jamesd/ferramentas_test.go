package main

import (
	"testing"
	"time"

	"james/internal/config"
	"james/internal/ferramentas"
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

// TestConstruirClaudeCodeAplicaConfig prova que Diretorio e Timeout chegam
// com o valor do config — mesma falha silenciosa das duas ferramentas
// anteriores: sem isso, a verificação ao vivo da Task 17 falhou com
// "diretório de trabalho do Claude Code não configurado" porque
// construirClaudeCode ainda era só o ponto de costura, sem aplicar nada.
func TestConstruirClaudeCodeAplicaConfig(t *testing.T) {
	cfg := &config.Config{
		ClaudeCodeDiretorio:       "C:\\projetos\\alvo",
		ClaudeCodeTimeoutSegundos: 120,
	}

	c := construirClaudeCode(cfg)

	if c == nil {
		t.Fatal("construirClaudeCode devolveu nil")
	}
	if c.Diretorio != "C:\\projetos\\alvo" {
		t.Errorf("Diretorio = %q, queria C:\\\\projetos\\\\alvo", c.Diretorio)
	}
	if c.Timeout != 120*time.Second {
		t.Errorf("Timeout = %s, queria %s", c.Timeout, 120*time.Second)
	}
}

func TestRegistrarTerminalNaoRegistraQuandoNaoSuportado(t *testing.T) {
	registro := ferramentas.NovoRegistro()
	if err := registrarTerminal(registro, &config.Config{}, false); err != nil {
		t.Fatalf("registrarTerminal devolveu erro: %v", err)
	}
	if _, ok := registro.Buscar("executar_comando_sistema"); ok {
		t.Error("terminal registrado numa plataforma sem suporte")
	}
}

func TestRegistrarTerminalRegistraQuandoSuportado(t *testing.T) {
	registro := ferramentas.NovoRegistro()
	if err := registrarTerminal(registro, &config.Config{}, true); err != nil {
		t.Fatalf("registrarTerminal devolveu erro: %v", err)
	}
	if _, ok := registro.Buscar("executar_comando_sistema"); !ok {
		t.Error("terminal não registrado numa plataforma com suporte")
	}
}
