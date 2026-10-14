//go:build unix

package ferramentas

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Um pipe com nome bloqueia open/read para sempre quando ninguém escreve nele:
// ler, buscar, editar ou copiar um deles não pode travar o jamesd.

func criarFIFO(t *testing.T, caminho string) {
	t.Helper()
	if err := syscall.Mkfifo(caminho, 0o600); err != nil {
		t.Skipf("sem mkfifo: %v", err)
	}
}

// semTravar roda f e falha o teste se passar de 3 s. A goroutine travada vaza
// até o fim do processo de teste, o que é aceitável só no caso de falha.
func semTravar(t *testing.T, f func()) {
	t.Helper()
	feito := make(chan struct{})
	go func() { defer close(feito); f() }()
	select {
	case <-feito:
	case <-time.After(3 * time.Second):
		t.Fatal("travou em arquivo que não é comum (pipe)")
	}
}

func TestLerArquivoRecusaPipe(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "pipe")
	criarFIFO(t, pipe)
	semTravar(t, func() {
		_, err := lerArquivo(pipe, 1, 100, false)
		if err == nil || !strings.Contains(err.Error(), "não é um arquivo comum") {
			t.Errorf("err = %v, queria recusa de arquivo não comum", err)
		}
	})
}

func TestLerArquivoRecusaDispositivo(t *testing.T) {
	semTravar(t, func() {
		if _, err := lerArquivo("/dev/zero", 1, 100, false); err == nil {
			t.Error("leu /dev/zero sem erro")
		}
	})
}

func TestBuscaPorConteudoPulaPipe(t *testing.T) {
	pasta := t.TempDir()
	criarFIFO(t, filepath.Join(pasta, "pipe"))
	os.WriteFile(filepath.Join(pasta, "ok.txt"), []byte("achei o tesouro\n"), 0o644)
	semTravar(t, func() {
		saida, err := buscarArquivos(pasta, opcoesBusca{Conteudo: "tesouro"})
		if err != nil || !strings.Contains(saida, "ok.txt:1") {
			t.Errorf("saida=%q err=%v, queria achar ok.txt e seguir", saida, err)
		}
	})
}

func TestEditarRecusaPipe(t *testing.T) {
	pipe := filepath.Join(t.TempDir(), "pipe")
	criarFIFO(t, pipe)
	semTravar(t, func() {
		_, err := prepararEdicao(pipe, "a", "b", false)
		if err == nil || !strings.Contains(err.Error(), "não é um arquivo comum") {
			t.Errorf("err = %v, queria recusa", err)
		}
	})
}

func TestCopiarRecusaPipe(t *testing.T) {
	pasta := t.TempDir()
	pipe := filepath.Join(pasta, "pipe")
	criarFIFO(t, pipe)
	semTravar(t, func() {
		_, err := copiarArquivo(pipe, filepath.Join(pasta, "copia"))
		if err == nil {
			t.Error("copiou um pipe sem erro")
		}
	})
	if _, err := os.Lstat(filepath.Join(pasta, "copia")); err == nil {
		t.Error("sobrou destino da cópia recusada")
	}
}

func TestCopiarPastaComPipeNaoTrava(t *testing.T) {
	pasta := t.TempDir()
	origem := filepath.Join(pasta, "origem")
	os.Mkdir(origem, 0o755)
	criarFIFO(t, filepath.Join(origem, "pipe"))
	semTravar(t, func() {
		if _, err := copiarArquivo(origem, filepath.Join(pasta, "copia")); err == nil {
			t.Error("copiou pasta com pipe sem erro")
		}
	})
}
