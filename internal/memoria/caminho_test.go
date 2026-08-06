package memoria

import "testing"

func TestSlugAssuntoNormaliza(t *testing.T) {
	casos := []struct {
		entrada  string
		esperado string
	}{
		{"preferencias-de-codigo", "preferencias-de-codigo"},
		{"Preferências de Código", "preferencias-de-codigo"},
		{"  espaços  nas  pontas  ", "espacos-nas-pontas"},
		{"Projeto James!!!", "projeto-james"},
		{"acentuação: ç ã é í ó ú ñ", "acentuacao-c-a-e-i-o-u-n"},
		{"../../etc/passwd", "etc-passwd"},
		{"C:\\Windows\\System32", "c-windows-system32"},
		{"...", ""},
		{"", ""},
	}

	for _, c := range casos {
		if obtido := SlugAssunto(c.entrada); obtido != c.esperado {
			t.Errorf("SlugAssunto(%q) = %q, queria %q", c.entrada, obtido, c.esperado)
		}
	}
}
