package memoria

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcrescentarCriaArquivoDoAssunto(t *testing.T) {
	raiz := filepath.Join(t.TempDir(), "Memoria")
	v := NovoVault(raiz, 2000)

	if err := v.Acrescentar("Preferências de Código", "O Arthur escreve tudo em português."); err != nil {
		t.Fatalf("Acrescentar devolveu erro: %v", err)
	}

	bruto, err := os.ReadFile(filepath.Join(raiz, "preferencias-de-codigo.md"))
	if err != nil {
		t.Fatalf("arquivo da nota não foi criado: %v", err)
	}
	if !strings.Contains(string(bruto), "O Arthur escreve tudo em português.") {
		t.Errorf("conteúdo gravado = %q, não contém o texto da nota", bruto)
	}
}

func TestAcrescentarSomaNoMesmoAssunto(t *testing.T) {
	raiz := filepath.Join(t.TempDir(), "Memoria")
	v := NovoVault(raiz, 2000)

	if err := v.Acrescentar("projeto-james", "Back-end em Go."); err != nil {
		t.Fatalf("Acrescentar (1) devolveu erro: %v", err)
	}
	if err := v.Acrescentar("projeto-james", "Front-end em React."); err != nil {
		t.Fatalf("Acrescentar (2) devolveu erro: %v", err)
	}

	bruto, _ := os.ReadFile(filepath.Join(raiz, "projeto-james.md"))
	if !strings.Contains(string(bruto), "Back-end em Go.") || !strings.Contains(string(bruto), "Front-end em React.") {
		t.Errorf("a segunda nota sobrescreveu a primeira; conteúdo = %q", bruto)
	}
}

func TestAcrescentarAchataQuebraDeLinha(t *testing.T) {
	raiz := filepath.Join(t.TempDir(), "Memoria")
	v := NovoVault(raiz, 2000)

	if err := v.Acrescentar("teste", "primeira linha\nsegunda linha"); err != nil {
		t.Fatalf("Acrescentar devolveu erro: %v", err)
	}

	bruto, _ := os.ReadFile(filepath.Join(raiz, "teste.md"))
	if strings.Count(strings.TrimSpace(string(bruto)), "\n") != 0 {
		t.Errorf("nota multilinha quebrou o formato de lista; conteúdo = %q", bruto)
	}
}

func TestAcrescentarRejeitaTextoVazio(t *testing.T) {
	v := NovoVault(filepath.Join(t.TempDir(), "Memoria"), 2000)
	if err := v.Acrescentar("teste", "   "); err == nil {
		t.Error("esperava erro para texto vazio, veio nil")
	}
}
