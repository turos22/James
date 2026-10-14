package ferramentas

import (
	"strings"
	"testing"
)

// Spec 2026-10-14 arquivos, D6. As credenciais são montadas em tempo de
// execução para a varredura do repositório não acusar este arquivo.

func chaveAnthropicDeTeste() string {
	return "sk-" + "ant-" + "api03-" + "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"
}

func blocoPEM(corpo ...string) string {
	linhas := append([]string{"-----BEGIN " + "RSA PRIVATE KEY-----"}, corpo...)
	linhas = append(linhas, "-----END "+"RSA PRIVATE KEY-----")
	return strings.Join(linhas, "\n")
}

const oculto4 = "••••"

func TestMascararCredencialReconhecivelEmQualquerArquivo(t *testing.T) {
	chave := chaveAnthropicDeTeste()
	entrada := "linha 1\nusa a chave " + chave + " aqui\nlinha 3\n"

	saida, ocultos := Mascarar(entrada, false)

	if strings.Contains(saida, chave) {
		t.Fatalf("a chave sobrou na saída: %q", saida)
	}
	if saida != "linha 1\nusa a chave "+oculto4+" aqui\nlinha 3\n" {
		t.Errorf("o resto do texto mudou: %q", saida)
	}
	if len(ocultos) != 1 || ocultos[0] != chave {
		t.Errorf("ocultos = %v", ocultos)
	}
}

func TestMascararSenhaDeUrlMantemOResto(t *testing.T) {
	saida, ocultos := Mascarar("DATABASE_URL=postgres://james:"+"segredo123"+"@localhost:5434/james?sslmode=disable", false)

	if saida != "DATABASE_URL=postgres://james:"+oculto4+"@localhost:5434/james?sslmode=disable" {
		t.Errorf("saida = %q", saida)
	}
	if len(ocultos) != 1 || ocultos[0] != "segredo123" {
		t.Errorf("ocultos = %v", ocultos)
	}
}

func TestMascararBlocoPEMInteiro(t *testing.T) {
	entrada := "antes\n" + blocoPEM("MIIEvQIBADANBgkqhkiG9w0BAQEFAASC", "KgwggSkAgEAAoIBAQC7VJTUt9Us8cKj", "fim==") + "\ndepois\n"

	saida, ocultos := Mascarar(entrada, false)

	if saida != "antes\n[chave privada omitida: 5 linhas]\ndepois\n" {
		t.Errorf("saida = %q", saida)
	}
	if strings.Contains(saida, "MIIEvQ") || strings.Contains(saida, "KgwggSk") {
		t.Error("o corpo da chave sobrou")
	}
	if len(ocultos) != 3 {
		t.Errorf("ocultos = %v, queria as três linhas do corpo", ocultos)
	}
}

func TestMascararPEMSemFimVaiAteOFimDoTexto(t *testing.T) {
	// A leitura em pedaços pode cortar o bloco no meio.
	entrada := "ok\n-----BEGIN " + "OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAA\nQQQQ"

	saida, _ := Mascarar(entrada, false)

	if strings.Contains(saida, "b3BlbnNz") || strings.Contains(saida, "QQQQ") {
		t.Errorf("o corpo do bloco cortado vazou: %q", saida)
	}
	if !strings.HasPrefix(saida, "ok\n[chave privada omitida") {
		t.Errorf("saida = %q", saida)
	}
}

func TestMascararArquivoSensivelEscondeOValorDeCadaLinha(t *testing.T) {
	casos := []struct{ entrada, esperado, oculto string }{
		{"CHAVE=valor1", "CHAVE=" + oculto4 + " (oculto, 6 caracteres)", "valor1"},
		{"export CHAVE=\"valor1\"", "export CHAVE=\"" + oculto4 + " (oculto, 6 caracteres)\"", "valor1"},
		{"CHAVE='abc def'", "CHAVE='" + oculto4 + " (oculto, 7 caracteres)'", "abc def"},
		{"CHAVE = valor1", "CHAVE = " + oculto4 + " (oculto, 6 caracteres)", "valor1"},
		{"CHAVE=valor1 # nota", "CHAVE=" + oculto4 + " (oculto, 6 caracteres) # nota", "valor1"},
		{"senha: valor1", "senha: " + oculto4 + " (oculto, 6 caracteres)", "valor1"},
		{"  senha: \"valor1\"", "  senha: \"" + oculto4 + " (oculto, 6 caracteres)\"", "valor1"},
		{`{"token": "valor1", "outro": 3}`, `{"token": "` + oculto4 + ` (oculto, 6 caracteres)", "outro": 3}`, "valor1"},
		{"machine x login y password valor1", "machine x login y password " + oculto4 + " (oculto, 6 caracteres)", "valor1"},
		{"//registry.npmjs.org/:_authToken=valor1", "//registry.npmjs.org/:_authToken=" + oculto4 + " (oculto, 6 caracteres)", "valor1"},
		{"PORT=80", "PORT=" + oculto4 + " (oculto, 2 caracteres)", "80"},
	}
	for _, c := range casos {
		saida, ocultos := Mascarar(c.entrada, true)
		if saida != c.esperado {
			t.Errorf("entrada %q\n  saída    %q\n  esperada %q", c.entrada, saida, c.esperado)
		}
		if len(ocultos) != 1 || ocultos[0] != c.oculto {
			t.Errorf("entrada %q: ocultos = %v, queria [%q]", c.entrada, ocultos, c.oculto)
		}
	}
}

