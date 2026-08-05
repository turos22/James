package ferramentas

import (
	"fmt"
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
