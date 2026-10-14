package higiene

import (
	"os"
	"path/filepath"
	"strings"
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

func TestVarrerIgnoraArquivoEnv(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, ".env")
	if err := os.WriteFile(arquivo, []byte("TAVILY_API_KEY="+chaveFalsa()), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf(".env deveria ser ignorado — é onde segredo local mora por design, "+
			"e já está protegido pelo .gitignore; vieram %d achados: %+v", len(achados), achados)
	}
}

func TestVarrerIgnoraDiretorioCerts(t *testing.T) {
	raiz := t.TempDir()
	if err := os.Mkdir(filepath.Join(raiz, "certs"), 0o700); err != nil {
		t.Fatalf("não consegui criar o diretório certs: %v", err)
	}
	arquivo := filepath.Join(raiz, "certs", "tailscale.key")
	// Quebrado em duas literais pra não bater no próprio regex de PEM
	// quando este arquivo de teste é varrido por TestRepositorioLimpo —
	// mesmo espírito de TestVarrerAcusaBlocoPEM acima.
	pem := "-----BEGIN " + "PRIVATE KEY-----\nfake\n-----END PRIVATE KEY-----"
	if err := os.WriteFile(arquivo, []byte(pem), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf("certs/ deveria ser ignorado — certificado TLS da VPN gerado localmente por "+
			"máquina, já protegido pelo .gitignore; vieram %d achados: %+v", len(achados), achados)
	}
}

