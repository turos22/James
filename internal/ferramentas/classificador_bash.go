package ferramentas

import "strings"

// Denylist bash. Mesmo modelo de ameaça e mesmas três famílias da versão
// PowerShell em classificador.go — leia o comentário de lá antes de mexer
// aqui. Spec: docs/superpowers/specs/2026-10-09-james-fedora-design.md §4.3.
//
// Divergências deliberadas em relação ao Windows:
//   - systemctl pede confirmação SEMPRE, até `status`. Separar leitura de
//     escrita exigiria analisar o subcomando; mesmo argumento do registro.
//   - kill/pkill/killall entram: matar um editor aberto perde trabalho.
//   - ip fica de fora: `ip a` é leitura corriqueira.
//   - redirecionamento (> arquivo) não reprova, como Out-File no Windows.
//
// LIMITE HONESTO, igual ao do PowerShell: r''m, $(printf rm) e IFS montado
// escapam. A lista fecha a evasão acidental, não a deliberada.

var verbosDestrutivosBash = map[string]bool{
	// Apagar e truncar
	"rm": true, "rmdir": true, "shred": true, "unlink": true, "truncate": true, "srm": true,

	// Disco e partição (mkfs.* vem pelo prefixo)
	"dd": true, "mkfs": true, "fdisk": true, "sfdisk": true, "gdisk": true, "parted": true,
	"wipefs": true, "mkswap": true, "swapoff": true, "mount": true, "umount": true,
	"cryptsetup": true, "lvremove": true, "vgremove": true, "pvremove": true,

	// Contas, permissões, posse
	"sudo": true, "su": true, "doas": true, "pkexec": true, "run0": true,
	"chmod": true, "chown": true, "chgrp": true, "setfacl": true, "chattr": true,
	"useradd": true, "userdel": true, "usermod": true, "groupadd": true,
	"groupdel": true, "gpasswd": true, "passwd": true, "chpasswd": true, "visudo": true,

	// O "registro e boot" do Linux
	"grubby": true, "grub2-mkconfig": true, "grub2-install": true,
	"efibootmgr": true, "sysctl": true, "setenforce": true, "semanage": true, "bootctl": true,

	// Firewall e rede
	"firewall-cmd": true, "iptables": true, "ip6tables": true, "nft": true,
	"ufw": true, "nmcli": true,

	// Persistência e serviços
	"systemctl": true, "service": true, "crontab": true, "at": true, "loginctl": true,

	// Energia, sessão e processos
	"shutdown": true, "reboot": true, "poweroff": true, "halt": true,
	"kill": true, "pkill": true, "killall": true,

	// Pacotes — o análogo do msiexec
	"dnf": true, "dnf5": true, "yum": true, "rpm": true, "flatpak": true,

	// Baixar e executar. curl e wget como VERBO, igual ao Windows (iwr,
	// wget, curl já estão lá): não só o `| sh`. `exec` pega também o
	// `-exec` do find, porque o TrimLeft tira o hífen — e find -exec roda
	// comando arbitrário, então pegar é o certo.
	"curl": true, "wget": true, "eval": true, "source": true, "exec": true,
	"xdg-open": true, "gio": true,
	"bash": true, "sh": true, "zsh": true, "dash": true,

	// Criar, copiar, mover e editar arquivos (spec 2026-10-14 arquivos, D15):
	// a regra do Arthur é que mexer no disco pede aprovação. Casam em qualquer
	// posição do comando, como o resto da tabela. Os que também têm um uso só
	// de leitura (unzip -l, tar -t, sed sem -i...) ficam em escreveEmArquivoBash,
	// que olha as opções.
	"touch": true, "mkdir": true, "cp": true, "mv": true, "ln": true,
	"tee": true, "rsync": true, "patch": true, "zip": true, "split": true,
}

var prefixosDestrutivosBash = []string{"mkfs"}

