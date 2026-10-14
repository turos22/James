package ferramentas

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
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

// --- ler (spec 2026-10-14 arquivos, seção 4) -----------------------------------

func linhasNumeradas(n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "linha %d\n", i)
	}
	return b.String()
}

func TestLerDevolveLinhasNumeradas(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "a.txt")
	escrever(t, arquivo, "primeira\nsegunda\nterceira\n")

	r, err := lerArquivo(arquivo, 1, 0, false)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}

	for _, esperado := range []string{"     1  primeira", "     2  segunda", "     3  terceira"} {
		if !strings.Contains(r.Texto, esperado) {
			t.Errorf("faltou %q:\n%s", esperado, r.Texto)
		}
	}
	if strings.Contains(r.Texto, "continua") {
		t.Error("avisou de continuação num arquivo que cabe inteiro")
	}
}

func TestLerCorta200LinhasEDizDeOndeContinuar(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "a.txt")
	escrever(t, arquivo, linhasNumeradas(450))

	r, _ := lerArquivo(arquivo, 1, 0, false)

	if !strings.Contains(r.Texto, "   200  linha 200") || strings.Contains(r.Texto, "   201  linha 201") {
		t.Errorf("devia ir até a linha 200:\n...%s", r.Texto[len(r.Texto)-150:])
	}
	if !strings.Contains(r.Texto, "a_partir_da_linha: 201") {
		t.Errorf("não diz de onde continuar:\n...%s", r.Texto[len(r.Texto)-150:])
	}

	segunda, _ := lerArquivo(arquivo, 201, 0, false)
	if !strings.Contains(segunda.Texto, "   201  linha 201") || !strings.Contains(segunda.Texto, "   400  linha 400") {
		t.Errorf("a segunda página está errada:\n%s", segunda.Texto[:200])
	}
}

func TestLerRespeitaOTetoDeLinhasEDeBytes(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "a.txt")
	escrever(t, arquivo, linhasNumeradas(5000))

	r, _ := lerArquivo(arquivo, 1, 99999, false)
	if strings.Contains(r.Texto, "  2001  linha 2001") {
		t.Error("passou do teto de 2000 linhas")
	}

	// 32 KB por chamada, mesmo com poucas linhas longas.
	longas := filepath.Join(t.TempDir(), "longas.txt")
	escrever(t, longas, strings.Repeat(strings.Repeat("x", 4000)+"\n", 30))
	r2, _ := lerArquivo(longas, 1, 0, false)
	if len(r2.Texto) > 34*1024 {
		t.Errorf("devolveu %d bytes, o teto é 32 KB", len(r2.Texto))
	}
	if !strings.Contains(r2.Texto, "continua na linha") {
		t.Error("cortou por bytes sem avisar de onde continuar")
	}
}

func TestLerCortaLinhaGiganteComAviso(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "min.js")
	escrever(t, arquivo, strings.Repeat("a", 20000)+"\nfim\n")

	r, _ := lerArquivo(arquivo, 1, 0, false)

	if !strings.Contains(r.Texto, "linha cortada") || !strings.Contains(r.Texto, "20000 bytes") {
		t.Errorf("sem aviso de linha cortada:\n%s", r.Texto[:100])
	}
	if !strings.Contains(r.Texto, "fim") {
		t.Error("a linha seguinte sumiu")
	}
}