func TestVarrerNaoIgnoraArquivoParecidoComEnv(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, ".env.production")
	if err := os.WriteFile(arquivo, []byte("TAVILY_API_KEY="+chaveFalsa()), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Errorf(".env.production não é .env — a exclusão é por nome exato, "+
			"este arquivo deveria continuar sendo varrido; vieram %d achados: %+v", len(achados), achados)
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

func TestVarrerAcusaConnectionStringComSenha(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "config_real.toml")
	conteudo := "url = \"" + "postgres" + "://james:" + "SegredoReal123" + "@localhost:5433/james\""
	if err := os.WriteFile(arquivo, []byte(conteudo), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado de connection string, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaAWSAccessKey(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "notas.txt")
	chave := "AKIA" + "IOSFODNN7EXAMPLE"
	if err := os.WriteFile(arquivo, []byte(chave), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado de AWS key, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaTokenGitHub(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "notas.txt")
	token := "ghp_" + "000000000000000000000000000000000000"
	if err := os.WriteFile(arquivo, []byte(token), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado de token GitHub, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaJWT(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "notas.txt")
	jwt := "eyJhbGciOiJIUzI1NiJ9" + "." + "eyJzdWIiOiIxMjM0NTY3ODkwIn0" + "." + "dGVzdHNpZ25hdHVyZQ"
	if err := os.WriteFile(arquivo, []byte(jwt), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado de JWT, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaBlocoPEM(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "chave.txt")
	// Quebrado em duas literais pra não bater no próprio regex de PEM
	// quando este arquivo de teste é varrido por TestRepositorioLimpo —
	// mesmo espírito de chaveFalsa() acima.
	pem := "-----BEGIN " + "RSA PRIVATE KEY-----\nMIIExemplo\n-----END RSA PRIVATE KEY-----"
	if err := os.WriteFile(arquivo, []byte(pem), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado de bloco PEM, vieram %d: %+v", len(achados), achados)
	}
}

func TestVarrerIgnoraEnvExample(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, ".env.example")
	// Quebrado em mais literais que o necessário pro conteúdo em si (o
	// arquivo .env.example é ignorado por nome de qualquer forma), mas
	// assim o texto-fonte deste teste não bate no próprio regex de
	// connection-string quando TestRepositorioLimpo varre este arquivo.
	conteudo := "DATABASE_URL=" + "postgres" + "://james:" + "SENHA@localhost:5433/james"
	if err := os.WriteFile(arquivo, []byte(conteudo), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf(".env.example é template, deveria ser ignorado; vieram %d achados: %+v", len(achados), achados)
	}
}

func TestVarrerIgnoraConnectionStringEmArquivoDeTeste(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "conexao_test.go")
	conteudo := `url := "` + "postgres" + `://x:` + `y@localhost:5433/james"`
	if err := os.WriteFile(arquivo, []byte(conteudo), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf("DSN de mentira em arquivo _test.go deveria ser ignorada; vieram %d achados: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaAWSKeyMesmoEmArquivoDeTeste(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "conexao_test.go")
	chave := "AKIA" + "IOSFODNN7EXAMPLE"
	if err := os.WriteFile(arquivo, []byte(chave), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("padrões que não são connection-string continuam varrendo _test.go; esperava 1 achado, vieram %d: %+v", len(achados), achados)
	}
}

// O venv do device (dispositivo/.venv) é dependência de terceiros, fora do
// git. SpeechRecognition embute uma chave pública do Google e o urllib3 tem
// exemplos de URL com senha: ambos são falso positivo.
func TestVarrerIgnoraVenvPython(t *testing.T) {
	raiz := t.TempDir()
	pasta := filepath.Join(raiz, "dispositivo", ".venv", "lib")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatalf("não consegui criar o venv: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pasta, "vendido.py"), []byte(chaveFalsa()), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf(".venv deveria ser ignorado, vieram %d achados: %+v", len(achados), achados)
	}
}

// .executar/ guarda logs, o binário do jamesd e o perfil do Chrome do
// executar.sh (fora do git). O perfil do Chrome tem chaves públicas do
// Google embutidas: falso positivo.
func TestVarrerIgnoraPastaDeExecucao(t *testing.T) {
	raiz := t.TempDir()
	pasta := filepath.Join(raiz, ".executar", "chrome-perfil")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatalf("não consegui criar a pasta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pasta, "000003.log"), []byte(chaveFalsa()), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf(".executar deveria ser ignorado, vieram %d achados: %+v", len(achados), achados)
	}
}

// __pycache__/ é bytecode gerado ao rodar o device (fora do git). Ele carrega
// as strings do código-fonte, inclusive exemplos de URL com senha: falso
// positivo.
func TestVarrerIgnoraBytecodePython(t *testing.T) {
	raiz := t.TempDir()
	pasta := filepath.Join(raiz, "dispositivo", "james_device", "__pycache__")
	if err := os.MkdirAll(pasta, 0o755); err != nil {
		t.Fatalf("não consegui criar a pasta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pasta, "x.cpython-313.pyc"), []byte(chaveFalsa()), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf("__pycache__ deveria ser ignorado, vieram %d achados: %+v", len(achados), achados)
	}
}

// --- TrechosDeCredencial / ParecaCredencial (spec 2026-10-14 arquivos, D17) ---

// chaveAnthropic monta uma chave no formato real em tempo de execução, pelo
// mesmo motivo de chaveFalsa: o literal não pode aparecer neste arquivo.
func chaveAnthropic() string {
	return "sk-" + "ant-" + "api03-" + "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"
}

func TestTrechosDeCredencialAchaCadaFormato(t *testing.T) {
	casos := map[string]string{
		"google":     chaveFalsa(),
		"tavily":     "tvly" + "-" + "abcdef1234567890",
		"nvidia":     "nvapi" + "-" + "abcdef1234567890",
		"openrouter": "sk-or-" + "v1-" + "abcdef1234567890",
		"anthropic":  chaveAnthropic(),
		"aws":        "AKIA" + "ABCDEFGHIJKLMNOP",
		"github":     "ghp" + "_" + strings.Repeat("a1", 20),
		"jwt":        "eyJ" + "hbGciOi.eyJzdWIiOi.abc123DEF",
		"pem":        "-----BEGIN " + "RSA PRIVATE KEY-----",
	}
	for nome, credencial := range casos {
		texto := "antes " + credencial + " depois"
		trechos := TrechosDeCredencial(texto)
		if len(trechos) == 0 {
			t.Errorf("%s: nenhum trecho achado em %q", nome, texto)
			continue
		}
		if !ParecaCredencial(texto) {
			t.Errorf("%s: ParecaCredencial devolveu false", nome)
		}
		for _, tr := range trechos {
			if tr[0] < 0 || tr[1] > len(texto) || tr[0] >= tr[1] {
				t.Errorf("%s: trecho inválido %v em um texto de %d bytes", nome, tr, len(texto))
			}
		}
	}
}

func TestTrechoDaChaveVaiDoPrefixoAteOFimDoToken(t *testing.T) {
	chave := chaveAnthropic()
	texto := `CLAUDE_API_KEY="` + chave + `" # comentário`

	trechos := TrechosDeCredencial(texto)

	if len(trechos) != 1 {
		t.Fatalf("trechos = %v", trechos)
	}
	if got := texto[trechos[0][0]:trechos[0][1]]; got != chave {
		t.Errorf("trecho = %q, queria exatamente a chave", got)
	}
}

func TestTrechoDaUrlComSenhaEASenha(t *testing.T) {
	texto := "postgres://james:" + "segredo123" + "@localhost:5434/james"

	trechos := TrechosDeCredencial(texto)

	if len(trechos) != 1 {
		t.Fatalf("trechos = %v", trechos)
	}
	if got := texto[trechos[0][0]:trechos[0][1]]; got != "segredo123" {
		t.Errorf("trecho = %q, queria só a senha", got)
	}
}

func TestTextoComumNaoParecemCredencial(t *testing.T) {
	for _, texto := range []string{
		"o projeto usa uma chave de API",
		"localhost:8080",
		"password",
		"https://exemplo.com/a?b=1",
		"sk-",
		"sk-ant-teste", // fixture curta de teste: não tem o corpo de uma chave real
		"",
	} {
		if ParecaCredencial(texto) {
			t.Errorf("ParecaCredencial(%q) = true, queria false", texto)
		}
	}
}

func TestVarrerAcusaChaveDaAnthropic(t *testing.T) {
	raiz := t.TempDir()
	if err := os.WriteFile(filepath.Join(raiz, "x.go"), []byte(`const k = "`+chaveAnthropic()+`"`), 0o600); err != nil {
		t.Fatal(err)
	}

	achados, err := Varrer(raiz)

	if err != nil || len(achados) != 1 || achados[0].Padrao != "chave-anthropic" {
		t.Errorf("achados = %+v, err = %v", achados, err)
	}
}