// No bash a crase é substituição de comando, não escape como no
// PowerShell — mas o efeito pra classificação é o mesmo: esconde o que roda.
var marcasDeOfuscacaoBash = []string{
	"`", "base64", "xxd -r", "$'\\", "${ifs}", "\\x",
}

// Leitura também reprova, com o mesmo argumento do registro no Windows:
// separar leitura de escrita exigiria analisar o verbo, e o saldo é ruim.
var caminhosSensiveisBash = []string{
	"/etc/", "/boot/", "/dev/sd", "/dev/nvme", "/sys/", "/proc/sys/",
	".ssh/", ".bashrc", ".bash_profile", ".profile", ".zshrc",
	"/autostart", "/systemd/", "/var/spool/cron", "sudoers",

	// Arquivos de segredo (spec 2026-10-14 arquivos, D15). A ferramenta
	// `arquivos` lê esses com a máscara; o shell leria o valor cru e contornaria
	// a máscara, então continua pedindo aprovação. ".env" não casa com ".venv".
	".env", ".aws/", ".gnupg/", ".netrc", ".git-credentials", ".npmrc", ".pypirc",
	"id_rsa", "id_ed25519", "id_ecdsa",
}

var denylistBash = denylist{
	verbos:      verbosDestrutivosBash,
	prefixos:    prefixosDestrutivosBash,
	ofuscacao:   marcasDeOfuscacaoBash,
	caminhos:    caminhosSensiveisBash,
	separadores: separadores + "`$",
	porBasename: true,
	extras:      escreveEmArquivoBash,
}

// --- escrita em arquivo que depende das opções (spec 2026-10-14, D15) ---------

// simplesDoBash divide o comando em comandos simples e tokens, respeitando
// aspas simples, aspas duplas e barra invertida (as aspas saem do token). ok é
// falso quando o comando termina dentro de aspas: sem saber o que roda, quem
// chama falha para o lado seguro.
func simplesDoBash(comando string) (comandos [][]string, ok bool) {
	var atual []string
	var token strings.Builder
	temToken := false
	fecharToken := func() {
		if temToken {
			atual = append(atual, token.String())
			token.Reset()
			temToken = false
		}
	}
	fecharComando := func() {
		fecharToken()
		if len(atual) > 0 {
			comandos = append(comandos, atual)
			atual = nil
		}
	}

	letras := []rune(comando)
	for i := 0; i < len(letras); i++ {
		r := letras[i]
		switch {
		case r == '\\':
			if i+1 < len(letras) {
				i++
				token.WriteRune(letras[i])
				temToken = true
			}
		case r == '\'':
			fim := -1
			for j := i + 1; j < len(letras); j++ {
				if letras[j] == '\'' {
					fim = j
					break
				}
			}
			if fim < 0 {
				return nil, false
			}
			token.WriteString(string(letras[i+1 : fim]))
			temToken = true
			i = fim
		case r == '"':
			fechou := false
			for j := i + 1; j < len(letras); j++ {
				if letras[j] == '\\' && j+1 < len(letras) {
					token.WriteRune(letras[j+1])
					j++
					continue
				}
				if letras[j] == '"' {
					fechou = true
					i = j
					break
				}
				token.WriteRune(letras[j])
			}
			if !fechou {
				return nil, false
			}
			temToken = true
		case strings.ContainsRune(";|&\n(){}`$", r):
			fecharComando()
		case r == ' ' || r == '\t' || r == '\r':
			fecharToken()
		default:
			token.WriteRune(r)
			temToken = true
		}
	}
	fecharComando()
	return comandos, true
}

// Programas que só repassam o comando seguinte: o que importa é o que vem depois.
var embrulhosDeComando = map[string]bool{
	"env": true, "nice": true, "nohup": true, "time": true, "command": true, "builtin": true, "xargs": true,
}

