package voz

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Texto falado: o que sai do modelo vem em Markdown e em pedaços, e a voz
// precisa de frases limpas, uma por vez, assim que ficam prontas.

const (
	// fraseDeCodigo é tudo o que se fala de um bloco de código: ler código em
	// voz alta é inútil e longo, e o texto completo continua na tela.
	fraseDeCodigo = "veja o código na tela."

	cercaDeCodigo = "```"

	// maxRunasSemPontuacao: sem este corte, uma resposta sem ponto deixaria a
	// voz calada até o fim do texto.
	maxRunasSemPontuacao = 200
)

// abreviaturas que terminam em ponto sem acabar a frase.
var abreviaturas = map[string]bool{
	"dr": true, "dra": true, "sr": true, "sra": true, "srta": true,
	"prof": true, "profa": true, "av": true,
}

// QuebradorDeFrases acumula deltas de texto e devolve frases completas.
//
// Regra do ponto no FIM do buffer: espera o próximo caractere antes de
// quebrar. "Haiku 5." seguido de "5" é a versão 5.5, não duas frases, e os
// deltas do modelo podem cair exatamente ali. Quem precisa da última frase na
// hora (porque uma ferramenta vai rodar e a voz não pode ficar calada) chama
// Fim.
//
// Blocos de código (```) nunca viram texto: viram uma frase só, "veja o
// código na tela".
type QuebradorDeFrases struct {
	buf      string
	emCodigo bool
}

// Acrescentar recebe o próximo delta e devolve as frases que já fecharam.
func (q *QuebradorDeFrases) Acrescentar(delta string) []string {
	q.buf += delta
	return q.extrair(false)
}

// Fim devolve o que sobrou, sem esperar mais texto, e zera o quebrador.
func (q *QuebradorDeFrases) Fim() string {
	return strings.Join(q.extrair(true), " ")
}

func (q *QuebradorDeFrases) extrair(final bool) []string {
	var saida []string
	emitir := func(s string) {
		if s = strings.TrimSpace(s); s != "" {
			saida = append(saida, s)
		}
	}

	for {
		if q.emCodigo {
			i := strings.Index(q.buf, cercaDeCodigo)
			if i < 0 {
				if final {
					emitir(fraseDeCodigo)
					q.buf, q.emCodigo = "", false
				}
				return saida
			}
			q.buf = q.buf[i+len(cercaDeCodigo):]
			q.emCodigo = false
			emitir(fraseDeCodigo)
			continue
		}

		cerca := strings.Index(q.buf, cercaDeCodigo)
		antes := q.buf
		if cerca >= 0 {
			antes = q.buf[:cerca]
		}

		if frase, n, achou := proximaFrase(antes, final && cerca < 0); achou {
			emitir(frase)
			q.buf = q.buf[n:]
			continue
		}
		if cerca >= 0 {
			// O texto antes da cerca é uma frase completa, mesmo sem ponto
			// ("Veja:").
			emitir(antes)
			q.buf = q.buf[cerca+len(cercaDeCodigo):]
			q.emCodigo = true
			continue
		}
		if final {
			emitir(q.buf)
			q.buf = ""
			return saida
		}
		if utf8.RuneCountInString(q.buf) > maxRunasSemPontuacao {
			frase, n := cortarNoEspaco(q.buf, maxRunasSemPontuacao)
			emitir(frase)
			q.buf = q.buf[n:]
			continue
		}
		return saida
	}
}

// proximaFrase procura o fim da primeira frase de s: um ponto, ! ou ?
// seguido de espaço, ou uma quebra de linha. Devolve a frase e quantos bytes
// consumir. aceitaFimDoBuffer deixa um ponto no último caractere valer.
func proximaFrase(s string, aceitaFimDoBuffer bool) (frase string, consumido int, achou bool) {
	for i := 0; i < len(s); {
		r, w := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '\n':
			return s[:i], i + w, true

		case '.', '!', '?':
			j := i + w
			for j < len(s) && strings.IndexByte(".!?", s[j]) >= 0 {
				j++
			}
			fimDaPontuacao := j
			k := j
			for k < len(s) {
				r2, w2 := utf8.DecodeRuneInString(s[k:])
				if !strings.ContainsRune("\"')»”’", r2) {
					break
				}
				k += w2
			}
			if k >= len(s) {
				if aceitaFimDoBuffer {
					return s, len(s), true
				}
				return "", 0, false // espera o próximo caractere
			}
			r3, w3 := utf8.DecodeRuneInString(s[k:])
			if !unicode.IsSpace(r3) {
				i = k // "3.14": o ponto não fecha a frase
				continue
			}
			if r == '.' && fimDaPontuacao == i+w && terminaEmAbreviatura(s[:i]) {
				i = k
				continue
			}
			// Frase nova começa em maiúscula, dígito, aspas ou travessão. Uma
			// minúscula depois do espaço ("vamos... e saiu") é a mesma frase.
			m := k + w3
			for m < len(s) {
				r4, w4 := utf8.DecodeRuneInString(s[m:])
				if !unicode.IsSpace(r4) {
					break
				}
				m += w4
			}
			if m >= len(s) {
				if aceitaFimDoBuffer {
					return s[:k], len(s), true
				}
				return "", 0, false // ainda não sei o que vem depois: espera
			}
			if r4, _ := utf8.DecodeRuneInString(s[m:]); unicode.IsLower(r4) {
				i = k // segue varrendo, pode haver uma quebra de linha adiante
				continue
			}
			return s[:k], k + w3, true
		}
		i += w
	}
	return "", 0, false
}

