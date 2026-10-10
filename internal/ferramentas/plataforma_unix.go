//go:build unix

package ferramentas

// TerminalSuportado: a denylist bash existe e tem testes
// (classificador_bash_test.go), condição da spec 2026-10-09 D3 pra ligar.
const TerminalSuportado = true

func NovoComandoSistema() *ComandoSistema {
	return &ComandoSistema{
		Executor:      executarBash,
		Classificador: ComandoBashPrecisaConfirmacao,
		NomeShell:     "bash no Fedora Linux",
	}
}
