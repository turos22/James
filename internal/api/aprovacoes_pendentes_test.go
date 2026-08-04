package api

import (
	"testing"
	"time"
)

func TestAguardarEResolverEntregaDecisao(t *testing.T) {
	r := NovoRegistroAprovacoesPendentes()
	espera := r.aguardar("id-1")

	ok := r.resolver("id-1", true)
	if !ok {
		t.Fatal("resolver deveria achar o canal pendente")
	}

	select {
	case aprovado := <-espera:
		if !aprovado {
			t.Error("esperava aprovado=true")
		}
	case <-time.After(time.Second):
		t.Fatal("canal não recebeu a decisão a tempo")
	}
}

func TestResolverIDDesconhecidoDevolveFalse(t *testing.T) {
	r := NovoRegistroAprovacoesPendentes()
	if r.resolver("não-existe", true) {
		t.Error("resolver deveria devolver false pra id não pendente")
	}
}

func TestRemoverDescartaEntradaSemVazarNoMapa(t *testing.T) {
	r := NovoRegistroAprovacoesPendentes()
	r.aguardar("id-1")

	r.remover("id-1")

	if len(r.canais) != 0 {
		t.Errorf("remover deveria ter esvaziado o mapa, restaram %d entradas", len(r.canais))
	}
	if r.resolver("id-1", true) {
		t.Error("resolver não deveria mais achar a entrada removida")
	}
}

func TestRemoverIDDesconhecidoNaoQuebra(t *testing.T) {
	r := NovoRegistroAprovacoesPendentes()
	r.remover("não-existe")
}
