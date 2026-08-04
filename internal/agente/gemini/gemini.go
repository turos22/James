// Package gemini implementa agente.Provider sobre a API do Gemini.
package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"

	"james/internal/agente"
)

type Provider struct {
	cliente *genai.Client
	modelo  string
}

var _ agente.Provider = (*Provider)(nil)

// Novo cria o Provider real. Conferir a lista vigente de modelos Gemini
// antes de fixar `modelo` em produção — gemini-2.5-flash-lite é proibido
// (internal/config já recusa isso no boot).
func Novo(ctx context.Context, apiKey, modelo string) (*Provider, error) {
	cliente, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("erro ao criar cliente Gemini: %w", err)
	}
	return &Provider{cliente: cliente, modelo: modelo}, nil
}

func (p *Provider) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	contents := paraGenaiContents(req.Mensagens)
	config := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: req.Sistema}}},
		Tools:             []*genai.Tool{paraGenaiTool(req.Declaracoes)},
	}

	resp, err := p.cliente.Models.GenerateContent(ctx, p.modelo, contents, config)
	if err != nil {
		return agente.Resposta{}, fmt.Errorf("gemini: %w", err)
	}
	return paraResposta(resp), nil
}

// GerarStream nesta fase não faz streaming token-a-token do SDK — chama
// Gerar e emite o texto final de uma vez. Streaming incremental de verdade
// fica como melhoria futura (não bloqueia nenhum critério de aceitação da
// Fase 1, que só exige o evento `token` existir no protocolo WS).
func (p *Provider) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	resp, err := p.Gerar(ctx, req)
	if err != nil {
		return agente.Resposta{}, err
	}
	if resp.Texto != "" && emitir != nil {
		emitir(resp.Texto)
	}
	return resp, nil
}
