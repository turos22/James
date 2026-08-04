package ferramentas

import (
	"context"
	"errors"
	"testing"

	"james/internal/dispositivo"
)

type ponteFalsa struct {
	acaoRecebida dispositivo.AcaoTela
	saida        string
	erro         error
}

func (p *ponteFalsa) EnviarAcao(ctx context.Context, acao dispositivo.AcaoTela) (string, error) {
	p.acaoRecebida = acao
	return p.saida, p.erro
}

func TestManipularInterfaceRepassaAcaoParaPonte(t *testing.T) {
	ponte := &ponteFalsa{saida: "clique feito"}
	m := NovoManipularInterface(ponte)

	saida, err := m.Executar(context.Background(), map[string]any{
		"acao": "click", "x": float64(100), "y": float64(200),
	})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if saida != "clique feito" {
		t.Errorf("saida = %q, queria clique feito", saida)
	}
	if ponte.acaoRecebida.X != 100 || ponte.acaoRecebida.Y != 200 {
		t.Errorf("coordenadas repassadas = %+v", ponte.acaoRecebida)
	}
}

func TestManipularInterfaceRejeitaAcaoAusente(t *testing.T) {
	m := NovoManipularInterface(&ponteFalsa{})
	if _, err := m.Executar(context.Background(), map[string]any{}); err == nil {
		t.Fatal("esperava erro por parâmetro acao ausente, veio nil")
	}
}

func TestManipularInterfacePropagaDispositivoAusente(t *testing.T) {
	ponte := &ponteFalsa{erro: dispositivo.ErroDispositivoAusente}
	m := NovoManipularInterface(ponte)

	_, err := m.Executar(context.Background(), map[string]any{"acao": "click"})
	if !errors.Is(err, dispositivo.ErroDispositivoAusente) {
		t.Fatalf("erro = %v, queria ErroDispositivoAusente (R3: resposta, não crash)", err)
	}
}

func TestManipularInterfaceNivelPerigoso(t *testing.T) {
	m := NovoManipularInterface(&ponteFalsa{})
	if m.Nivel() != NivelPerigoso {
		t.Error("manipular_interface deve ser NivelPerigoso")
	}
}
