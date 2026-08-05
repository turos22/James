package ferramentas

import (
	"context"
	"fmt"
	"os/exec"
)

// ComandoSistema executa PowerShell. Não há allowlist de comando — a
// contenção escolhida (Fase 3, decisão D2) é confirmação humana pra comando
// de escrita, via ConfirmacaoNecessaria, mais timeout e limite de saída.
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

// ConfirmacaoNecessaria implementa ExigeConfirmacao: comando de escrita pede
// confirmação humana mesmo do perfil Privilegiado. Argumento ausente, vazio
// ou de tipo errado devolve true (fail closed) — a validação de argumento em
// si continua acontecendo em Executar.
func (c *ComandoSistema) ConfirmacaoNecessaria(args map[string]any) bool {
	comando, ok := args["comando"].(string)
	if !ok {
		return true
	}
	return !ComandoEhLeitura(comando)
}

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
