package ferramentas

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Spec 2026-10-14 arquivos, D5 e D13.

func comHomeDeTeste(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	// O diretório temporário pode estar atrás de um link (/tmp, /var/folders):
	// a home de teste é o destino real, como a de verdade.
	if real, err := filepath.EvalSymlinks(home); err == nil {
		home = real
	}
	t.Setenv("HOME", home)
	return home
}

func TestResolverCaminhoExpandeATilde(t *testing.T) {
	home := comHomeDeTeste(t)

	casos := map[string]string{
		"~":           home,
		"~/":          home,
		"~/x":         filepath.Join(home, "x"),
		"~/a/../b":    filepath.Join(home, "b"),
		"/abs/x":      "/abs/x",
		"/a/../b":     "/b",
		"/abs/x/":     "/abs/x",
		"/abs//x/./y": "/abs/x/y",
		"  /abs/x  ":  "/abs/x",
	}
	for entrada, esperado := range casos {
		got, err := ResolverCaminho(entrada)
		if err != nil || got != esperado {
			t.Errorf("ResolverCaminho(%q) = %q, %v; queria %q", entrada, got, err, esperado)
		}
	}
}

func TestResolverCaminhoRecusaRelativoEOutroUsuario(t *testing.T) {
	comHomeDeTeste(t)
	for _, entrada := range []string{"", "   ", "x/y", "./x", "../x", "x", "~outro/x", "~outro"} {
		_, err := ResolverCaminho(entrada)
		if err == nil {
			t.Errorf("ResolverCaminho(%q) aceitou", entrada)
			continue
		}
		if !strings.Contains(err.Error(), "caminho completo") {
			t.Errorf("ResolverCaminho(%q): a mensagem não orienta: %v", entrada, err)
		}
	}
}

func TestCaminhoProtegidoPisoDeSistemaEHome(t *testing.T) {
	home := comHomeDeTeste(t)

	for _, caminho := range []string{
		"/", "/etc", "/etc/", "/usr", "/var", "/boot", "/bin", "/sbin", "/lib", "/lib64",
		"/dev", "/proc", "/sys", "/opt", "/srv", "/root", home, home + "/",
	} {
		if !CaminhoProtegido(caminho) {
			t.Errorf("CaminhoProtegido(%q) = false, queria true", caminho)
		}
	}
	for _, caminho := range []string{
		"/etc/hosts", "/usr/local/x", "/var/log/x", "/tmp/x", "/opt/x",
		filepath.Join(home, "projeto"), filepath.Join(home, ".config"), "/home/outro",
	} {
		if CaminhoProtegido(caminho) {
			t.Errorf("CaminhoProtegido(%q) = true, queria false", caminho)
		}
	}
}

func TestCaminhoProtegidoOlhaODestinoDoLink(t *testing.T) {
	home := comHomeDeTeste(t)
	alvoSistema := filepath.Join(home, "para-etc")
	alvoHome := filepath.Join(home, "casa")
	comum := filepath.Join(home, "projeto")
	if err := os.Mkdir(comum, 0o755); err != nil {
		t.Fatal(err)
	}
	atalhoComum := filepath.Join(home, "atalho-projeto")
	for atalho, alvo := range map[string]string{alvoSistema: "/etc", alvoHome: home, atalhoComum: comum} {
		if err := os.Symlink(alvo, atalho); err != nil {
			t.Fatal(err)
		}
	}

	if !CaminhoProtegido(alvoSistema) {
		t.Error("um link para /etc escapou do piso")
	}
	if !CaminhoProtegido(alvoHome) {
		t.Error("um link para a home escapou do piso")
	}
	if CaminhoProtegido(atalhoComum) {
		t.Error("um link para uma pasta de projeto foi tratado como protegido")
	}
}

func TestCaminhoProtegidoSeOCaminhoNaoExisteOlhaSoOPedido(t *testing.T) {
	comHomeDeTeste(t)
	if CaminhoProtegido("/tmp/nao/existe/mesmo") {
		t.Error("caminho inexistente fora do piso foi protegido")
	}
	if !CaminhoProtegido("/etc/") {
		t.Error("/etc com barra no fim escapou")
	}
}

func TestCaminhoProtegidoSemHomeNaoQuebra(t *testing.T) {
	t.Setenv("HOME", "")
	if CaminhoProtegido("/tmp/x") {
		t.Error("sem HOME, /tmp/x foi protegido")
	}
	if !CaminhoProtegido("/etc") {
		t.Error("sem HOME, /etc deixou de ser protegido")
	}
}
