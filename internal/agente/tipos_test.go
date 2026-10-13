package agente

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResultadoFerramentaImagemNaoSerializaParaJSON(t *testing.T) {
	r := ResultadoFerramenta{Nome: "ver_tela", Saida: "e1", Imagem: []byte{0x89, 'P', 'N', 'G'}}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("erro ao serializar: %v", err)
	}
	if strings.Contains(string(b), "Imagem") || strings.Contains(string(b), "iVBOR") {
		t.Errorf("Imagem vazou para o JSON persistido: %s", b)
	}
}

// O valor zero de Modo tem que ser o conservador: quem esquece de preencher
// ganha pensamento (mais lento, mais correto), não a resposta apressada da voz.
func TestModoZeroEhTexto(t *testing.T) {
	var req Requisicao
	if req.Modo != ModoTexto {
		t.Errorf("Modo zero = %v, queria ModoTexto", req.Modo)
	}
	if ModoVoz == ModoTexto {
		t.Error("ModoVoz e ModoTexto não podem ser o mesmo valor")
	}
}

func TestUsoSomarAcumulaTudo(t *testing.T) {
	a := Uso{Entrada: 100, Saida: 10, CacheLeitura: 50, Pensamento: 5}
	b := Uso{Entrada: 200, Saida: 20, CacheLeitura: 0, Pensamento: 7}

	got := a.Somar(b)

	want := Uso{Entrada: 300, Saida: 30, CacheLeitura: 50, Pensamento: 12}
	if got != want {
		t.Errorf("Somar = %+v, queria %+v", got, want)
	}
	if a.Entrada != 100 {
		t.Error("Somar não pode alterar o receptor")
	}
}

func TestUsoZeroSomadoNaoMuda(t *testing.T) {
	a := Uso{Entrada: 1, Saida: 2}
	if got := a.Somar(Uso{}); got != a {
		t.Errorf("Somar(zero) = %+v, queria %+v", got, a)
	}
}
