//go:build unix

package ferramentas

import (
	"reflect"
	"testing"
)

func TestComandoDaLixeiraEListaDeArgumentosComTracos(t *testing.T) {
	// Lista de argumentos, nunca shell, e "--" antes do caminho: um nome de
	// arquivo que começa com hífen não pode virar opção do gio.
	got := comandoDaLixeira("/home/a/-rf estranho; x")

	esperado := []string{"gio", "trash", "--", "/home/a/-rf estranho; x"}
	if !reflect.DeepEqual(got, esperado) {
		t.Errorf("comando = %q", got)
	}
}
