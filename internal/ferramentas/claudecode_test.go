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
