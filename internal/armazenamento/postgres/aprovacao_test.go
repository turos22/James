package postgres

import (
	"context"
	"testing"
)

func TestAprovacaoRepositorioCriaEDecide(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	aprovacoes := NovoAprovacaoRepositorio(pool)

	sessao, _ := sessoes.Criar(context.Background(), "sessão pra aprovação")

	criada, err := aprovacoes.Criar(context.Background(), sessao.ID, "executar_comando_sistema",
		[]byte(`{"comando":"Get-Process"}`), "celular")
	if err != nil {
		t.Fatalf("Criar devolveu erro: %v", err)
	}
	if criada.Estado != "pendente" {
		t.Errorf("Estado = %q, queria pendente", criada.Estado)
	}

	if err := aprovacoes.Decidir(context.Background(), criada.ID, "aprovada"); err != nil {
		t.Fatalf("Decidir devolveu erro: %v", err)
	}

	final, err := aprovacoes.BuscarPorID(context.Background(), criada.ID)
	if err != nil {
		t.Fatalf("BuscarPorID devolveu erro: %v", err)
	}
	if final.Estado != "aprovada" {
		t.Errorf("Estado = %q, queria aprovada", final.Estado)
	}
	if final.DecididaEm == nil {
		t.Error("DecididaEm deveria estar preenchido após Decidir")
	}
}

func TestAprovacaoRepositorioNaoDecideDuasVezes(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	aprovacoes := NovoAprovacaoRepositorio(pool)

	sessao, _ := sessoes.Criar(context.Background(), "sessão pra aprovação dupla")
	criada, _ := aprovacoes.Criar(context.Background(), sessao.ID, "executar_comando_sistema",
		[]byte(`{}`), "celular")

	if err := aprovacoes.Decidir(context.Background(), criada.ID, "negada"); err != nil {
		t.Fatalf("primeira decisão devolveu erro: %v", err)
	}
	if err := aprovacoes.Decidir(context.Background(), criada.ID, "aprovada"); err == nil {
		t.Fatal("esperava erro ao decidir aprovação que já não estava pendente")
	}
}
