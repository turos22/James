package memoria

import (
	"path/filepath"
	"strings"
	"testing"
)

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

func TestResolverCaminhoFicaDentroDaRaiz(t *testing.T) {
	raiz := t.TempDir()

	caminho, err := ResolverCaminho(raiz, "Preferências de Código")
	if err != nil {
		t.Fatalf("ResolverCaminho devolveu erro: %v", err)
	}

	raizAbs, _ := filepath.Abs(raiz)
	if !strings.HasPrefix(caminho, raizAbs+string(filepath.Separator)) {
		t.Fatalf("caminho %q escapou da raiz %q", caminho, raizAbs)
	}
	if filepath.Base(caminho) != "preferencias-de-codigo.md" {
		t.Errorf("nome de arquivo = %q, queria %q", filepath.Base(caminho), "preferencias-de-codigo.md")
	}
}

func TestResolverCaminhoRejeitaTravessia(t *testing.T) {
	raiz := t.TempDir()

	// Nenhuma destas entradas pode produzir caminho fora da raiz. Todas são
	// coisas que o modelo consegue mandar no parâmetro assunto.
	entradas := []string{
		"../../prompts/sistema.md",
		"..\\..\\prompts\\sistema.md",
		"/etc/passwd",
		"C:\\Windows\\System32\\drivers\\etc\\hosts",
		"..",
		"",
		"   ",
		"///",
	}

	for _, entrada := range entradas {
		caminho, err := ResolverCaminho(raiz, entrada)
		if err != nil {
			continue // rejeitar é resultado válido
		}
		raizAbs, _ := filepath.Abs(raiz)
		if !strings.HasPrefix(caminho, raizAbs+string(filepath.Separator)) {
			t.Errorf("entrada %q produziu caminho fora da raiz: %q", entrada, caminho)
		}
	}
}
