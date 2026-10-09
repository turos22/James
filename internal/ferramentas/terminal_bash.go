//go:build unix

package ferramentas

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// esperaPosCancelamento limita quanto o Wait espera os pipes fecharem depois
// do kill. Cinto e suspensório do kill de grupo abaixo.
const esperaPosCancelamento = 2 * time.Second

// executarBash roda o comando num bash limpo (sem profile nem rc, que mudam
// de máquina pra máquina e podem imprimir lixo no stdout).
//
// O UTF-8 é nativo, então não há o truque de codepage do PowerShell: só
// LC_ALL=C.UTF-8, pra saída de ferramenta não depender do locale da sessão.
//
// Timeout mata o GRUPO de processos. exec.CommandContext sozinho mata só o
// bash; um filho em segundo plano (`algo &`) herda o stdout, segura o pipe
// aberto e o Wait não volta. Setpgid põe o bash e todo filho num grupo
// próprio, e Cancel mata o grupo inteiro (pid negativo).
func executarBash(ctx context.Context, comando string) (ResultadoComando, error) {
	cmd := exec.CommandContext(ctx, "bash", "--noprofile", "--norc", "-c", comando)
	cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = esperaPosCancelamento

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	res := ResultadoComando{Stdout: stdout.String(), Stderr: stderr.String()}

	// Mesmo contrato do executarPowerShell: exit code diferente de zero é
	// resultado, não erro de infraestrutura. Timeout é o Executar que
	// reconhece, olhando o ctx.
	var saiuComErro *exec.ExitError
	if errors.As(err, &saiuComErro) {
		res.ExitCode = saiuComErro.ExitCode()
		return res, nil
	}
	return res, err
}