func TestLerRecusaBinarioEAceitaUTF8Quebrado(t *testing.T) {
	pasta := t.TempDir()
	binario := filepath.Join(pasta, "x.bin")
	if err := os.WriteFile(binario, []byte("abc\x00def"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := lerArquivo(binario, 1, 0, false)
	if err != nil || !strings.Contains(r.Texto, "arquivo binário, não leio (7 bytes)") {
		t.Errorf("binário: %q, %v", r.Texto, err)
	}

	quebrado := filepath.Join(pasta, "q.txt")
	if err := os.WriteFile(quebrado, []byte("ok \xff\xfe fim\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err = lerArquivo(quebrado, 1, 0, false)
	if err != nil || !strings.Contains(r.Texto, "ok ") || !strings.Contains(r.Texto, "fim") {
		t.Errorf("UTF-8 quebrado: %q, %v", r.Texto, err)
	}
}

func TestLerCasosDeBorda(t *testing.T) {
	pasta := t.TempDir()
	vazio := filepath.Join(pasta, "vazio.txt")
	escrever(t, vazio, "")
	r, err := lerArquivo(vazio, 1, 0, false)
	if err != nil || !strings.Contains(r.Texto, "(arquivo vazio)") {
		t.Errorf("vazio: %q, %v", r.Texto, err)
	}

	if _, err := lerArquivo(pasta, 1, 0, false); err == nil || !strings.Contains(err.Error(), "use listar") {
		t.Errorf("pasta: %v", err)
	}
	if _, err := lerArquivo(filepath.Join(pasta, "nao-existe"), 1, 0, false); err == nil || !strings.Contains(err.Error(), "não existe") {
		t.Errorf("inexistente: %v", err)
	}

	curto := filepath.Join(pasta, "curto.txt")
	escrever(t, curto, "a\nb\n")
	if _, err := lerArquivo(curto, 10, 0, false); err == nil || !strings.Contains(err.Error(), "só tem 2 linhas") {
		t.Errorf("além do fim: %v", err)
	}

	if os.Geteuid() != 0 {
		fechado := filepath.Join(pasta, "fechado.txt")
		escrever(t, fechado, "x")
		if err := os.Chmod(fechado, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(fechado, 0o644) })
		if _, err := lerArquivo(fechado, 1, 0, false); err == nil || !strings.Contains(err.Error(), "sem permissão") {
			t.Errorf("sem permissão: %v", err)
		}
	}
}

func TestLerArquivoGrandeAvisaQueLeuSoOComeco(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "grande.log")
	escrever(t, arquivo, strings.Repeat("linha do log\n", 800_000)) // ~10 MB

	r, err := lerArquivo(arquivo, 1, 0, false)

	if err != nil || !strings.Contains(r.Texto, "primeiros 8 MB") {
		t.Errorf("sem aviso do teto de leitura: %v\n...%s", err, r.Texto[len(r.Texto)-200:])
	}
}

// --- máscara aplicada pelo ler -----------------------------------------------

func TestLerMascaraOEnvEMantemOsNomes(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), ".env")
	escrever(t, arquivo, "# Credenciais\nCLAUDE_API_KEY="+"valorSecreto123"+"\nPORT=8080\n")

	r, err := lerArquivo(arquivo, 1, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(r.Texto, "valorSecreto123") || strings.Contains(r.Texto, "8080") {
		t.Errorf("um valor vazou:\n%s", r.Texto)
	}
	for _, mantido := range []string{"# Credenciais", "CLAUDE_API_KEY=" + marcaOculta, "PORT=" + marcaOculta} {
		if !strings.Contains(r.Texto, mantido) {
			t.Errorf("faltou %q:\n%s", mantido, r.Texto)
		}
	}
}

func TestLerMascaraCredencialEPEMEmArquivoComum(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "notas.txt")
	chave := chaveAnthropicDeTeste()
	escrever(t, arquivo, "oi\nchave: "+chave+"\n"+blocoPEM("CORPODACHAVEAAAA", "OUTROCORPOBBBB")+"\nfim\n")

	r, _ := lerArquivo(arquivo, 1, 0, false)

	for _, vazou := range []string{chave, "CORPODACHAVEAAAA", "OUTROCORPOBBBB"} {
		if strings.Contains(r.Texto, vazou) {
			t.Errorf("vazou %q:\n%s", vazou, r.Texto)
		}
	}
	if !strings.Contains(r.Texto, "[chave privada omitida: 4 linhas]") || !strings.Contains(r.Texto, "fim") {
		t.Errorf("saída:\n%s", r.Texto)
	}
}

func TestLerMascaraPorLinkParaChaveSSH(t *testing.T) {
	pasta := t.TempDir()
	escrever(t, filepath.Join(pasta, ".ssh", "id_rsa"), "senha: "+"valorSecreto123"+"\n")
	atalho := filepath.Join(pasta, "atalho")
	if err := os.Symlink(filepath.Join(pasta, ".ssh", "id_rsa"), atalho); err != nil {
		t.Fatal(err)
	}

	r, _ := lerArquivo(atalho, 1, 0, false)

	if strings.Contains(r.Texto, "valorSecreto123") {
		t.Errorf("o link escapou da máscara:\n%s", r.Texto)
	}
}

