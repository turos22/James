package postgres

import (
	"context"
	"testing"
)

func TestAplicarMigracoesContraPostgresLocal(t *testing.T) {
	url := URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}

	pool, err := Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	defer pool.Close()

	if err := AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("primeira aplicação falhou: %v", err)
	}
	// Idempotência: rodar de novo não pode falhar nem duplicar.
	if err := AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("segunda aplicação (idempotente) falhou: %v", err)
	}

	var existe bool
	err = pool.QueryRow(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'sessoes')`).Scan(&existe)
	if err != nil {
		t.Fatalf("erro ao checar tabela sessoes: %v", err)
	}
	if !existe {
		t.Error("tabela sessoes deveria existir depois da migração")
	}
}

// A 0002 roda em banco que já tem dados — o do Arthur tem mais de mil
// sessões. DEFAULT '' e NULL permitido são o que tornam isso seguro (R10).
func TestMigracao0002PreservaSessaoExistente(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()

	repo := NovoSessaoRepositorio(pool)
	sessao, err := repo.Criar(ctx, "conversa anterior à 0002")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	// Reaplicar é no-op (AplicarMigracoes é idempotente), mas o que
	// interessa é o estado das colunas novas numa linha que já existia.
	if err := AplicarMigracoes(ctx, pool); err != nil {
		t.Fatalf("erro ao reaplicar migrações: %v", err)
	}

	var resumo string
	var resumidoAte *int64
	err = pool.QueryRow(ctx, `SELECT resumo, resumido_ate FROM sessoes WHERE id=$1`, sessao.ID).
		Scan(&resumo, &resumidoAte)
	if err != nil {
		t.Fatalf("erro ao ler colunas novas: %v", err)
	}
	if resumo != "" {
		t.Fatalf("resumo devia nascer vazio, veio %q", resumo)
	}
	if resumidoAte != nil {
		t.Fatalf("resumido_ate devia nascer NULL, veio %d", *resumidoAte)
	}
}
