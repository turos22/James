package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/ferramentas"
	"james/internal/memoria"
)

type providerFixo struct{ texto string }

func (p *providerFixo) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	return agente.Resposta{Texto: p.texto}, nil
}
func (p *providerFixo) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

func servidorWSDeTeste(t *testing.T) *httptest.Server {
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

	s := &Servidor{
		Sessoes:    postgres.NovoSessaoRepositorio(pool),
		Mensagens:  postgres.NovoMensagemRepositorio(pool),
		Aprovacoes: postgres.NovoAprovacaoRepositorio(pool),
		Loop: &agente.Loop{
			Provider:     &providerFixo{texto: "resposta de teste"},
			Ferramentas:  ferramentas.NovoRegistro(),
			MaxIteracoes: 5,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: 8000},
		SistemaPrompt:       "sistema de teste",
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
		TimeoutAprovacao:    time.Second,
	}
	return httptest.NewServer(s.Rotas())
}

// ferramentaSempreExigeConfirmacao reproduz o desenho de ClaudeCode.ConfirmacaoNecessaria
// (decisão D10): NivelSeguro, então qualquer perfil passa por Pode(), mas
// exige confirmação humana sempre, mesmo do Privilegiado.
type ferramentaSempreExigeConfirmacao struct {
	mu        sync.Mutex
	executada bool
}

func (f *ferramentaSempreExigeConfirmacao) Declaracao() ferramentas.Declaracao {
	return ferramentas.Declaracao{Nome: "sempre_confirma"}
}
func (f *ferramentaSempreExigeConfirmacao) Nivel() ferramentas.Nivel                  { return ferramentas.NivelSeguro }
func (f *ferramentaSempreExigeConfirmacao) ConfirmacaoNecessaria(map[string]any) bool { return true }
func (f *ferramentaSempreExigeConfirmacao) Executar(ctx context.Context, args map[string]any) (string, error) {
	f.mu.Lock()
	f.executada = true
	f.mu.Unlock()
	return "executou", nil
}
func (f *ferramentaSempreExigeConfirmacao) foiExecutada() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.executada
}

// providerChamaFerramentaUmaVez chama sempre_confirma na primeira iteração,
// e devolve uma resposta final na segunda (depois da ferramenta resolvida).
type providerChamaFerramentaUmaVez struct{ chamou bool }

func (p *providerChamaFerramentaUmaVez) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	if !p.chamou {
		p.chamou = true
		return agente.Resposta{Chamadas: []agente.ChamadaFerramenta{{ID: "1", Nome: "sempre_confirma"}}}, nil
	}
	return agente.Resposta{Texto: "concluido"}, nil
}
func (p *providerChamaFerramentaUmaVez) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

// TestWSMesmaConexaoConsegueAprovarSuaPropriaAcao trava um bug real,
// descoberto na verificação ao vivo da Fase 3 em 2026-08-06: perguntar_ao_claude_code
// passou a exigir confirmação SEMPRE, inclusive do Privilegiado (D10) — e o
// desktop usa uma única conexão WS pra mandar mensagem e pra aprovar. O laço
// de leitura de ws() processa "mensagem" de forma síncrona dentro do mesmo
// goroutine que devia ler o "aprovar" seguinte — a conexão fica travada
// esperando aprovar a si mesma, e só destrava (negando) depois do timeout de
// aprovação inteiro. Verificado ao vivo: exatos 5 minutos, batendo com o
// default de 300s.
func TestWSMesmaConexaoConsegueAprovarSuaPropriaAcao(t *testing.T) {
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

	registro := ferramentas.NovoRegistro()
	tool := &ferramentaSempreExigeConfirmacao{}
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("erro ao registrar ferramenta: %v", err)
	}

	s := &Servidor{
		Sessoes:    postgres.NovoSessaoRepositorio(pool),
		Mensagens:  postgres.NovoMensagemRepositorio(pool),
		Aprovacoes: postgres.NovoAprovacaoRepositorio(pool),
		Loop: &agente.Loop{
			Provider:     &providerChamaFerramentaUmaVez{},
			Ferramentas:  registro,
			MaxIteracoes: 5,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: 8000},
		SistemaPrompt:       "sistema de teste",
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
		// Curto de propósito: se o deadlock não estiver corrigido, o teste
		// nega rápido em vez de travar 300s — mas o cenário corrigido
		// resolve em milissegundos, bem abaixo disso.
		TimeoutAprovacao: 2 * time.Second,
	}
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	inicio := time.Now()
	if err := conn.WriteJSON(Envelope{Tipo: "mensagem", ID: "1", Dados: codificarDados(map[string]string{"texto": "faz algo"})}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	achouRespostaFinal := false
	for i := 0; i < 10; i++ {
		var recebido Envelope
		if err := conn.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler do WS: %v", err)
		}
		if recebido.Tipo == "aprovacao_requerida" {
			var dados map[string]any
			if err := json.Unmarshal(recebido.Dados, &dados); err != nil {
				t.Fatalf("erro ao decodificar dados de aprovacao_requerida: %v", err)
			}
			idAprovacao, _ := dados["id"].(string)
			if idAprovacao == "" {
				t.Fatal("dados.id veio vazio no evento aprovacao_requerida")
			}
			// A MESMA conexão que disparou a ação tenta aprová-la — é
			// exatamente o cenário do desktop usando perguntar_ao_claude_code.
			if err := conn.WriteJSON(Envelope{Tipo: "aprovar", ID: idAprovacao, Dados: codificarDados(map[string]string{})}); err != nil {
				t.Fatalf("erro ao enviar aprovar: %v", err)
			}
		}
		if recebido.Tipo == "resposta_final" {
			achouRespostaFinal = true
			break
		}
	}
	decorrido := time.Since(inicio)

	if !achouRespostaFinal {
		t.Fatal("esperava receber um envelope resposta_final")
	}
	// A asserção real é sobre a ferramenta ter executado, não sobre o texto
	// final: o provider de teste devolve "concluido" na segunda chamada
	// independente do resultado da ferramenta, então só o texto não prova
	// nada — se o deadlock estiver presente, o loop continua depois da
	// negação por timeout e chegaria ao mesmo texto sem a ferramenta rodar.
	if !tool.foiExecutada() {
		t.Error("a ferramenta nunca executou — a mesma conexão não conseguiu aprovar sua própria ação (deadlock)")
	}
	// Corrigido, isso resolve em milissegundos. Travado, resolve só depois do
	// TimeoutAprovacao inteiro (2s neste teste — 300s por padrão em produção,
	// confirmado ao vivo).
	if decorrido >= s.TimeoutAprovacao {
		t.Errorf("levou %s, >= TimeoutAprovacao (%s) — sinal de que caiu no timeout em vez de aprovar de verdade", decorrido, s.TimeoutAprovacao)
	}
}

func TestWSMensagemDevolveRespostaFinal(t *testing.T) {
	servidor := servidorWSDeTeste(t)
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(Envelope{Tipo: "mensagem", ID: "1", Dados: codificarDados(map[string]string{"texto": "oi"})}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	achouRespostaFinal := false
	for i := 0; i < 5; i++ {
		var recebido Envelope
		if err := conn.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler do WS: %v", err)
		}
		if recebido.Tipo == "resposta_final" {
			achouRespostaFinal = true
			break
		}
	}
	if !achouRespostaFinal {
		t.Fatal("esperava receber um envelope resposta_final")
	}
}
