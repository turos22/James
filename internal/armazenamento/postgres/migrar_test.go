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
