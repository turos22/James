package ferramentas

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Spec 2026-10-14 arquivos, D10, D12, D13.

func lixeiraFalsa(t *testing.T) *[]string {
	t.Helper()
	var chamadas []string
	original := executarLixeira
	executarLixeira = func(_ context.Context, caminho string) error {
		chamadas = append(chamadas, caminho)
		return nil
	}
	t.Cleanup(func() { executarLixeira = original })
	return &chamadas
}

// --- mover ---------------------------------------------------------------------

func TestMoverArquivo(t *testing.T) {
	pasta := t.TempDir()
	origem, destino := filepath.Join(pasta, "a.txt"), filepath.Join(pasta, "b.txt")
	escrever(t, origem, "conteúdo")

	saida, err := moverArquivo(origem, destino)

	if err != nil || lerTudo(t, destino) != "conteúdo" {
		t.Fatalf("erro %v", err)
	}
	if _, err := os.Stat(origem); err == nil {
		t.Error("a origem continua lá")
	}
	if !strings.Contains(saida, "movi") {
		t.Errorf("saida = %q", saida)
	}
}

func TestMoverNaoSobrescreve(t *testing.T) {
	pasta := t.TempDir()
	origem, destino := filepath.Join(pasta, "a.txt"), filepath.Join(pasta, "b.txt")
	escrever(t, origem, "novo")
	escrever(t, destino, "antigo")

	_, err := moverArquivo(origem, destino)

	if err == nil || !strings.Contains(err.Error(), "já existe") {
		t.Fatalf("err = %v", err)
	}
	if lerTudo(t, destino) != "antigo" || lerTudo(t, origem) != "novo" {
		t.Error("mexeu nos arquivos mesmo recusando")
	}
}

func TestMoverNaoMoveParaDentroDeSiMesmoNemOrigemInexistente(t *testing.T) {
	pasta := t.TempDir()
	escrever(t, filepath.Join(pasta, "d", "x"), "x")

	if _, err := moverArquivo(filepath.Join(pasta, "d"), filepath.Join(pasta, "d", "sub")); err == nil || !strings.Contains(err.Error(), "dentro") {
		t.Errorf("para dentro de si mesma: %v", err)
	}
	if _, err := moverArquivo(filepath.Join(pasta, "nao-existe"), filepath.Join(pasta, "z")); err == nil || !strings.Contains(err.Error(), "não existe") {
		t.Errorf("origem inexistente: %v", err)
	}
	if _, err := moverArquivo(filepath.Join(pasta, "d", "x"), filepath.Join(pasta, "sem-pai", "x")); err == nil {
		t.Error("moveu para uma pasta que não existe")
	}
}

func TestMoverEntreSistemasDeArquivosCopiaConfereERemove(t *testing.T) {
	pasta := t.TempDir()
	origem, destino := filepath.Join(pasta, "a", "x.txt"), filepath.Join(pasta, "b.txt")
	escrever(t, origem, "conteúdo importante")
	original := renomearParaMover
	renomearParaMover = func(string, string) error { return &os.LinkError{Op: "rename", Err: syscall.EXDEV} }
	t.Cleanup(func() { renomearParaMover = original })

	if _, err := moverArquivo(origem, destino); err != nil {
		t.Fatalf("erro: %v", err)
	}

	if lerTudo(t, destino) != "conteúdo importante" {
		t.Error("o destino não tem o conteúdo")
	}
	if _, err := os.Stat(origem); err == nil {
		t.Error("a origem não foi removida depois da cópia conferida")
	}
}

func TestMoverEntreSistemasNaoRemoveAOrigemSeACopiaFalhar(t *testing.T) {
	pasta := t.TempDir()
	origem := filepath.Join(pasta, "x.txt")
	escrever(t, origem, "conteúdo importante")
	original, originalCopia := renomearParaMover, renomear
	renomearParaMover = func(string, string) error { return &os.LinkError{Op: "rename", Err: syscall.EXDEV} }
	renomear = func(string, string) error { return errors.New("a cópia falhou") } // quebra o rename final da cópia
	t.Cleanup(func() { renomearParaMover, renomear = original, originalCopia })

	_, err := moverArquivo(origem, filepath.Join(pasta, "y.txt"))

	if err == nil {
		t.Fatal("devia ter falhado")
	}
	if lerTudo(t, origem) != "conteúdo importante" {
		t.Error("a origem foi perdida")
	}
}

