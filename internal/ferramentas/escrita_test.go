package ferramentas

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Spec 2026-10-14 arquivos, D10 e D16.

func modoDe(t *testing.T, caminho string) os.FileMode {
	t.Helper()
	info, err := os.Stat(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func lerTudo(t *testing.T, caminho string) string {
	t.Helper()
	b, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func semTemporarios(t *testing.T, pasta string) {
	t.Helper()
	itens, _ := os.ReadDir(pasta)
	for _, i := range itens {
		if strings.HasPrefix(i.Name(), ".james-") {
			t.Errorf("sobrou um temporário: %s", i.Name())
		}
	}
}

// --- criar ---------------------------------------------------------------------

func TestCriarFazOArquivoComModo0644(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "novo.txt")

	saida, err := criarArquivo(arquivo, "olá\n", false, false)

	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if lerTudo(t, arquivo) != "olá\n" || modoDe(t, arquivo) != 0o644 {
		t.Errorf("conteúdo %q, modo %v", lerTudo(t, arquivo), modoDe(t, arquivo))
	}
	if !strings.Contains(saida, "5 bytes") {
		t.Errorf("saida = %q", saida)
	}
	semTemporarios(t, filepath.Dir(arquivo))
}

func TestCriarNaoSobrescreveSemPedidoExplicito(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "x.txt")
	escrever(t, arquivo, "original")

	_, err := criarArquivo(arquivo, "novo", false, false)

	if err == nil || !strings.Contains(err.Error(), "sobrescrever: true") {
		t.Fatalf("err = %v", err)
	}
	if lerTudo(t, arquivo) != "original" {
		t.Error("o arquivo foi alterado mesmo assim")
	}
}

func TestCriarSobrescreverPreservaOModo(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "x.sh")
	escrever(t, arquivo, "antigo")
	if err := os.Chmod(arquivo, 0o750); err != nil {
		t.Fatal(err)
	}

	if _, err := criarArquivo(arquivo, "novo", true, false); err != nil {
		t.Fatalf("erro: %v", err)
	}

	if lerTudo(t, arquivo) != "novo" || modoDe(t, arquivo) != 0o750 {
		t.Errorf("conteúdo %q, modo %v", lerTudo(t, arquivo), modoDe(t, arquivo))
	}
}

func TestCriarPastaPaiInexistente(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "a", "b", "x.txt")

	if _, err := criarArquivo(arquivo, "x", false, false); err == nil || !strings.Contains(err.Error(), "criar_pastas: true") {
		t.Fatalf("sem criar_pastas: %v", err)
	}
	if _, err := os.Stat(filepath.Join(raiz, "a")); err == nil {
		t.Error("criou pastas sem ter sido pedido")
	}

	if _, err := criarArquivo(arquivo, "x", false, true); err != nil {
		t.Fatalf("com criar_pastas: %v", err)
	}
	if modoDe(t, filepath.Join(raiz, "a", "b")) != 0o755 {
		t.Errorf("modo das pastas = %v", modoDe(t, filepath.Join(raiz, "a", "b")))
	}
}

func TestCriarRecusaConteudoGrandeEPasta(t *testing.T) {
	raiz := t.TempDir()

	if _, err := criarArquivo(filepath.Join(raiz, "g.txt"), strings.Repeat("x", 1024*1024+1), false, false); err == nil || !strings.Contains(err.Error(), "1 MB") {
		t.Errorf("grande: %v", err)
	}
	if _, err := criarArquivo(raiz, "x", true, false); err == nil || !strings.Contains(err.Error(), "pasta") {
		t.Errorf("pasta: %v", err)
	}
	if _, err := criarArquivo(filepath.Join(raiz, "g.txt"), strings.Repeat("x", 1024*1024), false, false); err != nil {
		t.Errorf("exatamente 1 MB devia caber: %v", err)
	}
}

func TestCriarPaiQueEUmArquivo(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "arq"), "x")

	if _, err := criarArquivo(filepath.Join(raiz, "arq", "x.txt"), "x", false, true); err == nil {
		t.Error("criou dentro de um arquivo")
	}
}

// --- editar --------------------------------------------------------------------

func TestEditarTrocaUmaOcorrencia(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "a.go")
	escrever(t, arquivo, "um\ndois\ntres\n")

	saida, err := editarArquivo(arquivo, "dois", "DOIS", false)

	if err != nil || lerTudo(t, arquivo) != "um\nDOIS\ntres\n" {
		t.Fatalf("erro %v, conteúdo %q", err, lerTudo(t, arquivo))
	}
	if !strings.Contains(saida, "1 trecho") {
		t.Errorf("saida = %q", saida)
	}
}

func TestEditarContaOcorrenciasEErraSemMexer(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "a.txt")
	escrever(t, arquivo, "x\nrepetido\nrepetido\n")

	_, zero := editarArquivo(arquivo, "nao-existe", "y", false)
	if zero == nil || !strings.Contains(zero.Error(), "0 ocorrências") {
		t.Errorf("zero: %v", zero)
	}
	_, varias := editarArquivo(arquivo, "repetido", "y", false)
	if varias == nil || !strings.Contains(varias.Error(), "2 vezes") || !strings.Contains(varias.Error(), "todas: true") {
		t.Errorf("várias: %v", varias)
	}
	if lerTudo(t, arquivo) != "x\nrepetido\nrepetido\n" {
		t.Error("o arquivo mudou apesar do erro")
	}
}

