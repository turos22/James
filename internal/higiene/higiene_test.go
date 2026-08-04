package higiene

import (
	"os"
	"path/filepath"
	"testing"
)

// chaveFalsa monta um literal de credencial em tempo de execução para que
// este próprio arquivo de teste não seja acusado pela varredura.
func chaveFalsa() string {
	return "AIza" + "Sy" + "0000000000000000000000000000000000"
}

func TestVarrerAcusaCredencialEmArquivoTexto(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "config.go")
	if err := os.WriteFile(arquivo, []byte(`const chave = "`+chaveFalsa()+`"`), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado, vieram %d: %+v", len(achados), achados)
	}
	if achados[0].Arquivo != arquivo {
		t.Errorf("arquivo do achado = %q, queria %q", achados[0].Arquivo, arquivo)
	}
}

func TestVarrerIgnoraDiretoriosDeTerceiros(t *testing.T) {
	raiz := t.TempDir()
	pasta := filepath.Join(raiz, "node_modules")
	if err := os.Mkdir(pasta, 0o755); err != nil {
		t.Fatalf("não consegui criar node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pasta, "vendido.js"), []byte(chaveFalsa()), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf("node_modules deveria ser ignorado, vieram %d achados: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaCredencialDentroDeBinario(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "app.exe")
	conteudo := append([]byte{0x4D, 0x5A, 0x90, 0x00}, []byte(chaveFalsa())...)
	if err := os.WriteFile(arquivo, conteudo, 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("credencial dentro de binário deveria ser acusada, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerIgnoraExtensaoDeBibliotecaNativa(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "biblioteca.dll")
	if err := os.WriteFile(arquivo, []byte(chaveFalsa()), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf(".dll deveria ser ignorada, vieram %d achados: %+v", len(achados), achados)
	}
}

// TestRepositorioLimpo é a guarda de verdade: falha enquanto existir
// qualquer credencial conhecida na árvore do projeto.
func TestRepositorioLimpo(t *testing.T) {
	achados, err := Varrer("../..")
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	for _, a := range achados {
		t.Errorf("credencial encontrada: %s (padrão %q)", a.Arquivo, a.Padrao)
	}
}
