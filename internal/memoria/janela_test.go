package memoria

import (
	"bytes"
	"image"
	"image/png"
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

// pngDeTeste gera um PNG real do tamanho pedido. Bytes zerados não servem
// mais: depois que o custo passou a sair da dimensão da imagem, o header
// virou o que importa.
func pngDeTeste(t *testing.T, largura, altura int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, largura, altura))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("erro ao gerar PNG de teste: %v", err)
	}
	return buf.Bytes()
}

func TestEstimarTokensImagemEhFixoConformeMedidoNaAPIReal(t *testing.T) {
	// Valores medidos ao vivo (Task 15, CA3) contra o gemini-2.5-flash real,
	// via GenerateContent + UsageMetadata.PromptTokensDetails: 258 tokens
	// pra qualquer dimensão testada, de 300x200 a 5120x2880. A tabela de
	// tiling da doc oficial não se confirmou pra este modelo nesta chamada.
	casos := []struct {
		nome     string
		largura  int
		altura   int
		esperado int
	}{
		{"pequena", 300, 200, 258},
		{"exemplo da doc oficial, que não se confirmou: 960x540", 960, 540, 258},
		{"1920x1080", 1920, 1080, 258},
		{"4K: 3840x2160", 3840, 2160, 258},
		{"dimensão inválida", 0, 0, 258},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := EstimarTokensImagem(c.largura, c.altura); got != c.esperado {
				t.Errorf("EstimarTokensImagem(%d, %d) = %d, queria %d", c.largura, c.altura, got, c.esperado)
			}
		})
	}
}

func TestCustoDeImagemNaoUsaMaisTamanhoEmBytes(t *testing.T) {
	// Um PNG 960x540 de área uniforme comprime pra ~3,2KB (len/4 = 797). A
	// régua antiga por bytes cobraria isso; o custo real medido contra a API
	// (CA3, Task 15) é 258 tokens fixos — bem menos, porque o que importa
	// pro Gemini não é o tamanho do arquivo, e neste modelo nem parece ser a
	// dimensão da imagem.
	imagem := pngDeTeste(t, 960, 540)
	if len(imagem)/4 <= custoFixoImagem {
		t.Fatalf("o PNG de teste ficou pequeno demais (%d bytes) para provar a diferença entre as duas réguas", len(imagem))
	}

	custo := custoMensagem(agente.Mensagem{Papel: agente.PapelUsuario, Imagem: imagem})
	if custo != custoFixoImagem {
		t.Errorf("custo = %d, queria %d (texto vazio, só a imagem conta; a régua antiga por bytes daria ~%d)", custo, custoFixoImagem, len(imagem)/4)
	}
}

func TestCustoDeImagemQuebradaCaiNoFallbackConservador(t *testing.T) {
	// Header ilegível: cobrar caro continua sendo o lado seguro pra um
	// orçamento, que é o argumento original deste arquivo.
	lixo := make([]byte, 10000)
	custo := custoMensagem(agente.Mensagem{Papel: agente.PapelUsuario, Imagem: lixo})
	if custo != 2500 {
		t.Errorf("custo = %d, queria 2500 (fallback len/4)", custo)
	}
}

func TestMontarContaImagemComoCusto(t *testing.T) {
	imagemGrande := pngDeTeste(t, 800, 600)
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "pergunta antiga"},
		{Papel: agente.PapelUsuario, Texto: "com screenshot", Imagem: imagemGrande},
		{Papel: agente.PapelUsuario, Texto: "última, curta"},
	}

	t.Run("orcamento apertado corta a mensagem com imagem", func(t *testing.T) {
		// A imagem sozinha (custoFixoImagem tokens) já estoura o orçamento
		// de 120, que é o ponto: ela tem que ser cortada junto com o resto
		// das antigas.
		j := Janela{OrcamentoTokens: 120}
		resultado := j.Montar(mensagens)
		for _, m := range resultado {
			if len(m.Imagem) > 0 {
				t.Errorf("mensagem com screenshot de %d bytes deveria ter sido cortada (custo zero pra imagem reabriria o bug #4), mas sobreviveu: %+v", len(m.Imagem), m)
			}
		}
	})

	t.Run("orcamento generoso mantem a mensagem com imagem", func(t *testing.T) {
		// Prova que é o CUSTO da imagem que decide, não um bug que corta
		// tudo (ou que devolve slice vazia): com orçamento acima do custo
		// real (1032 + texto), a mensagem com imagem sobrevive.
		j := Janela{OrcamentoTokens: 2000}
		resultado := j.Montar(mensagens)
		var achouImagem bool
		for _, m := range resultado {
			if len(m.Imagem) > 0 {
				achouImagem = true
			}
		}
		if !achouImagem {
			t.Error("com orçamento generoso, a mensagem com imagem deveria ter sobrevivido, mas nenhuma mensagem no resultado tem Imagem")
		}
	})
}

func TestMontarContaImagemDeResultadoDeFerramentaComoCusto(t *testing.T) {
	imagemGrande := pngDeTeste(t, 800, 600) // mesmo helper da Task 3, já existe no arquivo de teste

	t.Run("orcamento apertado corta a mensagem com imagem no resultado", func(t *testing.T) {
		j := Janela{OrcamentoTokens: 120}
		mensagens := []agente.Mensagem{
			{Papel: agente.PapelUsuario, Texto: "pergunta antiga"},
			{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
				{Nome: "ver_tela", Saida: "e1 Button", Imagem: imagemGrande},
			}},
			{Papel: agente.PapelUsuario, Texto: "última, curta"},
		}
		resultado := j.Montar(mensagens)
		for _, m := range resultado {
			for _, r := range m.Resultados {
				if len(r.Imagem) > 0 {
					t.Errorf("resultado de ferramenta com screenshot de %d bytes deveria ter sido cortado, mas sobreviveu: %+v", len(r.Imagem), r)
				}
			}
		}
	})
}
