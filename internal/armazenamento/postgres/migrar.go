package postgres

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"

	"james/internal/armazenamento"
)

// AplicarMigracoes aplica, em ordem e dentro de transação, cada .sql de
// internal/armazenamento/migracoes que ainda não constar em
// schema_migracoes. Idempotente: rodar de novo não reaplica nem falha.
func AplicarMigracoes(ctx context.Context, pool *pgxpool.Pool) error {
	if _, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migracoes (
			nome        TEXT PRIMARY KEY,
			aplicada_em TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("erro ao criar schema_migracoes: %w", err)
	}

	entradas, err := armazenamento.ArquivosMigracao.ReadDir("migracoes")
	if err != nil {
		return fmt.Errorf("erro ao listar migrações embutidas: %w", err)
	}

	nomes := make([]string, 0, len(entradas))
	for _, e := range entradas {
		nomes = append(nomes, e.Name())
	}
	sort.Strings(nomes)

	for _, nome := range nomes {
		var jaAplicada bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migracoes WHERE nome=$1)`, nome).Scan(&jaAplicada); err != nil {
			return fmt.Errorf("erro ao checar migração %s: %w", nome, err)
		}
		if jaAplicada {
			continue
		}

		conteudo, err := armazenamento.ArquivosMigracao.ReadFile("migracoes/" + nome)
		if err != nil {
			return fmt.Errorf("erro ao ler migração %s: %w", nome, err)
		}

		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("erro ao abrir transação pra migração %s: %w", nome, err)
		}
		if _, err := tx.Exec(ctx, string(conteudo)); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("erro ao aplicar migração %s: %w", nome, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migracoes (nome) VALUES ($1)`, nome); err != nil {
			tx.Rollback(ctx)
			return fmt.Errorf("erro ao registrar migração %s: %w", nome, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("erro ao commitar migração %s: %w", nome, err)
		}
	}
	return nil
}
