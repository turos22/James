package ferramentas

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// truncar corta s no teto de bytes e marca o corte no próprio texto. Nunca
// trunca em silêncio: o modelo precisa saber que não viu tudo, senão conclui
// coisa errada com confiança. maxBytes <= 0 desliga a truncagem.
//
// O corte recua até o início de um rune pra não partir caractere multibyte no
// meio e entregar UTF-8 inválido ao modelo.
func truncar(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}

	corte := maxBytes
	for corte > 0 && !utf8.RuneStart(s[corte]) {
		corte--
	}
	return s[:corte] + fmt.Sprintf("\n[saída truncada: %d de %d bytes]", corte, len(s))
}

// ResultadoComando é o que um Executor de comando devolve. As duas correntes
// vêm separadas de propósito: CombinedOutput misturava stdout com stderr e o
// modelo não tinha como distinguir resultado de diagnóstico.
type ResultadoComando struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// formatarResultadoComando monta o texto que vai pro modelo, truncando cada
// corrente separadamente pra que uma stderr enorme não coma o orçamento da
// stdout (e vice-versa).
func formatarResultadoComando(res ResultadoComando, maxBytes int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "exit_code: %d\n", res.ExitCode)

	if stdout := truncar(res.Stdout, maxBytes); stdout != "" {
		fmt.Fprintf(&b, "--- stdout ---\n%s\n", stdout)
	}
	if stderr := truncar(res.Stderr, maxBytes); stderr != "" {
		fmt.Fprintf(&b, "--- stderr ---\n%s\n", stderr)
	}
	return b.String()
}
