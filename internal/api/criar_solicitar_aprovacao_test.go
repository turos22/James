package api

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/permissao"
)

// clienteCapturaID grava o id real de cada evento recebido (clienteFalso, em
// clientes_test.go, só grava o tipo) — precisamos do id de verdade da
// aprovação criada dentro de criarSolicitarAprovacao pra consultar o estado
// dela no banco depois.
type clienteCapturaID struct {
	perfil permissao.Perfil
	ids    []string
}

func (c *clienteCapturaID) Perfil() permissao.Perfil { return c.perfil }
func (c *clienteCapturaID) Enviar(tipo, id string, dados any) error {
	c.ids = append(c.ids, id)
	return nil
}

func servidorDeTesteParaAprovacao(t *testing.T) *Servidor {
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
	return &Servidor{
		Sessoes:             postgres.NovoSessaoRepositorio(pool),
		Aprovacoes:          postgres.NovoAprovacaoRepositorio(pool),
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
	}
}

func TestCriarSolicitarAprovacaoExpiraELimpaAoEstourarTimeout(t *testing.T) {
	s := servidorDeTesteParaAprovacao(t)
	s.TimeoutAprovacao = 20 * time.Millisecond

	sessao, err := s.Sessoes.Criar(context.Background(), "sessão de teste aprovação — timeout")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	captura := &clienteCapturaID{perfil: permissao.Privilegiado}
	s.Clientes.Registrar(captura)

	solicitar := s.criarSolicitarAprovacao(sessao.ID)
	aprovado, err := solicitar(context.Background(), agente.ChamadaFerramenta{Nome: "ferramenta-perigosa"})
	if err != nil {
		t.Fatalf("solicitar não deveria devolver erro no caminho de timeout: %v", err)
	}
	if aprovado {
		t.Error("timeout deveria resultar em não-aprovado")
	}

	if len(captura.ids) != 1 {
		t.Fatalf("esperava 1 broadcast de aprovacao_requerida, recebeu %d", len(captura.ids))
	}
	aprovacaoID, err := uuid.Parse(captura.ids[0])
	if err != nil {
		t.Fatalf("id de aprovação inválido: %v", err)
	}

	registro, err := s.Aprovacoes.BuscarPorID(context.Background(), aprovacaoID)
	if err != nil {
		t.Fatalf("erro ao buscar aprovação: %v", err)
	}
	if registro.Estado != "expirada" {
		t.Errorf("esperava estado expirada após timeout, veio %q", registro.Estado)
	}

	if len(s.AprovacoesPendentes.canais) != 0 {
		t.Errorf("esperava mapa de pendentes limpo após timeout, restaram %d entradas", len(s.AprovacoesPendentes.canais))
	}
}

func TestCriarSolicitarAprovacaoExpiraELimpaAoDesconectar(t *testing.T) {
	s := servidorDeTesteParaAprovacao(t)
	s.TimeoutAprovacao = time.Hour // não deve disparar; queremos exercitar o ctx.Done()

	sessao, err := s.Sessoes.Criar(context.Background(), "sessão de teste aprovação — desconexão")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	captura := &clienteCapturaID{perfil: permissao.Privilegiado}
	s.Clientes.Registrar(captura)

	ctx, cancelar := context.WithCancel(context.Background())
	solicitar := s.criarSolicitarAprovacao(sessao.ID)

	tipoResultado := struct {
		aprovado bool
		err      error
	}{}
	resultado := make(chan struct {
		aprovado bool
		err      error
	}, 1)
	go func() {
		aprovado, err := solicitar(ctx, agente.ChamadaFerramenta{Nome: "ferramenta-perigosa"})
		resultado <- struct {
			aprovado bool
			err      error
		}{aprovado, err}
	}()

	time.Sleep(50 * time.Millisecond) // dá tempo do broadcast acontecer antes de cancelar
	cancelar()

	select {
	case r := <-resultado:
		tipoResultado = r
	case <-time.After(2 * time.Second):
		t.Fatal("solicitar não retornou a tempo após cancelamento do contexto")
	}
	if tipoResultado.aprovado {
		t.Error("desconexão não deveria resultar em aprovado")
	}
	if tipoResultado.err == nil {
		t.Error("esperava ctx.Err() de volta")
	}

	if len(captura.ids) != 1 {
		t.Fatalf("esperava 1 broadcast de aprovacao_requerida, recebeu %d", len(captura.ids))
	}
	aprovacaoID, err := uuid.Parse(captura.ids[0])
	if err != nil {
		t.Fatalf("id de aprovação inválido: %v", err)
	}

	registro, err := s.Aprovacoes.BuscarPorID(context.Background(), aprovacaoID)
	if err != nil {
		t.Fatalf("erro ao buscar aprovação: %v", err)
	}
	if registro.Estado != "expirada" {
		t.Errorf("esperava estado expirada após desconexão, veio %q", registro.Estado)
	}

	if len(s.AprovacoesPendentes.canais) != 0 {
		t.Errorf("esperava mapa de pendentes limpo após desconexão, restaram %d entradas", len(s.AprovacoesPendentes.canais))
	}
}
