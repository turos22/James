package api

import (
	"sync"

	"github.com/gorilla/websocket"

	"james/internal/permissao"
)

// ClienteConectado é o que o ClientesHub precisa de um cliente WS —
// desacoplado do transporte real pra ser testável sem socket de verdade.
type ClienteConectado interface {
	Perfil() permissao.Perfil
	Enviar(tipo, id string, dados any) error
}

type conexaoCliente struct {
	ws     *websocket.Conn
	mu     sync.Mutex
	perfil permissao.Perfil
}

func (c *conexaoCliente) Perfil() permissao.Perfil { return c.perfil }

func (c *conexaoCliente) Enviar(tipo, id string, dados any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteJSON(Envelope{Tipo: tipo, ID: id, Dados: codificarDados(dados)})
}

// ClientesHub rastreia os clientes conectados só pra poder fazer broadcast
// de aprovacao_requerida — cada conexão trata sua própria conversa direto.
type ClientesHub struct {
	mu    sync.Mutex
	todos map[ClienteConectado]bool
}

func NovoClientesHub() *ClientesHub {
	return &ClientesHub{todos: make(map[ClienteConectado]bool)}
}

func (h *ClientesHub) Registrar(c ClienteConectado) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.todos[c] = true
}

func (h *ClientesHub) Remover(c ClienteConectado) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.todos, c)
}

// BroadcastAprovacaoRequerida só atinge clientes Privilegiado — só quem
// pode decidir precisa ver o pedido de aprovação.
func (h *ClientesHub) BroadcastAprovacaoRequerida(id string, dados any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.todos {
		if c.Perfil() == permissao.Privilegiado {
			_ = c.Enviar("aprovacao_requerida", id, dados)
		}
	}
}
