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
