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

const (
	tokensPorTile = 258
	// ladoMaximoTileUnico é o maior lado (em px) até o qual a imagem inteira
	// ainda cabe em um único tile — acima disso o custo passa a crescer por
	// tile.
	ladoMaximoTileUnico = 384
)

// EstimarTokensImagem devolve o custo em tokens de uma imagem pela tabela do
// Gemini: 258 tokens quando ambos os lados cabem em 384 px, e 258 por tile
// acima disso, com o tile derivado de floor(menor lado / 1.5). Dimensão
// inválida (largura ou altura <= 0) cai no custo de um tile — nunca zero,
// que é o lado seguro pra um orçamento. O custo depende só da proporção da
// imagem, não da resolução absoluta: 960x540 e 1920x1080 dão o mesmo tanto
// de tiles, porque o tile cresce junto com a imagem.
//
// Confirmado contra a doc oficial (https://ai.google.dev/gemini-api/docs/image-understanding)
// e ancorado no exemplo de lá: 960x540 tem crop 360 e dá 6 tiles (3x2). A
// régua antiga deste arquivo cobrava len(bytes)/4, o que errava por cerca de
// 80x e criava um teto que tornaria a visão inviável.
func EstimarTokensImagem(largura, altura int) int {
	if largura <= 0 || altura <= 0 {
		return tokensPorTile
	}
	if largura <= ladoMaximoTileUnico && altura <= ladoMaximoTileUnico {
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

// dividirArredondandoPraCima só é chamada com divisor >= 1 (garantido pela
// guarda "crop <= 0" em EstimarTokensImagem).
func dividirArredondandoPraCima(dividendo, divisor int) int {
	return (dividendo + divisor - 1) / divisor
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
