package api

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"james/internal/permissao"
)

// tempoLimiteEscrita evita que um socket travado (app do cliente em
// background, link de VPN instável) prenda a escrita indefinidamente — sem
// isso, um cliente lento seguraria o mutex do hub e travaria Registrar/
// Remover de todo mundo (broadcast escreve com o lock liberado, mas cada
// escrita individual ainda pode travar sem um deadline).
const tempoLimiteEscrita = 5 * time.Second

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
	_ = c.ws.SetWriteDeadline(time.Now().Add(tempoLimiteEscrita))
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
// pode decidir precisa ver o pedido de aprovação. Tira um snapshot da lista
// e libera o mutex antes de escrever: escrita em socket é I/O bloqueante, e
// segurar o lock do hub durante ela travaria Registrar/Remover de todas as
// outras conexões se um único cliente estiver com o socket travado.
func (h *ClientesHub) BroadcastAprovacaoRequerida(id string, dados any) {
	h.mu.Lock()
	privilegiados := make([]ClienteConectado, 0, len(h.todos))
	for c := range h.todos {
		if c.Perfil() == permissao.Privilegiado {
			privilegiados = append(privilegiados, c)
		}
	}
	h.mu.Unlock()

	for _, c := range privilegiados {
		_ = c.Enviar("aprovacao_requerida", id, dados)
	}
}

// BroadcastParaPrivilegiados manda qualquer evento (token, ferramenta_*,
// resposta_final, erro) pra todo cliente Privilegiado conectado — é como o
// dono da máquina vê a resposta a uma entrada de voz vinda do device, já
// que a voz não chega por um cliente WS específico. Mesmo padrão de
// BroadcastAprovacaoRequerida: snapshot sob lock, escreve fora do lock.
func (h *ClientesHub) BroadcastParaPrivilegiados(tipo, id string, dados any) {
	h.mu.Lock()
	privilegiados := make([]ClienteConectado, 0, len(h.todos))
	for c := range h.todos {
		if c.Perfil() == permissao.Privilegiado {
			privilegiados = append(privilegiados, c)
		}
	}
	h.mu.Unlock()

	for _, c := range privilegiados {
		_ = c.Enviar(tipo, id, dados)
	}
}
