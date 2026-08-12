package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "modernc.org/sqlite"

	"james/internal/armazenamento/postgres"
)

func criarFixtureSQLite(t *testing.T) string {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), "Athena.db")
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatalf("erro ao abrir sqlite de teste: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE mensagens (id INTEGER PRIMARY KEY, role TEXT, content TEXT, created_at TEXT)`); err != nil {
		t.Fatalf("erro ao criar schema sqlite: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO mensagens (role, content) VALUES ('user','oi'), ('assistant','olá')`); err != nil {
		t.Fatalf("erro ao inserir fixture: %v", err)
	}
	return caminho
}

func abrirPoolDeTeste(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := postgres.URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}
	pool, err := postgres.Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("erro ao aplicar migrações: %v", err)
	}
	return pool
}

// limparSessaoHistorico garante isolamento entre os dois testes abaixo —
// eles compartilham o mesmo Postgres local, sem transação por teste.
func limparSessaoHistorico(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `DELETE FROM sessoes WHERE titulo = $1`, tituloSessaoHistorico); err != nil {
		t.Fatalf("erro ao limpar sessão de histórico antes do teste: %v", err)
	}
}

func TestExecutarMigraMensagensEMapeiaPapel(t *testing.T) {
	pool := abrirPoolDeTeste(t)
	limparSessaoHistorico(t, pool)
	caminhoSQLite := criarFixtureSQLite(t)

	if err := executar(context.Background(), caminhoSQLite, pool); err != nil {
		t.Fatalf("executar devolveu erro: %v", err)
	}

	sessoes := postgres.NovoSessaoRepositorio(pool)
	lista, err := sessoes.ListarPaginado(context.Background(), postgres.FiltroSessoes{Busca: tituloSessaoHistorico})
	if err != nil {
		t.Fatalf("erro ao listar sessões: %v", err)
	}

	var achada *postgres.Sessao
	for i := range lista {
		if lista[i].Titulo == tituloSessaoHistorico {
			achada = &lista[i]
		}
	}
	if achada == nil {
		t.Fatal("esperava sessão 'Histórico Athena' criada")
	}

	mensagens := postgres.NovoMensagemRepositorio(pool)
	msgs, err := mensagens.ListarPorSessao(context.Background(), achada.ID, 10)
	if err != nil {
		t.Fatalf("erro ao listar mensagens: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("esperava 2 mensagens migradas, vieram %d", len(msgs))
	}
	if msgs[0].Papel != "usuario" || msgs[1].Papel != "modelo" {
		t.Errorf("papéis mapeados errado (user→usuario, assistant→modelo): %+v", msgs)
	}
}

func TestExecutarEIdempotente(t *testing.T) {
	pool := abrirPoolDeTeste(t)
	limparSessaoHistorico(t, pool)
	caminhoSQLite := criarFixtureSQLite(t)

	if err := executar(context.Background(), caminhoSQLite, pool); err != nil {
		t.Fatalf("primeira execução devolveu erro: %v", err)
	}
	if err := executar(context.Background(), caminhoSQLite, pool); err != nil {
		t.Fatalf("segunda execução (idempotente) devolveu erro: %v", err)
	}

	sessoes := postgres.NovoSessaoRepositorio(pool)
	lista, _ := sessoes.ListarPaginado(context.Background(), postgres.FiltroSessoes{Busca: tituloSessaoHistorico})
	total := 0
	for _, s := range lista {
		if s.Titulo == tituloSessaoHistorico {
			total++
		}
	}
	if total != 1 {
		t.Fatalf("esperava 1 sessão de histórico mesmo após rodar 2x, vieram %d", total)
	}
}

// TestExecutarFalhaNoMeioNaoDeixaSessaoParcial prova a transação: se um
// insert falhar no meio do laço, o rollback tem que desfazer tudo (sessão
// incluída) — senão rodar de novo acharia a sessão pela checagem de
// idempotência e pularia pra sempre as mensagens que faltaram, sem sinal
// nenhum de que a migração ficou incompleta (achado do revisor de
// qualidade). O byte nulo na segunda mensagem força o INSERT a falhar de
// propósito: Postgres rejeita \x00 em coluna de texto.
func TestExecutarFalhaNoMeioNaoDeixaSessaoParcial(t *testing.T) {
	pool := abrirPoolDeTeste(t)
	limparSessaoHistorico(t, pool)

	caminho := filepath.Join(t.TempDir(), "Athena.db")
	db, err := sql.Open("sqlite", caminho)
	if err != nil {
		t.Fatalf("erro ao abrir sqlite de teste: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE mensagens (id INTEGER PRIMARY KEY, role TEXT, content TEXT, created_at TEXT)`); err != nil {
		t.Fatalf("erro ao criar schema sqlite: %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO mensagens (role, content) VALUES ('user','oi'), ('assistant', ?), ('user','depois')`,
		"tem \x00 nulo no meio",
	); err != nil {
		t.Fatalf("erro ao inserir fixture: %v", err)
	}
	db.Close()

	if err := executar(context.Background(), caminho, pool); err == nil {
		t.Fatal("esperava erro por causa do byte nulo na segunda mensagem")
	}

	sessoes := postgres.NovoSessaoRepositorio(pool)
	lista, err := sessoes.ListarPaginado(context.Background(), postgres.FiltroSessoes{Busca: tituloSessaoHistorico})
	if err != nil {
		t.Fatalf("erro ao listar sessões: %v", err)
	}
	for _, s := range lista {
		if s.Titulo == tituloSessaoHistorico {
			t.Fatalf("esperava rollback completo (nenhuma sessão de histórico), achei sessão %s", s.ID)
		}
	}
}