func TestLerMascaraAntesDeCortarEmPaginas(t *testing.T) {
	// O bloco PEM começa na linha 2 e a leitura pede a partir da 3: o miolo não
	// pode aparecer só porque o BEGIN ficou na página anterior.
	arquivo := filepath.Join(t.TempDir(), "notas.txt")
	escrever(t, arquivo, "um\n"+blocoPEM("MIOLOSECRETO1", "MIOLOSECRETO2")+"\ndepois\n")

	r, _ := lerArquivo(arquivo, 3, 0, false)

	if strings.Contains(r.Texto, "MIOLOSECRETO") {
		t.Errorf("o miolo da chave vazou numa leitura paginada:\n%s", r.Texto)
	}
}

func TestLerSemMascaraDevolveOTextoCruEOsOcultos(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), ".env")
	escrever(t, arquivo, "CHAVE=valorSecreto123\n")

	r, err := lerArquivo(arquivo, 1, 0, true)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(r.Texto, "CHAVE=valorSecreto123") {
		t.Errorf("revelar devia mostrar o valor:\n%s", r.Texto)
	}
	if len(r.Ocultos) != 1 || r.Ocultos[0] != "valorSecreto123" {
		t.Errorf("Ocultos = %v", r.Ocultos)
	}
}

// --- buscar (spec 2026-10-14 arquivos, seção 4 e D9) ---------------------------

func buscar(t *testing.T, raiz string, o opcoesBusca) string {
	t.Helper()
	saida, err := buscarArquivos(raiz, o)
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	return saida
}

func TestBuscarPorNomeTrechoSemCaixaEGlob(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "src", "Servidor.go"), "x")
	escrever(t, filepath.Join(raiz, "src", "util.go"), "x")
	escrever(t, filepath.Join(raiz, "docs", "LEIA-ME.md"), "x")

	porTrecho := buscar(t, raiz, opcoesBusca{Nome: "servidor"})
	if !strings.Contains(porTrecho, "Servidor.go") || strings.Contains(porTrecho, "util.go") {
		t.Errorf("por trecho:\n%s", porTrecho)
	}
	porGlob := buscar(t, raiz, opcoesBusca{Nome: "*.GO"})
	if !strings.Contains(porGlob, "Servidor.go") || !strings.Contains(porGlob, "util.go") || strings.Contains(porGlob, "LEIA-ME") {
		t.Errorf("por glob:\n%s", porGlob)
	}
}

func TestBuscarAcheiPastaPeloNomeTambem(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "projetos", "x.txt"), "x")

	saida := buscar(t, raiz, opcoesBusca{Nome: "projetos"})

	if !strings.Contains(saida, "pasta") || !strings.Contains(saida, "projetos") {
		t.Errorf("saida:\n%s", saida)
	}
}

func TestBuscarPorConteudoDevolveCaminhoLinhaETrecho(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "a.txt"), "nada\nUsa o JamesD aqui\nfim\n")
	escrever(t, filepath.Join(raiz, "b.txt"), "outra coisa\n")
	escrever(t, filepath.Join(raiz, "longa.txt"), "jamesd "+strings.Repeat("x", 400)+"\n")

	saida := buscar(t, raiz, opcoesBusca{Conteudo: "jamesd"})

	if !strings.Contains(saida, filepath.Join(raiz, "a.txt")+":2: Usa o JamesD aqui") {
		t.Errorf("faltou a linha de a.txt:\n%s", saida)
	}
	if strings.Contains(saida, "b.txt") {
		t.Error("b.txt não tem o texto e apareceu")
	}
	for _, l := range strings.Split(saida, "\n") {
		if strings.Contains(l, "longa.txt") && utf8.RuneCountInString(l) > len(raiz)+160+30 {
			t.Errorf("o trecho da linha longa não foi cortado em 160: %d caracteres", utf8.RuneCountInString(l))
		}
	}
}

