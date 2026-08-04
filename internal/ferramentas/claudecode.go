package ferramentas

import (
	"context"
	"fmt"
	"os/exec"
)

// ClaudeCode porta app.go:170 (ChamarClaudeCode) como está: fire-and-forget,
// não captura saída, abre janela nova via rodar_claude.ps1. Consertar é
// Fase 3 (Anexo A #17, #18).
type ClaudeCode struct {
	Executor func(instrucao string) error
}

func NovoClaudeCode() *ClaudeCode {
	return &ClaudeCode{Executor: dispararRodarClaude}
}

func dispararRodarClaude(instrucao string) error {
	cmd := exec.Command("cmd", "/C", "start", "powershell", "-NoExit",
		"-ExecutionPolicy", "Bypass", "-File", "rodar_claude.ps1", instrucao)
	return cmd.Run()
}

func (c *ClaudeCode) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "perguntar_ao_claude_code",
		Descricao: "Usa o agente Claude Code pra interagir com o ambiente de desenvolvimento.",
		Parametros: map[string]Parametro{
			"instrucao": {Tipo: "string", Descricao: "instrução ou mensagem pro Claude Code"},
		},
		Obrigatorios: []string{"instrucao"},
	}
}

func (c *ClaudeCode) Nivel() Nivel { return NivelPerigoso }

func (c *ClaudeCode) Executar(ctx context.Context, args map[string]any) (string, error) {
	instrucao, ok := args["instrucao"].(string)
	if !ok || instrucao == "" {
		return "", fmt.Errorf("parâmetro instrucao ausente ou vazio")
	}
	if err := c.Executor(instrucao); err != nil {
		return "", fmt.Errorf("erro ao iniciar Claude Code: %w", err)
	}
	return "Claude Code iniciado. Tarefa enviada para o agente.", nil
}
