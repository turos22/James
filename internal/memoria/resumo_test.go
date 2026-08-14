package memoria

import (
	"context"
	"errors"
	"strings"
	"testing"

	"james/internal/agente"
)

type providerDeResumo struct {
	resposta string
	erro     error
	recebido agente.Requisicao
	chamadas int
}

func (p *providerDeResumo) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	p.chamadas++
	p.recebido = req
	if p.erro != nil {
		return agente.Resposta{}, p.erro
	}
	return agente.Resposta{Texto: p.resposta}, nil
}

func (p *providerDeResumo) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

func TestResumidorProduzOPrimeiroResumo(t *testing.T) {
	provider := &providerDeResumo{resposta: "o Arthur pediu o clima e depois a agenda"}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 1000}

	novo, err := resumidor.Resumir(context.Background(), "", []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "como está o clima?"},
		{Papel: agente.PapelModelo, Texto: "22 graus e nublado"},
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if novo != "o Arthur pediu o clima e depois a agenda" {
		t.Fatalf("resumo errado: %q", novo)
	}
	if provider.chamadas != 1 {
		t.Fatalf("esperava 1 chamada, houve %d", provider.chamadas)
	}
}

// O resumo anterior tem que entrar no prompt, senão cada rodada esquece
// tudo que veio antes e o "rolante" não rola.
func TestResumidorLevaOResumoAnteriorNoPrompt(t *testing.T) {
	provider := &providerDeResumo{resposta: "resumo novo"}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 1000}

	if _, err := resumidor.Resumir(context.Background(), "resumo velho aqui", []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "e a agenda?"},
	}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	var achou bool
	for _, m := range provider.recebido.Mensagens {
		if strings.Contains(m.Texto, "resumo velho aqui") {
			achou = true
		}
	}
	if !achou {
		t.Fatal("o resumo anterior não foi ao provider")
	}
}

func TestResumidorSemMensagensNaoChamaOProvider(t *testing.T) {
	provider := &providerDeResumo{resposta: "não devia ser chamado"}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 1000}

	novo, err := resumidor.Resumir(context.Background(), "resumo velho", nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if novo != "resumo velho" {
		t.Fatalf("devia devolver o resumo velho intacto, veio %q", novo)
	}
	if provider.chamadas != 0 {
		t.Fatalf("não devia chamar o provider, chamou %d vezes", provider.chamadas)
	}
}

func TestResumidorPropagaOErroDoProvider(t *testing.T) {
	provider := &providerDeResumo{erro: errors.New("cota estourada")}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 1000}

	if _, err := resumidor.Resumir(context.Background(), "", []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "oi"},
	}); err == nil {
		t.Fatal("esperava erro do provider")
	}
}

// R5: sem teto, o resumo cresce sem parar e come a janela que ele deveria
// proteger.
func TestResumidorCortaOResumoNoOrcamento(t *testing.T) {
	longo := strings.Repeat("palavra ", 5000)
	provider := &providerDeResumo{resposta: longo}
	resumidor := Resumidor{Provider: provider, OrcamentoTokens: 100}

	novo, err := resumidor.Resumir(context.Background(), "", []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "oi"},
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if EstimarTokens(novo) > 100 {
		t.Fatalf("resumo passou do orçamento: %d tokens", EstimarTokens(novo))
	}
}
