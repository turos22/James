package memoria

import (
	"strings"
	"testing"

	"james/internal/agente"
)

func TestMontarDentroDoOrcamentoDevolveTudo(t *testing.T) {
	j := Janela{OrcamentoTokens: 1000}
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "oi"},
		{Papel: agente.PapelModelo, Texto: "olá"},
	}

	resultado := j.Montar(mensagens)
	if len(resultado) != 2 {
		t.Fatalf("esperava as 2 mensagens dentro do orçamento, vieram %d", len(resultado))
	}
}

func TestMontarCortaMensagensAntigasQuandoEstouraOrcamento(t *testing.T) {
	texto := strings.Repeat("palavra ", 50) // ~400 caracteres, ~100 tokens estimados
	j := Janela{OrcamentoTokens: 120}        // cabe pouco mais de 1 mensagem desse tamanho
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: texto},
		{Papel: agente.PapelModelo, Texto: texto},
		{Papel: agente.PapelUsuario, Texto: "última, curta"},
	}

	resultado := j.Montar(mensagens)
	if len(resultado) >= len(mensagens) {
		t.Fatalf("esperava corte de mensagens antigas, vieram todas as %d", len(resultado))
	}
	if resultado[len(resultado)-1].Texto != "última, curta" {
		t.Error("a mensagem mais recente deveria sempre sobreviver ao corte")
	}
}

func TestMontarPreservaOrdemCronologica(t *testing.T) {
	j := Janela{OrcamentoTokens: 1000}
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "primeira"},
		{Papel: agente.PapelModelo, Texto: "segunda"},
		{Papel: agente.PapelUsuario, Texto: "terceira"},
	}

	resultado := j.Montar(mensagens)
	for i, esperado := range []string{"primeira", "segunda", "terceira"} {
		if resultado[i].Texto != esperado {
			t.Errorf("resultado[%d] = %q, queria %q — ordem cronológica quebrada", i, resultado[i].Texto, esperado)
		}
	}
}

func TestMontarSempreIncluiPeloMenosAUltimaMensagem(t *testing.T) {
	textoEnorme := strings.Repeat("x", 10000)
	j := Janela{OrcamentoTokens: 10} // orçamento menor que qualquer mensagem real
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "antiga"},
		{Papel: agente.PapelUsuario, Texto: textoEnorme},
	}

	resultado := j.Montar(mensagens)
	if len(resultado) != 1 {
		t.Fatalf("esperava 1 mensagem (nunca janela vazia), vieram %d", len(resultado))
	}
	if resultado[0].Texto != textoEnorme {
		t.Error("a última mensagem deveria sobreviver mesmo estourando o orçamento sozinha")
	}
}
