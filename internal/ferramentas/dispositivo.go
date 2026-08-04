package ferramentas

import (
	"context"
	"fmt"

	"james/internal/dispositivo"
)

// ManipularInterface porta app.go:194 (ManipularInterface) atrás da
// interface Ferramenta, trocando o socket TCP cru por internal/dispositivo.
// Coordenadas ainda vêm adivinhadas pelo modelo (Anexo A #20) — consertar é
// Fase 5.
type ManipularInterface struct {
	Ponte dispositivo.Ponte
}

func NovoManipularInterface(ponte dispositivo.Ponte) *ManipularInterface {
	return &ManipularInterface{Ponte: ponte}
}

func (m *ManipularInterface) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "manipular_interface",
		Descricao: "Interações com a interface gráfica: clicar em botões ou pressionar teclas.",
		Parametros: map[string]Parametro{
			"acao":  {Tipo: "string", Descricao: "'click' para mouse ou 'tecla' para pressionar teclas"},
			"x":     {Tipo: "integer", Descricao: "coordenada X (para clique)"},
			"y":     {Tipo: "integer", Descricao: "coordenada Y (para clique)"},
			"texto": {Tipo: "string", Descricao: "tecla a pressionar (para acao 'tecla', ex: 'space', 'enter')"},
		},
		Obrigatorios: []string{"acao"},
	}
}

func (m *ManipularInterface) Nivel() Nivel { return NivelPerigoso }

func (m *ManipularInterface) Executar(ctx context.Context, args map[string]any) (string, error) {
	acao, ok := args["acao"].(string)
	if !ok || acao == "" {
		return "", fmt.Errorf("parâmetro acao ausente ou vazio")
	}
	x, _ := args["x"].(float64)
	y, _ := args["y"].(float64)
	texto, _ := args["texto"].(string)

	saida, err := m.Ponte.EnviarAcao(ctx, dispositivo.AcaoTela{Acao: acao, X: int(x), Y: int(y), Texto: texto})
	if err != nil {
		return "", err
	}
	return saida, nil
}
