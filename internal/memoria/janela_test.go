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

func TestEstimarTokensImagemSegueTabelaDoGemini(t *testing.T) {
	casos := []struct {
		nome     string
		largura  int
		altura   int
		esperado int
	}{
		{"ambos os lados até 384 custam um tile", 300, 200, 258},
		{"exatamente 384 ainda é um tile", 384, 384, 258},
		{"exemplo da doc oficial: 960x540 dá 6 tiles", 960, 540, 1548},
		{"1920x1080 tem a mesma proporção, mesmos 6 tiles", 1920, 1080, 1548},
		{"dimensão inválida cai no custo de um tile", 0, 0, 258},
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
	// Um PNG 960x540 de área uniforme comprime pra ~3,2KB (len/4 = 797,
	// passa da guarda abaixo). Em 1920x1080 — mesma proporção, 4x a área —
	// comprime pra ~10,5KB (len/4 = 2623, estouraria a guarda): o PNG cresce
	// quase linearmente com a área depois que o compressor satura, por isso
	// a resolução menor é a que prova o ponto aqui. Pela régua antiga
	// (len/4) isso custaria quase nada; pela tabela do Gemini custa 1548
	// tokens (mesmo exemplo da doc oficial), porque o que pesa é a área, não
	// o arquivo.
	imagem := pngDeTeste(t, 960, 540)
	if len(imagem)/4 >= 1548 {
		t.Fatalf("o PNG de teste ficou grande demais (%d bytes) para provar a diferença entre as duas réguas", len(imagem))
	}

	custo := custoMensagem(agente.Mensagem{Papel: agente.PapelUsuario, Imagem: imagem})
	if custo != 1548 {
		t.Errorf("custo = %d, queria 1548 (texto vazio, só a imagem conta; a régua antiga por bytes daria ~%d)", custo, len(imagem)/4)
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
	// 800x600: menor lado 600, crop 400, 2x2 = 4 tiles = 1032 tokens.
	imagemGrande := pngDeTeste(t, 800, 600)
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "pergunta antiga"},
		{Papel: agente.PapelUsuario, Texto: "com screenshot", Imagem: imagemGrande},
		{Papel: agente.PapelUsuario, Texto: "última, curta"},
	}

	t.Run("orcamento apertado corta a mensagem com imagem", func(t *testing.T) {
		// A imagem sozinha (1032 tokens) já estoura o orçamento de 120, que
		// é o ponto: ela tem que ser cortada junto com o resto das antigas.
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
