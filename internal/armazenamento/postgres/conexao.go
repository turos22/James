// Package postgres concentra o acesso ao banco do James.
package postgres

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Abrir cria um pool de conexões a partir da URL informada e confirma que o
// banco responde antes de devolver. Falhar aqui é melhor que falhar na
// primeira query, no meio de uma conversa.
func Abrir(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, fmt.Errorf("URL do banco vazia: defina DATABASE_URL")
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar o pool: %w", err)
	}

	ctxPing, cancelar := context.WithTimeout(ctx, 5*time.Second)
	defer cancelar()

	if err := pool.Ping(ctxPing); err != nil {
		pool.Close()
		return nil, fmt.Errorf("banco não respondeu: %w", err)
	}

	return pool, nil
}

// URLDoAmbiente devolve a URL de conexão configurada em DATABASE_URL.
func URLDoAmbiente() string {
	return os.Getenv("DATABASE_URL")
}
