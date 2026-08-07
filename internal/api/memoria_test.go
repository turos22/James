package api

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/ferramentas"
	"james/internal/memoria"
	"james/internal/permissao"
)

// providerEspiao captura o Sistema que chegou ao Provider. É o único jeito de
// provar que a nota entrou no prompt de verdade, e não só que o arquivo
// existe no disco.
type providerEspiao struct {
	mu              sync.Mutex
	sistemaRecebido string
}

func (p *providerEspiao) Gerar(_ context.Context, req agente.Requisicao) (agente.Resposta, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sistemaRecebido = req.Sistema
	return agente.Resposta{Texto: "ok"}, nil
}

func (p *providerEspiao) GerarStream(ctx context.Context, req agente.Requisicao, _ func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

func (p *providerEspiao) Sistema() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sistemaRecebido
}

func servidorComEspiao(t *testing.T) (*Servidor, *providerEspiao) {
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

	espiao := &providerEspiao{}
	s := &Servidor{
		Sessoes:    postgres.NovoSessaoRepositorio(pool),
		Mensagens:  postgres.NovoMensagemRepositorio(pool),
		Aprovacoes: postgres.NovoAprovacaoRepositorio(pool),
		Loop: &agente.Loop{
			Provider:     espiao,
			Ferramentas:  ferramentas.NovoRegistro(),
			MaxIteracoes: 5,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: 8000},
		SistemaPrompt:       "sistema de teste",
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
		TimeoutAprovacao:    time.Second,
	}
	return s, espiao
}

func TestProcessarTurnoInjetaMemoriaNoSistemaPrompt(t *testing.T) {
	raiz := filepath.Join(t.TempDir(), "Memoria")
	vault := memoria.NovoVault(raiz, 2000)
	if err := vault.Acrescentar("arthur", "Prefere resposta curta e direta."); err != nil {
		t.Fatalf("Acrescentar devolveu erro: %v", err)
	}

	s, espiao := servidorComEspiao(t)
	s.Notas = vault

	var sessaoID uuid.UUID
	var historico []agente.Mensagem
	if _, _, err := s.processarTurno(context.Background(), permissao.Privilegiado, &sessaoID, &historico,
		"oi", func(agente.ChamadaFerramenta) {}, func(agente.ResultadoFerramenta) {}); err != nil {
		t.Fatalf("processarTurno devolveu erro: %v", err)
	}

	if !strings.Contains(espiao.Sistema(), "Prefere resposta curta e direta.") {
		t.Fatalf("a nota não chegou ao system prompt; recebido = %q", espiao.Sistema())
	}
	if !strings.Contains(espiao.Sistema(), "sistema de teste") {
		t.Error("o system prompt original sumiu; a memória substituiu em vez de somar")
	}
}

func TestProcessarTurnoSegueSemNotasQuandoVaultAusente(t *testing.T) {
	s, espiao := servidorComEspiao(t)
	s.Notas = memoria.NovoVault(filepath.Join(t.TempDir(), "nao-existe"), 2000)

	var sessaoID uuid.UUID
	var historico []agente.Mensagem
	if _, _, err := s.processarTurno(context.Background(), permissao.Privilegiado, &sessaoID, &historico,
		"oi", func(agente.ChamadaFerramenta) {}, func(agente.ResultadoFerramenta) {}); err != nil {
		t.Fatalf("vault ausente não pode derrubar o turno: %v", err)
	}
	if espiao.Sistema() != "sistema de teste" {
		t.Errorf("sem vault, o prompt tem que ficar igual ao original; recebido = %q", espiao.Sistema())
	}
}
