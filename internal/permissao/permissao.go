// Package permissao decide, por origem de conexão, se um cliente executa
// ferramentas perigosas sem aprovação humana.
package permissao

import (
	"net"

	"james/internal/ferramentas"
)

type Perfil string

const (
	Privilegiado Perfil = "privilegiado"
	Restrito     Perfil = "restrito"
)

// Pode informa se o perfil executa o nível sem aprovação humana.
func (p Perfil) Pode(n ferramentas.Nivel) bool {
	if p == Privilegiado {
		return true
	}
	return n == ferramentas.NivelSeguro
}

// ResolverPerfil decide o perfil a partir do endereço remoto de uma conexão
// HTTP/WS (net/http Request.RemoteAddr). Uma camada só: loopback é
// Privilegiado, qualquer outra origem é Restrito — sem lista de exceção por
// endereço (decisão P5: IP do Tailscale não é estável entre relogins).
// Endereço que não dá pra interpretar como IP fica Restrito por padrão
// (fail closed).
func ResolverPerfil(enderecoRemoto string) Perfil {
	host, _, err := net.SplitHostPort(enderecoRemoto)
	if err != nil {
		host = enderecoRemoto
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		return Privilegiado
	}
	return Restrito
}
