package postgres

import (
	"context"
	"fmt"
	"testing"
)

func TestMensagemRepositorioSalvaEListaEmOrdem(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(context.Background(), "sessão pra mensagens")
	if err != nil {
		t.Fatalf("erro ao criar sessão de apoio: %v", err)
	}

	if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", "oi", nil); err != nil {
		t.Fatalf("Salvar (1) devolveu erro: %v", err)
	}
	if _, err := mensagens.Salvar(context.Background(), sessao.ID, "modelo", "olá", nil); err != nil {
		t.Fatalf("Salvar (2) devolveu erro: %v", err)
	}

	lista, err := mensagens.ListarPorSessao(context.Background(), sessao.ID, 10)
	if err != nil {
		t.Fatalf("ListarPorSessao devolveu erro: %v", err)
	}
	if len(lista) != 2 {
		t.Fatalf("esperava 2 mensagens, vieram %d", len(lista))
	}
	if lista[0].Papel != "usuario" || lista[1].Papel != "modelo" {
		t.Errorf("ordem errada: %+v", lista)
	}
}

func TestMensagemRepositorioRespeitaLimite(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, _ := sessoes.Criar(context.Background(), "sessão pra limite")
	for i := 0; i < 5; i++ {
		if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", "msg", nil); err != nil {
			t.Fatalf("Salvar devolveu erro: %v", err)
		}
	}

	lista, err := mensagens.ListarPorSessao(context.Background(), sessao.ID, 2)
	if err != nil {
		t.Fatalf("ListarPorSessao devolveu erro: %v", err)
	}
	if len(lista) != 2 {
		t.Fatalf("esperava 2 mensagens (limite), vieram %d", len(lista))
	}
}

func TestMensagemRepositorioSalvarAtualizaSessao(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(context.Background(), "sessão pra checar atualizada_em")
	if err != nil {
		t.Fatalf("erro ao criar sessão de apoio: %v", err)
	}

	if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", "oi", nil); err != nil {
		t.Fatalf("Salvar devolveu erro: %v", err)
	}

	atualizada, err := sessoes.BuscarPorID(context.Background(), sessao.ID)
	if err != nil {
		t.Fatalf("BuscarPorID devolveu erro: %v", err)
	}
	if !atualizada.AtualizadaEm.After(sessao.AtualizadaEm) {
		t.Errorf("AtualizadaEm = %v, esperava depois da criação (%v) — Salvar deveria bater a sessão", atualizada.AtualizadaEm, sessao.AtualizadaEm)
	}
}

// TestListarRecentesPorSessaoDivergeDeListarPorSessaoQuandoEstoraLimite prova
// a direção de cada método quando a sessão tem mais linhas que o limite:
// ListarPorSessao (usado pelo REST, que exibe a conversa do começo) devolve
// as mais ANTIGAS; ListarRecentesPorSessao (usado pela retomada via WS, que
// precisa continuar de onde parou) devolve as mais RECENTES, em ordem
// cronológica. O contraste explícito entre os dois no mesmo teste é
// deliberado — sem ele, alguém no futuro pode "consertar" um dos dois
// achando que a diferença é bug, quando é a semântica inteira da correção.
func TestListarRecentesPorSessaoDivergeDeListarPorSessaoQuandoEstoraLimite(t *testing.T) {
	pool := abrirPoolMigrado(t)
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(context.Background(), "sessão pra divergência de limite")
	if err != nil {
		t.Fatalf("erro ao criar sessão de apoio: %v", err)
	}

	// 5 mensagens distinguíveis pelo conteúdo, gravadas em ordem.
	for i := 0; i < 5; i++ {
		conteudo := fmt.Sprintf("mensagem %d", i)
		if _, err := mensagens.Salvar(context.Background(), sessao.ID, "usuario", conteudo, nil); err != nil {
			t.Fatalf("Salvar (%d) devolveu erro: %v", i, err)
		}
	}

	const limite = 2

	antigas, err := mensagens.ListarPorSessao(context.Background(), sessao.ID, limite)
	if err != nil {
		t.Fatalf("ListarPorSessao devolveu erro: %v", err)
	}
	if len(antigas) != limite {
		t.Fatalf("ListarPorSessao: esperava %d mensagens, vieram %d", limite, len(antigas))
	}
	// REST: mostra a conversa do começo — as duas mais ANTIGAS, em ordem.
	if antigas[0].Conteudo != "mensagem 0" || antigas[1].Conteudo != "mensagem 1" {
		t.Errorf("ListarPorSessao veio errado: %+v", antigas)
	}

	recentes, err := mensagens.ListarRecentesPorSessao(context.Background(), sessao.ID, limite)
	if err != nil {
		t.Fatalf("ListarRecentesPorSessao devolveu erro: %v", err)
	}
	if len(recentes) != limite {
		t.Fatalf("ListarRecentesPorSessao: esperava %d mensagens, vieram %d", limite, len(recentes))
	}
	// Retomada: continua de onde parou — as duas mais RECENTES, mas ainda em
	// ordem cronológica (não invertida), pra reconstruirHistorico e
	// memoria.Janela.Montar não verem a conversa de trás pra frente.
	if recentes[0].Conteudo != "mensagem 3" || recentes[1].Conteudo != "mensagem 4" {
		t.Errorf("ListarRecentesPorSessao veio errado: %+v", recentes)
	}

	// O contraste em si: pedindo o mesmo limite da mesma sessão, os dois
	// métodos têm que divergir quando a sessão estoura o limite.
	if antigas[0].Conteudo == recentes[0].Conteudo {
		t.Fatal("ListarPorSessao e ListarRecentesPorSessao vieram iguais — deviam divergir quando a sessão estoura o limite")
	}
}

