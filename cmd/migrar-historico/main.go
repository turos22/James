// Comando migrar-historico: ETL único e idempotente do Athena.db (SQLite)
// pro Postgres do James. Roda uma vez, à mão, depois que o jamesd já subiu
// e aplicou as migrações (decisão P3 — não roda no boot do serviço).
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"

	"james/internal/armazenamento/postgres"
)

const tituloSessaoHistorico = "Histórico Athena"

func main() {
	caminhoSQLite := flag.String("sqlite", "./Athena.db", "caminho pro Athena.db original")
	flag.Parse()

	ctx := context.Background()
	pool, err := postgres.Abrir(ctx, postgres.URLDoAmbiente())
	if err != nil {
		slog.Error("migração falhou", "erro", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := postgres.AplicarMigracoes(ctx, pool); err != nil {
		slog.Error("migração falhou", "erro", err)
		os.Exit(1)
	}

	if err := executar(ctx, *caminhoSQLite, pool); err != nil {
		slog.Error("migração falhou", "erro", err)
		os.Exit(1)
	}
	slog.Info("migração concluída")
}

func executar(ctx context.Context, caminhoSQLite string, pool *pgxpool.Pool) error {
	sessoes := postgres.NovoSessaoRepositorio(pool)
	mensagens := postgres.NovoMensagemRepositorio(pool)

	existentes, err := sessoes.Listar(ctx)
	if err != nil {
		return fmt.Errorf("erro ao checar sessões existentes: %w", err)
	}
	for _, s := range existentes {
		if s.Titulo == tituloSessaoHistorico {
			slog.Info("sessão de histórico já existe, nada a fazer", "sessao_id", s.ID)
			return nil
		}
	}

	db, err := sql.Open("sqlite", caminhoSQLite)
	if err != nil {
		return fmt.Errorf("erro ao abrir %s: %w", caminhoSQLite, err)
	}
	defer db.Close()

	linhas, err := db.QueryContext(ctx, `SELECT role, content FROM mensagens ORDER BY id ASC`)
	if err != nil {
		return fmt.Errorf("erro ao ler mensagens do sqlite: %w", err)
	}
	defer linhas.Close()

	sessao, err := sessoes.Criar(ctx, tituloSessaoHistorico)
	if err != nil {
		return fmt.Errorf("erro ao criar sessão de histórico: %w", err)
	}

	total := 0
	for linhas.Next() {
		var role, content string
		if err := linhas.Scan(&role, &content); err != nil {
			return fmt.Errorf("erro ao ler linha do sqlite: %w", err)
		}
		papel := "usuario"
		if role == "assistant" {
			papel = "modelo"
		}
		if _, err := mensagens.Salvar(ctx, sessao.ID, papel, content, nil); err != nil {
			return fmt.Errorf("erro ao salvar mensagem migrada: %w", err)
		}
		total++
	}
	if err := linhas.Err(); err != nil {
		return fmt.Errorf("erro ao percorrer mensagens do sqlite: %w", err)
	}

	slog.Info("mensagens migradas", "total", total, "sessao_id", sessao.ID)
	return nil
}
