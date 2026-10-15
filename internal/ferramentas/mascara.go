package ferramentas

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"james/internal/higiene"
)

// Máscara de valores secretos (spec 2026-10-14 arquivos, D6).
//
// O resultado de uma ferramenta sai do jamesd por quatro caminhos: vai à
// Anthropic (o modelo lê), ao Postgres, à janela e ao resumo da conversa, e
// ao app. Mascarar tem de acontecer ANTES de a ferramenta devolver o texto;
// depois seria tarde. O nome da chave, o comentário e a estrutura do arquivo
// continuam visíveis: o James ainda responde "o .env tem a chave do Claude
// preenchida?" sem o valor sair da máquina.

const marcaOculta = "••••"

var (
	// BEGIN até END; sem END (leitura cortada no meio do bloco), até o fim.
	rePEM = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?(?:-----END [A-Z ]*PRIVATE KEY-----|\z)`)

	// CHAVE=valor, com export e espaços. A chave aceita ':' e '/' (.npmrc:
	// //registry.npmjs.org/:_authToken=...).
	reAtribuicao = regexp.MustCompile(`^(\s*(?:export\s+)?[^\s=#][^\s=]*\s*=\s*)(.*)$`)

	// # CHAVE=valor: chave antiga deixada comentada continua sendo chave.
	reAtribuicaoComentada = regexp.MustCompile(`^(\s*#+\s*(?:export\s+)?[A-Z][A-Z0-9_]*\s*=\s*)(\S.*)$`)

	// chave: valor (YAML). A chave não tem espaço nem ':' e o ':' é seguido de
	// espaço, o que deixa https://host de fora.
	reYAML = regexp.MustCompile(`^(\s*[^\s=:#"'{\[][^\s=:]*\s*:\s+)(.+)$`)

	// "chave": "valor" (JSON). Só valor string é escondido.
	reJSON = regexp.MustCompile(`("(?:[^"\\]|\\.)+"\s*:\s*)"((?:[^"\\]|\\.)*)"`)

	// .netrc: machine x login y password z
	reNetrc = regexp.MustCompile(`(?i)(\bpassword\s+)(\S+)`)
)

// fechamento acha a primeira aspa `aspa` que NÃO está escapada (precedida de um
// número ímpar de barras invertidas). -1 se o valor não fecha.
func fechamento(s string, aspa byte) int {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case aspa:
			return i
		}
	}
	return -1
}

func substituto(valor string) string {
	return fmt.Sprintf("%s (oculto, %d caracteres)", marcaOculta, utf8.RuneCountInString(valor))
}

// esconderValor troca o valor (com ou sem aspas, com ou sem comentário depois)
// pelo marcador. Devolve o texto novo e o valor original; ok é falso quando
// não há o que esconder (vazio, ou já mascarado numa passada anterior).
func esconderValor(resto string, minimo int) (novo, original string, ok bool) {
	if resto == "" {
		return resto, "", false
	}
	if aspa := resto[0]; aspa == '"' || aspa == '\'' {
		if fim := fechamento(resto[1:], aspa); fim >= 0 {
			interno := resto[1 : 1+fim]
			if utf8.RuneCountInString(interno) < minimo || strings.HasPrefix(interno, marcaOculta) {
				return resto, "", false
			}
			return string(aspa) + substituto(interno) + string(aspa) + resto[2+fim:], interno, true
		}
	}
	valor, cauda := resto, ""
	if i := strings.Index(resto, " #"); i >= 0 {
		valor, cauda = resto[:i], resto[i:]
	}
	sobra := valor[len(strings.TrimRight(valor, " \t")):]
	valor = strings.TrimRight(valor, " \t")
	if utf8.RuneCountInString(valor) < minimo || strings.HasPrefix(valor, marcaOculta) {
		return resto, "", false
	}
	return substituto(valor) + sobra + cauda, valor, true
}

func mascararLinhaSensivel(linha string) (nova string, ocultos []string) {
	terminacao := ""
	if strings.HasSuffix(linha, "\r") {
		linha, terminacao = strings.TrimSuffix(linha, "\r"), "\r"
	}
	trim := strings.TrimSpace(linha)

	if strings.HasPrefix(trim, "#") {
		if m := reAtribuicaoComentada.FindStringSubmatch(linha); m != nil {
			if novo, original, ok := esconderValor(m[2], 6); ok {
				return m[1] + novo + terminacao, []string{original}
			}
		}
		return linha + terminacao, nil
	}
	// JSON ANTES da atribuição: a chave da atribuição aceita qualquer coisa até
	// o primeiro "=", e num JSON minificado com "=" no valor (base64 com padding)
	// ela tomaria o começo do segredo como nome da chave e o deixaria à mostra.
	if strings.Contains(linha, `":`) && reJSON.MatchString(linha) {
		nova = reJSON.ReplaceAllStringFunc(linha, func(trecho string) string {
			m := reJSON.FindStringSubmatch(trecho)
			if m[2] == "" || strings.HasPrefix(m[2], marcaOculta) {
				return trecho
			}
			ocultos = append(ocultos, m[2])
			return m[1] + `"` + substituto(m[2]) + `"`
		})
		return nova + terminacao, ocultos
	}
	if m := reAtribuicao.FindStringSubmatch(linha); m != nil {
		if novo, original, ok := esconderValor(m[2], 1); ok {
			return m[1] + novo + terminacao, []string{original}
		}
		return linha + terminacao, nil
	}
	if m := reYAML.FindStringSubmatch(linha); m != nil {
		if novo, original, ok := esconderValor(strings.TrimSpace(m[2]), 1); ok {
			return m[1] + novo + terminacao, []string{original}
		}
		return linha + terminacao, nil
	}
	if reNetrc.MatchString(linha) {
		nova = reNetrc.ReplaceAllStringFunc(linha, func(trecho string) string {
			m := reNetrc.FindStringSubmatch(trecho)
			if strings.HasPrefix(m[2], marcaOculta) {
				return trecho
			}
			ocultos = append(ocultos, m[2])
			return m[1] + substituto(m[2])
		})
		return nova + terminacao, ocultos
	}
	return linha + terminacao, nil
}

