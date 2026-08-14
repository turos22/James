package memoria

import (
	"context"
	"fmt"
	"strings"

	"james/internal/agente"
)

// promptResumo instrui o modelo a produzir o resumo rolante. Fala em
// primeira pessoa do James porque o texto volta pro system prompt dele: um
// resumo escrito como relatório de terceiro faz o modelo se referir a si
// mesmo na terceira pessoa no turno seguinte.
const promptResumo = `Você mantém o resumo de uma conversa entre o Arthur e o James.

Receberá o resumo atual (pode estar vazio) e o trecho de conversa que acabou de sair
da janela de contexto. Devolva UM resumo novo, que substitui o atual e incorpora o
trecho.

Regras:
- Preserve fatos, decisões, nomes, números e pendências. São o que o James vai
  precisar lembrar.
- Descarte cortesia, repetição e o passo a passo de ferramenta que já terminou.
- Escreva em português, em prosa corrida, sem cabeçalho e sem marcador.
- Não invente nada que não esteja no material recebido.
- Devolva apenas o resumo, sem preâmbulo.`

// Resumidor produz o resumo rolante de uma sessão. É separado da Janela de
// propósito: a Janela decide o que cabe no contexto, o Resumidor decide o
// que fazer com o que não coube.
type Resumidor struct {
	Provider agente.Provider
	// OrcamentoTokens é o teto do resumo. Sem ele, o resumo cresce a cada
	// rodada e passa a comer a janela que deveria proteger.
	OrcamentoTokens int
}

// Resumir devolve o resumo atualizado. Sem mensagens novas, devolve o
// anterior intacto e não gasta chamada ao provider.
func (r Resumidor) Resumir(ctx context.Context, resumoAtual string, novas []agente.Mensagem) (string, error) {
	if len(novas) == 0 {
		return resumoAtual, nil
	}

	var corpo strings.Builder
	corpo.WriteString("RESUMO ATUAL:\n")
	if resumoAtual == "" {
		corpo.WriteString("(vazio — esta é a primeira rodada)\n")
	} else {
		corpo.WriteString(resumoAtual)
		corpo.WriteString("\n")
	}
	corpo.WriteString("\nTRECHO QUE SAIU DA JANELA:\n")
	for _, m := range novas {
		// Uma mensagem pode ter Texto, Chamadas e Resultados em qualquer
		// combinação (PapelModelo com function calling sem texto de
		// acompanhamento fica só com Chamadas; PapelFerramenta nunca tem
		// Texto, só Resultados) — por isso os três são escritos
		// independentemente, não com um "continue" que descartaria a
		// mensagem inteira quando Texto estiver vazio.
		if m.Texto != "" {
			fmt.Fprintf(&corpo, "%s: %s\n", m.Papel, m.Texto)
		}
		for _, c := range m.Chamadas {
			fmt.Fprintf(&corpo, "%s chamou %s(%v)\n", m.Papel, c.Nome, c.Args)
		}
		for _, res := range m.Resultados {
			if res.Erro != "" {
				fmt.Fprintf(&corpo, "%s falhou: %s\n", res.Nome, res.Erro)
			} else {
				fmt.Fprintf(&corpo, "%s devolveu: %s\n", res.Nome, res.Saida)
			}
		}
	}

	resposta, err := r.Provider.Gerar(ctx, agente.Requisicao{
		Sistema:   promptResumo,
		Mensagens: []agente.Mensagem{{Papel: agente.PapelUsuario, Texto: corpo.String()}},
	})
	if err != nil {
		return "", fmt.Errorf("erro ao resumir a conversa: %w", err)
	}

	return cortarNoOrcamento(strings.TrimSpace(resposta.Texto), r.OrcamentoTokens), nil
}

// cortarNoOrcamento é a rede de segurança para quando o modelo ignora a
// instrução de ser breve. Corta em byte porque EstimarTokens é a mesma régua
// grosseira de 4 bytes por token usada no resto do pacote.
func cortarNoOrcamento(texto string, orcamentoTokens int) string {
	if orcamentoTokens <= 0 || EstimarTokens(texto) <= orcamentoTokens {
		return texto
	}
	limite := orcamentoTokens * 4
	if limite > len(texto) {
		return texto
	}
	// Não corta no meio de uma runa multibyte.
	for limite > 0 && !isInicioDeRuna(texto[limite]) {
		limite--
	}
	return strings.TrimSpace(texto[:limite])
}

func isInicioDeRuna(b byte) bool {
	return b&0xC0 != 0x80
}