// --- copiar --------------------------------------------------------------------

func TestCopiarArquivoPreservaOModoENaoSobrescreve(t *testing.T) {
	pasta := t.TempDir()
	origem, destino := filepath.Join(pasta, "a.sh"), filepath.Join(pasta, "b.sh")
	escrever(t, origem, "echo")
	if err := os.Chmod(origem, 0o750); err != nil {
		t.Fatal(err)
	}

	if _, err := copiarArquivo(origem, destino); err != nil {
		t.Fatalf("erro: %v", err)
	}
	if lerTudo(t, destino) != "echo" || modoDe(t, destino) != 0o750 || lerTudo(t, origem) != "echo" {
		t.Errorf("cópia: %q modo %v", lerTudo(t, destino), modoDe(t, destino))
	}
	semTemporarios(t, pasta)

	if _, err := copiarArquivo(origem, destino); err == nil || !strings.Contains(err.Error(), "já existe") {
		t.Errorf("sobrescrita: %v", err)
	}
}

func TestCopiarPastaInteiraComLinkCopiadoComoLink(t *testing.T) {
	pasta := t.TempDir()
	origem := filepath.Join(pasta, "proj")
	escrever(t, filepath.Join(origem, "a.txt"), "a")
	escrever(t, filepath.Join(origem, "sub", "b.txt"), "b")
	if err := os.Symlink("/etc", filepath.Join(origem, "para-etc")); err != nil {
		t.Fatal(err)
	}
	destino := filepath.Join(pasta, "copia")

	saida, err := copiarArquivo(origem, destino)

	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if lerTudo(t, filepath.Join(destino, "sub", "b.txt")) != "b" {
		t.Error("faltou um arquivo da subpasta")
	}
	alvo, err := os.Readlink(filepath.Join(destino, "para-etc"))
	if err != nil || alvo != "/etc" {
		t.Errorf("o link devia ter sido copiado como link: %q, %v", alvo, err)
	}
	if !strings.Contains(saida, "3 itens") && !strings.Contains(saida, "arquivos") {
		t.Errorf("saida = %q", saida)
	}
}

func TestCopiarPastaGrandeDemaisFalhaAntesDeComecar(t *testing.T) {
	pasta := t.TempDir()
	origem := filepath.Join(pasta, "muitos")
	for i := 0; i < 8; i++ {
		escrever(t, filepath.Join(origem, fmt.Sprintf("f%d", i)), "x")
	}
	original := maxArquivosNaCopia
	maxArquivosNaCopia = 5
	t.Cleanup(func() { maxArquivosNaCopia = original })
	destino := filepath.Join(pasta, "copia")

	_, err := copiarArquivo(origem, destino)

	if err == nil || !strings.Contains(err.Error(), "arquivos") {
		t.Fatalf("err = %v", err)
	}
	if _, err := os.Stat(destino); err == nil {
		t.Error("começou a copiar antes de recusar")
	}
}

func TestCopiarPastaPorBytesEParaDentroDeSiMesma(t *testing.T) {
	pasta := t.TempDir()
	origem := filepath.Join(pasta, "p")
	escrever(t, filepath.Join(origem, "a"), strings.Repeat("x", 100))
	original := maxBytesNaCopia
	maxBytesNaCopia = 50
	t.Cleanup(func() { maxBytesNaCopia = original })

	if _, err := copiarArquivo(origem, filepath.Join(pasta, "c")); err == nil || !strings.Contains(err.Error(), "limite de cópia") {
		t.Errorf("por bytes: %v", err)
	}
	maxBytesNaCopia = original
	if _, err := copiarArquivo(origem, filepath.Join(origem, "dentro")); err == nil || !strings.Contains(err.Error(), "dentro") {
		t.Errorf("para dentro de si mesma: %v", err)
	}
}

