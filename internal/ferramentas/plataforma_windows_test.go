package ferramentas

import "testing"

func TestTerminalNoWindowsUsaPowerShell(t *testing.T) {
	if !TerminalSuportado {
		t.Fatal("TerminalSuportado = false no Windows")
	}
	c := NovoComandoSistema()
	if c.NomeShell != "PowerShell" {
		t.Errorf("NomeShell = %q, queria PowerShell", c.NomeShell)
	}
	if !c.ConfirmacaoNecessaria(map[string]any{"comando": "Remove-Item x"}) {
		t.Error("Remove-Item deveria exigir confirmação no Windows")
	}
	if c.ConfirmacaoNecessaria(map[string]any{"comando": "Get-Process"}) {
		t.Error("Get-Process não deveria exigir confirmação no Windows")
	}
}