// aspaAbertaNaLinha diz se a linha começa uma atribuição cujo valor abre aspas
// e NÃO as fecha na mesma linha (valor em várias linhas). Devolve a aspa, ou 0.
func aspaAbertaNaLinha(linha string) byte {
	linha = strings.TrimSuffix(linha, "\r")
	if strings.HasPrefix(strings.TrimSpace(linha), "#") {
		return 0
	}
	m := reAtribuicao.FindStringSubmatch(linha)
	if m == nil || m[2] == "" {
		return 0
	}
	if aspa := m[2][0]; (aspa == '"' || aspa == '\'') && fechamento(m[2][1:], aspa) < 0 {
		return aspa
	}
	return 0
}

// mascararContinuacao esconde uma linha que está DENTRO de um valor entre
// aspas de várias linhas. Se a aspa fecha nela, só o que vem antes da aspa é
// valor (o resto, como um comentário, fica). Devolve também se o valor fechou.
func mascararContinuacao(linha string, aspa byte) (nova string, fechou bool, oculto string) {
	terminacao := ""
	if strings.HasSuffix(linha, "\r") {
		linha, terminacao = strings.TrimSuffix(linha, "\r"), "\r"
	}
	fim := fechamento(linha, aspa)
	if fim < 0 {
		if strings.TrimSpace(linha) == "" {
			return linha + terminacao, false, ""
		}
		return substituto(linha) + terminacao, false, linha
	}
	if fim == 0 {
		return linha + terminacao, true, ""
	}
	return substituto(linha[:fim]) + linha[fim+1:] + terminacao, true, linha[:fim]
}

// Mascarar esconde os valores secretos de um texto lido de arquivo.
//
//   - Em qualquer arquivo: bloco de chave privada PEM e credencial de formato
//     reconhecível (higiene.TrechosDeCredencial).
//   - Em arquivo sensível: também o valor de toda linha CHAVE=valor, chave:
//     valor, "chave": "valor" e password z do .netrc, inclusive chave antiga
//     comentada.
//
// ocultos são os valores escondidos, na ordem em que aparecem e sem repetir:
// é o que o revelar registra para a trava da memória. Aplicar duas vezes dá o
// mesmo texto, e a segunda passada não acha nada.
func Mascarar(conteudo string, sensivel bool) (texto string, ocultos []string) {
	texto = rePEM.ReplaceAllStringFunc(conteudo, func(bloco string) string {
		linhas := strings.Split(bloco, "\n")
		for _, l := range linhas {
			l = strings.TrimSpace(l)
			if l != "" && !strings.HasPrefix(l, "-----") {
				ocultos = append(ocultos, l)
			}
		}
		return fmt.Sprintf("[chave privada omitida: %d linhas]", len(linhas))
	})

	if sensivel {
		linhas := strings.Split(texto, "\n")
		var aberta byte // aspa de um valor em várias linhas que ainda não fechou
		for i, linha := range linhas {
			if aberta != 0 {
				nova, fechou, achado := mascararContinuacao(linha, aberta)
				linhas[i] = nova
				if achado != "" {
					ocultos = append(ocultos, achado)
				}
				if fechou {
					aberta = 0
				}
				continue
			}
			aberta = aspaAbertaNaLinha(linha) // olha a linha CRUA, antes de mascarar
			nova, achados := mascararLinhaSensivel(linha)
			linhas[i] = nova
			ocultos = append(ocultos, achados...)
		}
		texto = strings.Join(linhas, "\n")
	}

	trechos := higiene.TrechosDeCredencial(texto)
	sort.Slice(trechos, func(a, b int) bool { return trechos[a][0] < trechos[b][0] })
	var validos [][2]int
	fimAnterior := 0
	for _, t := range trechos {
		if t[0] >= fimAnterior {
			validos = append(validos, t)
			fimAnterior = t[1]
		}
	}
	for _, t := range validos { // na ordem em que aparecem
		ocultos = append(ocultos, texto[t[0]:t[1]])
	}
	for i := len(validos) - 1; i >= 0; i-- { // substitui de trás para frente: os índices não se movem
		t := validos[i]
		texto = texto[:t[0]] + marcaOculta + texto[t[1]:]
	}

	return texto, semRepeticao(ocultos)
}

func semRepeticao(valores []string) []string {
	visto := make(map[string]bool, len(valores))
	var unicos []string
	for _, v := range valores {
		if v != "" && !visto[v] {
			visto[v] = true
			unicos = append(unicos, v)
		}
	}
	return unicos
}
