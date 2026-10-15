// Package higiene detecta credenciais deixadas na árvore do repositório.
//
// Existe para que "não commitar segredo" seja verificação automática em vez
// de disciplina humana. Roda junto com o resto da suíte em `go test ./...`.
package higiene

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Achado é uma ocorrência de credencial num arquivo da árvore.
type Achado struct {
	Arquivo string
	Padrao  string
}

// padroes são os prefixos das credenciais que já circularam neste projeto.
// Montados por concatenação de propósito: assim o literal completo nunca
// aparece neste arquivo e a varredura não acusa a si mesma.
var padroes = []string{
	"AIza" + "Sy",    // Google AI Studio (Gemini)
	"tvly" + "-",     // Tavily
	"nvapi" + "-",    // NVIDIA
	"sk-or-" + "v1-", // OpenRouter
}

// padroesRegex cobre segredos sem prefixo fixo — precisam de forma, não
// substring. Cada entrada carrega um nome descritivo pro Achado.
var padroesRegex = []struct {
	nome   string
	padrao *regexp.Regexp
}{
	{"connection-string-com-senha", regexp.MustCompile(`://[^:/@\s]+:[^:/@\s]+@`)},
	{"aws-access-key", regexp.MustCompile(`AKIA[0-9A-Z]{16}`)},
	{"token-github", regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{36,}`)},
	{"jwt", regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)},
	{"chave-privada-pem", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	// Chave da Anthropic (a que o James usa). O prefixo é montado por
	// concatenação pelo mesmo motivo de padroes, e o corpo de 20 ou mais
	// caracteres deixa de fora fixtures curtas de teste (sk-ant-teste).
	{"chave-anthropic", regexp.MustCompile(`sk-` + `ant-[A-Za-z0-9_-]{20,}`)},
}

// TrechosDeCredencial devolve o início e o fim (em bytes) de cada credencial
// reconhecível no texto, pela MESMA lista da varredura do repositório (uma
// lista só, sem cópia). Serve a quem precisa esconder o valor (a máscara das
// ferramentas de arquivo) e a quem só precisa saber se há uma (a memória).
//
// Para prefixo fixo, o trecho vai do prefixo até o fim do token (espaço, aspa,
// vírgula, ponto e vírgula ou fim). Para URL com senha, o trecho é só a senha.
func TrechosDeCredencial(texto string) [][2]int {
	var trechos [][2]int
	for _, padrao := range padroes {
		for resto := 0; resto < len(texto); {
			i := strings.Index(texto[resto:], padrao)
			if i < 0 {
				break
			}
			inicio := resto + i
			fim := inicio + len(padrao)
			for fim < len(texto) && !strings.ContainsRune(" \t\r\n\"'`,;)]}<>", rune(texto[fim])) {
				fim++
			}
			trechos = append(trechos, [2]int{inicio, fim})
			resto = fim
		}
	}
	for _, pr := range padroesRegex {
		for _, m := range pr.padrao.FindAllStringSubmatchIndex(texto, -1) {
			if pr.nome == "connection-string-com-senha" {
				// O padrão cobre esquema, usuário, senha e arroba; o segredo é só a senha.
				corpo := texto[m[0]:m[1]]
				sep := strings.Index(corpo[3:], ":")
				if sep < 0 {
					continue
				}
				trechos = append(trechos, [2]int{m[0] + 3 + sep + 1, m[1] - 1})
				continue
			}
			trechos = append(trechos, [2]int{m[0], m[1]})
		}
	}
	sort.Slice(trechos, func(a, b int) bool { return trechos[a][0] < trechos[b][0] })
	return trechos
}

// ParecaCredencial diz se o texto contém alguma credencial reconhecível.
func ParecaCredencial(texto string) bool {
	return len(TrechosDeCredencial(texto)) > 0
}

