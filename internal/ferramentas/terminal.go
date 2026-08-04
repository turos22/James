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
