// Package dispositivo é a ponte entre o jamesd e o james-device (Python):
// converte pedidos de ação de tela em mensagens WS e casa a resposta pelo
// id de correlação. Sem device conectado, devolve erro dedicado — nunca
// panic (R3, corrige o bug de app.go:218).
package dispositivo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

var ErroDispositivoAusente = errors.New("máquina física indisponível: james-device não conectado")

type AcaoTela struct {
	Acao  string `json:"acao"`
	X     int    `json:"x,omitempty"`
	Y     int    `json:"y,omitempty"`
	Texto string `json:"texto,omitempty"`
}

// Conexao é o suficiente do lado WS que o Hub precisa — internal/api
// implementa isso sobre o gorilla/websocket de verdade.
type Conexao interface {
	Enviar(envelope []byte) error
}

// Ponte é o que ferramentas.ManipularInterface precisa — *Hub implementa.
type Ponte interface {
	EnviarAcao(ctx context.Context, acao AcaoTela) (string, error)
}

type resultadoAcao struct {
	saida string
	erro  string
}

type Hub struct {
	mu        sync.Mutex
	conexao   Conexao
	pendentes map[string]chan resultadoAcao
	Timeout   time.Duration
}

func NovoHub() *Hub {
	return &Hub{pendentes: make(map[string]chan resultadoAcao), Timeout: 10 * time.Second}
}

func (h *Hub) Registrar(c Conexao) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conexao = c
}

func (h *Hub) Desregistrar() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conexao = nil
}

// EnviarAcao manda a ação pro device conectado e espera o resultado_acao
// correspondente, com timeout (R4).
func (h *Hub) EnviarAcao(ctx context.Context, acao AcaoTela) (string, error) {
	h.mu.Lock()
	conexao := h.conexao
	h.mu.Unlock()
	if conexao == nil {
		return "", ErroDispositivoAusente
	}

	id := uuid.NewString()
	espera := make(chan resultadoAcao, 1)
	h.mu.Lock()
	h.pendentes[id] = espera
	h.mu.Unlock()
	defer func() {
		h.mu.Lock()
		delete(h.pendentes, id)
		h.mu.Unlock()
	}()

	envelope, err := json.Marshal(map[string]any{"tipo": "acao_tela", "id": id, "dados": acao})
	if err != nil {
		return "", err
	}
	if err := conexao.Enviar(envelope); err != nil {
		return "", fmt.Errorf("erro ao enviar ação pro device: %w", err)
	}

	select {
	case r := <-espera:
		if r.erro != "" {
			return "", fmt.Errorf("device: %s", r.erro)
		}
		return r.saida, nil
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(h.Timeout):
		return "", fmt.Errorf("timeout esperando resultado_acao do device")
	}
}

// ResolverResultado é chamado pelo endpoint WS do device quando chega um
// resultado_acao — casa pelo id de correlação com quem está esperando em
// EnviarAcao.
func (h *Hub) ResolverResultado(id, saida, erro string) {
	h.mu.Lock()
	espera, ok := h.pendentes[id]
	h.mu.Unlock()
	if !ok {
		return
	}
	espera <- resultadoAcao{saida: saida, erro: erro}
}