// --- apagar --------------------------------------------------------------------

func TestApagarMandaParaALixeira(t *testing.T) {
	chamadas := lixeiraFalsa(t)
	arquivo := filepath.Join(t.TempDir(), "x.txt")
	escrever(t, arquivo, "x")

	saida, err := apagarArquivo(context.Background(), arquivo)

	if err != nil || len(*chamadas) != 1 || (*chamadas)[0] != arquivo {
		t.Fatalf("err %v, chamadas %v", err, *chamadas)
	}
	if !strings.Contains(saida, "lixeira") {
		t.Errorf("saida = %q", saida)
	}
}

func TestApagarComALixeiraFalhandoNuncaApagaDeVez(t *testing.T) {
	original := executarLixeira
	executarLixeira = func(context.Context, string) error { return errors.New("sem lixeira neste sistema de arquivos") }
	t.Cleanup(func() { executarLixeira = original })
	arquivo := filepath.Join(t.TempDir(), "x.txt")
	escrever(t, arquivo, "importante")

	_, err := apagarArquivo(context.Background(), arquivo)

	if err == nil || !strings.Contains(err.Error(), "lixeira") {
		t.Fatalf("err = %v", err)
	}
	if lerTudo(t, arquivo) != "importante" {
		t.Error("o arquivo sumiu: a falha da lixeira virou apagar de vez")
	}
}

func TestApagarInexistenteFalhaAntesDaLixeira(t *testing.T) {
	chamadas := lixeiraFalsa(t)

	_, err := apagarArquivo(context.Background(), filepath.Join(t.TempDir(), "nao-existe"))

	if err == nil || !strings.Contains(err.Error(), "não existe") || len(*chamadas) != 0 {
		t.Errorf("err %v, chamadas %v", err, *chamadas)
	}
}

// --- piso ----------------------------------------------------------------------

func TestPisoApagarEMoverRecusamMesmoChamadosDireto(t *testing.T) {
	home := comHomeDeTeste(t)
	chamadas := lixeiraFalsa(t)
	linkParaEtc := filepath.Join(home, "para-etc")
	if err := os.Symlink("/etc", linkParaEtc); err != nil {
		t.Fatal(err)
	}

	for _, protegido := range []string{"/etc", "/usr", home, linkParaEtc} {
		if _, err := apagarArquivo(context.Background(), protegido); err == nil || !strings.Contains(err.Error(), "protegida") {
			t.Errorf("apagar %q: %v", protegido, err)
		}
		if _, err := moverArquivo(protegido, filepath.Join(home, "destino")); err == nil || !strings.Contains(err.Error(), "protegida") {
			t.Errorf("mover %q: %v", protegido, err)
		}
	}
	if len(*chamadas) != 0 {
		t.Errorf("a lixeira foi chamada para um caminho protegido: %v", *chamadas)
	}
}

func TestPisoNaoImpedeCopiarNemCriar(t *testing.T) {
	home := comHomeDeTeste(t)
	hosts := "/etc/hosts"
	if _, err := os.Stat(hosts); err != nil {
		t.Skip("sem /etc/hosts")
	}

	if _, err := copiarArquivo(hosts, filepath.Join(home, "hosts-copia")); err != nil {
		t.Errorf("copiar de /etc/hosts (leitura) foi recusado: %v", err)
	}
}

func TestValidarMoverCopiarApagarNaoEscrevemNada(t *testing.T) {
	pasta := t.TempDir()
	origem := filepath.Join(pasta, "a.txt")
	escrever(t, origem, "x")
	destino := filepath.Join(pasta, "b.txt")

	if err := validarMover(origem, destino); err != nil {
		t.Errorf("validarMover: %v", err)
	}
	if err := validarCopiar(origem, destino); err != nil {
		t.Errorf("validarCopiar: %v", err)
	}
	if err := validarApagar(origem); err != nil {
		t.Errorf("validarApagar: %v", err)
	}
	if _, err := os.Stat(destino); err == nil {
		t.Error("a validação criou o destino")
	}
	if lerTudo(t, origem) != "x" {
		t.Error("a validação mexeu na origem")
	}
}
