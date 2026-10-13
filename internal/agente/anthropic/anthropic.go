package anthropic

import (
	"context"
	"fmt"
	"log/slog"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"james/internal/agente"
)

// Provider fala com a API da Anthropic. Implementa agente.Provider.
type Provider struct {
	cliente sdk.Client
	modelo  string
}

// Novo cria o provedor. As opções extras vêm depois das padrão e vencem (é
// como os testes apontam pra um servidor falso).
//
// O SDK retenta duas vezes por padrão; aqui é UMA. A espera por retries
// empilhados é tempo parado no ouvido de quem falou, e uma falha que persiste
// depois da segunda tentativa não vai passar na terceira.
func Novo(apiKey, modelo string, opts ...option.RequestOption) *Provider {
	base := []option.RequestOption{option.WithAPIKey(apiKey), option.WithMaxRetries(1)}
	return &Provider{cliente: sdk.NewClient(append(base, opts...)...), modelo: modelo}
}

// ErroRecusa é o modelo se recusando a responder (stop_reason "refusal"), o
// que a API devolve com HTTP 200. Categoria é o motivo, quando há.
type ErroRecusa struct{ Categoria string }

func (e *ErroRecusa) Error() string {
	if e.Categoria == "" {
		return "o modelo recusou o pedido"
	}
	return fmt.Sprintf("o modelo recusou o pedido (categoria %s)", e.Categoria)
}

// Gerar devolve a resposta completa. Usa o mesmo caminho do streaming: é o
// que a documentação recomenda pra respostas longas, e evita timeout de HTTP.
func (p *Provider) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	return p.GerarStream(ctx, req, nil)
}

// GerarStream chama emitir a cada pedaço de texto assim que chega, e devolve
// a resposta completa no fim. emitir nil é válido.
func (p *Provider) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	fluxo := p.cliente.Messages.NewStreaming(ctx, paraParams(req, p.modelo))
	defer fluxo.Close()

	var msg sdk.Message
	for fluxo.Next() {
		evento := fluxo.Current()
		if err := msg.Accumulate(evento); err != nil {
			return agente.Resposta{}, fmt.Errorf("acumulando o stream: %w", err)
		}
		if emitir == nil {
			continue
		}
		if delta, ok := evento.AsAny().(sdk.ContentBlockDeltaEvent); ok {
			if texto, ok := delta.Delta.AsAny().(sdk.TextDelta); ok && texto.Text != "" {
				emitir(texto.Text)
			}
		}
	}
	if err := fluxo.Err(); err != nil {
		return agente.Resposta{}, err
	}

	switch msg.StopReason {
	case sdk.StopReasonRefusal:
		return agente.Resposta{}, &ErroRecusa{Categoria: string(msg.StopDetails.Category)}
	case sdk.StopReasonMaxTokens:
		// Resposta cortada pelo limite: devolve o que veio, mas deixa rastro.
		slog.Warn("resposta do modelo cortada por max_tokens", "modelo", p.modelo)
	}
	return paraResposta(msg), nil
}
