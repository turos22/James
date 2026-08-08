// Package memoria monta o contexto que vai pro Provider: a janela de
// recência da conversa (janela.go) e a memória durável em markdown
// (vault.go). São dois mecanismos independentes, com orçamentos separados.
package memoria

import (
	"bytes"
	"image"
	_ "image/png" // registra o decoder — DecodeConfig só lê o header

	"james/internal/agente"
)

type Janela struct {
	OrcamentoTokens int
}

// estimarTokensBytes é a régua grosseira compartilhada (4 caracteres/bytes
// por token). Usada tanto pra texto (EstimarTokens) quanto como fallback de
// imagem que não decodifica (custoImagem).
func estimarTokensBytes(n int) int {
	return (n + 3) / 4
}

// EstimarTokens é uma aproximação grosseira (4 caracteres por token) —
// suficiente pra decidir o corte da janela, não pra faturamento exato.
func EstimarTokens(texto string) int {
	return estimarTokensBytes(len(texto))
}

// custoFixoImagem é o custo em tokens de qualquer imagem no gemini-2.5-flash
// nesta chamada de API (sem media_resolution configurado).
//
// Medido ao vivo contra a API real (Task 15, CA3 da Fase 5), via
// GenerateContent + UsageMetadata.PromptTokensDetails isolando a
// modalidade "imagem" do resto do prompt: 258 tokens para 300x200, 384x384,
// 385x385, 960x540, 1920x1080, 3840x2160 e 5120x2880 — sempre o mesmo
// valor, do menor caso ao maior testado.
//
// A tabela de tiling documentada em
// ai.google.dev/gemini-api/docs/image-understanding (custo crescendo por
// tile acima de 384px) NÃO se confirmou para este modelo nesta chamada.
// O endpoint CountTokens também não serve pra verificar isso: devolve o
// mesmo valor fixo (~259) pra qualquer dimensão testada, até 8000x4000 —
// só GenerateContent com o usageMetadata real revela o custo verdadeiro.
// Documentação não é medição; aqui a medição discordou da doc que orientou
// a primeira versão desta função, e a medição venceu.
const custoFixoImagem = 258

// EstimarTokensImagem devolve o custo em tokens de uma imagem. Largura e
// altura ficam na assinatura por compatibilidade com o chamador (que já tem
// a dimensão decodificada em mãos) e porque um ajuste futuro de billing do
// Gemini pode voltar a depender delas — mas hoje o valor é fixo, verificado
// contra a API real (ver custoFixoImagem).
func EstimarTokensImagem(largura, altura int) int {
	return custoFixoImagem
}

func custoMensagem(m agente.Mensagem) int {
	custo := EstimarTokens(m.Texto)
	for _, r := range m.Resultados {
		custo += EstimarTokens(r.Saida) + EstimarTokens(r.Erro) + custoImagem(r.Imagem)
	}
	custo += custoImagem(m.Imagem)
	return custo
}

// custoImagem usa a dimensão da imagem, não o tamanho do arquivo. Quando o
// header não decodifica, cai na régua antiga (len/4): o argumento original
// deste arquivo — superestimar é o lado seguro pra um orçamento — continua
// valendo exatamente aí, para imagem quebrada.
func custoImagem(imagem []byte) int {
	if len(imagem) == 0 {
		return 0
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(imagem))
	if err != nil {
		return estimarTokensBytes(len(imagem))
	}
	return EstimarTokensImagem(cfg.Width, cfg.Height)
}

// Montar devolve, em ordem cronológica, as mensagens mais recentes que
// cabem em OrcamentoTokens. A última mensagem sempre entra, mesmo que
// sozinha já estoure o orçamento — janela vazia nunca é o resultado certo.
func (j Janela) Montar(mensagens []agente.Mensagem) []agente.Mensagem {
	var total int
	var reversa []agente.Mensagem

	for i := len(mensagens) - 1; i >= 0; i-- {
		custo := custoMensagem(mensagens[i])
		if total+custo > j.OrcamentoTokens && len(reversa) > 0 {
			break
		}
		total += custo
		reversa = append(reversa, mensagens[i])
	}

	resultado := make([]agente.Mensagem, len(reversa))
	for i, m := range reversa {
		resultado[len(reversa)-1-i] = m
	}
	return resultado
}
