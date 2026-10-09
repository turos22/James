//go:build unix

package ferramentas

import (
	"context"
	"strings"
	"testing"
)

// Até a denylist bash existir com testes (Tasks 4 e 6), o terminal no Linux
// não executa nada. Regra do Arthur: sem denylist, rm -rf ~ passaria sem
// aprovação.
func TestTerminalDesligadoNoUnix(t *testing.T) {
	if TerminalSuportado {
		t.Fatal("TerminalSuportado = true no unix antes da denylist bash")
	}
	c := NovoComandoSistema()
	_, err := c.Executar(context.Background(), map[string]any{"comando": "echo oi"})
	if err == nil || !strings.Contains(err.Error(), "desligado") {
		t.Errorf("Executar deveria recusar com 'desligado', veio %v", err)
	}
	if !c.ConfirmacaoNecessaria(map[string]any{"comando": "ls"}) {
		t.Error("terminal desligado deveria ser fail closed na confirmação")
	}
}
