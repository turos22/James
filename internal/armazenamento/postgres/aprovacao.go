package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Aprovacao struct {
	ID            uuid.UUID
	SessaoID      uuid.UUID
	Ferramenta    string
	Argumentos    []byte
	SolicitadaPor string
	Estado        string
	CriadaEm      time.Time
	DecididaEm    *time.Time
}

type AprovacaoRepositorio struct {
	pool *pgxpool.Pool
}

func NovoAprovacaoRepositorio(pool *pgxpool.Pool) *AprovacaoRepositorio {
	return &AprovacaoRepositorio{pool: pool}
}

func (r *AprovacaoRepositorio) Criar(ctx context.Context, sessaoID uuid.UUID, ferramenta string, argumentos []byte, solicitadaPor string) (Aprovacao, error) {
	id := uuid.New()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO aprovacoes (id, sessao_id, ferramenta, argumentos, solicitada_por, estado)
		 VALUES ($1,$2,$3,$4,$5,'pendente')`,
		id, sessaoID, ferramenta, argumentos, solicitadaPor)
	if err != nil {
		return Aprovacao{}, fmt.Errorf("erro ao criar aprovação: %w", err)
	}
	return r.BuscarPorID(ctx, id)
}

func (r *AprovacaoRepositorio) BuscarPorID(ctx context.Context, id uuid.UUID) (Aprovacao, error) {
	var a Aprovacao
	err := r.pool.QueryRow(ctx,
		`SELECT id, sessao_id, ferramenta, argumentos, solicitada_por, estado, criada_em, decidida_em
		 FROM aprovacoes WHERE id=$1`, id,
	).Scan(&a.ID, &a.SessaoID, &a.Ferramenta, &a.Argumentos, &a.SolicitadaPor, &a.Estado, &a.CriadaEm, &a.DecididaEm)
	if err != nil {
		return Aprovacao{}, fmt.Errorf("erro ao buscar aprovação %s: %w", id, err)
	}
	return a, nil
}

// Decidir só aplica se a aprovação ainda estiver pendente — evita corrida
// entre aprovar/negar/expirar (decisão dupla é erro, não sobrescrita).
func (r *AprovacaoRepositorio) Decidir(ctx context.Context, id uuid.UUID, estado string) error {
	if estado != "aprovada" && estado != "negada" && estado != "expirada" {
		return fmt.Errorf("estado %q inválido pra decisão", estado)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE aprovacoes SET estado=$1, decidida_em=now() WHERE id=$2 AND estado='pendente'`, estado, id)
	if err != nil {
		return fmt.Errorf("erro ao decidir aprovação %s: %w", id, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("aprovação %s não estava pendente", id)
	}
	return nil
}