func TestBuscarPulaBinarioEArquivoGrande(t *testing.T) {
	raiz := t.TempDir()
	if err := os.WriteFile(filepath.Join(raiz, "x.bin"), []byte("alvo\x00alvo"), 0o644); err != nil {
		t.Fatal(err)
	}
	escrever(t, filepath.Join(raiz, "enorme.txt"), "alvo\n"+strings.Repeat("y\n", 1_100_000)) // > 2 MB
	escrever(t, filepath.Join(raiz, "ok.txt"), "alvo\n")

	saida := buscar(t, raiz, opcoesBusca{Conteudo: "alvo"})

	if strings.Contains(saida, "x.bin") || strings.Contains(saida, "enorme.txt") || !strings.Contains(saida, "ok.txt") {
		t.Errorf("saida:\n%s", saida)
	}
}

func TestBuscarNomeEConteudoJuntos(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "a.go"), "alvo\n")
	escrever(t, filepath.Join(raiz, "a.txt"), "alvo\n")

	saida := buscar(t, raiz, opcoesBusca{Nome: "*.go", Conteudo: "alvo"})

	if !strings.Contains(saida, "a.go:1") || strings.Contains(saida, "a.txt") {
		t.Errorf("saida:\n%s", saida)
	}
}

func TestBuscarNaoLeVazarOValorDeUmSegredo(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, ".env"), "CLAUDE_API_KEY=valorSecreto123\nPORT=8080\n")

	porNome := buscar(t, raiz, opcoesBusca{Conteudo: "CLAUDE", IncluirOcultos: true})
	if !strings.Contains(porNome, ".env:1: CLAUDE_API_KEY="+marcaOculta) || strings.Contains(porNome, "valorSecreto123") {
		t.Errorf("buscar pelo nome da chave:\n%s", porNome)
	}

	// Sem oráculo: perguntar pelo próprio valor não confirma nem vaza nada.
	pelovalor := buscar(t, raiz, opcoesBusca{Conteudo: "valorSecreto123", IncluirOcultos: true})
	if strings.Contains(pelovalor, "valorSecreto123") || strings.Contains(pelovalor, ".env") {
		t.Errorf("buscar pelo valor virou oráculo:\n%s", pelovalor)
	}
	if !strings.Contains(pelovalor, "nenhum resultado") {
		t.Errorf("saida:\n%s", pelovalor)
	}
}

func TestBuscarMascaraCredencialEmArquivoComum(t *testing.T) {
	raiz := t.TempDir()
	chave := chaveAnthropicDeTeste()
	escrever(t, filepath.Join(raiz, "notas.txt"), "minha chave é "+chave+" ok\n")

	saida := buscar(t, raiz, opcoesBusca{Conteudo: "minha chave"})

	if strings.Contains(saida, chave) || !strings.Contains(saida, marcaOculta) {
		t.Errorf("saida:\n%s", saida)
	}
}

func TestBuscarPulaPastasDeCacheMasNaoSeOCaminhoEstaDentro(t *testing.T) {
	raiz := t.TempDir()
	for _, pasta := range []string{".git", "node_modules", ".venv", "__pycache__", ".cache", "vendor"} {
		escrever(t, filepath.Join(raiz, pasta, "dentro.txt"), "alvo\n")
	}
	escrever(t, filepath.Join(raiz, "src", "fora.txt"), "alvo\n")

	saida := buscar(t, raiz, opcoesBusca{Conteudo: "alvo", IncluirOcultos: true})
	if strings.Contains(saida, "dentro.txt") || !strings.Contains(saida, "fora.txt") {
		t.Errorf("devia pular as pastas de cache:\n%s", saida)
	}

	dentro := buscar(t, filepath.Join(raiz, "node_modules"), opcoesBusca{Conteudo: "alvo"})
	if !strings.Contains(dentro, "dentro.txt") {
		t.Errorf("pediu a pasta de cache e ela foi pulada:\n%s", dentro)
	}
}

func TestBuscarOcultosSoComPedido(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, ".oculto", "a.txt"), "alvo\n")
	escrever(t, filepath.Join(raiz, ".escondido.txt"), "alvo\n")

	if saida := buscar(t, raiz, opcoesBusca{Conteudo: "alvo"}); strings.Contains(saida, "a.txt") || strings.Contains(saida, "escondido") {
		t.Errorf("sem ocultos:\n%s", saida)
	}
	if saida := buscar(t, raiz, opcoesBusca{Conteudo: "alvo", IncluirOcultos: true}); !strings.Contains(saida, "a.txt") || !strings.Contains(saida, "escondido") {
		t.Errorf("com ocultos:\n%s", saida)
	}
}

