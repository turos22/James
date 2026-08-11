package ferramentas

import (
	"strings"
)

// Este arquivo mudou de postura em 2026-08-11, a pedido do Arthur.
//
// ANTES: allowlist de leitura. Só um punhado de verbos comprovadamente
// inofensivos executava direto; tudo o mais pedia confirmação humana. Segura,
// e insuportável na prática — `Get-Process | Where-Object { ... }` já
// reprovava por conter `{`, então o card de aprovação aparecia o tempo todo e
// deixou de ser sinal pra virar ruído.
//
// AGORA: denylist de destruição. O James roda livre, e a confirmação fica
// reservada a três famílias: o que APAGA, o que ABRE A MÁQUINA (contas,
// permissões, registro, firewall, antivírus, persistência) e o que EXECUTA
// CÓDIGO DE PROCEDÊNCIA DESCONHECIDA (baixar-e-rodar, Invoke-Expression,
// payload codificado).
//
// LIMITE HONESTO desta abordagem, que precisa estar escrito aqui e não só na
// cabeça de quem escreveu: uma denylist de tokens NÃO resiste a um adversário
// que queira escapar dela. PowerShell monta nome de comando em tempo de
// execução (`&('Rem'+'ove-Item')`), e nenhuma lista pega isso. O modelo de
// ameaça aqui é OUTRO: é o James errando a mão — chamando algo destrutivo
// porque entendeu mal o pedido — não o James tentando driblar o próprio
// dono. Pra erro honesto, a lista de tokens é exatamente a ferramenta certa.
// As primitivas mais comuns de ofuscação (crase, base64, -EncodedCommand)
// entram na lista justamente pra fechar a evasão ACIDENTAL.

// verbosDestrutivos são comandos e aliases que exigem confirmação em qualquer
// posição do comando — não só como verbo do primeiro estágio. Um
// `Get-Process; Remove-Item x` é tão destrutivo quanto o `Remove-Item x`
// sozinho.
// Os apelidos abaixo foram conferidos um a um nesta máquina com
// `(Get-Alias <x>).Definition`, não deduzidos. A conferência valeu a pena: a
// primeira versão desta lista tinha "sc" achando que era o sc.exe de serviço,
// quando `sc` é apelido de **Set-Content** — bloqueava escrita comum, que é
// justamente o que esta política existe pra liberar. E faltava `ri`, que é
// Remove-Item: `ri C:\dados -Recurse` passava livre.
var verbosDestrutivos = map[string]bool{
	// Apagar e truncar
	"rm": true, "del": true, "erase": true, "rd": true, "rmdir": true,
	"ri": true, "rp": true, "rv": true, "rjb": true, "rmo": true,
	"rbp": true, "rsn": true, "rsnp": true,
	"clear-content": true, "clear-item": true, "clear-itemproperty": true,
	"clear-recyclebin": true, "clear-variable": true, "clear-history": true,
	"clv": true, "clhy": true,

	// Disco e partição
	"format": true, "format-volume": true, "clear-disk": true,
	"initialize-disk": true, "diskpart": true, "chkdsk": true,

	// Contas, permissões, posse
	"icacls": true, "cacls": true, "takeown": true, "set-acl": true,
	"net": true, "net1": true, "runas": true,
	"new-localuser": true, "set-localuser": true, "enable-localuser": true,
	"disable-localuser": true, "new-localgroup": true,
	"add-localgroupmember": true, "set-localgroup": true,

	// Registro e boot
	"reg": true, "regedit": true, "regedt32": true, "bcdedit": true,
	"set-executionpolicy": true,

	// Antivírus, firewall, rede
	"add-mppreference": true, "set-mppreference": true, "netsh": true,
	"set-netfirewallrule": true, "new-netfirewallrule": true,
	"disable-netfirewallrule": true, "disable-netadapter": true,

	// Persistência e serviços. "sc" NÃO entra: nesta máquina é apelido de
	// Set-Content, não o sc.exe de controle de serviço. Só o nome com
	// extensão chama o executável de verdade.
	"schtasks": true, "register-scheduledtask": true, "new-service": true,
	"set-service": true, "stop-service": true, "spsv": true, "sc.exe": true,

	// Energia e sessão
	"stop-computer": true, "restart-computer": true, "shutdown": true,
	"logoff": true,

	// Baixar e executar — o vetor de malware que o Arthur nomeou.
	// invoke-item/ii abre o arquivo com o handler padrão do sistema: é o
	// equivalente programático do duplo clique, e num .exe ou .lnk isso
	// executa.
	"invoke-expression": true, "iex": true,
	"invoke-item": true, "ii": true,
	"invoke-webrequest": true, "iwr": true, "wget": true, "curl": true,
	"invoke-restmethod": true, "irm": true,
	"start-bitstransfer": true, "bitsadmin": true, "certutil": true,
	"msiexec": true, "mshta": true, "rundll32": true, "regsvr32": true,
}

