//go:build unix

package ferramentas

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// comandoDaLixeira é lista de argumentos, nunca shell, e leva "--" antes do
// caminho: um nome de arquivo que começa com hífen não pode virar opção do gio.
func comandoDaLixeira(caminho string) []string {
	return []string{"gio", "trash", "--", caminho}
}

// executarLixeira manda o caminho para a lixeira do desktop (a do KDE e a do
// GNOME seguem o mesmo padrão freedesktop, e o gio fala com as duas). É uma
// variável para os testes não tocarem na lixeira de verdade.
var executarLixeira = func(ctx context.Context, caminho string) error {
	ctx, cancelar := context.WithTimeout(ctx, 10*time.Second)
	defer cancelar()
	comando := comandoDaLixeira(caminho)
	saida, err := exec.CommandContext(ctx, comando[0], comando[1:]...).CombinedOutput()
	if err != nil {
		mensagem := strings.TrimSpace(string(saida))
		if mensagem == "" {
			mensagem = err.Error()
		}
		return errors.New(mensagem)
	}
	return nil
}
