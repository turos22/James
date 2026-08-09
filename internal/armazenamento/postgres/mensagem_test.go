package postgres

import (
	"context"
	"fmt"
	"testing"
)

func TestMensagemRepositorioSalvaEListaEmOrdem(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(context.Background(), "sessão pra mensagens")
	if err != nil {
		t.Fatalf("erro ao criar sessão de apoio: %v", err)
	}

	if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", "oi", nil); err != nil {
		t.Fatalf("Salvar (1) devolveu erro: %v", err)
	}
	if _, err := mensagens.Salvar(context.Background(), sessao.ID, "modelo", "olá", nil); err != nil {
		t.Fatalf("Salvar (2) devolveu erro: %v", err)
	}

	lista, err := mensagens.ListarPorSessao(context.Background(), sessao.ID, 10)
	if err != nil {
		t.Fatalf("ListarPorSessao devolveu erro: %v", err)
	}
	if len(lista) != 2 {
		t.Fatalf("esperava 2 mensagens, vieram %d", len(lista))
	}
	if lista[0].Papel != "usuario" || lista[1].Papel != "modelo" {
		t.Errorf("ordem errada: %+v", lista)
	}
}

func TestMensagemRepositorioRespeitaLimite(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, _ := sessoes.Criar(context.Background(), "sessão pra limite")
	for i := 0; i < 5; i++ {
		if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", "msg", nil); err != nil {
			t.Fatalf("Salvar devolveu erro: %v", err)
		}
	}

	lista, err := mensagens.ListarPorSessao(context.Background(), sessao.ID, 2)
	if err != nil {
		t.Fatalf("ListarPorSessao devolveu erro: %v", err)
	}
	if len(lista) != 2 {
		t.Fatalf("esperava 2 mensagens (limite), vieram %d", len(lista))
	}
}

func TestMensagemRepositorioSalvarAtualizaSessao(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(context.Background(), "sessão pra checar atualizada_em")
	if err != nil {
		t.Fatalf("erro ao criar sessão de apoio: %v", err)
	}

	if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", "oi", nil); err != nil {
		t.Fatalf("Salvar devolveu erro: %v", err)
	}

	atualizada, err := sessoes.BuscarPorID(context.Background(), sessao.ID)
	if err != nil {
		t.Fatalf("BuscarPorID devolveu erro: %v", err)
	}
	if !atualizada.AtualizadaEm.After(sessao.AtualizadaEm) {
		t.Errorf("AtualizadaEm = %v, esperava depois da criação (%v) — Salvar deveria bater a sessão", atualizada.AtualizadaEm, sessao.AtualizadaEm)
	}
}

// TestListarRecentesPorSessaoDivergeDeListarPorSessaoQuandoEstoraLimite prova
// a direção de cada método quando a sessão tem mais linhas que o limite:
// ListarPorSessao (usado pelo REST, que exibe a conversa do começo) devolve
// as mais ANTIGAS; ListarRecentesPorSessao (usado pela retomada via WS, que
// precisa continuar de onde parou) devolve as mais RECENTES, em ordem
// cronológica. O contraste explícito entre os dois no mesmo teste é
// deliberado — sem ele, alguém no futuro pode "consertar" um dos dois
// achando que a diferença é bug, quando é a semântica inteira da correção.
func TestListarRecentesPorSessaoDivergeDeListarPorSessaoQuandoEstoraLimite(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(context.Background(), "sessão pra divergência de limite")
	if err != nil {
		t.Fatalf("erro ao criar sessão de apoio: %v", err)
	}

	// 5 mensagens distinguíveis pelo conteúdo, gravadas em ordem.
	for i := 0; i < 5; i++ {
		conteudo := fmt.Sprintf("mensagem %d", i)
		if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", conteudo, nil); err != nil {
			t.Fatalf("Salvar (%d) devolveu erro: %v", i, err)
		}
	}

	const limite = 2

	antigas, err := mensagens.ListarPorSessao(context.Background(), sessao.ID, limite)
	if err != nil {
		t.Fatalf("ListarPorSessao devolveu erro: %v", err)
	}
	if len(antigas) != limite {
		t.Fatalf("ListarPorSessao: esperava %d mensagens, vieram %d", limite, len(antigas))
	}
	// REST: mostra a conversa do começo — as duas mais ANTIGAS, em ordem.
	if antigas[0].Conteudo != "mensagem 0" || antigas[1].Conteudo != "mensagem 1" {
		t.Errorf("ListarPorSessao veio errado: %+v", antigas)
	}

	recentes, err := mensagens.ListarRecentesPorSessao(context.Background(), sessao.ID, limite)
	if err != nil {
		t.Fatalf("ListarRecentesPorSessao devolveu erro: %v", err)
	}
	if len(recentes) != limite {
		t.Fatalf("ListarRecentesPorSessao: esperava %d mensagens, vieram %d", limite, len(recentes))
	}
	// Retomada: continua de onde parou — as duas mais RECENTES, mas ainda em
	// ordem cronológica (não invertida), pra reconstruirHistorico e
	// memoria.Janela.Montar não verem a conversa de trás pra frente.
	if recentes[0].Conteudo != "mensagem 3" || recentes[1].Conteudo != "mensagem 4" {
		t.Errorf("ListarRecentesPorSessao veio errado: %+v", recentes)
	}

	// O contraste em si: pedindo o mesmo limite da mesma sessão, os dois
	// métodos têm que divergir quando a sessão estoura o limite.
	if antigas[0].Conteudo == recentes[0].Conteudo {
		t.Fatal("ListarPorSessao e ListarRecentesPorSessao vieram iguais — deviam divergir quando a sessão estoura o limite")
	}
}
