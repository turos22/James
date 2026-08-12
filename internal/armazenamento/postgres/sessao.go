package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Sessao struct {
	ID           uuid.UUID
	Titulo       string
	CriadaEm     time.Time
	AtualizadaEm time.Time
	Resumo       string
	// ResumidoAte é nil quando nada foi resumido ainda.
	ResumidoAte *int64
}

type SessaoRepositorio struct {
	pool *pgxpool.Pool
}

func NovoSessaoRepositorio(pool *pgxpool.Pool) *SessaoRepositorio {
	return &SessaoRepositorio{pool: pool}
}

func (r *SessaoRepositorio) Criar(ctx context.Context, titulo string) (Sessao, error) {
	id := uuid.New()
	if _, err := r.pool.Exec(ctx, `INSERT INTO sessoes (id, titulo) VALUES ($1, $2)`, id, titulo); err != nil {
		return Sessao{}, fmt.Errorf("erro ao criar sessão: %w", err)
	}
	return r.BuscarPorID(ctx, id)
}

func (r *SessaoRepositorio) BuscarPorID(ctx context.Context, id uuid.UUID) (Sessao, error) {
	var s Sessao
	err := r.pool.QueryRow(ctx,
		`SELECT id, titulo, criada_em, atualizada_em, resumo, resumido_ate FROM sessoes WHERE id=$1`, id,
	).Scan(&s.ID, &s.Titulo, &s.CriadaEm, &s.AtualizadaEm, &s.Resumo, &s.ResumidoAte)
	if err != nil {
		return Sessao{}, fmt.Errorf("erro ao buscar sessão %s: %w", id, err)
	}
	return s, nil
}

func (r *SessaoRepositorio) Listar(ctx context.Context) ([]Sessao, error) {
	linhas, err := r.pool.Query(ctx,
		`SELECT id, titulo, criada_em, atualizada_em FROM sessoes ORDER BY atualizada_em DESC`)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar sessões: %w", err)
	}
	defer linhas.Close()

	var sessoes []Sessao
	for linhas.Next() {
		var s Sessao
		if err := linhas.Scan(&s.ID, &s.Titulo, &s.CriadaEm, &s.AtualizadaEm); err != nil {
			return nil, err
		}
		sessoes = append(sessoes, s)
	}
	return sessoes, linhas.Err()
}
