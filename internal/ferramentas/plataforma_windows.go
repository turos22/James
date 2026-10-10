package ferramentas

// TerminalSuportado diz se executar_comando_sistema deve ser registrado
// nesta plataforma. Ver cmd/jamesd/ferramentas.go, registrarTerminal.
const TerminalSuportado = true

func NovoComandoSistema() *ComandoSistema {
	return &ComandoSistema{
		Executor:      executarPowerShell,
		Classificador: ComandoPowerShellPrecisaConfirmacao,
		NomeShell:     "PowerShell",
	}
}
