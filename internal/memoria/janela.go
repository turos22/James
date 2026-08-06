// Package memoria monta o contexto que vai pro Provider: a janela de
// recência da conversa (janela.go) e a memória durável em markdown
// (vault.go). São dois mecanismos independentes, com orçamentos separados.
package memoria

import "james/internal/agente"

type Janela struct {
	OrcamentoTokens int
}

// EstimarTokens é uma aproximação grosseira (4 caracteres por token) —
// suficiente pra decidir o corte da janela, não pra faturamento exato.
func EstimarTokens(texto string) int {
	return (len(texto) + 3) / 4
}

func custoMensagem(m agente.Mensagem) int {
	custo := EstimarTokens(m.Texto)
	for _, r := range m.Resultados {
		custo += EstimarTokens(r.Saida) + EstimarTokens(r.Erro)
	}
	// Imagem (screenshot, quando houver) é o payload mais pesado que uma
	// mensagem pode carregar — ordens de grandeza maior que qualquer texto
	// vizinho. Tratar como custo zero deixaria passar batido exatamente o
	// cenário que este pacote existe pra evitar (bug #4). Não precisa ser
	// preciso (é bytes binários, não texto), só não pode ser zero — usa a
	// mesma proporção grosseira de EstimarTokens, que aqui tende a
	// superestimar, e superestimar é o lado seguro pra um orçamento.
	custo += (len(m.Imagem) + 3) / 4
	return custo
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
