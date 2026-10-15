package ferramentas

// TerminalSuportado diz se executar_comando_sistema deve ser registrado
// nesta plataforma. Ver cmd/jamesd/ferramentas.go, registrarTerminal.
const TerminalSuportado = true

// ArquivosSuportado é falso no Windows: a lixeira e a denylist de escrita
// ainda não existem lá (spec 2026-10-14 arquivos, D18), e "Windows não pode
// quebrar" quer dizer que nada muda.
const ArquivosSuportado = false

func NovoComandoSistema() *ComandoSistema {
	return &ComandoSistema{
		Executor:      executarPowerShell,
		Classificador: ComandoPowerShellPrecisaConfirmacao,
		NomeShell:     "PowerShell",
	}
}
