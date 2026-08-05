package ferramentas

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

// ComandoSistema executa PowerShell. Não há allowlist de comando — a
// contenção escolhida (Fase 3, decisão D2) é confirmação humana pra comando
// de escrita, via ConfirmacaoNecessaria, mais timeout e limite de saída.
type ComandoSistema struct {
	// Executor recebe o ctx pra que o timeout mate o processo de verdade, e
	// não só abandone a espera.
	Executor func(ctx context.Context, comando string) (ResultadoComando, error)

	// Timeout zero desliga o limite de tempo. MaxBytesSaida zero desliga a
	// truncagem. Ambos vêm do config.toml (Task 8).
	Timeout       time.Duration
	MaxBytesSaida int
}

func NovoComandoSistema() *ComandoSistema {
	return &ComandoSistema{Executor: executarPowerShell}
}

func executarPowerShell(ctx context.Context, comando string) (ResultadoComando, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", comando)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	res := ResultadoComando{Stdout: stdout.String(), Stderr: stderr.String()}
	var saiuComErro *exec.ExitError
	if errors.As(err, &saiuComErro) {
		res.ExitCode = saiuComErro.ExitCode()
		return res, nil
	}
	return res, err
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

	ctxComando := ctx
	if c.Timeout > 0 {
		var cancelar context.CancelFunc
		ctxComando, cancelar = context.WithTimeout(ctx, c.Timeout)
		defer cancelar()
	}

	res, err := c.Executor(ctxComando, comando)

	// Timeout é reportado COMO timeout, distinguível de erro de execução: o
	// modelo precisa saber que o comando não terminou, e não que ele falhou.
	if errors.Is(ctxComando.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("comando excedeu o tempo limite de %s e foi encerrado", c.Timeout)
	}
	if err != nil {
		return "", fmt.Errorf("erro ao executar: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("comando terminou com exit code %d: %s",
			res.ExitCode, truncar(res.Stderr, c.MaxBytesSaida))
	}
	return formatarResultadoComando(res, c.MaxBytesSaida), nil
}
