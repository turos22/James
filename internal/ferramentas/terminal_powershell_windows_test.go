package ferramentas

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestExecutarPowerShellDeVerdadeComAcentoEAspas roda o PowerShell real: é o
// único jeito de provar que o comando atravessa as duas camadas de parsing de
// shell sem se quebrar. Não usa mock de propósito.
func TestExecutarPowerShellDeVerdadeComAcentoEAspas(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o PowerShell real")
	}
	res, err := executarPowerShell(context.Background(), `Write-Output "ação 'com' aspas"`)
	if err != nil {
		t.Fatalf("executarPowerShell devolveu erro: %v", err)
	}
	if !strings.Contains(res.Stdout, "ação 'com' aspas") {
		t.Errorf("stdout = %q, queria o texto com acento e aspas intactos", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit code = %d, queria 0", res.ExitCode)
	}
}

// TestExecutarPowerShellDeVerdadeRespeitaOTimeout prova que o processo é
// morto de verdade, não só abandonado.
func TestExecutarPowerShellDeVerdadeRespeitaOTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o PowerShell real")
	}
	c := &ComandoSistema{Executor: executarPowerShell, Timeout: 2 * time.Second, MaxBytesSaida: 1000}

	inicio := time.Now()
	_, err := c.Executar(context.Background(), map[string]any{"comando": "Start-Sleep -Seconds 30"})
	decorrido := time.Since(inicio)

	if err == nil {
		t.Fatal("esperava erro de timeout, veio nil")
	}
	if !strings.Contains(err.Error(), "tempo limite") {
		t.Errorf("erro deveria dizer que foi timeout, veio %q", err.Error())
	}
	if decorrido > 10*time.Second {
		t.Errorf("demorou %s — o processo não foi morto no timeout", decorrido)
	}
}

func TestExecutarPowerShellDeVerdadeCapturaExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o PowerShell real")
	}
	res, err := executarPowerShell(context.Background(), "exit 3")
	if err != nil {
		t.Fatalf("executarPowerShell devolveu erro: %v", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("exit code = %d, queria 3", res.ExitCode)
	}
}

// TestExecutarPowerShellDeVerdadeNaoDeixaBOM é cinto e suspensório: mesmo
// com UTF8Encoding::new($false) (que não deveria gravar BOM), confirma que
// nenhum U+FEFF sobrevive até o Go. BOM vazando pro modelo é sujeira que não
// aparece na tela e reaparece como bug estranho lá na frente.
func TestExecutarPowerShellDeVerdadeNaoDeixaBOM(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o PowerShell real")
	}
	res, err := executarPowerShell(context.Background(), `Write-Output "ok"`)
	if err != nil {
		t.Fatalf("executarPowerShell devolveu erro: %v", err)
	}
	bom := string(rune(0xFEFF))
	if strings.HasPrefix(res.Stdout, bom) {
		t.Errorf("stdout começa com BOM: %q", res.Stdout)
	}
}
