package ferramentas

import "strings"

// tokensProibidos reprovam o comando em qualquer posição, mesmo que o verbo
// pareça leitura: cada um permite encadear, expandir ou executar código fora
// do verbo declarado. As chaves entram porque um ScriptBlock roda código
// arbitrário — isso reprova Where-Object { ... }, que de fato é leitura, e a
// perda é aceita de propósito sob a postura fail closed.
var tokensProibidos = []string{";", "&", "`", "$(", "@(", "{", "}", ">", "<", "--%", "::"}

// prefixosLeitura são prefixos de verbo tratados como somente-leitura. Get é
// definido como somente-leitura pelas diretrizes de verbo do PowerShell.
// Decisão deliberada de conveniência sobre rigor: um módulo de terceiro pode
// definir um Get-* com efeito colateral. A alternativa (lista exaustiva do
// core) quebraria em qualquer instalação diferente.
var prefixosLeitura = []string{"get-"}

// comandosLeitura é a allowlist de nomes completos e aliases. Fica numa
// variável só pra poder ser apertada ou afrouxada em um lugar.
var comandosLeitura = map[string]bool{
	"test-path": true, "resolve-path": true, "split-path": true, "join-path": true,
	"select-object": true, "select-string": true, "where-object": true,
	"sort-object": true, "group-object": true, "compare-object": true,
	"measure-object": true, "format-list": true, "format-table": true,
	"format-wide": true, "out-string": true, "convertto-json": true,
	"convertfrom-json": true, "write-output": true,

	"dir": true, "ls": true, "gci": true, "cat": true, "gc": true, "type": true,
	"pwd": true, "echo": true, "select": true, "where": true, "sort": true,
	"measure": true, "ft": true, "fl": true,

	"whoami": true, "hostname": true, "date": true,
}

// ComandoEhLeitura devolve true apenas para comandos comprovadamente sem
// efeito colateral. Fail closed: qualquer dúvida devolve false.
//
// A postura é allowlist de leitura, não blocklist de perigo — não se tenta
// enumerar o mal, enumera-se o pouco que é comprovadamente inofensivo. Falso
// positivo custa um clique; falso negativo custa a máquina.
func ComandoEhLeitura(comando string) bool {
	c := strings.TrimSpace(comando)
	if c == "" {
		return false
	}

	for _, token := range tokensProibidos {
		if strings.Contains(c, token) {
			return false
		}
	}

	// Todo estágio do pipeline precisa passar: um único estágio de escrita
	// reprova o comando inteiro.
	for _, estagio := range strings.Split(c, "|") {
		if !estagioEhLeitura(estagio) {
			return false
		}
	}
	return true
}

func estagioEhLeitura(estagio string) bool {
	campos := strings.Fields(strings.TrimSpace(estagio))
	if len(campos) == 0 {
		return false
	}

	verbo := strings.ToLower(campos[0])
	if comandosLeitura[verbo] {
		return true
	}
	for _, prefixo := range prefixosLeitura {
		if strings.HasPrefix(verbo, prefixo) && len(verbo) > len(prefixo) {
			return true
		}
	}
	return false
}
