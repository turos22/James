package permissao

import (
	"testing"

	"james/internal/ferramentas"
)

func TestPerfilPode(t *testing.T) {
	casos := []struct {
		nome   string
		perfil Perfil
		nivel  ferramentas.Nivel
		quer   bool
	}{
		{"privilegiado executa seguro", Privilegiado, ferramentas.NivelSeguro, true},
		{"privilegiado executa perigoso", Privilegiado, ferramentas.NivelPerigoso, true},
		{"restrito executa seguro", Restrito, ferramentas.NivelSeguro, true},
		{"restrito não executa perigoso sem aprovação", Restrito, ferramentas.NivelPerigoso, false},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := c.perfil.Pode(c.nivel); got != c.quer {
				t.Errorf("Pode() = %v, queria %v", got, c.quer)
			}
		})
	}
}

func TestResolverPerfil(t *testing.T) {
	casos := []struct {
		nome           string
		enderecoRemoto string
		quer           Perfil
	}{
		{"loopback IPv4 é privilegiado", "127.0.0.1:54321", Privilegiado},
		{"loopback IPv6 é privilegiado", "[::1]:54321", Privilegiado},
		{"IP do tailnet é restrito", "100.96.219.29:54321", Restrito},
		{"IP de LAN é restrito", "192.168.15.23:54321", Restrito},
		{"endereço malformado é restrito por padrão (fail closed)", "não-é-um-endereço", Restrito},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := ResolverPerfil(c.enderecoRemoto); got != c.quer {
				t.Errorf("ResolverPerfil(%q) = %v, queria %v", c.enderecoRemoto, got, c.quer)
			}
		})
	}
}