// prefixosDestrutivos pegam a família inteira de uma vez. "remove-" cobre
// Remove-Item, Remove-Service, Remove-LocalUser, Remove-Partition e todo
// Remove-* que um módulo de terceiro venha a definir — enumerar um a um
// envelheceria mal e falharia aberto.
var prefixosDestrutivos = []string{"remove-", "uninstall-", "unregister-"}

// marcasDeOfuscacao reprovam por substring, não por token: o ponto delas é
// justamente aparecer grudado no meio de outra coisa. Comando que esconde o
// que faz não pode ser classificado, e o que não dá pra classificar vai pro
// humano.
var marcasDeOfuscacao = []string{
	"-encodedcommand", "-enc ", "frombase64string", "`",
}

// caminhosSensiveis reprovam por substring, como as marcas de ofuscação, e
// pelo mesmo motivo: o que importa é o ALVO, não o verbo. Set-ItemProperty e
// New-ItemProperty não podem entrar inteiros na denylist — servem pra atributo
// de arquivo, uso corriqueiro — mas apontados pra uma hive do registro viram
// persistência, e a chave Run é o caminho clássico.
//
// Reprova leitura de registro junto, de propósito: separar leitura de escrita
// aqui exigiria analisar o verbo além do alvo, e o saldo é ruim. Ler o
// registro é raro no uso normal e custa um clique; deixar passar uma escrita
// na Run custa persistência.
var caminhosSensiveis = []string{
	"hklm:", "hkcu:", "hkcr:", "hku:", "hkcc:", "hkey_",
}

// separadores quebram o comando em tokens. A lista cobre o que o PowerShell
// usa pra encadear, agrupar e expandir — é o que impede um verbo destrutivo
// de se esconder colado num delimitador (`echo $(Remove-Item x)`).
const separadores = " \t\r\n;|&()[]{}<>,=\"'"

// ComandoPrecisaConfirmacao devolve true quando o comando precisa de
// confirmação humana antes de executar. Fail closed: comando vazio, ilegível
// ou ofuscado devolve true.
func ComandoPrecisaConfirmacao(comando string) bool {
	c := strings.TrimSpace(comando)
	if c == "" {
		return true
	}

	minusculo := strings.ToLower(c)
	for _, marca := range marcasDeOfuscacao {
		if strings.Contains(minusculo, marca) {
			return true
		}
	}
	for _, caminho := range caminhosSensiveis {
		if strings.Contains(minusculo, caminho) {
			return true
		}
	}

	for _, token := range strings.FieldsFunc(minusculo, ehSeparador) {
		// `&Remove-Item` e caminhos como `.\remove-tudo.ps1` chegam aqui com
		// enfeite na frente; o que interessa é o nome do comando.
		token = strings.TrimLeft(token, "&.\\/-")
		if token == "" {
			continue
		}
		if verbosDestrutivos[token] {
			return true
		}
		for _, prefixo := range prefixosDestrutivos {
			if strings.HasPrefix(token, prefixo) && len(token) > len(prefixo) {
				return true
			}
		}
	}

	return false
}

func ehSeparador(r rune) bool {
	return strings.ContainsRune(separadores, r)
}
