package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

// TestMain carrega o .env da raiz para que os testes de integração enxerguem
// DATABASE_URL sem depender de o shell já ter exportado a variável.
func TestMain(m *testing.M) {
	_ = godotenv.Load("../../../.env")
	os.Exit(m.Run())
}

func TestAbrirRecusaURLVazia(t *testing.T) {
	if _, err := Abrir(context.Background(), ""); err == nil {
		t.Fatal("esperava erro para URL vazia, veio nil")
	}
}

func TestAbrirRecusaURLMalformada(t *testing.T) {
	if _, err := Abrir(context.Background(), "não-é-uma-url"); err == nil {
		t.Fatal("esperava erro para URL malformada, veio nil")
	}
}

func TestAbrirConectaNoPostgresLocal(t *testing.T) {
	url := URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}

	pool, err := Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	defer pool.Close()

	var um int
	if err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&um); err != nil {
		t.Fatalf("query de sanidade falhou: %v", err)
	}
	if um != 1 {
		t.Errorf("SELECT 1 devolveu %d, queria 1", um)
	}
}
