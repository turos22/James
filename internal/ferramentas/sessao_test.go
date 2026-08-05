package ferramentas

import (
	"context"
	"testing"
)

func TestSessaoVaiEVoltaPeloContexto(t *testing.T) {
	ctx := ComSessao(context.Background(), "11111111-2222-3333-4444-555555555555")
	if got := SessaoDo(ctx); got != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("SessaoDo = %q, queria o id posto por ComSessao", got)
	}
}

func TestSessaoAusenteDevolveVazio(t *testing.T) {
	if got := SessaoDo(context.Background()); got != "" {
		t.Errorf("SessaoDo em ctx sem sessão = %q, queria vazio", got)
	}
}
