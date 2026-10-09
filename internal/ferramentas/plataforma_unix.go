//go:build unix

package ferramentas

import (
	"context"
	"errors"
)

// TerminalSuportado fica false até a denylist bash existir com testes
// (spec 2026-10-09-james-fedora, D3). O classificador da plataforma
// conhecia só PowerShell; portar o terminal pra bash sem a denylist deixaria
// comando destrutivo passar sem aprovação.
const TerminalSuportado = false

func NovoComandoSistema() *ComandoSistema {
	// Classificador nil: fail closed. Não deveria nem ser chamado, porque o
	// jamesd não registra a ferramenta, mas a ferramenta não depende disso.
	return &ComandoSistema{Executor: terminalDesligado, NomeShell: "bash"}
}

func terminalDesligado(context.Context, string) (ResultadoComando, error) {
	return ResultadoComando{}, errors.New("terminal desligado neste sistema: denylist linux ausente")
}