// diretoriosIgnorados nunca entram na varredura: são dependências de
// terceiros ou artefatos de build que não versionamos.
var diretoriosIgnorados = map[string]bool{
	".git":         true,
	"node_modules": true,
	".venv":        true, // venv do device Python (dispositivo/.venv), fora do git
	".executar":    true, // logs, binarios e perfil do Chrome do executar.sh, fora do git
	"__pycache__":  true, // bytecode do device Python, gerado ao rodar, fora do git
	"TTs":          true,
	"vosk":         true,
	"dist":         true,
	"docs":         true, // specs/planos mostram formato de connection
	// string como exemplo (placeholder tipo "SENHA"), não segredo real —
	// o risco de verdade (config.toml, .env, código-fonte, binário
	// compilado) continua coberto fora de docs/
	"certs": true, // certificado TLS da VPN Tailscale (tailscale cert),
	// gerado localmente por máquina — já coberto pelo .gitignore, nunca é
	// commitado. Mesma razão de .env em arquivosIgnorados: a guarda existe
	// pra impedir commit de segredo, não pra exigir zero segredo em disco.
}

// extensoesIgnoradas pula bibliotecas nativas de terceiros. São 68 MB de
// .dll e .lib que jamais conteriam uma credencial nossa, e lê-los a cada
// `go test ./...` só custa tempo. Binários que nós compilamos (.exe)
// continuam sendo varridos — são justamente o caso perigoso.
var extensoesIgnoradas = map[string]bool{
	".dll": true,
	".lib": true,
}

// arquivosIgnorados são nomes de arquivo (não caminho) que ficam de fora da
// varredura por conterem segredo local por design. .env é o exemplo central:
// já está protegido pelo .gitignore e nunca é commitado — a guarda existe
// para impedir COMMIT de segredo, não para exigir zero segredo em disco.
// Match exato de nome — .env.production ou .env.local continuam sendo varridos.
var arquivosIgnorados = map[string]bool{
	".env":         true,
	".env.example": true,
}

// Varrer percorre raiz recursivamente e devolve toda ocorrência de
// credencial conhecida, inclusive dentro de binários compilados.
func Varrer(raiz string) ([]Achado, error) {
	var achados []Achado

	err := filepath.WalkDir(raiz, func(caminho string, entrada fs.DirEntry, err error) error {
		if err != nil {
			// Entrada inacessível (permissão negada, link quebrado) não é
			// achado nem falha do Varrer — mesma filosofia do erro de leitura
			// de arquivo abaixo: pula e segue a varredura do resto da árvore.
			return nil
		}
		if entrada.IsDir() {
			if diretoriosIgnorados[entrada.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if extensoesIgnoradas[strings.ToLower(filepath.Ext(caminho))] {
			return nil
		}
		if arquivosIgnorados[strings.ToLower(entrada.Name())] {
			return nil
		}

		conteudo, err := os.ReadFile(caminho)
		if err != nil {
			// Arquivo ilegível não é achado: não cabe a esta função falhar
			// por permissão ou lock de escrita.
			return nil
		}

		for _, padrao := range padroes {
			if bytes.Contains(conteudo, []byte(padrao)) {
				achados = append(achados, Achado{Arquivo: caminho, Padrao: padrao})
			}
		}

		// Arquivo de teste (_test.go) é o lugar mais comum de aparecer uma
		// string de conexão de mentira, só pra exercitar parsing — mesma
		// forma de um segredo real, mas sem ser um. Pular só esse padrão
		// pra código de teste; os outros quatro (AWS, GitHub, JWT, PEM)
		// continuam varrendo tudo, porque não têm o mesmo problema de
		// colisão com fixture comum.
		ehArquivoDeTeste := strings.HasSuffix(entrada.Name(), "_test.go")
		for _, pr := range padroesRegex {
			if pr.nome == "connection-string-com-senha" && ehArquivoDeTeste {
				continue
			}
			if pr.padrao.Match(conteudo) {
				achados = append(achados, Achado{Arquivo: caminho, Padrao: pr.nome})
			}
		}
		return nil
	})

	return achados, err
}
