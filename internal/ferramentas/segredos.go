package ferramentas

import (
	"strings"
	"sync"
	"time"
)

// SegredosLidos guarda os valores secretos que o Arthur mandou REVELAR (spec
// 2026-10-14 arquivos, D7 e D11) para o salvar_na_memoria recusar uma nota que
// os contenha. A máscara (mascara.go) já impede o caso comum; esta trava cobre
// o valor revelado de propósito.
//
// Vale por 1 hora (revelar de novo renova) e guarda no máximo 500 valores, o
// mais antigo sai primeiro. Valor com menos de 6 caracteres não entra: seria
// falso positivo constante (portas, true, números).
type SegredosLidos struct {
	mu       sync.Mutex
	entradas []entradaDeSegredo
	agora    func() time.Time
}

type entradaDeSegredo struct {
	valor string
	desde time.Time
}

const (
	validadeDoSegredo = time.Hour
	tetoDeSegredos    = 500
	minimoDoSegredo   = 6
)

func NovoSegredosLidos() *SegredosLidos {
	return &SegredosLidos{agora: time.Now}
}

// Registrar guarda os valores. Um *SegredosLidos nil não guarda nada.
func (s *SegredosLidos) Registrar(valores []string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	agora := s.agora()
	s.expirarLocked(agora)
	for _, v := range valores {
		if len(v) < minimoDoSegredo {
			continue
		}
		renovado := false
		for i := range s.entradas {
			if s.entradas[i].valor == v {
				s.entradas[i].desde = agora
				renovado = true
				break
			}
		}
		if !renovado {
			s.entradas = append(s.entradas, entradaDeSegredo{valor: v, desde: agora})
		}
	}
	if excesso := len(s.entradas) - tetoDeSegredos; excesso > 0 {
		s.entradas = append([]entradaDeSegredo(nil), s.entradas[excesso:]...)
	}
}

// Contem diz se o texto tem algum valor guardado (comparação exata).
func (s *SegredosLidos) Contem(texto string) bool {
	if s == nil || texto == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.expirarLocked(s.agora())
	for _, e := range s.entradas {
		if strings.Contains(texto, e.valor) {
			return true
		}
	}
	return false
}

func (s *SegredosLidos) tamanho() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entradas)
}

func (s *SegredosLidos) expirarLocked(agora time.Time) {
	validas := s.entradas[:0]
	for _, e := range s.entradas {
		if agora.Sub(e.desde) <= validadeDoSegredo {
			validas = append(validas, e)
		}
	}
	s.entradas = validas
}
