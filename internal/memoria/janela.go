// Package memoria monta o contexto que vai pro Provider: a janela de
// recência da conversa (janela.go) e a memória durável em markdown
// (vault.go). São dois mecanismos independentes, com orçamentos separados.
package memoria

import (
	"bytes"
	"image"
	_ "image/jpeg" // registra o decoder — DecodeConfig só lê o header
	_ "image/png"

	"james/internal/agente"
)

type Janela struct {
	OrcamentoTokens int
}

// EstimarTokens é uma aproximação grosseira (4 caracteres por token) —
// suficiente pra decidir o corte da janela, não pra faturamento exato.
func EstimarTokens(texto string) int {
	return (len(texto) + 3) / 4
}

const (
	tokensPorTile = 258
	ladoTileUnico = 384
)

// EstimarTokensImagem devolve o custo em tokens de uma imagem pela tabela do
// Gemini: 258 tokens quando ambos os lados cabem em 384 px, e 258 por tile
// acima disso, com o tile derivado de floor(menor lado / 1.5).
//
// Confirmado contra a doc oficial (ai.google.dev/gemini-api/docs/image-
// understanding) e ancorado no exemplo de lá: 960x540 tem crop 360 e dá 6
// tiles (3x2). A régua antiga deste arquivo cobrava len(bytes)/4, o que
// errava por cerca de 80x e criava um teto que tornaria a visão inviável.
func EstimarTokensImagem(largura, altura int) int {
	if largura <= 0 || altura <= 0 {
		return tokensPorTile
	}
	if largura <= ladoTileUnico && altura <= ladoTileUnico {
		return tokensPorTile
	}

	menor := largura
	if altura < menor {
		menor = altura
	}
	crop := menor * 2 / 3 // floor(menor / 1.5)
	if crop <= 0 {
		crop = 1
	}
	return dividirArredondandoPraCima(largura, crop) * dividirArredondandoPraCima(altura, crop) * tokensPorTile
}

func dividirArredondandoPraCima(a, b int) int {
	return (a + b - 1) / b
}

func custoMensagem(m agente.Mensagem) int {
	custo := EstimarTokens(m.Texto)
	for _, r := range m.Resultados {
		custo += EstimarTokens(r.Saida) + EstimarTokens(r.Erro)
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
		return (len(imagem) + 3) / 4
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
