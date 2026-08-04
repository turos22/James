package postgres

import (
	"context"
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
