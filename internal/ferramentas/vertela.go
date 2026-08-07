package ferramentas

import (
	"context"

	"james/internal/dispositivo"
)

// VerTela é a sexta ferramenta do registro. Devolve a imagem de uma
// superfície mais a lista de elementos clicáveis dela, e é o que faz o item
// "VISÃO PROATIVA" do system prompt deixar de apontar pro vazio.
//
// NivelSeguro por decisão D5 da spec: sem aprovação, em qualquer perfil —
// inclusive celular vendo a tela do desktop. Não é um descuido: a
// ferramenta não altera estado (o que justificaria NivelSeguro pela régua
// normal), mas ela lê a tela inteira e manda pra nuvem sem gate nenhum.
// Consequência aceita explicitamente pelo Arthur (ver R7 da spec da Fase 5).
type VerTela struct {
	Ponte dispositivo.Ponte
}

func NovoVerTela(ponte dispositivo.Ponte) *VerTela {
	return &VerTela{Ponte: ponte}
}

func (v *VerTela) Declaracao() Declaracao {
	return Declaracao{
		Nome: "ver_tela",
		Descricao: "Captura a tela e lista os elementos clicáveis nela, cada um com id, nome, tipo e posição. " +
			"Chame antes de manipular_interface para saber onde as coisas estão em vez de adivinhar coordenadas.",
		Parametros: map[string]Parametro{
			"superficie": {Tipo: "string", Descricao: "id do monitor (ex: DISPLAY1). Omitir para usar o monitor da janela em foco"},
		},
	}
}

func (v *VerTela) Nivel() Nivel { return NivelSeguro }

func (v *VerTela) ExecutarComImagem(ctx context.Context, args map[string]any) (string, []byte, error) {
	superficie, _ := args["superficie"].(string)

	r, err := v.Ponte.EnviarAcao(ctx, dispositivo.AcaoTela{Acao: "ver_tela", Superficie: superficie})
	if err != nil {
		return "", nil, err
	}
	return r.Saida, r.Imagem, nil
}

// Executar existe porque o Registro só aceita Ferramenta. O loop prefere
// ExecutarComImagem quando a ferramenta a implementa, então este caminho só
// roda pra quem chamar a ferramenta fora do loop — e aí a imagem se perde,
// que é o comportamento certo pra um consumidor que não sabe o que fazer
// com ela.
func (v *VerTela) Executar(ctx context.Context, args map[string]any) (string, error) {
	saida, _, err := v.ExecutarComImagem(ctx, args)
	return saida, err
}
