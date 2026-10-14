package ferramentas

import (
	"context"
	"strings"
	"testing"

	"james/internal/dispositivo"
)

// Ferramentas da camada 1 da spec 2026-10-13 B2: janelas, mídia e abrir.

func executar(t *testing.T, f Ferramenta, args map[string]any) (dispositivo.AcaoTela, string, error) {
	t.Helper()
	ponte := &ponteFalsa{saida: "ok do device"}
	switch v := f.(type) {
	case *Janelas:
		v.Ponte = ponte
	case *Midia:
		v.Ponte = ponte
	case *Abrir:
		v.Ponte = ponte
	}
	saida, err := f.Executar(context.Background(), args)
	return ponte.acaoRecebida, saida, err
}

func TestJanelasMapeiaAsAcoes(t *testing.T) {
	casos := map[string]string{"listar": "janelas_listar", "focar": "janela_focar", "fechar": "janela_fechar"}
	for acao, esperada := range casos {
		enviada, saida, err := executar(t, NovoJanelas(nil), map[string]any{"acao": acao, "alvo": "firefox"})
		if err != nil {
			t.Fatalf("%s: %v", acao, err)
		}
		if enviada.Acao != esperada || saida != "ok do device" {
			t.Errorf("%s → %+v / %q", acao, enviada, saida)
		}
		if acao != "listar" && enviada.Alvo != "firefox" {
			t.Errorf("%s: alvo não chegou ao device: %+v", acao, enviada)
		}
	}
}

func TestJanelasFocarSemAlvoEhErroSemChamarOPonte(t *testing.T) {
	enviada, _, err := executar(t, NovoJanelas(nil), map[string]any{"acao": "focar"})
	if err == nil || !strings.Contains(err.Error(), "alvo") {
		t.Fatalf("err = %v", err)
	}
	if enviada.Acao != "" {
		t.Error("chamou o device mesmo sem alvo")
	}
}

func TestJanelasAcaoDesconhecida(t *testing.T) {
	if _, _, err := executar(t, NovoJanelas(nil), map[string]any{"acao": "minimizar"}); err == nil {
		t.Fatal("ação desconhecida não deu erro")
	}
}

func TestJanelasSoFecharPedeAprovacao(t *testing.T) {
	j := NovoJanelas(nil)
	if !j.ConfirmacaoNecessaria(map[string]any{"acao": "fechar", "alvo": "dolphin"}) {
		t.Error("fechar não pediu aprovação")
	}
	for _, acao := range []string{"listar", "focar"} {
		if j.ConfirmacaoNecessaria(map[string]any{"acao": acao}) {
			t.Errorf("%s pediu aprovação", acao)
		}
	}
}

func TestMidiaMandaASubacaoNoTextoEOPlayerNoAlvo(t *testing.T) {
	enviada, _, err := executar(t, NovoMidia(nil), map[string]any{"acao": "proxima", "player": "spotify"})
	if err != nil {
		t.Fatal(err)
	}
	if enviada != (dispositivo.AcaoTela{Acao: "midia", Texto: "proxima", Alvo: "spotify"}) {
		t.Errorf("enviada = %+v", enviada)
	}
	enviada, _, _ = executar(t, NovoMidia(nil), map[string]any{"acao": "estado"})
	if enviada != (dispositivo.AcaoTela{Acao: "midia", Texto: "estado"}) {
		t.Errorf("sem player: %+v", enviada)
	}
}

func TestMidiaAcaoDesconhecida(t *testing.T) {
	if _, _, err := executar(t, NovoMidia(nil), map[string]any{"acao": "rebobinar"}); err == nil {
		t.Fatal("ação desconhecida não deu erro")
	}
}

func TestAbrirComESemLinha(t *testing.T) {
	enviada, _, err := executar(t, NovoAbrir(nil), map[string]any{"alvo": "dolphin"})
	if err != nil || enviada != (dispositivo.AcaoTela{Acao: "abrir", Alvo: "dolphin"}) {
		t.Fatalf("sem linha: %+v %v", enviada, err)
	}
	enviada, _, err = executar(t, NovoAbrir(nil), map[string]any{"alvo": "/repo/main.go", "linha": 12.0})
	if err != nil || enviada != (dispositivo.AcaoTela{Acao: "abrir", Alvo: "/repo/main.go", Linha: 12}) {
		t.Fatalf("com linha: %+v %v", enviada, err)
	}
}

func TestAbrirValidaAlvoELinha(t *testing.T) {
	for _, args := range []map[string]any{
		{},
		{"alvo": "  "},
		{"alvo": "a.go", "linha": 0.0},
		{"alvo": "a.go", "linha": 3.5},
		{"alvo": "a.go", "linha": "12"},
	} {
		if enviada, _, err := executar(t, NovoAbrir(nil), args); err == nil || enviada.Acao != "" {
			t.Errorf("%v: err = %v, enviada = %+v", args, err, enviada)
		}
	}
}

func TestNiveisDaCamada1(t *testing.T) {
	if NovoJanelas(nil).Nivel() != NivelPerigoso || NovoAbrir(nil).Nivel() != NivelPerigoso {
		t.Error("janelas e abrir controlam a máquina: NivelPerigoso")
	}
	if NovoMidia(nil).Nivel() != NivelSeguro {
		t.Error("mídia só mexe no player: NivelSeguro")
	}
}

func TestDeclaracoesDaCamada1(t *testing.T) {
	for _, d := range []Declaracao{NovoJanelas(nil).Declaracao(), NovoMidia(nil).Declaracao(), NovoAbrir(nil).Declaracao()} {
		if d.Nome == "" || d.Descricao == "" || len(d.Obrigatorios) == 0 {
			t.Errorf("declaração incompleta: %+v", d)
		}
		for _, o := range d.Obrigatorios {
			if _, ok := d.Parametros[o]; !ok {
				t.Errorf("%s: obrigatório %q sem parâmetro", d.Nome, o)
			}
		}
	}
}

func TestJanelasDescreveOFechar(t *testing.T) {
	if d := NovoJanelas(nil).DescricaoDaConfirmacao(map[string]any{"acao": "fechar", "alvo": "Dolphin"}); d != `fechar a janela "Dolphin"` {
		t.Errorf("descrição = %q", d)
	}
}

// As duas ferramentas que pedem aprovação por argumento também descrevem.
var (
	_ DescreveConfirmacao = (*ManipularInterface)(nil)
	_ DescreveConfirmacao = (*Janelas)(nil)
)
