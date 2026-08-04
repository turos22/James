package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Mensagem struct {
	ID       int64
	SessaoID uuid.UUID
	Papel    string
	Conteudo string
	Chamadas []byte
	CriadaEm time.Time
}

type MensagemRepositorio struct {
	pool *pgxpool.Pool
}

func NovoMensagemRepositorio(pool *pgxpool.Pool) *MensagemRepositorio {
	return &MensagemRepositorio{pool: pool}
}

// Salvar grava uma mensagem e atualiza sessoes.atualizada_em — sem isso a
// coluna nunca mudaria depois da criação da sessão, e Listar() (que ordena
// por atualizada_em DESC) na prática ordenaria por criação, não por
// atividade recente. chamadas pode ser nil (mensagem sem chamada de
// ferramenta) — vira NULL em chamadas JSONB.
func (r *MensagemRepositorio) Salvar(ctx context.Context, sessaoID uuid.UUID, papel, conteudo string, chamadas []byte) (Mensagem, error) {
	var m Mensagem
	err := r.pool.QueryRow(ctx,
		`INSERT INTO mensagens (sessao_id, papel, conteudo, chamadas) VALUES ($1,$2,$3,$4)
		 RETURNING id, sessao_id, papel, conteudo, chamadas, criada_em`,
		sessaoID, papel, conteudo, chamadas,
	).Scan(&m.ID, &m.SessaoID, &m.Papel, &m.Conteudo, &m.Chamadas, &m.CriadaEm)
	if err != nil {
		return Mensagem{}, fmt.Errorf("erro ao salvar mensagem: %w", err)
	}

	if _, err := r.pool.Exec(ctx, `UPDATE sessoes SET atualizada_em = now() WHERE id = $1`, sessaoID); err != nil {
		return Mensagem{}, fmt.Errorf("erro ao atualizar sessão: %w", err)
	}

	return m, nil
}

func (r *MensagemRepositorio) ListarPorSessao(ctx context.Context, sessaoID uuid.UUID, limite int) ([]Mensagem, error) {
	linhas, err := r.pool.Query(ctx,
		`SELECT id, sessao_id, papel, conteudo, chamadas, criada_em FROM mensagens
		 WHERE sessao_id=$1 ORDER BY criada_em ASC LIMIT $2`, sessaoID, limite)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar mensagens da sessão %s: %w", sessaoID, err)
	}
	defer linhas.Close()

	var mensagens []Mensagem
	for linhas.Next() {
		var m Mensagem
		if err := linhas.Scan(&m.ID, &m.SessaoID, &m.Papel, &m.Conteudo, &m.Chamadas, &m.CriadaEm); err != nil {
			return nil, err
		}
		mensagens = append(mensagens, m)
	}
	return mensagens, linhas.Err()
}
