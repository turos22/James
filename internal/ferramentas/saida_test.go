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
	// "ção" tem 5 bytes: ç=2, ã=2, o=1. Cortar em 1 ou 3 cai no MEIO de um
	// rune e é o que de fato exercita o recuo; cortar em 2 ou 4 calha de já
	// ser fronteira de rune e passaria mesmo sem recuo nenhum. Varrer todos
	// os tamanhos cobre os dois casos sem depender de escolher o certo.
	for maxBytes := 1; maxBytes <= 5; maxBytes++ {
		got := truncar("ção", maxBytes)
		corpo := strings.SplitN(got, "\n", 2)[0]
		if !utf8.ValidString(corpo) {
			t.Errorf("truncar(\"ção\", %d) produziu UTF-8 inválido: %q", maxBytes, corpo)
		}
	}
}

// TestTruncarRecuaAteFronteiraDeRune prova o recuo diretamente, sem depender
// de validade de UTF-8: cortar em 3 bytes cai no meio do ã, e o resultado
// precisa recuar pro fim do ç.
func TestTruncarRecuaAteFronteiraDeRune(t *testing.T) {
	corpo := strings.SplitN(truncar("ção", 3), "\n", 2)[0]
	if corpo != "ç" {
		t.Errorf("truncar(\"ção\", 3) = %q, queria %q — o corte deveria recuar do meio do ã", corpo, "ç")
	}
}

func TestFormatarResultadoComandoMostraAsTresPartes(t *testing.T) {
	got := formatarResultadoComando(ResultadoComando{
		Stdout:   "linha de saída",
		Stderr:   "aviso qualquer",
		ExitCode: 0,
	}, 1000)

	for _, querido := range []string{"linha de saída", "aviso qualquer", "0"} {
		if !strings.Contains(got, querido) {
			t.Errorf("saída formatada não contém %q: %q", querido, got)
		}
	}
}

func TestFormatarResultadoComandoOmiteStderrVazio(t *testing.T) {
	got := formatarResultadoComando(ResultadoComando{Stdout: "ok", ExitCode: 0}, 1000)
	if strings.Contains(got, "stderr") {
		t.Errorf("stderr vazio não deveria aparecer: %q", got)
	}
}

func TestFormatarResultadoComandoTruncaCadaCorrente(t *testing.T) {
	got := formatarResultadoComando(ResultadoComando{
		Stdout:   strings.Repeat("a", 500),
		Stderr:   strings.Repeat("b", 500),
		ExitCode: 1,
	}, 50)
	if !strings.Contains(got, "truncada") {
		t.Errorf("esperava marca de truncagem, veio %q", got)
	}
}
