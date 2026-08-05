package ferramentas

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
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

// variaveisQueSequestram são as variáveis de ambiente que apontariam o Claude
// Code pra outro provider em vez da assinatura Pro via OAuth. O script
// rodar_claude.ps1 fazia essa limpeza em PowerShell, onde nada era testado;
// aqui vira código com teste (Anexo A #17).
var variaveisQueSequestram = []string{
	"ANTHROPIC_BASE_URL=",
	"ANTHROPIC_API_KEY=",
	"ANTHROPIC_AUTH_TOKEN=",
}

// AmbienteSemAnthropic devolve o ambiente recebido sem as variáveis que
// sequestrariam a autenticação. Compara pelo prefixo com "=" pra não remover
// variável de nome parecido (ANTHROPIC_MODEL, por exemplo, sobrevive).
func AmbienteSemAnthropic(ambiente []string) []string {
	limpo := make([]string, 0, len(ambiente))
	for _, v := range ambiente {
		sequestra := false
		for _, proibida := range variaveisQueSequestram {
			if strings.HasPrefix(v, proibida) {
				sequestra = true
				break
			}
		}
		if !sequestra {
			limpo = append(limpo, v)
		}
	}
	return limpo
}

// MontarArgs monta a linha de comando do Claude Code headless. sessaoID vazio
// omite as flags de sessão. retomar escolhe entre --session-id (primeira
// chamada, cria a sessão) e --resume (chamadas seguintes, continuam nela) —
// verificado por execução real (spike da Task 13): reusar --session-id no
// mesmo id dá erro "Session ID already in use", e --resume no mesmo id
// preserva o contexto.
func MontarArgs(instrucao, sessaoID string, retomar bool) []string {
	args := []string{
		"-p", instrucao,
		"--output-format", "json",
		"--permission-mode", "acceptEdits",
	}
	if sessaoID == "" {
		return args
	}
	if retomar {
		return append(args, "--resume", sessaoID)
	}
	return append(args, "--session-id", sessaoID)
}
