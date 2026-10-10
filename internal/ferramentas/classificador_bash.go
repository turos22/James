package ferramentas

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
}

var denylistBash = denylist{
	verbos:      verbosDestrutivosBash,
	prefixos:    prefixosDestrutivosBash,
	ofuscacao:   marcasDeOfuscacaoBash,
	caminhos:    caminhosSensiveisBash,
	separadores: separadores + "`$",
	porBasename: true,
}

// ComandoBashPrecisaConfirmacao devolve true quando o comando bash precisa
// de confirmação humana antes de executar. Fail closed: vazio ou ofuscado
// devolve true.
func ComandoBashPrecisaConfirmacao(comando string) bool {
	return denylistBash.precisaConfirmacao(comando)
}
