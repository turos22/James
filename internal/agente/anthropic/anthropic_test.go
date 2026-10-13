package anthropic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/option"

	"james/internal/agente"
	"james/internal/ferramentas"
)

// servidorFalso serve uma fixture SSE e guarda o que o cliente mandou.
type servidorFalso struct {
	*httptest.Server
	mu       sync.Mutex
	corpos   []map[string]any
	cabecas  []http.Header
	caminhos []string
}

func novoServidor(t *testing.T, fixture string) *servidorFalso {
	t.Helper()
	bruto, err := os.ReadFile("testdata/" + fixture)
	if err != nil {
		t.Fatalf("fixture %s: %v", fixture, err)
	}
	s := &servidorFalso{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, _ := io.ReadAll(r.Body)
		var m map[string]any
		_ = json.Unmarshal(corpo, &m)
		s.mu.Lock()
		s.corpos = append(s.corpos, m)
		s.cabecas = append(s.cabecas, r.Header.Clone())
		s.caminhos = append(s.caminhos, r.URL.Path)
		s.mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write(bruto)
	}))
	t.Cleanup(s.Close)
	return s
}

func provedorContra(s *httptest.Server, extra ...option.RequestOption) *Provider {
	opts := append([]option.RequestOption{option.WithBaseURL(s.URL)}, extra...)
	return Novo("chave-de-teste", "claude-haiku-5-5", opts...)
}

func pergunta() agente.Requisicao {
	return agente.Requisicao{
		Sistema:   "Responda curto.",
		Mensagens: []agente.Mensagem{{Papel: agente.PapelUsuario, Texto: "Qual é a capital da Austrália?"}},
	}
}

func TestGerarStreamEmiteOsDeltasNaOrdem(t *testing.T) {
	srv := novoServidor(t, "texto.sse")
	var deltas []string

	resp, err := provedorContra(srv.Server).GerarStream(context.Background(), pergunta(), func(d string) { deltas = append(deltas, d) })

	if err != nil {
		t.Fatalf("GerarStream devolveu erro: %v", err)
	}
	if len(deltas) < 2 {
		t.Fatalf("%d deltas; streaming de verdade manda vários pedaços: %v", len(deltas), deltas)
	}
	if juntos := strings.Join(deltas, ""); juntos != resp.Texto {
		t.Errorf("deltas juntos = %q, Texto = %q", juntos, resp.Texto)
	}
	if resp.Texto != "A capital da Austrália é Camberra." {
		t.Errorf("Texto = %q", resp.Texto)
	}
	if resp.Uso.Entrada != 43 || resp.Uso.Saida != 16 {
		t.Errorf("Uso = %+v, queria entrada 43 e saída 16 (vêm do message_delta)", resp.Uso)
	}
}

func TestGerarStreamDecodificaAChamadaDeFerramenta(t *testing.T) {
	srv := novoServidor(t, "ferramenta.sse")
	req := pergunta()
	req.Declaracoes = []ferramentas.Declaracao{{
		Nome: "executar_comando_sistema", Descricao: "Executa comandos.",
		Parametros:   map[string]ferramentas.Parametro{"comando": {Tipo: "string"}},
		Obrigatorios: []string{"comando"},
	}}

	resp, err := provedorContra(srv.Server).GerarStream(context.Background(), req, nil)

	if err != nil {
		t.Fatalf("GerarStream devolveu erro: %v", err)
	}
	if len(resp.Chamadas) != 1 {
		t.Fatalf("Chamadas = %+v, queria uma", resp.Chamadas)
	}
	c := resp.Chamadas[0]
	if c.Nome != "executar_comando_sistema" || c.Args["comando"] != "ls -la" || !strings.HasPrefix(c.ID, "toolu_") {
		t.Errorf("chamada = %+v", c)
	}
}