func TestMascararArquivoSensivelDeixaComentarioVazioESemValor(t *testing.T) {
	entrada := "# Credenciais do James\n\nCHAVE=\n# só uma nota sobre o arquivo\nOUTRA=x\n"

	saida, _ := Mascarar(entrada, true)

	for _, linha := range []string{"# Credenciais do James", "CHAVE=", "# só uma nota sobre o arquivo"} {
		if !strings.Contains(saida, linha+"\n") {
			t.Errorf("a linha %q mudou: %q", linha, saida)
		}
	}
	if strings.Contains(saida, "OUTRA=x") {
		t.Errorf("OUTRA=x devia estar mascarada: %q", saida)
	}
}

func TestMascararChaveComentadaEmArquivoSensivelTambem(t *testing.T) {
	// Chave antiga deixada comentada continua sendo chave.
	saida, ocultos := Mascarar("# CLAUDE_API_KEY=valorAntigo123\n", true)

	if strings.Contains(saida, "valorAntigo123") {
		t.Errorf("a chave comentada vazou: %q", saida)
	}
	if !strings.HasPrefix(saida, "# CLAUDE_API_KEY=") || len(ocultos) != 1 {
		t.Errorf("saida = %q, ocultos = %v", saida, ocultos)
	}
}

func TestMascararArquivoComumDeixaConfiguracaoVisivel(t *testing.T) {
	entrada := "PORT=8080\nDEBUG=true\nhost: localhost\n"

	saida, ocultos := Mascarar(entrada, false)

	if saida != entrada || len(ocultos) != 0 {
		t.Errorf("saida = %q, ocultos = %v", saida, ocultos)
	}
}

func TestMascararOcultosSemRepeticaoNemPlaceholder(t *testing.T) {
	_, ocultos := Mascarar("A=repetido\nB=repetido\nC=outro123\n", true)

	if len(ocultos) != 2 || ocultos[0] != "repetido" || ocultos[1] != "outro123" {
		t.Errorf("ocultos = %v", ocultos)
	}
}

func TestMascararEEstavel(t *testing.T) {
	entrada := "A=valor1\n" + "url=postgres://u:" + "senha99" + "@h/db\n" + blocoPEM("AAAABBBBCCCC") + "\n"
	uma, _ := Mascarar(entrada, true)

	duas, ocultos := Mascarar(uma, true)

	if uma != duas {
		t.Errorf("a segunda passada mudou o texto:\n%q\n%q", uma, duas)
	}
	if len(ocultos) != 0 {
		t.Errorf("a segunda passada achou valores onde só há máscara: %v", ocultos)
	}
}

func TestMascararTextoSemSegredoPassaIntacto(t *testing.T) {
	entrada := "func main() {\n\tfmt.Println(\"oi\") // senha: nada aqui\n}\n"

	if saida, ocultos := Mascarar(entrada, false); saida != entrada || len(ocultos) != 0 {
		t.Errorf("saida = %q, ocultos = %v", saida, ocultos)
	}
}

func TestMascararVariasCredenciaisSaemNaOrdemDoTexto(t *testing.T) {
	a := "ghp" + "_" + strings.Repeat("a1", 20)
	b := chaveAnthropicDeTeste()

	saida, ocultos := Mascarar("primeiro "+a+" e depois "+b+" fim", false)

	if saida != "primeiro "+oculto4+" e depois "+oculto4+" fim" {
		t.Errorf("saida = %q", saida)
	}
	if len(ocultos) != 2 || ocultos[0] != a || ocultos[1] != b {
		t.Errorf("ocultos = %v", ocultos)
	}
}
