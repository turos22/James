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

// ErroDispositivoAusente é devolvido por EnviarAcao quando não há nenhum
// james-device conectado no momento do pedido — nunca um panic (R3).
var ErroDispositivoAusente = errors.New("máquina física indisponível: james-device não conectado")

// AcaoTela é o pedido de ação de tela que vira mensagem WS pro james-device.
// X e Y são LOCAIS à superfície (D15 da spec da Fase 5) — o device soma a
// origem do monitor antes de agir.
type AcaoTela struct {
	Acao       string `json:"acao"`
	X          int    `json:"x,omitempty"`
	Y          int    `json:"y,omitempty"`
	Texto      string `json:"texto,omitempty"`
	Superficie string `json:"superficie,omitempty"`
	ElementoID string `json:"elemento_id,omitempty"`
	Nome       string `json:"nome,omitempty"`
	Tipo       string `json:"tipo,omitempty"`
}

// Resultado é o que o device devolve. A imagem só vem preenchida em
// ver_tela; as outras ações devolvem só texto.
type Resultado struct {
	Saida  string
	Imagem []byte
}

// Conexao é o suficiente do lado WS que o Hub precisa — internal/api
// implementa isso sobre o gorilla/websocket de verdade.
type Conexao interface {
	Enviar(envelope []byte) error
}

// Ponte é o que ferramentas.ManipularInterface e ferramentas.VerTela
// precisam — *Hub implementa.
type Ponte interface {
	EnviarAcao(ctx context.Context, acao AcaoTela) (Resultado, error)
}

type resultadoAcao struct {
	saida  string
	erro   string
	imagem []byte
}

// Hub é a ponte com o james-device: mantém a conexão WS atual e casa cada
// EnviarAcao pendente com o resultado_acao correspondente pelo id.
type Hub struct {
	mu        sync.Mutex
	conexao   Conexao
	pendentes map[string]chan resultadoAcao

	// Timeout é quanto EnviarAcao espera por um resultado_acao antes de
	// desistir (R4). Deve ser ajustado antes do Hub entrar em uso
	// concorrente — não é protegido por mutex e mutar em runtime, com
	// EnviarAcao já em andamento em outra goroutine, é uma corrida de dados.
	Timeout time.Duration

	// TimeoutPorAcao permite teto maior pra ação cara: ver_tela varre a
	// árvore UIA de um monitor inteiro, o que não cabe no mesmo limite de um
	// clique. Ação sem entrada aqui usa Timeout. Mesma regra de Timeout:
	// ajustar antes do Hub entrar em uso concorrente, não em runtime.
	TimeoutPorAcao map[string]time.Duration
}

// NovoHub cria um Hub pronto pra uso, sem device conectado.
func NovoHub() *Hub {
	return &Hub{pendentes: make(map[string]chan resultadoAcao), Timeout: 10 * time.Second}
}

// Registrar associa a conexão WS do device conectado. Substitui
// silenciosamente qualquer conexão registrada antes — a antiga não recebe
// nenhuma notificação; é responsabilidade de quem chama fechar/descartar a
// conexão anterior antes de registrar uma nova, se isso importar.
func (h *Hub) Registrar(c Conexao) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conexao = c
}

// Desregistrar remove a conexão atual. A partir daí, EnviarAcao volta a
// devolver ErroDispositivoAusente.
func (h *Hub) Desregistrar() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conexao = nil
}

// timeoutDe devolve o timeout específico da ação, se houver um cadastrado em
// TimeoutPorAcao, ou o Timeout global caso contrário.
func (h *Hub) timeoutDe(acao string) time.Duration {
	if t, ok := h.TimeoutPorAcao[acao]; ok && t > 0 {
		return t
	}
	return h.Timeout
}

// EnviarAcao manda a ação pro device conectado e espera o resultado_acao
// correspondente, com timeout (R4).
func (h *Hub) EnviarAcao(ctx context.Context, acao AcaoTela) (Resultado, error) {
	h.mu.Lock()
	conexao := h.conexao
	h.mu.Unlock()
	if conexao == nil {
		return Resultado{}, ErroDispositivoAusente
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
		return Resultado{}, err
	}
	if err := conexao.Enviar(envelope); err != nil {
		return Resultado{}, fmt.Errorf("erro ao enviar ação pro device: %w", err)
	}

	select {
	case r := <-espera:
		if r.erro != "" {
			return Resultado{}, fmt.Errorf("device: %s", r.erro)
		}
		return Resultado{Saida: r.saida, Imagem: r.imagem}, nil
	case <-ctx.Done():
		return Resultado{}, ctx.Err()
	case <-time.After(h.timeoutDe(acao.Acao)):
		return Resultado{}, fmt.Errorf("timeout esperando resultado_acao do device")
	}
}

// ResolverResultado é chamado pelo endpoint WS do device quando chega um
// resultado_acao — casa pelo id de correlação com quem está esperando em
// EnviarAcao.
//
// O id é removido de pendentes ANTES do send, e o send é não-bloqueante: um
// segundo resultado_acao pro mesmo id (frame duplicado ou device com bug)
// nunca trava. Um id genuinamente atrasado (já expirado ou já resolvido)
// vira um no-op limpo — não um deadlock. Isso importa porque, no endpoint WS
// real (Task 18), ResolverResultado roda dentro da única goroutine que lê a
// conexão do device; travar aqui trava a leitura da conexão inteira.
func (h *Hub) ResolverResultado(id, saida, erro string, imagem []byte) {
	h.mu.Lock()
	espera, ok := h.pendentes[id]
	if ok {
		delete(h.pendentes, id)
	}
	h.mu.Unlock()
	if !ok {
		return
	}
	select {
	case espera <- resultadoAcao{saida: saida, erro: erro, imagem: imagem}:
	default:
		// Ninguém mais está esperando nesse channel (não deveria acontecer
		// já que acabamos de tirá-lo de pendentes sob lock, mas o select
		// não-bloqueante garante que mesmo assim nunca travamos aqui).
	}
}
