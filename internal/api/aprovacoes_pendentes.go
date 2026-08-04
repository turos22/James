package api

import "sync"

// registroAprovacoesPendentes casa a decisão que chega por "aprovar"/"negar"
// no WS com quem está bloqueado esperando em SolicitarAprovacao.
type registroAprovacoesPendentes struct {
	mu     sync.Mutex
	canais map[string]chan bool
}

func NovoRegistroAprovacoesPendentes() *registroAprovacoesPendentes {
	return &registroAprovacoesPendentes{canais: make(map[string]chan bool)}
}

func (r *registroAprovacoesPendentes) aguardar(id string) chan bool {
	c := make(chan bool, 1)
	r.mu.Lock()
	r.canais[id] = c
	r.mu.Unlock()
	return c
}

func (r *registroAprovacoesPendentes) resolver(id string, aprovado bool) bool {
	r.mu.Lock()
	c, ok := r.canais[id]
	if ok {
		delete(r.canais, id)
	}
	r.mu.Unlock()
	if !ok {
		return false
	}
	c <- aprovado
	return true
}