func ehAtribuicaoDeVariavel(token string) bool {
	i := strings.IndexByte(token, '=')
	if i <= 0 {
		return false
	}
	for j, r := range token[:i] {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (j > 0 && r >= '0' && r <= '9')) {
			return false
		}
	}
	return true
}

// nomeEArgs acha o programa de um comando simples, pulando atribuições de
// variável (FOO=1 touch a) e embrulhos (env, xargs, nice...).
func nomeEArgs(tokens []string) (nome string, args []string) {
	embrulhado := false
	for i, t := range tokens {
		if ehAtribuicaoDeVariavel(t) {
			continue
		}
		base := strings.ToLower(t[strings.LastIndex(t, "/")+1:])
		if embrulhosDeComando[base] {
			embrulhado = true
			continue
		}
		if embrulhado && strings.HasPrefix(t, "-") {
			continue
		}
		return base, tokens[i+1:]
	}
	return "", nil
}

// opcaoCurta diz se algum token de opção curta (-xzf, -i.bak) tem uma das
// letras, lendo cada token até a primeira letra que consome o resto como
// argumento (paradas): em "perl -Mstrict" o "i" de "strict" não é a opção -i.
func opcaoCurta(args []string, letras, paradas string) bool {
	for _, t := range args {
		if len(t) < 2 || t[0] != '-' || t[1] == '-' {
			continue
		}
		for _, c := range t[1:] {
			if strings.ContainsRune(paradas, c) {
				break
			}
			if strings.ContainsRune(letras, c) {
				return true
			}
		}
	}
	return false
}

func opcaoLonga(args []string, nomes ...string) bool {
	for _, t := range args {
		if !strings.HasPrefix(t, "--") {
			continue
		}
		base, _, _ := strings.Cut(t, "=")
		for _, n := range nomes {
			if base == n {
				return true
			}
		}
	}
	return false
}

func tarEscreve(args []string) bool {
	if opcaoLonga(args, "--create", "--extract", "--get", "--append", "--update", "--delete", "--catenate", "--concatenate") {
		return true
	}
	if opcaoCurta(args, "cxruA", "fCTXbL") {
		return true
	}
	// Estilo antigo, sem hífen: tar xf a.tar
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		for _, c := range args[0] {
			if strings.ContainsRune("fCTXbL", c) {
				break
			}
			if strings.ContainsRune("cxruA", c) {
				return true
			}
		}
	}
	return false
}

// gitMudaArquivos diz se o subcomando do git apaga ou sobrescreve arquivos da
// árvore de trabalho: clean, reset --hard e restore. Limites conhecidos:
// checkout/switch -f e stash descartam alterações e não são pegos aqui.
func gitMudaArquivos(args []string) bool {
	sub := ""
	resto := args
	for i := 0; i < len(args); i++ {
		t := args[i]
		if t == "-C" || t == "-c" { // opção global com valor
			i++
			continue
		}
		if strings.HasPrefix(t, "-") {
			continue
		}
		sub, resto = t, args[i+1:]
		break
	}
	switch sub {
	case "clean", "restore":
		return true
	case "reset":
		return opcaoLonga(resto, "--hard")
	}
	return false
}

// comandoEscreve diz se este comando simples cria, copia, move ou edita
// arquivos pelas opções que passou.
func comandoEscreve(nome string, args []string) bool {
	switch nome {
	case "install":
		return true
	case "unzip":
		return !opcaoCurta(args, "ltvpz", "dxP")
	case "gzip", "gunzip", "bzip2", "bunzip2", "xz", "unxz", "zstd", "unzstd", "lz4", "unlz4":
		return !(opcaoCurta(args, "clt", "") || opcaoLonga(args, "--stdout", "--to-stdout", "--list", "--test"))
	case "7z", "7za", "7zr", "7zz":
		for _, t := range args {
			if !strings.HasPrefix(t, "-") {
				return t != "l" && t != "t" && t != "i"
			}
		}
		return true
	case "tar":
		return tarEscreve(args)
	case "find":
		// -exec já cai no verbo "exec" da tabela (-ok não é coberto).
		for _, t := range args {
			switch t {
			case "-delete", "-fprint", "-fprint0", "-fprintf", "-fls":
				return true
			}
		}
	case "sort":
		return opcaoCurta(args, "o", "kStT") || opcaoLonga(args, "--output")
	case "git":
		return gitMudaArquivos(args)
	case "sed":
		return opcaoCurta(args, "i", "efl") || opcaoLonga(args, "--in-place")
	case "perl":
		return opcaoCurta(args, "i", "eEMmIFxCVDd")
	case "awk", "gawk", "mawk", "nawk":
		for _, t := range args {
			if strings.Contains(t, "inplace") {
				return true
			}
		}
	}
	return false
}

