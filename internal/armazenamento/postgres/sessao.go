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

// FiltroSessoes descreve uma página da lista de sessões. Antes é o cursor
// keyset: devolve só o que for MAIS ANTIGO que ele. Keyset e não OFFSET
// porque a lista é ordenada por atividade recente e muda embaixo de quem
// rola — com OFFSET, uma sessão que recebe mensagem durante a rolagem faz
// item pular ou repetir.
type FiltroSessoes struct {
	Limite int
	Antes  *time.Time
	Busca  string
}

const limiteMaximoSessoes = 200

// ListarPaginado devolve uma página de sessões, mais recente primeiro.
// Substituiu o Listar() sem limite, que devolvia a tabela inteira — a aba
// Sessões chegou a renderizar 1646 botões porque a suíte de teste escreve
// no banco de trabalho e quase nunca limpa.
func (r *SessaoRepositorio) ListarPaginado(ctx context.Context, filtro FiltroSessoes) ([]Sessao, error) {
	limite := filtro.Limite
	if limite <= 0 || limite > limiteMaximoSessoes {
		limite = 50
	}

	consulta := `SELECT id, titulo, criada_em, atualizada_em, resumo, resumido_ate
	             FROM sessoes WHERE ($1::timestamptz IS NULL OR atualizada_em < $1)
	               AND ($2 = '' OR titulo ILIKE '%' || $2 || '%')
	             ORDER BY atualizada_em DESC LIMIT $3`

	linhas, err := r.pool.Query(ctx, consulta, filtro.Antes, filtro.Busca, limite)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar sessões: %w", err)
	}
	defer linhas.Close()

	sessoes := []Sessao{}
	for linhas.Next() {
		var s Sessao
		if err := linhas.Scan(&s.ID, &s.Titulo, &s.CriadaEm, &s.AtualizadaEm, &s.Resumo, &s.ResumidoAte); err != nil {
			return nil, err
		}
		sessoes = append(sessoes, s)
	}
	return sessoes, linhas.Err()
}

// AtualizarResumo grava o resumo rolante e até onde ele cobre. Não mexe em
// atualizada_em de propósito: resumir não é atividade da conversa, e mexer
// ali reordenaria a lista do Histórico sem o Arthur ter feito nada.
func (r *SessaoRepositorio) AtualizarResumo(ctx context.Context, id uuid.UUID, resumo string, resumidoAte int64) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE sessoes SET resumo=$2, resumido_ate=$3 WHERE id=$1`, id, resumo, resumidoAte)
	if err != nil {
		return fmt.Errorf("erro ao atualizar resumo da sessão %s: %w", id, err)
	}
	return nil
}
