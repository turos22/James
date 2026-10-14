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
//
// Alvo e Linha entraram com a fatia B2 (janelas, mídia, abrir). Todos os
// campos novos são omitempty: o JSON das ações antigas não muda, e device
// velho com jamesd novo continua funcionando.
type AcaoTela struct {
	Acao       string `json:"acao"`
	X          int    `json:"x,omitempty"`
	Y          int    `json:"y,omitempty"`
	Texto      string `json:"texto,omitempty"`
	Superficie string `json:"superficie,omitempty"`
	ElementoID string `json:"elemento_id,omitempty"`
	Nome       string `json:"nome,omitempty"`
	Tipo       string `json:"tipo,omitempty"`
	Alvo       string `json:"alvo,omitempty"`
	Linha      int    `json:"linha,omitempty"`
}

// Elemento é um item da lista do ver_tela, do jeito que o device mandou.
// É o que deixa o jamesd saber o NOME de "e7" antes de clicar, e pedir
// aprovação quando o nome é destrutivo (D10 da spec 2026-10-13 B2).
type Elemento struct {
	ID   string `json:"id"`
	Nome string `json:"nome"`
	Tipo string `json:"tipo"`
}

// Resultado é o que o device devolve. A imagem e os elementos só vêm
// preenchidos em ver_tela; as outras ações devolvem só texto.
type Resultado struct {
	Saida     string
	Imagem    []byte
	Elementos []Elemento
}

// Resposta é o resultado_acao decodificado, como chega do WS.
type Resposta struct {
	Saida     string
	Erro      string
	Imagem    []byte
	Elementos []Elemento
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

// Hub é a ponte com o james-device: mantém a conexão WS atual e casa cada
// EnviarAcao pendente com o resultado_acao correspondente pelo id.
type Hub struct {
	mu        sync.Mutex
	conexao   Conexao
	pendentes map[string]chan Resposta

	// ultimoMapa é a lista do último ver_tela bem-sucedido do device ATUAL.
	// Troca a cada ver_tela e zera quando o device muda: o "e7" de um device
	// não diz nada sobre a tela de outro.
	ultimoMapa []Elemento

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
	return &Hub{
		pendentes:      make(map[string]chan Resposta),
		Timeout:        10 * time.Second,
		TimeoutPorAcao: make(map[string]time.Duration),
	}
}

// Registrar associa a conexão WS do device conectado. Substitui
// silenciosamente qualquer conexão registrada antes — a antiga não recebe
// nenhuma notificação; é responsabilidade de quem chama fechar/descartar a
// conexão anterior antes de registrar uma nova, se isso importar.
func (h *Hub) Registrar(c Conexao) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conexao != c {
		h.ultimoMapa = nil
	}
	h.conexao = c
}

// Desregistrar remove a conexão informada, e só ela: se outra já tomou o
// lugar dela em Registrar, esta chamada é um no-op. A partir daí EnviarAcao
// volta a devolver ErroDispositivoAusente — a menos que exista uma conexão
// mais nova registrada.
//
// Receber a conexão como parâmetro (em vez de zerar o que estiver lá) não é
// zelo excessivo: o device reconecta sozinho a cada 2s (conexao.py) e o lado
// Go só percebe um socket meio-aberto quando o read falha, o que pode demorar.
// A ordem "a nova registra, e só depois a velha morre" é o caso NORMAL de uma
// queda de rede, não a borda. Com o Desregistrar incondicional de antes, o
// defer da conexão velha apagava o registro da NOVA, e o Hub passava a
// responder ErroDispositivoAusente para sempre — com um device vivo do outro
// lado e sem nada que o fizesse reconectar, porque do ponto de vista dele a
// conexão estava perfeita. Coberto por
// TestReconexaoDoDeviceNaoDesregistraAConexaoNova (internal/api).
func (h *Hub) Desregistrar(c Conexao) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conexao == c {
		h.conexao = nil
		h.ultimoMapa = nil
	}
}

// Enviar manda um envelope ao device conectado sem esperar resposta (aviso,
// como o "silencio" do áudio do turno de texto). Sem device, devolve
// ErroDispositivoAusente.
func (h *Hub) Enviar(envelope []byte) error {
	h.mu.Lock()
	conexao := h.conexao
	h.mu.Unlock()
	if conexao == nil {
		return ErroDispositivoAusente
	}
	return conexao.Enviar(envelope)
}

// UltimoMapa devolve uma cópia da lista do último ver_tela bem-sucedido.
func (h *Hub) UltimoMapa() []Elemento {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]Elemento(nil), h.ultimoMapa...)
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
	espera := make(chan Resposta, 1)
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
		if r.Erro != "" {
			h.esquecerMapaSeVerTela(acao, conexao)
			return Resultado{}, fmt.Errorf("device: %s", r.Erro)
		}
		if acao.Acao == "ver_tela" {
			h.mu.Lock()
			if h.conexao == conexao {
				h.ultimoMapa = append([]Elemento(nil), r.Elementos...)
			}
			h.mu.Unlock()
		}
		return Resultado{Saida: r.Saida, Imagem: r.Imagem, Elementos: r.Elementos}, nil
	case <-ctx.Done():
		h.esquecerMapaSeVerTela(acao, conexao)
		return Resultado{}, ctx.Err()
	case <-time.After(h.timeoutDe(acao.Acao)):
		h.esquecerMapaSeVerTela(acao, conexao)
		return Resultado{}, fmt.Errorf("timeout esperando resultado_acao do device")
	}
}

// esquecerMapaSeVerTela: um ver_tela que não chegou inteiro pode ter
// renumerado os ids no device. O mapa velho passaria a dizer o nome errado de
// "e7" na hora de decidir se o clique pede aprovação.
func (h *Hub) esquecerMapaSeVerTela(acao AcaoTela, conexao Conexao) {
	if acao.Acao != "ver_tela" {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.conexao == conexao {
		h.ultimoMapa = nil
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
//
// A imagem passa a pertencer ao Hub a partir desta chamada — quem chama não
// deve reutilizar o buffer depois (a slice é entregue direto pro consumidor
// de EnviarAcao, sem cópia).
func (h *Hub) ResolverResultado(id string, resposta Resposta) {
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
	case espera <- resposta:
	default:
		// Ninguém mais está esperando nesse channel (não deveria acontecer
		// já que acabamos de tirá-lo de pendentes sob lock, mas o select
		// não-bloqueante garante que mesmo assim nunca travamos aqui).
	}
}