func TestEditarTodasTrocaETodasEDizQuantas(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "a.txt")
	escrever(t, arquivo, "a a a b")

	saida, err := editarArquivo(arquivo, "a", "z", true)

	if err != nil || lerTudo(t, arquivo) != "z z z b" || !strings.Contains(saida, "3 trechos") {
		t.Errorf("erro %v, conteúdo %q, saida %q", err, lerTudo(t, arquivo), saida)
	}
}

func TestEditarRecusaTrechoVazioBinarioEGrande(t *testing.T) {
	pasta := t.TempDir()
	texto := filepath.Join(pasta, "a.txt")
	escrever(t, texto, "abc")
	if _, err := editarArquivo(texto, "", "x", false); err == nil || !strings.Contains(err.Error(), "vazio") {
		t.Errorf("vazio: %v", err)
	}

	binario := filepath.Join(pasta, "x.bin")
	if err := os.WriteFile(binario, []byte("abc\x00def"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := editarArquivo(binario, "abc", "x", false); err == nil || !strings.Contains(err.Error(), "binário") {
		t.Errorf("binário: %v", err)
	}

	grande := filepath.Join(pasta, "g.txt")
	escrever(t, grande, strings.Repeat("a", 1024*1024+10))
	if _, err := editarArquivo(grande, "a", "b", false); err == nil || !strings.Contains(err.Error(), "1 MB") {
		t.Errorf("grande: %v", err)
	}

	if _, err := editarArquivo(pasta, "a", "b", false); err == nil || !strings.Contains(err.Error(), "pasta") {
		t.Errorf("pasta: %v", err)
	}
	if _, err := editarArquivo(filepath.Join(pasta, "nao-existe"), "a", "b", false); err == nil || !strings.Contains(err.Error(), "não existe") {
		t.Errorf("inexistente: %v", err)
	}
}

func TestEditarPreservaOModo(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "x.sh")
	escrever(t, arquivo, "echo a\n")
	if err := os.Chmod(arquivo, 0o750); err != nil {
		t.Fatal(err)
	}

	if _, err := editarArquivo(arquivo, "a", "b", false); err != nil {
		t.Fatal(err)
	}

	if modoDe(t, arquivo) != 0o750 {
		t.Errorf("modo = %v", modoDe(t, arquivo))
	}
}

func TestEditarAchaOTrechoComQuebraDeLinhaEmArquivoCRLF(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "sistema.md")
	if err := os.WriteFile(arquivo, []byte("linha 1\r\nlinha 2\r\nlinha 3\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := editarArquivo(arquivo, "linha 1\nlinha 2", "linha 1\nlinha DOIS", false)

	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if got := lerTudo(t, arquivo); got != "linha 1\r\nlinha DOIS\r\nlinha 3\r\n" {
		t.Errorf("conteúdo = %q, o CRLF devia ter sido mantido", got)
	}
}

func TestEditarEmLinkEditaODestinoEMantemOLink(t *testing.T) {
	pasta := t.TempDir()
	real := filepath.Join(pasta, "real.txt")
	escrever(t, real, "antes")
	atalho := filepath.Join(pasta, "atalho")
	if err := os.Symlink(real, atalho); err != nil {
		t.Fatal(err)
	}

	if _, err := editarArquivo(atalho, "antes", "depois", false); err != nil {
		t.Fatal(err)
	}

	if lerTudo(t, real) != "depois" {
		t.Error("o destino não foi editado")
	}
	if info, _ := os.Lstat(atalho); info.Mode()&os.ModeSymlink == 0 {
		t.Error("o link virou um arquivo comum")
	}
}

// --- atomicidade ---------------------------------------------------------------

func TestEscritaFalhaNoRenameDeixaOOriginalIntactoESemTemporario(t *testing.T) {
	pasta := t.TempDir()
	arquivo := filepath.Join(pasta, "a.txt")
	escrever(t, arquivo, "original")
	original := renomear
	renomear = func(string, string) error { return errors.New("rename falhou") }
	t.Cleanup(func() { renomear = original })

	_, errEditar := editarArquivo(arquivo, "original", "novo", false)
	_, errCriar := criarArquivo(arquivo, "novo", true, false)

	if errEditar == nil || errCriar == nil {
		t.Fatalf("errEditar = %v, errCriar = %v", errEditar, errCriar)
	}
	if lerTudo(t, arquivo) != "original" {
		t.Error("o original foi alterado")
	}
	semTemporarios(t, pasta)
}

func TestValidarEditarContaSemEscrever(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "a.txt")
	escrever(t, arquivo, "x x y")

	n, err := validarEditar(arquivo, "x", "z", true)

	if err != nil || n != 2 {
		t.Errorf("n = %d, err = %v", n, err)
	}
	if lerTudo(t, arquivo) != "x x y" {
		t.Error("validar escreveu")
	}
}

func TestValidarCriarDizSeSobrescreve(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "a.txt")

	bytes, sobrescreve, err := validarCriar(arquivo, "abc", false, false)
	if err != nil || bytes != 3 || sobrescreve {
		t.Errorf("novo: %d %v %v", bytes, sobrescreve, err)
	}
	escrever(t, arquivo, "x")
	_, sobrescreve, err = validarCriar(arquivo, "abc", true, false)
	if err != nil || !sobrescreve {
		t.Errorf("existente: %v %v", sobrescreve, err)
	}
}
