package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// abrirPoolMigrado pula o teste se DATABASE_URL não estiver definida (mesma
// convenção de conexao_test.go), senão devolve um pool com o schema em dia.
func abrirPoolMigrado(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}
	pool, err := Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("erro ao aplicar migrações: %v", err)
	}
	return pool
}
