package memoria

import (
	"strings"
)

// acentos transliteram o português pra ASCII antes do slug. Sem isso,
// "preferências" viraria "prefer-ncias" — funcional, mas ilegível no
// Obsidian, que é justamente o motivo de a memória ser um arquivo por
// assunto.
var acentos = strings.NewReplacer(
	"á", "a", "à", "a", "ã", "a", "â", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "õ", "o", "ô", "o", "ö", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ç", "c", "ñ", "n",
)

// SlugAssunto reduz o assunto vindo do modelo a um nome de arquivo seguro:
// minúsculas ASCII, dígitos e hífen, nada mais. É a primeira das duas
// barreiras contra travessia de caminho. A segunda é ResolverCaminho, que
// deliberadamente NÃO confia no resultado desta — ver o comentário lá.
func SlugAssunto(assunto string) string {
	texto := acentos.Replace(strings.ToLower(strings.TrimSpace(assunto)))

	var b strings.Builder
	pendenteHifen := false
	for _, r := range texto {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			if pendenteHifen && b.Len() > 0 {
				b.WriteByte('-')
			}
			pendenteHifen = false
			b.WriteRune(r)
			continue
		}
		pendenteHifen = true
	}
	return b.String()
}