func TestRequisicaoSaiComStreamModoEPensamento(t *testing.T) {
	casos := []struct {
		nome     string
		modo     agente.Modo
		thinking string
		esforco  string
	}{
		{"voz desliga o pensamento", agente.ModoVoz, "disabled", ""},
		{"texto pensa com esforço baixo", agente.ModoTexto, "adaptive", "low"},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			srv := novoServidor(t, "texto.sse")
			req := pergunta()
			req.Modo = c.modo

			if _, err := provedorContra(srv.Server).GerarStream(context.Background(), req, nil); err != nil {
				t.Fatalf("GerarStream devolveu erro: %v", err)
			}

			corpo := srv.corpos[0]
			if srv.caminhos[0] != "/v1/messages" {
				t.Errorf("caminho = %s", srv.caminhos[0])
			}
			if corpo["stream"] != true {
				t.Errorf("stream = %v, queria true", corpo["stream"])
			}
			if corpo["max_tokens"] != float64(4096) || corpo["model"] != "claude-haiku-5-5" {
				t.Errorf("max_tokens/model = %v/%v", corpo["max_tokens"], corpo["model"])
			}
			thinking, _ := corpo["thinking"].(map[string]any)
			if thinking["type"] != c.thinking {
				t.Errorf("thinking = %v, queria %s", corpo["thinking"], c.thinking)
			}
			saida, _ := corpo["output_config"].(map[string]any)
			if saida["effort"] != nilSeVazio(c.esforco) {
				t.Errorf("effort = %v, queria %q", saida["effort"], c.esforco)
			}
			if srv.cabecas[0].Get("x-api-key") != "chave-de-teste" {
				t.Error("a chave não foi no cabeçalho x-api-key")
			}
		})
	}
}

func nilSeVazio(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func TestRecusaViraErroTipadoComCategoria(t *testing.T) {
	srv := novoServidor(t, "recusa.sse")

	_, err := provedorContra(srv.Server).GerarStream(context.Background(), pergunta(), nil)

	var recusa *ErroRecusa
	if !errors.As(err, &recusa) {
		t.Fatalf("erro = %v, queria *ErroRecusa", err)
	}
	if recusa.Categoria != "cyber" {
		t.Errorf("Categoria = %q, queria cyber", recusa.Categoria)
	}
	if msg := recusa.Error(); !strings.Contains(msg, "recus") {
		t.Errorf("mensagem = %q, deveria dizer que foi recusado", msg)
	}
}

func TestErroDoServidorViraErroEPoucaEspera(t *testing.T) {
	var hits int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		hits++
		mu.Unlock()
		// retry-after-ms curto: o teste não espera o backoff padrão do SDK.
		w.Header().Set("Retry-After-Ms", "1")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"falhou"}}`))
	}))
	defer srv.Close()

	_, err := provedorContra(srv).GerarStream(context.Background(), pergunta(), nil)

	if err == nil {
		t.Fatal("esperava erro do servidor, veio nil")
	}
	// Uma tentativa e UMA nova: a voz não pode ficar esperando retries em fila.
	if hits != 2 {
		t.Errorf("%d requisições, queria 2 (1 + 1 retry)", hits)
	}
}

func TestGerarDevolveOMesmoQueGerarStream(t *testing.T) {
	srv := novoServidor(t, "texto.sse")
	p := provedorContra(srv.Server)

	porStream, err1 := p.GerarStream(context.Background(), pergunta(), nil)
	semStream, err2 := p.Gerar(context.Background(), pergunta())

	if err1 != nil || err2 != nil {
		t.Fatalf("erros: %v / %v", err1, err2)
	}
	if porStream.Texto != semStream.Texto || porStream.Uso != semStream.Uso {
		t.Errorf("Gerar = %+v, GerarStream = %+v", semStream, porStream)
	}
}

func TestContextoCanceladoDevolveErro(t *testing.T) {
	srv := novoServidor(t, "texto.sse")
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()

	if _, err := provedorContra(srv.Server).GerarStream(ctx, pergunta(), nil); err == nil {
		t.Fatal("ctx cancelado deveria devolver erro")
	}
}

// Garante, em tempo de compilação, que o provedor serve ao loop.
var _ agente.Provider = (*Provider)(nil)
