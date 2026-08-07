package ferramentas

import (
	"context"
	"testing"
)

func TestVerTelaDevolveSaidaEImagem(t *testing.T) {
	ponte := &ponteFalsa{
		saida:  "superficie: DISPLAY1 (primaria) 1920x1080 escala 100%",
		imagem: []byte{1, 2, 3},
	}
	v := NovoVerTela(ponte)

	saida, imagem, err := v.ExecutarComImagem(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("erro: %v", err)
	}
	if saida == "" {
		t.Error("saida veio vazia")
	}
	if len(imagem) != 3 {
		t.Errorf("imagem tem %d bytes, queria 3", len(imagem))
	}
	if ponte.acaoRecebida.Acao != "ver_tela" {
		t.Errorf("acao = %q, queria ver_tela", ponte.acaoRecebida.Acao)
	}
}

func TestVerTelaRepassaSuperficiePedida(t *testing.T) {
	ponte := &ponteFalsa{}
	v := NovoVerTela(ponte)

	if _, _, err := v.ExecutarComImagem(context.Background(), map[string]any{"superficie": "DISPLAY7"}); err != nil {
		t.Fatalf("erro: %v", err)
	}
	if ponte.acaoRecebida.Superficie != "DISPLAY7" {
		t.Errorf("superficie = %q, queria DISPLAY7", ponte.acaoRecebida.Superficie)
	}
}

func TestVerTelaEhSegura(t *testing.T) {
	// Decisão D5 da spec: NivelSeguro, sem aprovação, qualquer perfil. A
	// consequência (o celular enxerga a tela do desktop) foi aceita
	// explicitamente pelo Arthur e está registrada em R7.
	if NovoVerTela(&ponteFalsa{}).Nivel() != NivelSeguro {
		t.Error("ver_tela deveria ser NivelSeguro")
	}
}

func TestVerTelaImplementaAsDuasInterfaces(t *testing.T) {
	var _ Ferramenta = NovoVerTela(&ponteFalsa{})
	var _ FerramentaComImagem = NovoVerTela(&ponteFalsa{})
}