func terminaEmAbreviatura(prefixo string) bool {
	palavra := prefixo
	if i := strings.LastIndexFunc(prefixo, unicode.IsSpace); i >= 0 {
		palavra = prefixo[i+1:]
	}
	return abreviaturas[strings.ToLower(palavra)]
}

// cortarNoEspaco devolve os primeiros max runas de s, recuando até o último
// espaço pra não partir palavra. Consumido inclui o espaço do corte.
func cortarNoEspaco(s string, max int) (string, int) {
	fim := 0
	for n := 0; n < max && fim < len(s); n++ {
		_, w := utf8.DecodeRuneInString(s[fim:])
		fim += w
	}
	if i := strings.LastIndexByte(s[:fim], ' '); i > 0 {
		return s[:i], i + 1
	}
	return s[:fim], fim
}

var (
	reCodigoFechado   = regexp.MustCompile("(?s)```.*?```")
	reCodigoAberto    = regexp.MustCompile("(?s)```.*$")
	reCodigoEmLinha   = regexp.MustCompile("`([^`]*)`")
	reImagem          = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	reLink            = regexp.MustCompile(`\[([^\]]+)\]\([^)]*\)`)
	reURL             = regexp.MustCompile(`https?://[^\s)]+`)
	reTitulo          = regexp.MustCompile(`^\s{0,3}#{1,6}\s+`)
	reMarcadorDeLista = regexp.MustCompile(`^\s*(?:[-*•]|\d+[.)])\s+`)
	reNegritoSublinha = regexp.MustCompile(`__(.+?)__`)
	reItalicoSublinha = regexp.MustCompile(`(^|[\s(])_([^_]+?)_([\s).,;:!?]|$)`)
	reSublinhaNoMeio  = regexp.MustCompile(`([\p{L}\p{N}])_([\p{L}\p{N}])`)
	reEspacos         = regexp.MustCompile(`\s+`)
	reTemConteudo     = regexp.MustCompile(`[\p{L}\p{N}]`)
)

// LimparParaFala tira o Markdown do texto pra ele ser falado. O texto na tela
// não passa por aqui e continua com a formatação.
func LimparParaFala(texto string) string {
	texto = reCodigoFechado.ReplaceAllString(texto, fraseDeCodigo)
	texto = reCodigoAberto.ReplaceAllString(texto, fraseDeCodigo)
	texto = reCodigoEmLinha.ReplaceAllString(texto, "$1")
	texto = reImagem.ReplaceAllString(texto, "$1")
	texto = reLink.ReplaceAllString(texto, "$1")
	texto = reURL.ReplaceAllString(texto, "o link está na tela")

	var linhas []string
	for _, linha := range strings.Split(texto, "\n") {
		linha = reTitulo.ReplaceAllString(linha, "")
		item := reMarcadorDeLista.MatchString(linha)
		linha = strings.TrimSpace(reMarcadorDeLista.ReplaceAllString(linha, ""))
		if linha == "" {
			continue
		}
		// Item de lista sem pontuação ganha ponto: é o que separa um do outro
		// na fala.
		if item && !strings.ContainsAny(linha[len(linha)-1:], ".!?:;") {
			linha += "."
		}
		linhas = append(linhas, linha)
	}
	texto = strings.Join(linhas, " ")

	texto = reNegritoSublinha.ReplaceAllString(texto, "$1")
	texto = strings.ReplaceAll(texto, "*", "")
	texto = reItalicoSublinha.ReplaceAllString(texto, "$1$2$3")
	// Duas passadas: "a_b_c" tem a letra do meio compartilhada entre dois
	// casamentos, e o regexp não sobrepõe.
	for i := 0; i < 2; i++ {
		texto = reSublinhaNoMeio.ReplaceAllString(texto, "$1 $2")
	}

	texto = strings.TrimSpace(reEspacos.ReplaceAllString(texto, " "))
	if !reTemConteudo.MatchString(texto) {
		return ""
	}
	return texto
}
