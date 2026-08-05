package ferramentas

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncarNaoMexeNoQueCabe(t *testing.T) {
	if got := truncar("abc", 10); got != "abc" {
		t.Errorf("truncar = %q, queria %q", got, "abc")
	}
}

func TestTruncarMaxZeroNaoTrunca(t *testing.T) {
	if got := truncar("abcdef", 0); got != "abcdef" {
		t.Errorf("truncar com max 0 = %q, queria a string inteira", got)
	}
}

func TestTruncarMarcaOCorte(t *testing.T) {
	got := truncar(strings.Repeat("a", 100), 10)
	if !strings.HasPrefix(got, strings.Repeat("a", 10)) {
		t.Errorf("truncar deveria começar com os 10 primeiros bytes, veio %q", got)
	}
	if !strings.Contains(got, "truncada") {
		t.Errorf("truncagem precisa ser marcada explicitamente, veio %q", got)
	}
	if !strings.Contains(got, "100") {
		t.Errorf("a marca precisa informar o tamanho original, veio %q", got)
	}
}

// TestTruncarNaoQuebraRune trava o bug de cortar no meio de um caractere
// multibyte: "ção" tem 4 bytes, cortar em 2 partiria o ç ao meio e produziria
// texto inválido pro modelo.
func TestTruncarNaoQuebraRune(t *testing.T) {
	got := truncar("ção", 2)
	corpo := strings.SplitN(got, "\n", 2)[0]
	if !utf8.ValidString(corpo) {
		t.Errorf("truncar produziu UTF-8 inválido: %q", corpo)
	}
}
