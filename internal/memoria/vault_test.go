package memoria

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestAcrescentarConcorrenteNaoPerdeNota(t *testing.T) {
	raiz := filepath.Join(t.TempDir(), "Memoria")
	v := NovoVault(raiz, 2000)

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := v.Acrescentar("concorrencia", fmt.Sprintf("nota numero %d", i)); err != nil {
				t.Errorf("Acrescentar(%d) devolveu erro: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	bruto, err := os.ReadFile(filepath.Join(raiz, "concorrencia.md"))
	if err != nil {
		t.Fatalf("arquivo não foi criado: %v", err)
	}

	// Contagem de linha e conferência de conteúdo são melhorias reais sobre
	// nenhuma asserção, mas nenhuma das duas prova que o mutex é necessário
	// nesta máquina: escrita O_APPEND pequena é atômica no NTFS/Windows nesse
	// padrão, e o arquivo sai correto mesmo com o mutex removido de propósito
	// (verificado manualmente durante o review, 50/50 execuções sem falha).
	// Este teste é guarda contra regressão grosseira de lógica (ex.: esquecer
	// O_APPEND, truncar o arquivo), não prova de necessidade do mutex nesta
	// plataforma — isso exigiria -race, que não roda neste ambiente (ver
	// Task 4 do plano).
	linhas := strings.Split(strings.TrimSpace(string(bruto)), "\n")
	if len(linhas) != n {
		t.Fatalf("esperava %d notas, o arquivo tem %d linhas — escrita concorrente perdeu nota", n, len(linhas))
	}

	vistas := make(map[string]bool, n)
	for _, l := range linhas {
		vistas[l] = true
	}
	for i := 0; i < n; i++ {
		esperada := fmt.Sprintf("- nota numero %d", i)
		if !vistas[esperada] {
			t.Errorf("nota %q ausente ou corrompida no arquivo final", esperada)
		}
	}
}
