package ferramentas

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Caminhos das ferramentas de arquivo (spec 2026-10-14 arquivos, D5 e D13).

var errCaminhoRelativo = errors.New("diga o caminho completo (absoluto ou começando por ~)")

// ResolverCaminho aceita caminho absoluto ou começando por ~ e devolve o
// caminho limpo. Relativo é erro: o jamesd não tem uma "pasta atual" que o
// Arthur enxergue, e adivinhar de onde partir leva a mexer no lugar errado.
func ResolverCaminho(entrada string) (string, error) {
	caminho := strings.TrimSpace(entrada)
	if caminho == "" {
		return "", errCaminhoRelativo
	}
	if caminho == "~" || strings.HasPrefix(caminho, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return "", errors.New("não consegui descobrir a sua home; diga o caminho absoluto")
		}
		caminho = filepath.Join(home, strings.TrimPrefix(caminho, "~"))
	}
	if !filepath.IsAbs(caminho) {
		return "", errCaminhoRelativo
	}
	return filepath.Clean(caminho), nil
}

// pastasDeSistema são as pastas de topo que apagar ou mover não pode tocar,
// nem com aprovação.
var pastasDeSistema = []string{
	"/", "/etc", "/usr", "/var", "/boot", "/bin", "/sbin", "/lib", "/lib64",
	"/dev", "/proc", "/sys", "/opt", "/srv", "/root",
}

// CaminhoProtegido diz se o caminho é o piso que apagar e mover recusam: a
// raiz, a home inteira e as pastas de sistema de topo. Olha o caminho pedido E
// o destino real, se for link simbólico: um atalho para /etc não escapa.
// Subpastas (/etc/hosts, ~/projeto) NÃO são protegidas.
func CaminhoProtegido(caminho string) bool {
	if protegidoLiteral(filepath.Clean(caminho)) {
		return true
	}
	if real, err := filepath.EvalSymlinks(caminho); err == nil {
		return protegidoLiteral(filepath.Clean(real))
	}
	return false
}

func protegidoLiteral(limpo string) bool {
	for _, pasta := range pastasDeSistema {
		if limpo == pasta {
			return true
		}
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return false
	}
	if limpo == filepath.Clean(home) {
		return true
	}
	// A home pode ser ela mesma um link (comum em máquina com /home separado).
	if real, err := filepath.EvalSymlinks(home); err == nil && limpo == filepath.Clean(real) {
		return true
	}
	return false
}
