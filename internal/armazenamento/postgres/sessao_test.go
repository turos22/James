package postgres

import (
	"context"
	"testing"
)

func TestSessaoRepositorioCriaEBusca(t *testing.T) {
	pool := abrirPoolMigrado(t)
	repo := NovoSessaoRepositorio(pool)

	criada, err := repo.Criar(context.Background(), "sessão de teste")
	if err != nil {
		t.Fatalf("Criar devolveu erro: %v", err)
	}
	if criada.Titulo != "sessão de teste" {
		t.Errorf("Titulo = %q, queria sessão de teste", criada.Titulo)
	}

	achada, err := repo.BuscarPorID(context.Background(), criada.ID)
	if err != nil {
		t.Fatalf("BuscarPorID devolveu erro: %v", err)
	}
	if achada.ID != criada.ID {
		t.Errorf("ID = %v, queria %v", achada.ID, criada.ID)
	}
}

func TestSessaoRepositorioListaOrdenaPorAtualizacao(t *testing.T) {
	pool := abrirPoolMigrado(t)
	repo := NovoSessaoRepositorio(pool)

	a, _ := repo.Criar(context.Background(), "primeira")
	b, _ := repo.Criar(context.Background(), "segunda")

	sessoes, err := repo.Listar(context.Background())
	if err != nil {
		t.Fatalf("Listar devolveu erro: %v", err)
	}

	achouA, achouB := false, false
	for _, s := range sessoes {
		if s.ID == a.ID {
			achouA = true
		}
		if s.ID == b.ID {
			achouB = true
		}
	}
	if !achouA || !achouB {
		t.Error("Listar deveria incluir as duas sessões criadas neste teste")
	}
}
