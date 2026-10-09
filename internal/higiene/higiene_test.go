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
