package ferramentas

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestComandoSistemaExecutaViaExecutorInjetado(t *testing.T) {
	var comandoRecebido string
	c := &ComandoSistema{
		Executor: func(ctx context.Context, comando string) (ResultadoComando, error) {
			comandoRecebido = comando
			return ResultadoComando{Stdout: "saída do comando"}, nil
		},
		MaxBytesSaida: 1000,
	}

	saida, err := c.Executar(context.Background(), map[string]any{"comando": "Get-Process"})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !strings.Contains(saida, "saída do comando") {
		t.Errorf("saida = %q, deveria conter a stdout do comando", saida)
	}
	if comandoRecebido != "Get-Process" {
		t.Errorf("comando repassado = %q, queria Get-Process", comandoRecebido)
	}
}

func TestComandoSistemaRejeitaArgumentoAusente(t *testing.T) {
	c := &ComandoSistema{Executor: func(context.Context, string) (ResultadoComando, error) {
		return ResultadoComando{}, nil
	}}
	if _, err := c.Executar(context.Background(), map[string]any{}); err == nil {
		t.Fatal("esperava erro por parâmetro comando ausente, veio nil")
	}
}

func TestComandoSistemaPropagaErroDoExecutor(t *testing.T) {
	c := &ComandoSistema{Executor: func(context.Context, string) (ResultadoComando, error) {
		return ResultadoComando{}, errors.New("falhou")
	}}
	if _, err := c.Executar(context.Background(), map[string]any{"comando": "x"}); err == nil {
		t.Fatal("esperava erro propagado do Executor, veio nil")
	}
}

func TestComandoSistemaExitCodeDiferenteDeZeroViraErro(t *testing.T) {
	c := &ComandoSistema{
		Executor: func(context.Context, string) (ResultadoComando, error) {
			return ResultadoComando{Stderr: "não encontrado", ExitCode: 1}, nil
		},
		MaxBytesSaida: 1000,
	}
	_, err := c.Executar(context.Background(), map[string]any{"comando": "Get-Foo"})
	if err == nil {
		t.Fatal("exit code 1 deveria virar erro, veio nil")
	}
	if !strings.Contains(err.Error(), "não encontrado") {
		t.Errorf("erro deveria carregar o stderr, veio %q", err.Error())
	}
}

func TestComandoSistemaNivelPerigoso(t *testing.T) {
	c := NovoComandoSistema()
	if c.Nivel() != NivelPerigoso {
		t.Error("executar_comando_sistema deve ser NivelPerigoso")
	}
}

func TestComandoSistemaConfirmacaoNecessaria(t *testing.T) {
	casos := []struct {
		nome    string
		args    map[string]any
		querido bool
	}{
		{"leitura não exige", map[string]any{"comando": "Get-Process"}, false},
		{"escrita exige", map[string]any{"comando": "Remove-Item x"}, true},
		{"argumento ausente exige", map[string]any{}, true},
		{"argumento vazio exige", map[string]any{"comando": ""}, true},
		{"argumento de tipo errado exige", map[string]any{"comando": 42}, true},
	}

	c := NovoComandoSistema()
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if got := c.ConfirmacaoNecessaria(caso.args); got != caso.querido {
				t.Errorf("ConfirmacaoNecessaria(%v) = %v, queria %v", caso.args, got, caso.querido)
			}
		})
	}
}

// TestComandoSistemaImplementaExigeConfirmacao trava a asserção de tipo que o
// loop faz: sem ela, o gate silenciosamente não pega a ferramenta.
func TestComandoSistemaImplementaExigeConfirmacao(t *testing.T) {
	var _ ExigeConfirmacao = NovoComandoSistema()
}
