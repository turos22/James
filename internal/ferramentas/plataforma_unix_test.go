//go:build unix

package ferramentas

import (
	"context"
	"strings"
	"testing"
)

func TestTerminalNoUnixUsaBashComDenylist(t *testing.T) {
	if !TerminalSuportado {
		t.Fatal("TerminalSuportado = false no unix com a denylist bash pronta")
	}
	c := NovoComandoSistema()
	if !strings.Contains(c.NomeShell, "bash") {
		t.Errorf("NomeShell = %q, deveria nomear bash", c.NomeShell)
	}
	if !c.ConfirmacaoNecessaria(map[string]any{"comando": "rm -rf ~"}) {
		t.Error("rm -rf ~ precisa exigir confirmação no Linux")
	}
	if c.ConfirmacaoNecessaria(map[string]any{"comando": "ls -la"}) {
		t.Error("ls -la não deveria exigir confirmação no Linux")
	}
	saida, err := c.Executar(context.Background(), map[string]any{"comando": "echo oi"})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !strings.Contains(saida, "oi") {
		t.Errorf("saída = %q, queria conter oi", saida)
	}
}
