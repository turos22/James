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

// Salvar grava uma mensagem e atualiza sessoes.atualizada_em num único
// statement atômico (CTE de escrita) — sem isso a coluna nunca mudaria
// depois da criação da sessão, e Listar() (que ordena por atualizada_em
// DESC) na prática ordenaria por criação, não por atividade recente.
// Atômico de propósito: duas instruções separadas (INSERT depois UPDATE)
// deixariam uma janela onde a mensagem já foi gravada mas Salvar devolve
// erro como se nada tivesse persistido — um chamador com retry duplicaria
// a mensagem no histórico. Com um statement só, erro significa que nada
// foi salvo, ponto final. chamadas pode ser nil (mensagem sem chamada de
// ferramenta) — vira NULL em chamadas JSONB.
func (r *MensagemRepositorio) Salvar(ctx context.Context, sessaoID uuid.UUID, papel, conteudo string, chamadas []byte) (Mensagem, error) {
	var m Mensagem
	err := r.pool.QueryRow(ctx,
		`WITH nova AS (
			INSERT INTO mensagens (sessao_id, papel, conteudo, chamadas)
			VALUES ($1, $2, $3, $4)
			RETURNING id, sessao_id, papel, conteudo, chamadas, criada_em
		)
		UPDATE sessoes SET atualizada_em = now() WHERE id = $1
		RETURNING (SELECT id FROM nova), (SELECT sessao_id FROM nova), (SELECT papel FROM nova),
		          (SELECT conteudo FROM nova), (SELECT chamadas FROM nova), (SELECT criada_em FROM nova)`,
		sessaoID, papel, conteudo, chamadas,
	).Scan(&m.ID, &m.SessaoID, &m.Papel, &m.Conteudo, &m.Chamadas, &m.CriadaEm)
	if err != nil {
		return Mensagem{}, fmt.Errorf("erro ao salvar mensagem: %w", err)
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

// ListarRecentesPorSessao devolve as `limite` mensagens mais RECENTES da
// sessão, já em ordem cronológica. Existe separado de ListarPorSessao
// porque as duas têm semântica oposta quando a sessão estoura o limite: o
// REST lista do começo para exibir a conversa, a retomada precisa do fim
// para continuar de onde parou.
func (r *MensagemRepositorio) ListarRecentesPorSessao(ctx context.Context, sessaoID uuid.UUID, limite int) ([]Mensagem, error) {
	linhas, err := r.pool.Query(ctx,
		`SELECT id, sessao_id, papel, conteudo, chamadas, criada_em FROM mensagens
		 WHERE sessao_id=$1 ORDER BY criada_em DESC LIMIT $2`, sessaoID, limite)
	if err != nil {
		return nil, fmt.Errorf("erro ao listar mensagens recentes da sessão %s: %w", sessaoID, err)
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
	if err := linhas.Err(); err != nil {
		return nil, err
	}

	// A query veio DESC (mais recente primeiro) pra o LIMIT pegar a cauda
	// certa; inverte aqui pra quem chama sempre receber em ordem
	// cronológica, igual ListarPorSessao — reconstruirHistorico e
	// memoria.Janela.Montar dependem dessa ordem.
	for i, j := 0, len(mensagens)-1; i < j; i, j = i+1, j-1 {
		mensagens[i], mensagens[j] = mensagens[j], mensagens[i]
	}

	return mensagens, nil
}
