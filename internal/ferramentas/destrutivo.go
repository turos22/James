package ferramentas

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Vocabulário de ação destrutiva na tela (D10 da spec 2026-10-13 B2).
//
// A regra do Arthur é "livre, exceto destrutivo": clicar em Excluir, Apagar,
// Remover, Mover para a lixeira e afins pede aprovação. Casa por PALAVRA
// inteira, sem acento e sem caixa: "Removedor de manchas" não é "Remover".
var palavrasDestrutivas = map[string]bool{
	"excluir": true, "apagar": true, "remover": true, "deletar": true, "esvaziar": true, "lixeira": true,
	"descartar": true, "desinstalar": true, "formatar": true,
	"delete": true, "remove": true, "erase": true, "trash": true, "discard": true, "uninstall": true,
}

// Frases cujas palavras sozinhas são inocentes: "Não salvar" descarta o que
// o Arthur escreveu.
var frasesDestrutivas = [][]string{{"nao", "salvar"}, {"dont", "save"}}

var semAcento = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

func palavras(texto string) []string {
	limpo, _, err := transform.String(semAcento, texto)
	if err != nil {
		limpo = texto
	}
	limpo = strings.NewReplacer("'", "", "’", "").Replace(strings.ToLower(limpo))
	return strings.FieldsFunc(limpo, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// Destrutivo diz se o nome de um elemento de tela indica ação que apaga ou
// descarta algo.
func Destrutivo(nome string) bool {
	ditas := palavras(nome)
	for _, p := range ditas {
		if palavrasDestrutivas[p] {
			return true
		}
	}
	for _, frase := range frasesDestrutivas {
		for i := 0; i+len(frase) <= len(ditas); i++ {
			igual := true
			for j, p := range frase {
				if ditas[i+j] != p {
					igual = false
					break
				}
			}
			if igual {
				return true
			}
		}
	}
	return false
}

// teclaApaga diz se a tecla (ou combinação, "shift+delete") inclui Delete:
// no Dolphin ela manda o arquivo para a lixeira, e com Shift apaga de vez.
func teclaApaga(tecla string) bool {
	for _, parte := range strings.Split(tecla, "+") {
		switch strings.ToLower(strings.TrimSpace(parte)) {
		case "delete", "del":
			return true
		}
	}
	return false
}
