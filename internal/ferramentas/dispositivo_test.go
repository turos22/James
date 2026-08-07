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
	imagem       []byte
	erro         error
}

func (p *ponteFalsa) EnviarAcao(ctx context.Context, acao dispositivo.AcaoTela) (dispositivo.Resultado, error) {
	p.acaoRecebida = acao
	return dispositivo.Resultado{Saida: p.saida, Imagem: p.imagem}, p.erro
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

func TestManipularInterfaceRepassaCamposDeElementoESuperficie(t *testing.T) {
	ponte := &ponteFalsa{saida: "cliquei"}
	m := NovoManipularInterface(ponte)

	_, err := m.Executar(context.Background(), map[string]any{
		"acao": "clicar_elemento", "superficie": "DISPLAY7",
		"elemento_id": "e7", "nome": "Salvar", "tipo": "Button",
	})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if ponte.acaoRecebida.Superficie != "DISPLAY7" {
		t.Errorf("Superficie = %q, queria DISPLAY7", ponte.acaoRecebida.Superficie)
	}
	if ponte.acaoRecebida.ElementoID != "e7" {
		t.Errorf("ElementoID = %q, queria e7", ponte.acaoRecebida.ElementoID)
	}
	if ponte.acaoRecebida.Nome != "Salvar" {
		t.Errorf("Nome = %q, queria Salvar", ponte.acaoRecebida.Nome)
	}
	if ponte.acaoRecebida.Tipo != "Button" {
		t.Errorf("Tipo = %q, queria Button", ponte.acaoRecebida.Tipo)
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
