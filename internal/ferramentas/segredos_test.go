package ferramentas

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// Spec 2026-10-14 arquivos, D11.

func segredosComRelogio() (*SegredosLidos, *time.Time) {
	agora := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	s := NovoSegredosLidos()
	s.agora = func() time.Time { return agora }
	return s, &agora
}

func TestSegredosLidosContemOQueFoiRegistrado(t *testing.T) {
	s, _ := segredosComRelogio()
	s.Registrar([]string{"abcdef123456", "outro-valor-aqui"})

	if !s.Contem("a chave é abcdef123456 mesmo") {
		t.Error("não achou o valor registrado dentro do texto")
	}
	if s.Contem("OUTRO-valor-aqui") {
		t.Error("a comparação deve ser exata (segredo é sensível a caixa)")
	}
	if s.Contem("o projeto usa uma chave de API") {
		t.Error("texto sem nenhum valor registrado foi marcado")
	}
	if s.Contem("") {
		t.Error("texto vazio foi marcado")
	}
}

func TestSegredosLidosIgnoraValorCurto(t *testing.T) {
	s, _ := segredosComRelogio()
	s.Registrar([]string{"80", "true", "abc12"})

	if s.Contem("a porta 80 está true com abc12") {
		t.Error("valores com menos de 6 caracteres não podem travar a memória (falso positivo constante)")
	}
}

func TestSegredosLidosEsqueceDepoisDeUmaHora(t *testing.T) {
	s, agora := segredosComRelogio()
	s.Registrar([]string{"abcdef123456"})

	*agora = agora.Add(59 * time.Minute)
	if !s.Contem("abcdef123456") {
		t.Fatal("esqueceu antes da hora")
	}
	*agora = agora.Add(2 * time.Minute)
	if s.Contem("abcdef123456") {
		t.Error("o valor passou de 1 hora e ainda trava a memória")
	}
}

func TestSegredosLidosRenovaOPrazoQuandoORegistroSeRepete(t *testing.T) {
	s, agora := segredosComRelogio()
	s.Registrar([]string{"abcdef123456"})
	*agora = agora.Add(50 * time.Minute)
	s.Registrar([]string{"abcdef123456"})
	*agora = agora.Add(50 * time.Minute)

	if !s.Contem("abcdef123456") {
		t.Error("revelar de novo devia renovar o prazo")
	}
}

func TestSegredosLidosTemTetoESaiOMaisAntigo(t *testing.T) {
	s, agora := segredosComRelogio()
	for i := 0; i < tetoDeSegredos+10; i++ {
		*agora = agora.Add(time.Second)
		s.Registrar([]string{fmt.Sprintf("valor-%04d-secreto", i)})
	}

	if s.Contem("valor-0000-secreto") {
		t.Error("o mais antigo devia ter saído")
	}
	if !s.Contem(fmt.Sprintf("valor-%04d-secreto", tetoDeSegredos+9)) {
		t.Error("o mais novo devia estar lá")
	}
	if s.tamanho() > tetoDeSegredos {
		t.Errorf("tamanho = %d, teto = %d", s.tamanho(), tetoDeSegredos)
	}
}

func TestSegredosLidosEhSeguroParaUsoConcorrente(t *testing.T) {
	s := NovoSegredosLidos()
	var grupo sync.WaitGroup
	for g := 0; g < 8; g++ {
		grupo.Add(1)
		go func(g int) {
			defer grupo.Done()
			for i := 0; i < 200; i++ {
				s.Registrar([]string{fmt.Sprintf("valor-%d-%d-secreto", g, i)})
				s.Contem(fmt.Sprintf("valor-%d-%d-secreto", g, i))
			}
		}(g)
	}
	grupo.Wait()
}

func TestSegredosLidosNilNaoQuebra(t *testing.T) {
	var s *SegredosLidos
	s.Registrar([]string{"abcdef123456"})
	if s.Contem("abcdef123456") {
		t.Error("um *SegredosLidos nil não guarda nada")
	}
}
