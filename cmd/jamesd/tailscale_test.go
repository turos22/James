package main

import (
	"net"
	"testing"
)

func TestResolverIPTailscaleSemInterfaceDevolveErro(t *testing.T) {
	interfacesOriginal := interfacesDoSistema
	defer func() { interfacesDoSistema = interfacesOriginal }()

	interfacesDoSistema = func() ([]net.Interface, error) {
		return []net.Interface{{Name: "Ethernet"}, {Name: "Wi-Fi"}}, nil
	}

	_, err := resolverIPTailscale()
	if err == nil {
		t.Fatal("esperava erro quando não há interface do Tailscale, veio nil")
	}
}

func TestResolverIPTailscaleComInterfaceDownDevolveErro(t *testing.T) {
	interfacesOriginal := interfacesDoSistema
	defer func() { interfacesDoSistema = interfacesOriginal }()

	interfacesDoSistema = func() ([]net.Interface, error) {
		// Flags=0 (sem net.FlagUp) simula o Tailscale instalado mas
		// deslogado/desconectado — não deve tentar resolver Addrs() nessa
		// interface nem devolver um IP obsoleto de sessão anterior.
		return []net.Interface{{Name: "Tailscale", Flags: 0}}, nil
	}

	_, err := resolverIPTailscale()
	if err == nil {
		t.Fatal("esperava erro quando a interface do Tailscale está down, veio nil")
	}
}