func TestBuscarCorta100ResultadosEAvisa(t *testing.T) {
	raiz := t.TempDir()
	for i := 0; i < 130; i++ {
		escrever(t, filepath.Join(raiz, fmt.Sprintf("f%03d.txt", i)), "alvo\n")
	}

	saida := buscar(t, raiz, opcoesBusca{Conteudo: "alvo"})

	if n := strings.Count(saida, "alvo"); n != 100 {
		t.Errorf("%d resultados, queria 100", n)
	}
	if !strings.Contains(saida, "cortada em 100 resultados") {
		t.Errorf("sem aviso de corte:\n...%s", saida[len(saida)-120:])
	}
}

func TestBuscarCortaPorTempoEAvisa(t *testing.T) {
	raiz := t.TempDir()
	for i := 0; i < 10; i++ {
		escrever(t, filepath.Join(raiz, fmt.Sprintf("f%d.txt", i)), "alvo\n")
	}
	instante := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	chamadas := 0
	relogio := func() time.Time {
		chamadas++
		if chamadas > 5 {
			instante = instante.Add(6 * time.Second)
		}
		return instante
	}

	saida := buscar(t, raiz, opcoesBusca{Conteudo: "alvo", agora: relogio})

	if !strings.Contains(saida, "5 s") {
		t.Errorf("sem aviso de tempo:\n%s", saida)
	}
}

func TestBuscarProfundidadeMaximaDe8(t *testing.T) {
	raiz := t.TempDir()
	fundo := raiz
	for i := 0; i < 10; i++ {
		fundo = filepath.Join(fundo, fmt.Sprintf("n%d", i))
	}
	escrever(t, filepath.Join(fundo, "fundo.txt"), "alvo\n")
	escrever(t, filepath.Join(raiz, "n0", "n1", "raso.txt"), "alvo\n")

	saida := buscar(t, raiz, opcoesBusca{Conteudo: "alvo"})

	if strings.Contains(saida, "fundo.txt") || !strings.Contains(saida, "raso.txt") {
		t.Errorf("saida:\n%s", saida)
	}
}

func TestBuscarNaoSegueLinkDePastaEPulaPastaSemPermissao(t *testing.T) {
	raiz := t.TempDir()
	escrever(t, filepath.Join(raiz, "real", "a.txt"), "alvo\n")
	if err := os.Symlink(raiz, filepath.Join(raiz, "ciclo")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(raiz, "real"), filepath.Join(raiz, "atalho")); err != nil {
		t.Fatal(err)
	}
	if os.Geteuid() != 0 {
		fechada := filepath.Join(raiz, "fechada")
		if err := os.Mkdir(fechada, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(fechada, 0o755) })
	}

	saida := buscar(t, raiz, opcoesBusca{Conteudo: "alvo"})

	if n := strings.Count(saida, "a.txt"); n != 1 {
		t.Errorf("a.txt apareceu %d vezes (link seguido?):\n%s", n, saida)
	}
}

func TestBuscarSemNomeNemConteudoEErro(t *testing.T) {
	if _, err := buscarArquivos(t.TempDir(), opcoesBusca{}); err == nil || !strings.Contains(err.Error(), "nome ou conteudo") {
		t.Errorf("err = %v", err)
	}
	arquivo := filepath.Join(t.TempDir(), "a.txt")
	escrever(t, arquivo, "x")
	if _, err := buscarArquivos(arquivo, opcoesBusca{Nome: "a"}); err == nil || !strings.Contains(err.Error(), "use ler") {
		t.Errorf("arquivo como raiz: %v", err)
	}
}

func TestBuscarSemResultado(t *testing.T) {
	saida := buscar(t, t.TempDir(), opcoesBusca{Nome: "nadaaqui"})
	if !strings.Contains(saida, "nenhum resultado") {
		t.Errorf("saida = %q", saida)
	}
}
