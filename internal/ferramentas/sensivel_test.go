package ferramentas

import (
	"os"
	"path/filepath"
	"testing"
)

// Spec 2026-10-14 arquivos, D8.

func TestCaminhoSensivelCasaOsArquivosDeSegredo(t *testing.T) {
	for _, caminho := range []string{
		"/h/a/.ssh/id_rsa", "/h/a/.ssh/config", "/h/a/.ssh/known_hosts",
		"/x/.env", "/x/.env.local", "/x/.env.production", "/x/proj/.env",
		"/h/a/.aws/credentials", "/h/a/.gnupg/pubring.kbx", "/h/a/.netrc",
		"/h/a/.git-credentials", "/h/a/.npmrc", "/h/a/.pypirc", "/h/a/.kube/config",
		"/h/a/.docker/config.json",
		"/x/cert.pem", "/x/chave.key", "/x/c.p12", "/x/c.pfx", "/x/CERT.PEM",
		"/h/a/id_rsa", "/h/a/id_ed25519", "/h/a/id_ecdsa.pub",
		"/etc/shadow", "/etc/gshadow", "/etc/sudoers", "/etc/sudoers.d/x",
	} {
		if !CaminhoSensivel(caminho) {
			t.Errorf("CaminhoSensivel(%q) = false, queria true", caminho)
		}
	}
}

func TestCaminhoSensivelNaoCasaOQueSoParece(t *testing.T) {
	for _, caminho := range []string{
		"/x/.env.example", "/x/.env.sample", "/x/.env.template", "/x/.ENV.EXAMPLE",
		"/x/.venv/bin/python", "/x/ambiente.go", "/x/keyboard.go", "/x/monkey.txt",
		"/x/README.md", "/h/a/projetos/id_cartao.txt", "/etc/hosts", "/etc/passwd",
		"/x/chave.keyboard", "/x/pem.go",
	} {
		if CaminhoSensivel(caminho) {
			t.Errorf("CaminhoSensivel(%q) = true, queria false", caminho)
		}
	}
}

func TestCaminhoSensivelOlhaODestinoDoLink(t *testing.T) {
	pasta := t.TempDir()
	ssh := filepath.Join(pasta, ".ssh")
	if err := os.Mkdir(ssh, 0o700); err != nil {
		t.Fatal(err)
	}
	segredo := filepath.Join(ssh, "id_rsa")
	comum := filepath.Join(pasta, "README.md")
	for _, arquivo := range []string{segredo, comum} {
		if err := os.WriteFile(arquivo, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	atalhoSegredo := filepath.Join(pasta, "atalho")
	atalhoComum := filepath.Join(pasta, "atalho2")
	quebrado := filepath.Join(pasta, "quebrado")
	for atalho, alvo := range map[string]string{atalhoSegredo: segredo, atalhoComum: comum, quebrado: filepath.Join(pasta, "nao-existe")} {
		if err := os.Symlink(alvo, atalho); err != nil {
			t.Fatal(err)
		}
	}

	if !CaminhoSensivel(atalhoSegredo) {
		t.Error("link para .ssh/id_rsa não foi tratado como sensível")
	}
	if CaminhoSensivel(atalhoComum) {
		t.Error("link para um arquivo comum foi tratado como sensível")
	}
	if CaminhoSensivel(quebrado) {
		t.Error("link quebrado foi tratado como sensível")
	}
}

func TestCaminhoSensivelVazioEFalso(t *testing.T) {
	if CaminhoSensivel("") {
		t.Error("caminho vazio é sensível")
	}
}
