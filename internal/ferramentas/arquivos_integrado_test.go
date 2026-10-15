package ferramentas_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"james/internal/ferramentas"
	"james/internal/memoria"
	"james/internal/permissao"
)

// Spec 2026-10-14 arquivos, D2 (perfil) e D11 (o mesmo SegredosLidos nas duas
// ferramentas). Pacote externo: permissao importa ferramentas.

func TestCelularPedeAprovacaoParaLerEODesktopNao(t *testing.T) {
	nivel := ferramentas.NovoArquivos(nil).Nivel()

	if permissao.Restrito.Pode(nivel) {
		t.Error("o celular (Restrito) podia ler arquivos sem aprovação")
	}
	if !permissao.Privilegiado.Pode(nivel) {
		t.Error("o desktop (Privilegiado) pedia aprovação para ler")
	}
}

func TestRevelarEMemoriaCompartilhamOMesmoSegredosLidos(t *testing.T) {
	pasta := t.TempDir()
	env := filepath.Join(pasta, ".env")
	if err := os.WriteFile(env, []byte("CHAVE=abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	segredos := ferramentas.NovoSegredosLidos()
	arquivos := ferramentas.NovoArquivos(segredos)
	memoriaTool := ferramentas.NovoSalvarNaMemoria(memoria.NovoVault(filepath.Join(pasta, "Memoria"), 2000))
	memoriaTool.Segredos = segredos

	if _, err := arquivos.Executar(context.Background(), map[string]any{"acao": "revelar", "caminho": env}); err != nil {
		t.Fatalf("revelar: %v", err)
	}
	_, err := memoriaTool.Executar(context.Background(), map[string]any{"assunto": "projeto", "texto": "a chave é abcdef123456"})

	if err == nil || !strings.Contains(err.Error(), "valor secreto") {
		t.Fatalf("a memória aceitou o valor revelado: %v", err)
	}
	if _, err := memoriaTool.Executar(context.Background(), map[string]any{"assunto": "projeto", "texto": "o projeto usa uma chave de API"}); err != nil {
		t.Errorf("a conclusão sem o valor foi recusada: %v", err)
	}
}