func TestIDDeCorteEListarNaoResumidas(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(ctx, "conversa de dez mensagens")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	var ids []int64
	for i := 0; i < 10; i++ {
		m, err := mensagens.Salvar(ctx, sessao.ID, "usuario", fmt.Sprintf("mensagem %d", i), nil)
		if err != nil {
			t.Fatalf("erro ao salvar: %v", err)
		}
		ids = append(ids, m.ID)
	}

	// Janela de 4: as 4 últimas ficam dentro, o corte é o id da 7ª (índice 6).
	corte, err := mensagens.IDDeCorte(ctx, sessao.ID, 4)
	if err != nil {
		t.Fatalf("erro no corte: %v", err)
	}
	if corte != ids[6] {
		t.Fatalf("corte errado: esperava %d, veio %d", ids[6], corte)
	}

	// Nada resumido ainda: tudo abaixo do corte é candidato.
	fora, err := mensagens.ListarNaoResumidas(ctx, sessao.ID, corte, 0)
	if err != nil {
		t.Fatalf("erro ao listar não resumidas: %v", err)
	}
	if len(fora) != 6 {
		t.Fatalf("esperava 6 mensagens fora da janela, vieram %d", len(fora))
	}
	if fora[0].ID != ids[0] {
		t.Fatalf("ordem errada: primeira devia ser %d, veio %d", ids[0], fora[0].ID)
	}

	// Já resumido até a terceira: sobram três.
	restantes, err := mensagens.ListarNaoResumidas(ctx, sessao.ID, corte, ids[2])
	if err != nil {
		t.Fatalf("erro ao listar restantes: %v", err)
	}
	if len(restantes) != 3 {
		t.Fatalf("esperava 3 restantes, vieram %d", len(restantes))
	}

	// Nada novo: já cobre tudo que está fora da janela.
	nenhuma, err := mensagens.ListarNaoResumidas(ctx, sessao.ID, corte, ids[5])
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if len(nenhuma) != 0 {
		t.Fatalf("esperava nenhuma, vieram %d", len(nenhuma))
	}
}

// R4: numa sessão retomada com mais de 200 mensagens o histórico em memória
// é só a cauda, e contar sobre ele resumiria o trecho errado — em silêncio.
// A ancoragem por id do Postgres é o que salva, e é isto que este teste
// prova: o corte não depende de quantas mensagens o processo tem em mãos.
func TestIDDeCorteIgnoraOQueOProcessoTemEmMemoria(t *testing.T) {
	pool := abrirPoolMigrado(t)
	ctx := context.Background()
	sessoes := NovoSessaoRepositorio(pool)
	mensagens := NovoMensagemRepositorio(pool)

	sessao, err := sessoes.Criar(ctx, "conversa de 250 mensagens")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}
	var ids []int64
	for i := 0; i < 250; i++ {
		m, err := mensagens.Salvar(ctx, sessao.ID, "usuario", fmt.Sprintf("m%d", i), nil)
		if err != nil {
			t.Fatalf("erro ao salvar: %v", err)
		}
		ids = append(ids, m.ID)
	}

	// A retomada carregaria só as 200 mais recentes. O corte para uma
	// janela de 10 tem que apontar para a 241ª da SESSÃO (índice 240), não
	// para a 241ª daquele recorte.
	corte, err := mensagens.IDDeCorte(ctx, sessao.ID, 10)
	if err != nil {
		t.Fatalf("erro no corte: %v", err)
	}
	if corte != ids[240] {
		t.Fatalf("corte ancorou no recorte em vez da sessão: esperava %d, veio %d", ids[240], corte)
	}
}
