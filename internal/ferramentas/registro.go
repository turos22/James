package ferramentas

import (
	"fmt"
	"sync"
)

type Registro interface {
	Registrar(f Ferramenta) error
	Buscar(nome string) (Ferramenta, bool)
	Declaracoes() []Declaracao
}

type registro struct {
	mu          sync.RWMutex
	ferramentas map[string]Ferramenta
}

// NovoRegistro devolve um Registro vazio, seguro pra uso concorrente.
func NovoRegistro() Registro {
	return &registro{ferramentas: make(map[string]Ferramenta)}
}

func (r *registro) Registrar(f Ferramenta) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	nome := f.Declaracao().Nome
	if nome == "" {
		return fmt.Errorf("ferramenta sem nome não pode ser registrada")
	}
	if _, existe := r.ferramentas[nome]; existe {
		return fmt.Errorf("ferramenta %q já registrada", nome)
	}
	r.ferramentas[nome] = f
	return nil
}

func (r *registro) Buscar(nome string) (Ferramenta, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.ferramentas[nome]
	return f, ok
}

func (r *registro) Declaracoes() []Declaracao {
	r.mu.RLock()
	defer r.mu.RUnlock()
	decls := make([]Declaracao, 0, len(r.ferramentas))
	for _, f := range r.ferramentas {
		decls = append(decls, f.Declaracao())
	}
	return decls
}
