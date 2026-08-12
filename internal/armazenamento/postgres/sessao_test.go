package postgres

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
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

	// Listar() saiu (Fase 7A, Task 11) — este teste agora exercita
	// ListarPaginado. marca isola o resultado do banco de trabalho
	// compartilhado, que acumula sessões de execuções anteriores da suíte.
	marca := uuid.NewString()
	a, _ := repo.Criar(context.Background(), marca+" primeira")
	b, _ := repo.Criar(context.Background(), marca+" segunda")

	sessoes, err := repo.ListarPaginado(context.Background(), FiltroSessoes{Limite: 10, Busca: marca})
	if err != nil {
		t.Fatalf("ListarPaginado devolveu erro: %v", err)
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
		t.Error("ListarPaginado deveria incluir as duas sessões criadas neste teste")
	}
}

func TestListarPaginadoRespeitaOLimite(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	repo := NovoSessaoRepositorio(pool)

	marca := uuid.NewString()
	for i := 0; i < 5; i++ {
		if _, err := repo.Criar(ctx, fmt.Sprintf("%s pagina %d", marca, i)); err != nil {
			t.Fatalf("erro ao criar sessão: %v", err)
		}
	}

	pagina, err := repo.ListarPaginado(ctx, FiltroSessoes{Limite: 3, Busca: marca})
	if err != nil {
		t.Fatalf("erro ao listar: %v", err)
	}
	if len(pagina) != 3 {
		t.Fatalf("esperava 3 sessões, veio %d", len(pagina))
	}
	// Mais recente primeiro.
	for i := 1; i < len(pagina); i++ {
		if pagina[i].AtualizadaEm.After(pagina[i-1].AtualizadaEm) {
			t.Fatalf("ordem errada: item %d é mais novo que o %d", i, i-1)
		}
	}
}

func TestListarPaginadoAvancaPeloCursor(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	repo := NovoSessaoRepositorio(pool)

	marca := uuid.NewString()
	for i := 0; i < 5; i++ {
		if _, err := repo.Criar(ctx, fmt.Sprintf("%s cursor %d", marca, i)); err != nil {
			t.Fatalf("erro ao criar sessão: %v", err)
		}
	}

	primeira, err := repo.ListarPaginado(ctx, FiltroSessoes{Limite: 2, Busca: marca})
	if err != nil {
		t.Fatalf("erro na primeira página: %v", err)
	}
	cursor := primeira[len(primeira)-1].AtualizadaEm

	segunda, err := repo.ListarPaginado(ctx, FiltroSessoes{Limite: 2, Busca: marca, Antes: &cursor})
	if err != nil {
		t.Fatalf("erro na segunda página: %v", err)
	}
	if len(segunda) == 0 {
		t.Fatal("segunda página veio vazia")
	}
	for _, s := range segunda {
		for _, anterior := range primeira {
			if s.ID == anterior.ID {
				t.Fatalf("sessão %s repetiu entre as páginas", s.ID)
			}
		}
	}
}

func TestListarPaginadoFiltraPorTitulo(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	repo := NovoSessaoRepositorio(pool)

	marca := uuid.NewString()
	if _, err := repo.Criar(ctx, marca+" abre a CALCULADORA"); err != nil {
		t.Fatalf("erro ao criar: %v", err)
	}
	if _, err := repo.Criar(ctx, marca+" pesquisa sobre Go"); err != nil {
		t.Fatalf("erro ao criar: %v", err)
	}

	// ILIKE: buscar minúsculo tem que achar o título maiúsculo.
	achadas, err := repo.ListarPaginado(ctx, FiltroSessoes{Limite: 10, Busca: "calculadora"})
	if err != nil {
		t.Fatalf("erro ao buscar: %v", err)
	}
	var encontrou bool
	for _, s := range achadas {
		if strings.Contains(s.Titulo, marca) {
			encontrou = true
		}
		if strings.Contains(s.Titulo, "pesquisa sobre Go") {
			t.Fatal("a busca trouxe sessão que não bate com o termo")
		}
	}
	if !encontrou {
		t.Fatal("a busca não achou a sessão que bate com o termo")
	}
}

func TestAtualizarResumoGravaOsDoisCampos(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	repo := NovoSessaoRepositorio(pool)

	sessao, err := repo.Criar(ctx, "conversa longa")
	if err != nil {
		t.Fatalf("erro ao criar: %v", err)
	}

	if err := repo.AtualizarResumo(ctx, sessao.ID, "o Arthur pediu o clima e depois a agenda", 42); err != nil {
		t.Fatalf("erro ao atualizar resumo: %v", err)
	}

	recarregada, err := repo.BuscarPorID(ctx, sessao.ID)
	if err != nil {
		t.Fatalf("erro ao recarregar: %v", err)
	}
	if recarregada.Resumo != "o Arthur pediu o clima e depois a agenda" {
		t.Fatalf("resumo não gravou: %q", recarregada.Resumo)
	}
	if recarregada.ResumidoAte == nil || *recarregada.ResumidoAte != 42 {
		t.Fatalf("resumido_ate não gravou: %v", recarregada.ResumidoAte)
	}
}
