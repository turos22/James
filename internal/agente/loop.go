package agente

import (
	"context"
	"errors"
	"fmt"
	"time"

	"james/internal/ferramentas"
	"james/internal/permissao"
)

// ErroTetoIteracoes é devolvido quando o loop estoura o teto sem chegar a
// uma resposta final — R5: mensagem honesta, nunca fallback mascarando
// fracasso.
var ErroTetoIteracoes = errors.New("atingiu o teto de iterações sem concluir")

type Loop struct {
	Provider     Provider
	Ferramentas  ferramentas.Registro
	MaxIteracoes int

	// TimeoutProvider limita cada chamada ao Provider.Gerar. Zero desliga.
	// Não embrulha o loop inteiro de propósito: a espera por aprovação
	// humana acontece ENTRE chamadas ao Provider, e um teto de segundos
	// mataria uma decisão que legitimamente leva minutos.
	TimeoutProvider time.Duration

	// AoIniciarFerramenta e AoConcluirFerramenta, quando não-nil, são
	// chamados em volta de cada despacho — é o que a API (Task 17) usa pra
	// emitir ferramenta_iniciada/ferramenta_concluida no WS em tempo real,
	// em vez de só no final do turno. Opcionais: nil é um loop mudo, útil
	// nos testes deste pacote.
	AoIniciarFerramenta  func(ChamadaFerramenta)
	AoConcluirFerramenta func(ResultadoFerramenta)
}

// SolicitarAprovacao pergunta a um cliente privilegiado se autoriza uma
// chamada perigosa vinda de um cliente Restrito. nil é válido quando não se
// espera nenhuma chamada perigosa vinda de perfil Restrito nesta execução
// (ex.: sessão do desktop) — se essa suposição for falsa, despachar trata
// como negado, nunca como aprovado por omissão.
type SolicitarAprovacao func(ctx context.Context, chamada ChamadaFerramenta) (aprovado bool, err error)

// Executar roda o loop até obter uma resposta sem chamada de ferramenta ou
// estourar MaxIteracoes. Devolve também o histórico de mensagens acumulado
// (para persistência pelo chamador).
func (l *Loop) Executar(ctx context.Context, sistema string, historico []Mensagem, entrada Mensagem, perfil permissao.Perfil, solicitarAprovacao SolicitarAprovacao) (Resposta, []Mensagem, error) {
	mensagens := append(append([]Mensagem{}, historico...), entrada)

	for i := 0; i < l.MaxIteracoes; i++ {
		// Defesa em profundidade: um cliente WS desconectado cancela o
		// ctx, e o loop não deve seguir despachando chamadas de ferramenta
		// (possivelmente já aprovadas, NivelPerigoso) entre iterações.
		if err := ctx.Err(); err != nil {
			return Resposta{}, mensagens, err
		}

		req := Requisicao{Sistema: sistema, Mensagens: mensagens, Declaracoes: l.Ferramentas.Declaracoes()}
		resp, err := l.gerar(ctx, req)
		if err != nil {
			return Resposta{}, mensagens, fmt.Errorf("provider: %w", err)
		}

		mensagens = append(mensagens, Mensagem{Papel: PapelModelo, Texto: resp.Texto, Chamadas: resp.Chamadas})

		if len(resp.Chamadas) == 0 {
			return resp, mensagens, nil
		}

		resultados := make([]ResultadoFerramenta, 0, len(resp.Chamadas))
		for _, chamada := range resp.Chamadas {
			resultados = append(resultados, l.despachar(ctx, chamada, perfil, solicitarAprovacao))
		}
		mensagens = append(mensagens, Mensagem{Papel: PapelFerramenta, Resultados: resultados})
	}

	return Resposta{}, mensagens, ErroTetoIteracoes
}

// gerar embrulha a chamada ao Provider num timeout próprio, escopado só a
// essa chamada — nunca ao loop inteiro. Método dedicado (em vez de um defer
// solto dentro do for de Executar) de propósito: um defer dentro de um laço
// só executaria no retorno da função inteira, não a cada iteração, e
// acumularia um cancelador por volta até o fim do turno.
func (l *Loop) gerar(ctx context.Context, req Requisicao) (Resposta, error) {
	ctxProvider := ctx
	if l.TimeoutProvider > 0 {
		var cancelar context.CancelFunc
		ctxProvider, cancelar = context.WithTimeout(ctx, l.TimeoutProvider)
		defer cancelar()
	}
	return l.Provider.Gerar(ctxProvider, req)
}

func (l *Loop) despachar(ctx context.Context, chamada ChamadaFerramenta, perfil permissao.Perfil, solicitarAprovacao SolicitarAprovacao) ResultadoFerramenta {
	if l.AoIniciarFerramenta != nil {
		l.AoIniciarFerramenta(chamada)
	}
	resultado := l.despacharSemHook(ctx, chamada, perfil, solicitarAprovacao)
	if l.AoConcluirFerramenta != nil {
		l.AoConcluirFerramenta(resultado)
	}
	return resultado
}

func (l *Loop) despacharSemHook(ctx context.Context, chamada ChamadaFerramenta, perfil permissao.Perfil, solicitarAprovacao SolicitarAprovacao) ResultadoFerramenta {
	tool, ok := l.Ferramentas.Buscar(chamada.Nome)
	if !ok {
		return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: fmt.Sprintf("ferramenta %q não encontrada", chamada.Nome)}
	}

	precisaAprovar := !perfil.Pode(tool.Nivel())
	// Aditivo por contrato (ver ferramentas.ExigeConfirmacao): só liga, nunca
	// desliga o que o Nivel() já exigiu.
	if exigente, ok := tool.(ferramentas.ExigeConfirmacao); ok && exigente.ConfirmacaoNecessaria(chamada.Args) {
		precisaAprovar = true
	}

	if precisaAprovar {
		if solicitarAprovacao == nil {
			return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: "ação negada pelo usuário"}
		}
		aprovado, err := solicitarAprovacao(ctx, chamada)
		if err != nil {
			return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: fmt.Sprintf("erro ao pedir aprovação: %v", err)}
		}
		if !aprovado {
			return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: "ação negada pelo usuário"}
		}
	}

	saida, err := tool.Executar(ctx, chamada.Args)
	if err != nil {
		return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Erro: err.Error()}
	}
	return ResultadoFerramenta{ID: chamada.ID, Nome: chamada.Nome, Saida: saida}
}