// Destinos de redirecionamento que não criam nem alteram arquivo nenhum.
var destinosInofensivos = map[string]bool{"/dev/null": true, "/dev/stdout": true, "/dev/stderr": true}

// redirecionaParaArquivo diz se o comando tem um ">" (>>, >|, &>, 2>) que
// escreve num arquivo. O varredor respeita aspas simples, aspas duplas e
// barra invertida, porque `grep ">" f` e `awk '$1 > 3'` não escrevem nada.
// Duplicar descritor (2>&1, >&2, >&-) e escrever em /dev/null, /dev/stdout ou
// /dev/stderr também não. `>(cmd)` (substituição de processo) e destino que
// não dá para ler pedem aprovação.
func redirecionaParaArquivo(comando string) bool {
	letras := []rune(comando)
	n := len(letras)
	for i := 0; i < n; i++ {
		switch letras[i] {
		case '\\':
			i++ // o próximo caractere é literal (\>)
		case '\'':
			for i++; i < n && letras[i] != '\''; i++ {
			}
		case '"':
			for i++; i < n && letras[i] != '"'; i++ {
				if letras[i] == '\\' {
					i++
				}
			}
		case '>':
			j := i + 1
			if j < n && (letras[j] == '>' || letras[j] == '|') {
				j++
			}
			if j < n && letras[j] == '&' {
				// >&2 e >&- duplicam ou fecham um descritor; >&arquivo escreve.
				if k := j + 1; k < n && (letras[k] == '-' || (letras[k] >= '0' && letras[k] <= '9')) {
					i = k
					continue
				}
				j++
			}
			for j < n && (letras[j] == ' ' || letras[j] == '\t') {
				j++
			}
			if j >= n || letras[j] == '(' {
				return true
			}
			inicio := j
			if letras[j] == '"' || letras[j] == '\'' {
				aspa := letras[j]
				inicio = j + 1
				for j++; j < n && letras[j] != aspa; j++ {
				}
			} else {
				for j < n && !strings.ContainsRune(" \t\r\n;|&()<>", letras[j]) {
					j++
				}
			}
			if !destinosInofensivos[string(letras[inicio:min(j, n)])] {
				return true
			}
			i = j
		}
	}
	return false
}

// escreveEmArquivoBash é o "extras" da denylist do bash: comandos que criam ou
// alteram arquivos conforme as opções, ou redirecionam a saída para um
// arquivo. Comando que não dá para ler (aspa aberta) pede aprovação.
func escreveEmArquivoBash(comando string) bool {
	comandos, ok := simplesDoBash(comando)
	if !ok || redirecionaParaArquivo(comando) {
		return true
	}
	for _, tokens := range comandos {
		nome, args := nomeEArgs(tokens)
		if nome != "" && comandoEscreve(nome, args) {
			return true
		}
	}
	return false
}

// ComandoBashPrecisaConfirmacao devolve true quando o comando bash precisa
// de confirmação humana antes de executar. Fail closed: vazio ou ofuscado
// devolve true.
func ComandoBashPrecisaConfirmacao(comando string) bool {
	return denylistBash.precisaConfirmacao(comando)
}
