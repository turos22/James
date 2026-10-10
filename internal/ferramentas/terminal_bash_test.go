//go:build unix

package ferramentas

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Contraparte dos TestExecutarPowerShellDeVerdade*: bash real, sem mock.

func TestExecutarBashDeVerdadeComAcentoEAspas(t *testing.T) {
	res, err := executarBash(context.Background(), `printf '%s\n' "ação 'com' aspas"`)
	if err != nil {
		t.Fatalf("executarBash devolveu erro: %v", err)
	}
	if strings.TrimSpace(res.Stdout) != "ação 'com' aspas" {
		t.Errorf("stdout = %q, queria o texto com acento e aspas intactos", res.Stdout)
	}
}

func TestExecutarBashDeVerdadeCapturaExitCode(t *testing.T) {
	res, err := executarBash(context.Background(), "exit 3")
	if err != nil {
		t.Fatalf("executarBash devolveu erro: %v", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("exit code = %d, queria 3", res.ExitCode)
	}
}

func TestExecutarBashDeVerdadeForcaUTF8(t *testing.T) {
	res, err := executarBash(context.Background(), `printf '%s' "$LC_ALL"`)
	if err != nil {
		t.Fatalf("executarBash devolveu erro: %v", err)
	}
	if res.Stdout != "C.UTF-8" {
		t.Errorf("LC_ALL = %q, queria C.UTF-8", res.Stdout)
	}
}

// TestExecutarBashDeVerdadeTimeoutMataOGrupo prova duas coisas: que o
// Executar volta no timeout mesmo com um filho em segundo plano segurando o
// stdout, e que esse filho MORRE — e não fica órfão rodando.
func TestExecutarBashDeVerdadeTimeoutMataOGrupo(t *testing.T) {
	arquivoPID := filepath.Join(t.TempDir(), "pid")
	c := &ComandoSistema{Executor: executarBash, Timeout: 2 * time.Second, MaxBytesSaida: 1000}

	inicio := time.Now()
	_, err := c.Executar(context.Background(), map[string]any{
		"comando": "sleep 30 & echo $! > " + arquivoPID + "; wait",
	})
	decorrido := time.Since(inicio)

	if err == nil || !strings.Contains(err.Error(), "tempo limite") {
		t.Fatalf("esperava erro de tempo limite, veio %v", err)
	}
	if decorrido > 10*time.Second {
		t.Errorf("demorou %s — o processo não foi morto no timeout", decorrido)
	}

	bruto, err := os.ReadFile(arquivoPID)
	if err != nil {
		t.Fatalf("o filho não chegou a gravar o PID: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(bruto)))
	if err != nil {
		t.Fatalf("PID ilegível %q: %v", bruto, err)
	}
	// Dá um respiro pro kernel recolher o processo.
	time.Sleep(200 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Errorf("o sleep em segundo plano (pid %d) sobreviveu ao timeout", pid)
	}
}
