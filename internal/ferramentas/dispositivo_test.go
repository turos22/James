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

func TestManipularInterfaceRepassaTextoParaDigitar(t *testing.T) {
	// Achado ao vivo (2026-08-15): "tecla" só aceita uma tecla nomeada
	// (pyautogui.press), nunca uma frase — não havia como digitar texto
	// livre numa caixa. "digitar" é a ação nova; este teste confere que o
	// texto chega inteiro até a ponte, sem truncar nem perder na conversão.
	ponte := &ponteFalsa{saida: "digitei 'Disney Plus'"}
	m := NovoManipularInterface(ponte)

	saida, err := m.Executar(context.Background(), map[string]any{
		"acao": "digitar", "texto": "Disney Plus",
	})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if ponte.acaoRecebida.Acao != "digitar" || ponte.acaoRecebida.Texto != "Disney Plus" {
		t.Errorf("acao repassada = %+v, queria Acao=digitar Texto=\"Disney Plus\"", ponte.acaoRecebida)
	}
	if saida != "digitei 'Disney Plus'" {
		t.Errorf("saida = %q", saida)
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

// --- B2: aprovação para clique e tecla destrutivos (D10) ---------------------

type ponteComMapa struct {
	ponteFalsa
	mapa []dispositivo.Elemento
}

func (p *ponteComMapa) UltimoMapa() []dispositivo.Elemento { return p.mapa }

func manipularComMapa(elementos ...dispositivo.Elemento) *ManipularInterface {
	return NovoManipularInterface(&ponteComMapa{mapa: elementos})
}

func TestClicarElementoDestrutivoPeloIdPedeAprovacao(t *testing.T) {
	m := manipularComMapa(dispositivo.Elemento{ID: "e7", Nome: "Excluir", Tipo: "Button"},
		dispositivo.Elemento{ID: "e1", Nome: "Novo", Tipo: "Button"})

	if !m.ConfirmacaoNecessaria(map[string]any{"acao": "clicar_elemento", "elemento_id": "e7"}) {
		t.Error("clicar em Excluir pelo id não pediu aprovação")
	}
	if m.ConfirmacaoNecessaria(map[string]any{"acao": "clicar_elemento", "elemento_id": "e1"}) {
		t.Error("clicar em Novo pediu aprovação")
	}
}

func TestClicarElementoDestrutivoPeloNomePedeAprovacaoMesmoSemMapa(t *testing.T) {
	m := NovoManipularInterface(&ponteFalsa{})

	if !m.ConfirmacaoNecessaria(map[string]any{"acao": "clicar_elemento", "nome": "Mover para a lixeira"}) {
		t.Error("nome destrutivo nos argumentos não pediu aprovação")
	}
}

// Achado do code-review: o Hub zera o mapa quando o WS reconecta (a cada 2 s,
// é normal) e o device mantém o dele; sem o mapa, o id não diz o que é. Na
// dúvida, pede aprovação: o custo é um clique do Arthur, o contrário é
// apagar sem avisar.
func TestIdQueOHubNaoConheceFalhaParaOLadoSeguroEPedeAprovacao(t *testing.T) {
	m := manipularComMapa(dispositivo.Elemento{ID: "e1", Nome: "Novo"})
	if !m.ConfirmacaoNecessaria(map[string]any{"acao": "clicar_elemento", "elemento_id": "e99"}) {
		t.Error("id fora do mapa não pediu aprovação")
	}
	vazio := manipularComMapa()
	if !vazio.ConfirmacaoNecessaria(map[string]any{"acao": "clicar_elemento", "elemento_id": "e7"}) {
		t.Error("mapa vazio (WS reconectou) não pediu aprovação")
	}
}

func TestIdSemFonteDeMapaTambemPedeAprovacao(t *testing.T) {
	m := NovoManipularInterface(&ponteFalsa{})
	if !m.ConfirmacaoNecessaria(map[string]any{"acao": "clicar_elemento", "elemento_id": "e7"}) {
		t.Error("id sem como conferir não pediu aprovação")
	}
}

func TestNomeInocenteSemIdNaoPedeAprovacao(t *testing.T) {
	m := NovoManipularInterface(&ponteFalsa{})
	if m.ConfirmacaoNecessaria(map[string]any{"acao": "clicar_elemento", "nome": "Novo"}) {
		t.Error("clicar em Novo por nome pediu aprovação")
	}
}

func TestTeclaDeletePedeAprovacao(t *testing.T) {
	m := NovoManipularInterface(&ponteFalsa{})
	for _, tecla := range []string{"delete", "Delete", "shift+delete", "Shift + Delete", "del"} {
		if !m.ConfirmacaoNecessaria(map[string]any{"acao": "tecla", "texto": tecla}) {
			t.Errorf("tecla %q não pediu aprovação", tecla)
		}
	}
	for _, tecla := range []string{"enter", "ctrl+l", "backspace"} {
		if m.ConfirmacaoNecessaria(map[string]any{"acao": "tecla", "texto": tecla}) {
			t.Errorf("tecla %q pediu aprovação", tecla)
		}
	}
}

func TestClickEDigitarNaoPedemAprovacao(t *testing.T) {
	m := manipularComMapa(dispositivo.Elemento{ID: "e1", Nome: "Excluir"})
	for _, args := range []map[string]any{
		{"acao": "click", "x": 1.0, "y": 1.0},
		{"acao": "digitar", "texto": "excluir tudo"},
	} {
		if m.ConfirmacaoNecessaria(args) {
			t.Errorf("%v pediu aprovação", args)
		}
	}
}

func TestDescricaoDaConfirmacaoDizOQueVaiAcontecer(t *testing.T) {
	m := manipularComMapa(dispositivo.Elemento{ID: "e7", Nome: "Excluir", Tipo: "Button"})

	if d := m.DescricaoDaConfirmacao(map[string]any{"acao": "clicar_elemento", "elemento_id": "e7"}); d != `clicar em "Excluir" (Button)` {
		t.Errorf("clicar_elemento por id: %q", d)
	}
	if d := m.DescricaoDaConfirmacao(map[string]any{"acao": "clicar_elemento", "nome": "Apagar"}); d != `clicar em "Apagar"` {
		t.Errorf("clicar_elemento por nome: %q", d)
	}
	if d := m.DescricaoDaConfirmacao(map[string]any{"acao": "tecla", "texto": "shift+delete"}); d != "apertar a tecla shift+delete" {
		t.Errorf("tecla: %q", d)
	}
	if d := m.DescricaoDaConfirmacao(map[string]any{"acao": "click"}); d != "" {
		t.Errorf("click: %q", d)
	}
}
