package main

import (
	"fmt"
	"net"
	"strings"
)

// interfacesDoSistema é var pra o teste poder substituir por uma lista
// falsa — net.Interface não é injetável de outro jeito.
var interfacesDoSistema = net.Interfaces

// resolverIPTailscale acha o IPv4 da interface do Tailscale em runtime.
// Nunca fixar esse IP em config: ele muda a cada relogin (spike de 04/08).
func resolverIPTailscale() (string, error) {
	ifaces, err := interfacesDoSistema()
	if err != nil {
		return "", fmt.Errorf("erro ao listar interfaces: %w", err)
	}
	for _, iface := range ifaces {
		if !strings.Contains(strings.ToLower(iface.Name), "tailscale") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			if ip4 := ipNet.IP.To4(); ip4 != nil {
				return ip4.String(), nil
			}
		}
	}
	return "", fmt.Errorf("interface do Tailscale não encontrada — ele está rodando e logado?")
}
