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
