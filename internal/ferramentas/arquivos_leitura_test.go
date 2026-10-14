package ferramentas

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Spec 2026-10-14 arquivos, seção 4 (listar).

func escrever(t *testing.T, caminho, conteudo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(caminho), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func linhasQueComecamCom(saida, prefixo string) []string {
	var achadas []string
	for _, l := range strings.Split(saida, "\n") {
		if strings.HasPrefix(l, prefixo) {
			achadas = append(achadas, l)
		}
	}
	return achadas
}

func TestListarPastasPrimeiroDepoisArquivosPorNomeSemCaixa(t *testing.T) {
	raiz := t.TempDir()
	for _, p := range []string{"zeta", "alfa"} {
		if err := os.Mkdir(filepath.Join(raiz, p), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	escrever(t, filepath.Join(raiz, "b.txt"), "bb")
	escrever(t, filepath.Join(raiz, "A.txt"), "a")

	saida, err := listarPasta(raiz, false)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	ordem := []string{"alfa/", "zeta/", "A.txt", "b.txt"}
	ultimo := -1
	for _, nome := range ordem {
		i := strings.Index(saida, nome)
		if i < 0 || i < ultimo {
			t.Fatalf("a ordem esperada é %v, saída:\n%s", ordem, saida)
		}
		ultimo = i
	}
}

func TestListarMostraTipoTamanhoEData(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "dados.txt")
	escrever(t, arquivo, strings.Repeat("x", 2048))
	data := time.Date(2026, 3, 9, 14, 5, 0, 0, time.Local)
	if err := os.Chtimes(arquivo, data, data); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(raiz, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	saida, _ := listarPasta(raiz, false)

	linha := linhasQueComecamCom(saida, "arquivo")
	if len(linha) != 1 || !strings.Contains(linha[0], "2.0 KB") || !strings.Contains(linha[0], "2026-03-09 14:05") || !strings.HasSuffix(linha[0], "dados.txt") {
		t.Errorf("linha do arquivo = %v", linha)
	}
	if pastas := linhasQueComecamCom(saida, "pasta"); len(pastas) != 1 || !strings.HasSuffix(pastas[0], "sub/") {
		t.Errorf("linha da pasta = %v", pastas)
	}
}

func TestListarOcultosSoComPedido(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, ".segredo"), "x")
	escrever(t, filepath.Join(raiz, "visivel"), "x")

	sem, _ := listarPasta(raiz, false)
	com, _ := listarPasta(raiz, true)

	if strings.Contains(sem, ".segredo") || !strings.Contains(sem, "visivel") {
		t.Errorf("sem ocultos:\n%s", sem)
	}
	if !strings.Contains(com, ".segredo") {
		t.Errorf("com ocultos:\n%s", com)
	}
}

func TestListarCorta200EntradasEAvisa(t *testing.T) {
	raiz := t.TempDir()
	for i := 0; i < 205; i++ {
		escrever(t, filepath.Join(raiz, fmt.Sprintf("f%03d.txt", i)), "x")
	}

	saida, _ := listarPasta(raiz, false)

	if n := len(linhasQueComecamCom(saida, "arquivo")); n != 200 {
		t.Errorf("%d linhas de arquivo, queria 200", n)
	}
	if !strings.Contains(saida, "e mais 5 entradas") {
		t.Errorf("sem o aviso de corte:\n%s", saida[len(saida)-120:])
	}
}

func TestListarLinkAparecePeloDestinoSemSerSeguido(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "real.txt"), "x")
	if err := os.Symlink(filepath.Join(raiz, "real.txt"), filepath.Join(raiz, "atalho")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(raiz, "nao-existe"), filepath.Join(raiz, "quebrado")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc", filepath.Join(raiz, "para-etc")); err != nil {
		t.Fatal(err)
	}

	saida, err := listarPasta(raiz, false)
	if err != nil {
		t.Fatalf("link quebrado derrubou a listagem: %v", err)
	}

	for _, esperado := range []string{"atalho -> " + filepath.Join(raiz, "real.txt"), "quebrado -> ", "para-etc -> /etc"} {
		if !strings.Contains(saida, esperado) {
			t.Errorf("faltou %q:\n%s", esperado, saida)
		}
	}
	if strings.Contains(saida, "hosts") {
		t.Error("o link para /etc foi seguido")
	}
}

func TestListarPastaVazia(t *testing.T) {
	saida, err := listarPasta(t.TempDir(), false)
	if err != nil || !strings.Contains(saida, "(pasta vazia)") {
		t.Errorf("saida = %q, err = %v", saida, err)
	}
}

func TestListarErrosClaros(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "arq.txt"), "x")

	if _, err := listarPasta(filepath.Join(raiz, "nao-existe"), false); err == nil || !strings.Contains(err.Error(), "não existe") {
		t.Errorf("inexistente: %v", err)
	}
	if _, err := listarPasta(filepath.Join(raiz, "arq.txt"), false); err == nil || !strings.Contains(err.Error(), "use ler") {
		t.Errorf("arquivo: %v", err)
	}
	if os.Geteuid() != 0 {
		fechada := filepath.Join(raiz, "fechada")
		if err := os.Mkdir(fechada, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(fechada, 0o755) })
		if _, err := listarPasta(fechada, false); err == nil || !strings.Contains(err.Error(), "sem permissão") {
			t.Errorf("sem permissão: %v", err)
		}
	}
}
