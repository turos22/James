package voz

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// todas empurra os deltas e junta tudo o que saiu, inclusive o resto do Fim.
func todas(deltas ...string) []string {
	var q QuebradorDeFrases
	var saida []string
	for _, d := range deltas {
		saida = append(saida, q.Acrescentar(d)...)
	}
	if resto := q.Fim(); resto != "" {
		saida = append(saida, resto)
	}
	return saida
}

func TestQuebradorLiberaAFraseQuandoChegaOQueVemDepoisDoPonto(t *testing.T) {
	var q QuebradorDeFrases

	if got := q.Acrescentar("A capital"); len(got) != 0 {
		t.Fatalf("frase incompleta saiu: %v", got)
	}
	got := q.Acrescentar(" é Canberra. Ela")
	if !reflect.DeepEqual(got, []string{"A capital é Canberra."}) {
		t.Fatalf("primeira frase = %v", got)
	}
	if got := q.Acrescentar(" fica"); len(got) != 0 {
		t.Fatalf("resto saiu antes do Fim: %v", got)
	}
	if resto := q.Fim(); resto != "Ela fica" {
		t.Errorf("Fim = %q, queria %q", resto, "Ela fica")
	}
}

// O ponto no FIM do buffer espera o próximo caractere: "Haiku 5." + "5" é a
// versão 5.5, não duas frases. Quem precisa da última frase na hora (porque
// uma ferramenta vai rodar) chama Fim.
func TestPontoNoFimDoBufferEsperaOProximoCaractere(t *testing.T) {
	var q QuebradorDeFrases

	if got := q.Acrescentar("Use o Haiku 5."); len(got) != 0 {
		t.Fatalf("soltou cedo demais: %v", got)
	}
	if got := q.Acrescentar("5 agora."); len(got) != 0 {
		t.Fatalf("quebrou no meio de 5.5: %v", got)
	}
	if resto := q.Fim(); resto != "Use o Haiku 5.5 agora." {
		t.Errorf("Fim = %q", resto)
	}
}

func TestQuebradorSeparaSimExclamacaoEInterrogacao(t *testing.T) {
	got := todas("Sim! Claro? Ok.")
	if want := []string{"Sim!", "Claro?", "Ok."}; !reflect.DeepEqual(got, want) {
		t.Errorf("frases = %v, queria %v", got, want)
	}
}

func TestQuebradorNaoQuebraNumeroDecimalNemAbreviatura(t *testing.T) {
	casos := []struct{ nome, texto string }{
		{"decimal", "3.14 é pi."},
		{"doutor", "Dr. Silva chegou."},
		{"professora", "A Profa. Ana explicou."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := todas(c.texto); !reflect.DeepEqual(got, []string{c.texto}) {
				t.Errorf("frases = %v, queria uma só: %q", got, c.texto)
			}
		})
	}
}

func TestQuebradorReticenciasEAspasFechandoFicamNaFrase(t *testing.T) {
	got := todas(`Ele disse "vamos..." e saiu. Depois voltou.`)
	if want := []string{`Ele disse "vamos..." e saiu.`, "Depois voltou."}; !reflect.DeepEqual(got, want) {
		t.Errorf("frases = %v, queria %v", got, want)
	}
}

func TestQuebradorQuebraNaLinhaNova(t *testing.T) {
	got := todas("linha um\nlinha dois\n\n\nlinha três")
	if want := []string{"linha um", "linha dois", "linha três"}; !reflect.DeepEqual(got, want) {
		t.Errorf("frases = %v, queria %v", got, want)
	}
}

// Sem este corte, uma resposta sem pontuação deixaria a voz calada até o fim.
func TestQuebradorCortaNoUltimoEspacoQuandoNaoHaPontuacao(t *testing.T) {
	texto := strings.TrimSpace(strings.Repeat("palavra ", 40)) // 319 runas, sem pontuação

	got := todas(texto)

	if len(got) < 2 {
		t.Fatalf("%d pedaço(s); queria cortado: %v", len(got), got)
	}
	for _, p := range got {
		if n := utf8.RuneCountInString(p); n > 200 {
			t.Errorf("pedaço com %d runas, passou do limite de 200", n)
		}
		if strings.HasPrefix(p, "alavra") || strings.HasSuffix(p, "palavr") {
			t.Errorf("cortou no meio de uma palavra: %q", p)
		}
	}
	if juntos := strings.Join(got, " "); juntos != texto {
		t.Errorf("perdeu ou repetiu texto ao cortar:\n got  %q\n want %q", juntos, texto)
	}
}

func TestQuebradorTrocaBlocoDeCodigoPorUmaFraseSo(t *testing.T) {
	got := todas("Veja:\n```go\nfunc main() {\n", "  println(\"oi. tchau.\")\n}\n```\nPronto.")
	want := []string{"Veja:", "veja o código na tela.", "Pronto."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("frases = %v, queria %v (o código não pode ser lido em voz alta)", got, want)
	}
}

func TestQuebradorReconheceACercaDeCodigoPartidaEntreDeltas(t *testing.T) {
	got := todas("Veja ``", "`\ncódigo()\n``", "`\nfim.")
	want := []string{"Veja", "veja o código na tela.", "fim."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("frases = %v, queria %v", got, want)
	}
}

func TestQuebradorCodigoSemFecharAindaViraAvisoNoFim(t *testing.T) {
	got := todas("Aqui:\n```\nrm -rf /")
	want := []string{"Aqui:", "veja o código na tela."}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("frases = %v, queria %v", got, want)
	}
}

func TestQuebradorFimSemNadaDevolveVazio(t *testing.T) {
	var q QuebradorDeFrases
	if got := q.Fim(); got != "" {
		t.Errorf("Fim = %q, queria vazio", got)
	}
}

func TestLimparParaFala(t *testing.T) {
	casos := []struct{ nome, entrada, esperado string }{
		{"negrito", "A capital é **Canberra**.", "A capital é Canberra."},
		{"itálico com asterisco", "Isso é *importante* mesmo.", "Isso é importante mesmo."},
		{"itálico com sublinhado", "Isso é _importante_ mesmo.", "Isso é importante mesmo."},
		{"lista", "- item um\n- item dois", "item um. item dois."},
		{"lista numerada", "1. primeiro\n2. segundo", "primeiro. segundo."},
		{"lista que já pontua", "- um.\n- dois!", "um. dois!"},
		{"código em linha", "use `ls -la`", "use ls -la"},
		{"bloco de código", "```go\nfmt.Println(1)\n```", "veja o código na tela."},
		{"bloco de código sem fechar", "```\nrm -rf /", "veja o código na tela."},
		{"link", "[docs](https://x.invalid)", "docs"},
		{"imagem", "![gráfico](https://x.invalid/a.png) acima", "gráfico acima"},
		{"url solta", "https://x.invalid/a", "o link está na tela"},
		{"url no meio", "veja https://x.invalid/a agora", "veja o link está na tela agora"},
		{"título", "# Título", "Título"},
		{"subtítulo", "### Resumo da conversa", "Resumo da conversa"},
		{"identificador com sublinhado", "chame executar_comando_sistema agora", "chame executar comando sistema agora"},
		{"espaços e quebras", "  um   dois\n\n  três  ", "um dois três"},
		{"só pontuação decorativa", "---", ""},
		{"vazio", "", ""},
		{"texto limpo passa igual", "Tudo certo por aqui.", "Tudo certo por aqui."},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := LimparParaFala(c.entrada); got != c.esperado {
				t.Errorf("LimparParaFala(%q) = %q, queria %q", c.entrada, got, c.esperado)
			}
		})
	}
}
