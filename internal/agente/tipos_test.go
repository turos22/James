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
