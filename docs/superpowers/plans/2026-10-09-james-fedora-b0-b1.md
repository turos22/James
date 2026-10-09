# Porte para o Fedora, B0 e B1: plano de implementação

> **Para quem executa:** um task por vez, na ordem. Os passos usam checkbox (`- [ ]`). TDD por task: teste primeiro, rodar e **ver falhar**, implementar, rodar e ver passar, commitar. A saída de cada "rodar" vai no relato da task. Se não rodou, diga que não rodou.

**Objetivo:** jamesd, SPA e Postgres rodando no Fedora com conversa por texto e TTS (B0), e o device com wake word e comando de voz (B1). O Windows continua funcionando.

**Arquitetura:** Go com build tags só na escolha da plataforma. Classificadores e executor PowerShell compilam em todo lugar, e o executor bash compila em `unix`. Python com `james_device/plataforma/`, escolhido por `sys.platform`, e o Win32 de `superficies.py` carregado sob demanda.

**Spec:** [`../specs/2026-10-09-james-fedora-design.md`](../specs/2026-10-09-james-fedora-design.md) (aprovada em 2026-10-09)
**Branch:** `feat/fedora`, a partir de `main` em `e5943b6`.

---

## Antes de começar: leia isto

1. **Terminal no Linux: a regra que não se negocia.** Da Task 3 até a Task 6, `executar_comando_sistema` **não é registrado** no Linux. A Task 6 (a que liga o terminal) só começa com as Tasks 4 e 5 commitadas e verdes. Se alguma das duas estiver vermelha, pare.
2. **Os 4 testes `TestExecutarPowerShellDeVerdade*` falham no Linux hoje.** Isso é esperado até a Task 3. Antes dela, use `go test ./... 2>&1 | grep -v PowerShellDeVerdade` para ler o resto.
3. **O banco virgem tem uma corrida de migração** (spec §9). Se aparecer `duplicate key value violates unique constraint "pg_type_typname_nsp_index"`, rode de novo. Na primeira subida de um volume novo, use `go test -p 1 ./...`.
4. **Lado Windows:** tudo o que está sem build tag compila nas duas plataformas. Ao fim de cada task Go, rode `GOOS=windows go vet ./...`. Ele compila também os arquivos `_test.go` do Windows, então é a rede contra quebrar o Windows sem ter a máquina.
5. **Plano versus spec.** A spec §4.1 previa `terminal_windows.go` com o executor PowerShell. Este plano refina isso pela própria D2 da spec: `codificarComando`, `prefixoUTF8` e `executarPowerShell` são Go puro e compilam no Linux, então ficam **sem tag** em `terminal_powershell.go`. Só a escolha da plataforma e os testes que rodam o shell de verdade têm tag. Além disso, a função de classificação vira um campo injetável de `ComandoSistema` (`Classificador`, Task 2), no mesmo padrão do `Executor` que já existe. Sem isso, `TestComandoSistemaConfirmacaoNecessaria` dependeria da plataforma.

**Rodar a suíte Go** (com o Postgres de pé, `docker compose up -d`):

```bash
go vet ./... && GOOS=windows go vet ./... && go test -count=1 ./...
```

**Rodar a suíte do device** (depois da Task 9):

```bash
dispositivo/.venv/bin/python -m pytest -q dispositivo
```

---

## Estrutura de arquivos

```
internal/ferramentas/
  classificador.go                     MODIFICAR  núcleo: type denylist + precisaConfirmacao
                                                  + tabelas PowerShell (sem mudar entrada)
  classificador_bash.go                CRIAR      tabelas bash + ComandoBashPrecisaConfirmacao
  classificador_test.go                MODIFICAR  passa a chamar ComandoPowerShellPrecisaConfirmacao
  classificador_bash_test.go           CRIAR
  terminal.go                          MODIFICAR  campos Classificador e NomeShell; sem PowerShell
  terminal_powershell.go               CRIAR      codificarComando, prefixoUTF8, executarPowerShell
  terminal_bash.go                     CRIAR      //go:build unix — executarBash
  plataforma_windows.go                CRIAR      TerminalSuportado, NovoComandoSistema (PowerShell)
  plataforma_unix.go                   CRIAR      //go:build unix — idem (desligado, depois bash)
  terminal_test.go                     MODIFICAR  perde os 4 testes de PowerShell real
  terminal_powershell_windows_test.go  CRIAR      os 4 testes de PowerShell real
  terminal_bash_test.go                CRIAR      //go:build unix
  plataforma_windows_test.go           CRIAR
  plataforma_unix_test.go              CRIAR      //go:build unix
cmd/jamesd/
  ferramentas.go                       MODIFICAR  registrarTerminal
  ferramentas_test.go                  MODIFICAR
  main.go                              MODIFICAR  usa registrarTerminal
internal/config/config.go, config_test.go   MODIFICAR  JAMES_CLAUDE_CODE_DIRETORIO
.env.example                           MODIFICAR  documenta a variável
executar.sh                            CRIAR
.gitignore                             MODIFICAR  .executar/, dispositivo/.venv/
dispositivo/
  pyproject.toml                       MODIFICAR  marcadores sys_platform
  james_device/superficies.py          MODIFICAR  só o puro; Win32 sob demanda
  james_device/plataforma/__init__.py  CRIAR
  james_device/plataforma/windows.py   CRIAR      Win32 movido + playsound + ponte real
  james_device/plataforma/linux.py     CRIAR      pw-play + ponte "indisponível"
  james_device/tela.py, captura.py     MODIFICAR  efeito DPI vem de plataforma.windows
  james_device/main.py                 MODIFICAR  usa plataforma
  tests/test_plataforma.py             CRIAR
  tests/test_tela.py                   MODIFICAR  skip fora do win32
```

---

## Task 0: Pré-requisitos do Fedora (Arthur)

Não é código. Sem isso, as Tasks 9 a 13 não rodam.

- [ ] **Passo 1:** o Arthur roda (precisa de `sudo`, então não é o agente que roda):

```bash
sudo dnf install portaudio-devel webkit2gtk4.1-devel gtk3-devel
```

- [ ] **Passo 2:** instalar o `edge-tts` fora de venv, porque o jamesd chama o binário:

```bash
pipx install edge-tts
```

- [ ] **Passo 3:** conferir.

```bash
rpm -q portaudio-devel webkit2gtk4.1-devel gtk3-devel && command -v edge-tts && edge-tts --version
```

Esperado: os três pacotes listados com versão e o caminho do `edge-tts`. Se faltar `pipx`, use `sudo dnf install pipx`.

---

## Task 1: Classificador com núcleo e tabelas separados

Refatoração sem mudança de comportamento. A rede é a suíte atual do classificador, que tem que continuar verde com a função renomeada.

**Arquivos:** `internal/ferramentas/classificador.go`, `classificador_test.go`, `terminal.go`, `terminal_test.go`

- [ ] **Passo 1: Renomear nos testes primeiro.** Em `classificador_test.go` e `terminal_test.go`, trocar todo `ComandoPrecisaConfirmacao(` por `ComandoPowerShellPrecisaConfirmacao(`, e os nomes nas mensagens de erro também. Renomear `TestComandoPrecisaConfirmacao` para `TestComandoPowerShellPrecisaConfirmacao`.

```bash
sed -i 's/\bComandoPrecisaConfirmacao\b/ComandoPowerShellPrecisaConfirmacao/g' internal/ferramentas/classificador_test.go internal/ferramentas/terminal_test.go
```

- [ ] **Passo 2: Rodar e ver falhar.**

```bash
go test ./internal/ferramentas/ 2>&1 | head -5
```

Esperado: `undefined: ComandoPowerShellPrecisaConfirmacao`.

- [ ] **Passo 3: Extrair o núcleo.** Em `classificador.go`, substituir a função `ComandoPrecisaConfirmacao` e a `ehSeparador` por:

```go
// denylist é a política de uma plataforma: as mesmas quatro listas, com o
// mesmo núcleo de decisão. Separar núcleo de tabela é o que deixa as duas
// denylists (PowerShell e bash) compilarem e serem testadas em qualquer
// sistema — uma denylist testada só na própria plataforma apodrece sem
// ninguém ver.
type denylist struct {
	verbos      map[string]bool
	prefixos    []string
	ofuscacao   []string
	caminhos    []string
	separadores string

	// porBasename compara o verbo pelo último segmento do caminho. No bash,
	// /usr/bin/rm é o rm; sem isso o TrimLeft abaixo deixa "usr/bin/rm" e o
	// verbo passa livre. No PowerShell fica desligado, preservando a
	// política de 2026-08-11 exatamente como era.
	porBasename bool
}

var denylistPowerShell = denylist{
	verbos:      verbosDestrutivos,
	prefixos:    prefixosDestrutivos,
	ofuscacao:   marcasDeOfuscacao,
	caminhos:    caminhosSensiveis,
	separadores: separadores,
}

// ComandoPowerShellPrecisaConfirmacao devolve true quando o comando
// PowerShell precisa de confirmação humana antes de executar. Fail closed:
// comando vazio, ilegível ou ofuscado devolve true.
func ComandoPowerShellPrecisaConfirmacao(comando string) bool {
	return denylistPowerShell.precisaConfirmacao(comando)
}

func (d denylist) precisaConfirmacao(comando string) bool {
	c := strings.TrimSpace(comando)
	if c == "" {
		return true
	}

	minusculo := strings.ToLower(c)
	for _, marca := range d.ofuscacao {
		if strings.Contains(minusculo, marca) {
			return true
		}
	}
	for _, caminho := range d.caminhos {
		if strings.Contains(minusculo, caminho) {
			return true
		}
	}

	ehSeparador := func(r rune) bool { return strings.ContainsRune(d.separadores, r) }
	for _, token := range strings.FieldsFunc(minusculo, ehSeparador) {
		// `&Remove-Item` e caminhos como `.\remove-tudo.ps1` chegam aqui com
		// enfeite na frente; o que interessa é o nome do comando.
		token = strings.TrimLeft(token, "&.\\/-")
		if d.porBasename {
			if i := strings.LastIndex(token, "/"); i >= 0 {
				token = token[i+1:]
			}
		}
		if token == "" {
			continue
		}
		if d.verbos[token] {
			return true
		}
		for _, prefixo := range d.prefixos {
			if strings.HasPrefix(token, prefixo) && len(token) > len(prefixo) {
				return true
			}
		}
	}

	return false
}
```

Em `terminal.go`, a chamada `ComandoPrecisaConfirmacao(comando)` vira `ComandoPowerShellPrecisaConfirmacao(comando)`. Ela é provisória e some na Task 2.

- [ ] **Passo 4: Rodar e ver passar** (exceto os 4 de PowerShell real, ver "Antes de começar" item 2).

```bash
go test -count=1 ./internal/ferramentas/ 2>&1 | grep -E '^(---|ok|FAIL)' | grep -v PowerShellDeVerdade
```

Esperado: nenhuma linha `--- FAIL` fora dos 4 conhecidos.

- [ ] **Passo 5:** `GOOS=windows go vet ./internal/ferramentas/` sem saída.

- [ ] **Passo 6: Commit.**

```bash
git add internal/ferramentas/ && git commit -m "refactor(ferramentas): classificador separa nucleo de tabela, PowerShell vira uma denylist entre outras"
```

---

## Task 2: `Classificador` injetável em `ComandoSistema`

**Arquivos:** `internal/ferramentas/terminal.go`, `terminal_test.go`

- [ ] **Passo 1: Testes.** Em `terminal_test.go`, `TestComandoSistemaConfirmacaoNecessaria` passa a montar a ferramenta explicitamente, em vez de usar `NovoComandoSistema()`, que vai depender da plataforma:

```go
	c := &ComandoSistema{Classificador: ComandoPowerShellPrecisaConfirmacao}
```

E acrescentar:

```go
// TestComandoSistemaSemClassificadorExigeConfirmacao: fail closed. Uma
// ComandoSistema montada sem política nunca pode executar direto.
func TestComandoSistemaSemClassificadorExigeConfirmacao(t *testing.T) {
	c := &ComandoSistema{}
	if !c.ConfirmacaoNecessaria(map[string]any{"comando": "ls"}) {
		t.Error("sem Classificador deveria exigir confirmação, não exigiu")
	}
}

func TestComandoSistemaUsaOClassificadorInjetado(t *testing.T) {
	var recebido string
	c := &ComandoSistema{Classificador: func(comando string) bool {
		recebido = comando
		return false
	}}
	if c.ConfirmacaoNecessaria(map[string]any{"comando": "qualquer coisa"}) {
		t.Error("classificador devolveu false, ConfirmacaoNecessaria devolveu true")
	}
	if recebido != "qualquer coisa" {
		t.Errorf("classificador recebeu %q, queria o comando cru", recebido)
	}
}

func TestComandoSistemaDescricaoNomeiaOShell(t *testing.T) {
	c := &ComandoSistema{NomeShell: "bash no Fedora Linux"}
	d := c.Declaracao()
	if !strings.Contains(d.Descricao, "bash no Fedora Linux") {
		t.Errorf("Descricao = %q, deveria nomear o shell", d.Descricao)
	}
	if !strings.Contains(d.Parametros["comando"].Descricao, "bash no Fedora Linux") {
		t.Errorf("descrição do parâmetro = %q, deveria nomear o shell", d.Parametros["comando"].Descricao)
	}
}
```

- [ ] **Passo 2: Rodar e ver falhar.** Esperado: `unknown field Classificador` e `unknown field NomeShell`.

```bash
go test ./internal/ferramentas/ 2>&1 | head -5
```

- [ ] **Passo 3: Implementar.** Em `terminal.go`, dentro de `ComandoSistema`:

```go
	// Classificador decide se o comando precisa de confirmação humana. É
	// injetado por plataforma (plataforma_*.go), como o Executor. Nil é fail
	// closed: tudo pede confirmação.
	Classificador func(comando string) bool

	// NomeShell aparece na descrição que o modelo lê. Sem ele o modelo não
	// sabe se escreve PowerShell ou bash.
	NomeShell string
```

O comentário do tipo deixa de dizer "executa PowerShell" e passa a dizer "executa comandos no shell da plataforma". `ConfirmacaoNecessaria` termina com:

```go
	if c.Classificador == nil {
		return true
	}
	return c.Classificador(comando)
```

E em `Declaracao`:

```go
		Descricao: fmt.Sprintf("Executa comandos %s. Se não souber o comando pra uma tarefa, use pesquisar_internet primeiro.", c.NomeShell),
		Parametros: map[string]Parametro{
			"comando": {Tipo: "string", Descricao: fmt.Sprintf("comando %s a executar", c.NomeShell)},
		},
```

`NovoComandoSistema` (ainda em `terminal.go` até a Task 3) passa a preencher `Classificador: ComandoPowerShellPrecisaConfirmacao, NomeShell: "PowerShell"`.

- [ ] **Passo 4: Rodar e ver passar** (com o mesmo filtro da Task 1). `GOOS=windows go vet ./internal/ferramentas/` sem saída.

- [ ] **Passo 5: Commit.**

```bash
git add internal/ferramentas/ && git commit -m "feat(ferramentas): classificador e nome do shell viram campos injetaveis do terminal"
```

---

## Task 3: Plataforma escolhe o terminal; no Linux ele fica desligado

**Arquivos:** `internal/ferramentas/terminal.go`, `terminal_powershell.go`, `plataforma_windows.go`, `plataforma_unix.go`, `terminal_test.go`, `terminal_powershell_windows_test.go`, `plataforma_windows_test.go`, `plataforma_unix_test.go`, `cmd/jamesd/ferramentas.go`, `ferramentas_test.go`, `main.go`

- [ ] **Passo 1: Testes.**

`internal/ferramentas/plataforma_unix_test.go`:

```go
//go:build unix

package ferramentas

import (
	"context"
	"strings"
	"testing"
)

// Até a denylist bash existir com testes (Tasks 4 e 6), o terminal no Linux
// não executa nada. Regra do Arthur: sem denylist, rm -rf ~ passaria sem
// aprovação.
func TestTerminalDesligadoNoUnix(t *testing.T) {
	if TerminalSuportado {
		t.Fatal("TerminalSuportado = true no unix antes da denylist bash")
	}
	c := NovoComandoSistema()
	_, err := c.Executar(context.Background(), map[string]any{"comando": "echo oi"})
	if err == nil || !strings.Contains(err.Error(), "desligado") {
		t.Errorf("Executar deveria recusar com 'desligado', veio %v", err)
	}
	if !c.ConfirmacaoNecessaria(map[string]any{"comando": "ls"}) {
		t.Error("terminal desligado deveria ser fail closed na confirmação")
	}
}
```

`internal/ferramentas/plataforma_windows_test.go`:

```go
package ferramentas

import "testing"

func TestTerminalNoWindowsUsaPowerShell(t *testing.T) {
	if !TerminalSuportado {
		t.Fatal("TerminalSuportado = false no Windows")
	}
	c := NovoComandoSistema()
	if c.NomeShell != "PowerShell" {
		t.Errorf("NomeShell = %q, queria PowerShell", c.NomeShell)
	}
	if !c.ConfirmacaoNecessaria(map[string]any{"comando": "Remove-Item x"}) {
		t.Error("Remove-Item deveria exigir confirmação no Windows")
	}
	if c.ConfirmacaoNecessaria(map[string]any{"comando": "Get-Process"}) {
		t.Error("Get-Process não deveria exigir confirmação no Windows")
	}
}
```

Em `cmd/jamesd/ferramentas_test.go`:

```go
func TestRegistrarTerminalNaoRegistraQuandoNaoSuportado(t *testing.T) {
	registro := ferramentas.NovoRegistro()
	if err := registrarTerminal(registro, &config.Config{}, false); err != nil {
		t.Fatalf("registrarTerminal devolveu erro: %v", err)
	}
	if _, ok := registro.Buscar("executar_comando_sistema"); ok {
		t.Error("terminal registrado numa plataforma sem suporte")
	}
}

func TestRegistrarTerminalRegistraQuandoSuportado(t *testing.T) {
	registro := ferramentas.NovoRegistro()
	if err := registrarTerminal(registro, &config.Config{}, true); err != nil {
		t.Fatalf("registrarTerminal devolveu erro: %v", err)
	}
	if _, ok := registro.Buscar("executar_comando_sistema"); !ok {
		t.Error("terminal não registrado numa plataforma com suporte")
	}
}
```

(acrescentar `"james/internal/ferramentas"` aos imports)

- [ ] **Passo 2: Rodar e ver falhar.** Esperado: `undefined: TerminalSuportado` e `undefined: registrarTerminal`.

```bash
go test ./internal/ferramentas/ ./cmd/jamesd/ 2>&1 | head -8
```

- [ ] **Passo 3: Mover o PowerShell.** Recortar de `terminal.go` para um arquivo novo, `terminal_powershell.go` (**sem** build tag): `codificarComando`, `prefixoUTF8`, `executarPowerShell` e os imports deles (`bytes`, `encoding/base64`, `os/exec`, `strings`, `unicode/utf16`, `errors`). Os comentários vão junto, sem mudança. Remover `NovoComandoSistema` de `terminal.go`.

- [ ] **Passo 4: Mover os 4 testes de PowerShell real.** Recortar `TestExecutarPowerShellDeVerdade*` (as quatro funções e seus comentários) de `terminal_test.go` para `terminal_powershell_windows_test.go`, com `package ferramentas` e os imports que usarem. O sufixo `_windows_test.go` já é a build tag. `TestCodificarComandoProduzUTF16LEBase64` e `TestPrefixoDeEncodingNaoVazaParaOClassificador` **ficam** em `terminal_test.go`, porque são lógica pura.

- [ ] **Passo 5: Plataformas.**

`plataforma_windows.go`:

```go
package ferramentas

// TerminalSuportado diz se executar_comando_sistema deve ser registrado
// nesta plataforma. Ver cmd/jamesd/ferramentas.go, registrarTerminal.
const TerminalSuportado = true

func NovoComandoSistema() *ComandoSistema {
	return &ComandoSistema{
		Executor:      executarPowerShell,
		Classificador: ComandoPowerShellPrecisaConfirmacao,
		NomeShell:     "PowerShell",
	}
}
```

`plataforma_unix.go`:

```go
//go:build unix

package ferramentas

import (
	"context"
	"errors"
)

// TerminalSuportado fica false até a denylist bash existir com testes
// (spec 2026-10-09-james-fedora, D3). O classificador da plataforma
// conhecia só PowerShell; portar o terminal pra bash sem a denylist deixaria
// comando destrutivo passar sem aprovação.
const TerminalSuportado = false

func NovoComandoSistema() *ComandoSistema {
	// Classificador nil: fail closed. Não deveria nem ser chamado, porque o
	// jamesd não registra a ferramenta, mas a ferramenta não depende disso.
	return &ComandoSistema{Executor: terminalDesligado, NomeShell: "bash"}
}

func terminalDesligado(context.Context, string) (ResultadoComando, error) {
	return ResultadoComando{}, errors.New("terminal desligado neste sistema: denylist linux ausente")
}
```

- [ ] **Passo 6: Registro condicional.** Em `cmd/jamesd/ferramentas.go`:

```go
// registrarTerminal registra executar_comando_sistema só onde a plataforma
// tem denylist própria (ferramentas.TerminalSuportado). Recebe o booleano
// por parâmetro pro teste cobrir os dois lados em qualquer sistema.
func registrarTerminal(registro ferramentas.Registro, cfg *config.Config, suportado bool) error {
	if !suportado {
		slog.Warn("terminal desligado neste sistema: denylist linux ausente")
		return nil
	}
	return registro.Registrar(construirComandoSistema(cfg))
}
```

(import `log/slog`). Em `main.go`, o bloco `registro.Registrar(construirComandoSistema(cfg))` vira:

```go
	if err := registrarTerminal(registro, cfg, ferramentas.TerminalSuportado); err != nil {
		return fmt.Errorf("registrar executar_comando_sistema: %w", err)
	}
```

- [ ] **Passo 7: Rodar e ver passar. Primeira vez com a suíte inteira verde no Linux.**

```bash
go vet ./... && GOOS=windows go vet ./... && go test -count=1 ./...
```

Esperado: todos os pacotes `ok`, inclusive `internal/ferramentas`.

- [ ] **Passo 8: Commit.**

```bash
git add internal/ferramentas/ cmd/jamesd/ && git commit -m "feat(ferramentas): terminal escolhido por plataforma e desligado no Linux ate a denylist bash"
```

---

## Task 4: Denylist bash

**Arquivos:** criar `internal/ferramentas/classificador_bash.go` e `classificador_bash_test.go`, ambos **sem** build tag.

- [ ] **Passo 1: Testes.** `classificador_bash_test.go`:

```go
package ferramentas

import "testing"

// TestComandoBashPrecisaConfirmacao espelha TestComandoPowerShellPrecisaConfirmacao:
// mesmas três famílias (apaga, abre a máquina, executa código de procedência
// desconhecida), mesmo modelo de ameaça (erro honesto do James, não
// adversário). Roda em qualquer sistema: a denylist é string pura.
func TestComandoBashPrecisaConfirmacao(t *testing.T) {
	casos := []struct {
		nome    string
		comando string
		querido bool
	}{
		// --- passa livre: o par de controle ---
		{"listar", "ls -la", false},
		{"ler arquivo", "cat arquivo.txt", false},
		{"buscar texto", "grep -r foo .", false},
		{"git", "git status", false},
		{"testes go", "go test ./...", false},
		{"criar pasta", "mkdir -p projeto/src", false},
		{"copiar", "cp origem.txt destino.txt", false},
		{"instalar dependência do projeto", "npm install", false},
		{"escrever arquivo", "echo 'ação' > notas.txt", false},
		{"pipeline de leitura", "ps aux | grep james", false},
		{"disco livre", "df -h", false},
		{"ip a é leitura e fica fora", "ip a", false},
		{"várias linhas de leitura", "date\npwd", false},

		// --- vazio ---
		{"vazio", "", true},
		{"só espaço", "   ", true},

		// --- apagar ---
		{"rm", "rm -rf pasta", true},
		{"rm por caminho absoluto", "/usr/bin/rm -rf pasta", true},
		{"rm escapando alias", "\\rm pasta", true},
		{"rm depois de outro comando", "ls; rm x", true},
		{"rm dentro de substituição", "echo $(rm x)", true},
		{"shred", "shred -u segredo.txt", true},
		{"truncate", "truncate -s 0 log.txt", true},

		// --- disco ---
		{"dd", "dd if=/dev/zero of=imagem.img bs=1M count=1", true},
		{"mkfs com sufixo", "mkfs.ext4 imagem.img", true},
		{"mkfs puro", "mkfs -t ext4 imagem.img", true},
		{"wipefs", "wipefs -a imagem.img", true},
		{"mount", "mount imagem.img /mnt", true},

		// --- contas e permissões ---
		{"sudo", "sudo ls", true},
		{"pkexec", "pkexec ls", true},
		{"chmod", "chmod -R 777 .", true},
		{"chown", "chown arthur:arthur x", true},
		{"useradd", "useradd backdoor", true},
		{"passwd", "passwd", true},

		// --- boot e segurança ---
		{"setenforce", "setenforce 0", true},
		{"grubby", "grubby --update-kernel=ALL --args=x", true},
		{"sysctl", "sysctl -w net.ipv4.ip_forward=1", true},

		// --- firewall e rede ---
		{"firewall-cmd", "firewall-cmd --list-all", true},
		{"nft", "nft flush ruleset", true},
		{"nmcli", "nmcli radio wifi off", true},

		// --- persistência e serviços ---
		{"systemctl até leitura", "systemctl status sshd", true},
		{"crontab", "crontab -e", true},
		{"loginctl", "loginctl enable-linger arthur", true},

		// --- energia e processos ---
		{"shutdown", "shutdown now", true},
		{"reboot", "reboot", true},
		{"kill", "kill -9 1234", true},
		{"pkill", "pkill firefox", true},

		// --- pacotes ---
		{"dnf", "dnf install htop", true},
		{"rpm", "rpm -e pacote", true},
		{"flatpak", "flatpak uninstall app", true},

		// --- baixar e executar ---
		{"curl sozinho", "curl https://exemplo.invalid", true},
		{"curl para sh", "curl https://exemplo.invalid/a.sh | sh", true},
		{"wget", "wget https://exemplo.invalid/a", true},
		{"eval", "eval \"$cmd\"", true},
		{"source", "source script.sh", true},
		{"bash -c", "bash -c 'echo oi'", true},
		{"xdg-open", "xdg-open arquivo.desktop", true},
		{"find -exec", "find . -name x -exec ls {} ;", true},

		// --- ofuscação ---
		{"crase", "echo `whoami`", true},
		{"base64", "echo aGVsbG8= | base64 -d", true},
		{"xxd reverso", "xxd -r -p dados.hex", true},
		{"ansi-c", "$'\\x72\\x6d' x", true},
		{"IFS", "cat${IFS}arquivo", true},
		{"escape hexa", "printf '\\x72\\x6d'", true},

		// --- caminhos sensíveis (leitura também, como o registro no Windows) ---
		{"/etc", "cat /etc/passwd", true},
		{"/boot", "ls /boot/", true},
		{"disco cru", "cat /dev/sda", true},
		{"nvme cru", "cat /dev/nvme0n1", true},
		{"ssh", "ls ~/.ssh/", true},
		{"bashrc", "echo x >> ~/.bashrc", true},
		{"profile", "echo x >> ~/.profile", true},
		{"autostart", "cp x.desktop ~/.config/autostart/", true},
		{"unit de usuário", "cp x.service ~/.config/systemd/user/", true},
		{"sudoers", "cat sudoers", true},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			if got := ComandoBashPrecisaConfirmacao(c.comando); got != c.querido {
				t.Errorf("ComandoBashPrecisaConfirmacao(%q) = %v, queria %v", c.comando, got, c.querido)
			}
		})
	}
}

// TestAsDuasDenylistsSaoIndependentes trava D2: a do bash não herda a do
// PowerShell, nem o contrário. Remove-Item é verbo no Windows, palavra
// solta no bash; systemctl é o inverso.
func TestAsDuasDenylistsSaoIndependentes(t *testing.T) {
	if ComandoBashPrecisaConfirmacao("Remove-Item x") {
		t.Error("bash não deveria conhecer Remove-Item")
	}
	if ComandoPowerShellPrecisaConfirmacao("systemctl status sshd") {
		t.Error("PowerShell não deveria conhecer systemctl")
	}
}
```

- [ ] **Passo 2: Rodar e ver falhar.** Esperado: `undefined: ComandoBashPrecisaConfirmacao`.

```bash
go test -run 'Bash|Independentes' ./internal/ferramentas/ 2>&1 | head -5
```

- [ ] **Passo 3: Implementar.** `classificador_bash.go`:

```go
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
```

- [ ] **Passo 4: Rodar e ver passar.** Se algum caso de controle falhar, **o caso não muda**: a lista é que está errada. Corrigir a lista e anotar no relato qual entrada mudou e por quê.

```bash
go test -count=1 -v -run 'Bash|Independentes' ./internal/ferramentas/ 2>&1 | grep -cE '^\s+--- PASS' ; go test -count=1 -run 'Bash|Independentes' ./internal/ferramentas/
```

Esperado: a contagem de subtestes PASS igual ao número de casos, e `ok`.

- [ ] **Passo 5:** a suíte inteira e `GOOS=windows go vet ./...`.

- [ ] **Passo 6: Commit.**

```bash
git add internal/ferramentas/ && git commit -m "feat(ferramentas): denylist bash espelhando as tres familias da versao PowerShell"
```

---

## Task 5: Executor bash

**Arquivos:** criar `internal/ferramentas/terminal_bash.go` e `terminal_bash_test.go`, ambos `//go:build unix`.

- [ ] **Passo 1: Testes.** `terminal_bash_test.go`:

```go
//go:build unix

package ferramentas

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Contraparte dos TestExecutarPowerShellDeVerdade*: bash real, sem mock.

func TestExecutarBashDeVerdadeComAcentoEAspas(t *testing.T) {
	res, err := executarBash(context.Background(), `printf '%s\n' "ação 'com' aspas"`)
	if err != nil {
		t.Fatalf("executarBash devolveu erro: %v", err)
	}
	if strings.TrimSpace(res.Stdout) != "ação 'com' aspas" {
		t.Errorf("stdout = %q, queria o texto com acento e aspas intactos", res.Stdout)
	}
}

func TestExecutarBashDeVerdadeCapturaExitCode(t *testing.T) {
	res, err := executarBash(context.Background(), "exit 3")
	if err != nil {
		t.Fatalf("executarBash devolveu erro: %v", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("exit code = %d, queria 3", res.ExitCode)
	}
}

func TestExecutarBashDeVerdadeForcaUTF8(t *testing.T) {
	res, err := executarBash(context.Background(), `printf '%s' "$LC_ALL"`)
	if err != nil {
		t.Fatalf("executarBash devolveu erro: %v", err)
	}
	if res.Stdout != "C.UTF-8" {
		t.Errorf("LC_ALL = %q, queria C.UTF-8", res.Stdout)
	}
}

// TestExecutarBashDeVerdadeTimeoutMataOGrupo prova duas coisas: que o
// Executar volta no timeout mesmo com um filho em segundo plano segurando o
// stdout, e que esse filho MORRE — e não fica órfão rodando.
func TestExecutarBashDeVerdadeTimeoutMataOGrupo(t *testing.T) {
	arquivoPID := filepath.Join(t.TempDir(), "pid")
	c := &ComandoSistema{Executor: executarBash, Timeout: 2 * time.Second, MaxBytesSaida: 1000}

	inicio := time.Now()
	_, err := c.Executar(context.Background(), map[string]any{
		"comando": "sleep 30 & echo $! > " + arquivoPID + "; wait",
	})
	decorrido := time.Since(inicio)

	if err == nil || !strings.Contains(err.Error(), "tempo limite") {
		t.Fatalf("esperava erro de tempo limite, veio %v", err)
	}
	if decorrido > 10*time.Second {
		t.Errorf("demorou %s — o processo não foi morto no timeout", decorrido)
	}

	bruto, err := os.ReadFile(arquivoPID)
	if err != nil {
		t.Fatalf("o filho não chegou a gravar o PID: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(bruto)))
	if err != nil {
		t.Fatalf("PID ilegível %q: %v", bruto, err)
	}
	// Dá um respiro pro kernel recolher o processo.
	time.Sleep(200 * time.Millisecond)
	if err := syscall.Kill(pid, 0); err == nil {
		_ = syscall.Kill(pid, syscall.SIGKILL)
		t.Errorf("o sleep em segundo plano (pid %d) sobreviveu ao timeout", pid)
	}
}
```

- [ ] **Passo 2: Rodar e ver falhar.** Esperado: `undefined: executarBash`.

```bash
go test -run ExecutarBash ./internal/ferramentas/ 2>&1 | head -5
```

- [ ] **Passo 3: Implementar.** `terminal_bash.go`:

```go
//go:build unix

package ferramentas

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// esperaPosCancelamento limita quanto o Wait espera os pipes fecharem depois
// do kill. Cinto e suspensório do kill de grupo abaixo.
const esperaPosCancelamento = 2 * time.Second

// executarBash roda o comando num bash limpo (sem profile nem rc, que mudam
// de máquina pra máquina e podem imprimir lixo no stdout).
//
// O UTF-8 é nativo, então não há o truque de codepage do PowerShell: só
// LC_ALL=C.UTF-8, pra saída de ferramenta não depender do locale da sessão.
//
// Timeout mata o GRUPO de processos. exec.CommandContext sozinho mata só o
// bash; um filho em segundo plano (`algo &`) herda o stdout, segura o pipe
// aberto e o Wait não volta. Setpgid põe o bash e todo filho num grupo
// próprio, e Cancel mata o grupo inteiro (pid negativo).
func executarBash(ctx context.Context, comando string) (ResultadoComando, error) {
	cmd := exec.CommandContext(ctx, "bash", "--noprofile", "--norc", "-c", comando)
	cmd.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
	cmd.WaitDelay = esperaPosCancelamento

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	res := ResultadoComando{Stdout: stdout.String(), Stderr: stderr.String()}

	// Mesmo contrato do executarPowerShell: exit code diferente de zero é
	// resultado, não erro de infraestrutura. Timeout é o Executar que
	// reconhece, olhando o ctx.
	var saiuComErro *exec.ExitError
	if errors.As(err, &saiuComErro) {
		res.ExitCode = saiuComErro.ExitCode()
		return res, nil
	}
	return res, err
}
```

- [ ] **Passo 4: Rodar e ver passar**, 3 vezes seguidas, porque o teste de timeout é o que pode oscilar:

```bash
go test -count=3 -v -run ExecutarBash ./internal/ferramentas/ 2>&1 | grep -E '^(---|ok|FAIL)'
```

- [ ] **Passo 5:** a suíte inteira e `GOOS=windows go vet ./...`. O arquivo é `unix`, então o vet do Windows nem o vê; é para garantir que nada mais quebrou.

- [ ] **Passo 6: Commit.**

```bash
git add internal/ferramentas/ && git commit -m "feat(ferramentas): executor bash com UTF-8 fixo e timeout que mata o grupo de processos"
```

---

## Task 6: Ligar o terminal no Linux

**Portão:** só começa com as Tasks 4 e 5 commitadas, e com esta saída verde **colada no relato**:

```bash
git log --oneline -6 && go test -count=1 -run 'Bash|Independentes|ExecutarBash' ./internal/ferramentas/
```

**Arquivos:** `internal/ferramentas/plataforma_unix.go`, `plataforma_unix_test.go`

- [ ] **Passo 1: Teste.** Substituir `TestTerminalDesligadoNoUnix` em `plataforma_unix_test.go` por:

```go
func TestTerminalNoUnixUsaBashComDenylist(t *testing.T) {
	if !TerminalSuportado {
		t.Fatal("TerminalSuportado = false no unix com a denylist bash pronta")
	}
	c := NovoComandoSistema()
	if !strings.Contains(c.NomeShell, "bash") {
		t.Errorf("NomeShell = %q, deveria nomear bash", c.NomeShell)
	}
	if !c.ConfirmacaoNecessaria(map[string]any{"comando": "rm -rf ~"}) {
		t.Error("rm -rf ~ precisa exigir confirmação no Linux")
	}
	if c.ConfirmacaoNecessaria(map[string]any{"comando": "ls -la"}) {
		t.Error("ls -la não deveria exigir confirmação no Linux")
	}
	saida, err := c.Executar(context.Background(), map[string]any{"comando": "echo oi"})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !strings.Contains(saida, "oi") {
		t.Errorf("saída = %q, queria conter oi", saida)
	}
}
```

- [ ] **Passo 2: Rodar e ver falhar.** Esperado: `TerminalSuportado = false no unix com a denylist bash pronta`.

- [ ] **Passo 3: Implementar.** `plataforma_unix.go` inteiro passa a ser:

```go
//go:build unix

package ferramentas

// TerminalSuportado: a denylist bash existe e tem testes
// (classificador_bash_test.go), condição da spec 2026-10-09 D3 pra ligar.
const TerminalSuportado = true

func NovoComandoSistema() *ComandoSistema {
	return &ComandoSistema{
		Executor:      executarBash,
		Classificador: ComandoBashPrecisaConfirmacao,
		NomeShell:     "bash no Fedora Linux",
	}
}
```

E em `cmd/jamesd/ferramentas.go`, a mensagem do `slog.Warn` passa a ser genérica: `"terminal desligado nesta plataforma: sem denylist própria"`.

- [ ] **Passo 4: Rodar a suíte inteira e o vet do Windows.** Tudo verde.

- [ ] **Passo 5: Commit.**

```bash
git add internal/ferramentas/ cmd/jamesd/ && git commit -m "feat(ferramentas): terminal ligado no Linux com bash e denylist propria"
```

---

## Task 7: `JAMES_CLAUDE_CODE_DIRETORIO`

**Arquivos:** `internal/config/config.go`, `config_test.go`, `.env.example`

- [ ] **Passo 1: Testes.** Em `config_test.go`:

```go
// O config.toml é versionado e compartilhado pelo Windows e pelo Fedora; o
// caminho do workspace não é. A env vence o toml (spec 2026-10-09 D6).
func TestCarregarEnvDoClaudeCodeVenceOToml(t *testing.T) {
	caminho := escreverToml(t, t.TempDir(), "[ferramentas.claude_code]\ndiretorio = \"C:\\\\no\\\\windows\"\n")
	t.Setenv("GEMINI_API_KEY", "chave")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JAMES_CLAUDE_CODE_DIRETORIO", "/home/arthur/workspace")

	cfg, err := Carregar(caminho)
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.ClaudeCodeDiretorio != "/home/arthur/workspace" {
		t.Errorf("ClaudeCodeDiretorio = %q, queria o da env", cfg.ClaudeCodeDiretorio)
	}
}

func TestCarregarEnvVaziaDoClaudeCodeNaoApagaOToml(t *testing.T) {
	caminho := escreverToml(t, t.TempDir(), "[ferramentas.claude_code]\ndiretorio = \"/do/toml\"\n")
	t.Setenv("GEMINI_API_KEY", "chave")
	t.Setenv("DATABASE_URL", "postgres://x")
	t.Setenv("JAMES_CLAUDE_CODE_DIRETORIO", "")

	cfg, err := Carregar(caminho)
	if err != nil {
		t.Fatalf("Carregar devolveu erro: %v", err)
	}
	if cfg.ClaudeCodeDiretorio != "/do/toml" {
		t.Errorf("ClaudeCodeDiretorio = %q, queria o do toml", cfg.ClaudeCodeDiretorio)
	}
}
```

- [ ] **Passo 2: Rodar e ver falhar.** O primeiro teste falha com `queria o da env`.

```bash
go test -run ClaudeCode ./internal/config/
```

- [ ] **Passo 3: Implementar.** Em `Carregar`, logo depois de montar `cfg`:

```go
	// O config.toml é versionado e serve Windows e Fedora; o caminho do
	// workspace do Claude Code é da máquina. Env não vazia vence.
	if d := os.Getenv("JAMES_CLAUDE_CODE_DIRETORIO"); d != "" {
		cfg.ClaudeCodeDiretorio = d
	}
```

Em `.env.example`, acrescentar (comentada, sem valor real):

```
# Opcional. Sobrescreve [ferramentas.claude_code].diretorio do config.toml
# nesta máquina (o toml é versionado e tem o caminho do Windows).
# JAMES_CLAUDE_CODE_DIRETORIO=/home/<usuario>/SI/Projetos/james-workspace
```

- [ ] **Passo 4: Rodar e ver passar.** Suíte inteira verde.

- [ ] **Passo 5: Commit.** **Não** adicionar o `.env`.

```bash
git add internal/config/ .env.example && git status --short && git commit -m "feat(config): JAMES_CLAUDE_CODE_DIRETORIO sobrescreve o diretorio do Claude Code por maquina"
```

---

## Task 8: `executar.sh`

Script de orquestração: não há teste unitário que valha o custo. A prova é `bash -n` aqui e a subida ao vivo na Task 13. `shellcheck` não está instalado; se o Arthur instalar, roda também.

**Arquivos:** criar `executar.sh`; modificar `.gitignore`

- [ ] **Passo 1: Escrever** `executar.sh`:

```bash
#!/usr/bin/env bash
# ============================================================
#  James - subida completa a partir do zero, no Linux.
#  Espelho do executar.bat; os comentarios de armadilha de la valem aqui.
#  Pre-requisito: daemon do Docker rodando e usuario no grupo docker.
#
#    1. pre-requisitos
#    2. Postgres (docker compose) e espera ficar saudavel
#    3. build da SPA -> precisa vir ANTES do jamesd (go:embed)
#    4. jamesd e james-device em segundo plano, logs em .executar/
#    5. shell Wails se der; senao o navegador. Ctrl+C encerra tudo.
# ============================================================
set -euo pipefail

# Sempre a partir da raiz do repo: o jamesd le config.toml, prompts/ e
# grava em audios/ por caminho relativo.
cd "$(dirname "$(readlink -f "$0")")"

LOGS=.executar
mkdir -p "$LOGS"
PIDS=()

encerrar() {
    for pid in "${PIDS[@]}"; do
        kill "$pid" 2>/dev/null || true
    done
}
trap encerrar EXIT
trap 'exit 130' INT TERM

falhar() {
    echo "  ERRO: $*" >&2
    echo
    echo "Subida abortada."
    exit 1
}

echo
echo "[1/5] Conferindo pre-requisitos..."
[ -f .env ] || falhar ".env nao encontrado na raiz. Copie .env.example para .env e preencha GEMINI_API_KEY, TAVILY_API_KEY, POSTGRES_PASSWORD e DATABASE_URL."
docker info >/dev/null 2>&1 || falhar "o Docker nao respondeu. Rode: sudo systemctl start docker"
echo "  ok"

echo
echo "[2/5] Subindo o Postgres..."
docker compose up -d || falhar "docker compose falhou."

# Esperar de verdade: num boot frio o Postgres leva segundos para aceitar
# conexao, e o jamesd morre no arranque se subir antes.
for tentativa in $(seq 1 60); do
    docker exec james-postgres pg_isready -U james -d james >/dev/null 2>&1 && break
    [ "$tentativa" -eq 60 ] && falhar "o Postgres nao ficou saudavel em 60s. Veja: docker logs james-postgres"
    sleep 1
done

# pg_isready roda DENTRO do container e nao prova que a 5434 chega nele.
# Ver o comentario longo no executar.bat: outro Postgres com a porta roubada
# vira "password authentication failed", que aponta para senha sem ser.
if ! docker ps --filter name=james-postgres --format '{{.Ports}}' | grep -q 5434; then
    echo "  Quem esta com a 5434 no momento:"
    docker ps --format '     {{.Names}}  ->  {{.Ports}}' | grep 5434 || true
    falhar "o james-postgres esta de pe, mas NAO publica a porta 5434. Libere a porta (docker stop <container>) ou troque em docker-compose.yml E no DATABASE_URL do .env."
fi
echo "  ok - james-postgres saudavel e publicando a 5434"

echo
echo "[3/5] Compilando a SPA e o backend..."
if [ ! -d web/node_modules ]; then
    echo "  node_modules ausente, instalando dependencias do front..."
    npm install --prefix web || falhar "npm install falhou."
fi
# ARMADILHA: web/embed.go usa //go:embed all:dist. Pular este build serve a
# interface antiga sem nenhum aviso.
npm run build --prefix web || falhar "o build da SPA falhou. O jamesd serviria a interface velha."
# Binario, e nao go run: go run deixa o processo real como neto, e o kill
# do trap so pegaria o go.
go build -o "$LOGS/jamesd" ./cmd/jamesd || falhar "o codigo Go nao compila."
echo "  ok"

echo
echo "[4/5] Subindo jamesd e james-device..."
"$LOGS/jamesd" >"$LOGS/jamesd.log" 2>&1 &
PIDS+=($!)
PID_JAMESD=$!

for tentativa in $(seq 1 60); do
    curl -s -m 2 http://127.0.0.1:8080/api/saude >/dev/null 2>&1 && break
    if ! kill -0 "$PID_JAMESD" 2>/dev/null; then
        tail -n 20 "$LOGS/jamesd.log" >&2
        falhar "o jamesd morreu no arranque. Log completo: $LOGS/jamesd.log"
    fi
    [ "$tentativa" -eq 60 ] && falhar "o jamesd nao respondeu em 60s. Veja $LOGS/jamesd.log"
    sleep 1
done
echo "  ok - jamesd de pe em http://127.0.0.1:8080 (log: $LOGS/jamesd.log)"

# cwd na raiz de proposito: o device le dispositivo/assets/ por caminho
# relativo. Sem microfone ele reclama no log e o resto segue pelo teclado.
PYTHON_DEVICE=dispositivo/.venv/bin/python
[ -x "$PYTHON_DEVICE" ] || PYTHON_DEVICE=python3
"$PYTHON_DEVICE" -m dispositivo.james_device.main >"$LOGS/device.log" 2>&1 &
PIDS+=($!)
echo "  ok - james-device em segundo plano (log: $LOGS/device.log)"

echo
echo "[5/5] Abrindo a interface (Ctrl+C encerra tudo)..."
if command -v wails >/dev/null 2>&1 && pkg-config --exists webkit2gtk-4.1; then
    (cd shell && wails dev -tags webkit2_41)
else
    echo "  shell Wails indisponivel (wails ou webkit2gtk4.1-devel ausente); abrindo no navegador."
    xdg-open http://127.0.0.1:8080 >/dev/null 2>&1 || true
    wait "$PID_JAMESD"
fi
```

- [ ] **Passo 2:** `chmod +x executar.sh` e, no `.gitignore`, acrescentar:

```
# Logs e binario do executar.sh
.executar/
# venv do device no Linux
dispositivo/.venv/
```

- [ ] **Passo 3: Verificar a sintaxe.**

```bash
bash -n executar.sh && echo sintaxe-ok && git check-ignore -v .executar/x dispositivo/.venv/x
```

- [ ] **Passo 4: Commit.**

```bash
git add executar.sh .gitignore && git commit -m "feat(ambiente): executar.sh sobe o James no Linux, espelho do executar.bat"
```

---

## Task 9: Device com dependências por plataforma e venv

**Arquivos:** `dispositivo/pyproject.toml`

- [ ] **Passo 1: Marcadores.** Em `dependencies`, trocar as três entradas por:

```toml
"pyautogui>=0.9.54; sys_platform == 'win32'", "playsound>=1.2.2,<1.3.0; sys_platform == 'win32'", "uiautomation>=2.0.18; sys_platform == 'win32'"
```

E acrescentar ao comentário do topo um parágrafo: no Linux, os sons saem pelo `pw-play` (`plataforma/linux.py`), e controle de tela é B2. Por isso essas três são só do Windows.

- [ ] **Passo 2: Venv** (precisa de `portaudio-devel`, Task 0).

```bash
python3 -m venv dispositivo/.venv && dispositivo/.venv/bin/pip install -q -e "dispositivo[dev]" && dispositivo/.venv/bin/pip list 2>/dev/null | grep -iE 'vosk|pyaudio|speechrec|pytest|pyautogui|playsound|uiautomation'
```

Esperado: `vosk`, `PyAudio`, `SpeechRecognition` e `pytest` presentes; `pyautogui`, `playsound` e `uiautomation` ausentes.

- [ ] **Passo 3: Linha de base.** Este é o "vermelho" das Tasks 10 a 12:

```bash
dispositivo/.venv/bin/python -m pytest -q dispositivo 2>&1 | tail -6
```

Esperado: 3 erros de coleta (`test_elementos`, `test_superficies`, `test_tela`), `WINFUNCTYPE`.

- [ ] **Passo 4: Commit.**

```bash
git add dispositivo/pyproject.toml && git commit -m "build(device): pyautogui, playsound e uiautomation so no Windows"
```

---

## Task 10: `superficies.py` só com o puro; Win32 em `plataforma/windows.py`

**Arquivos:** `dispositivo/james_device/superficies.py`, criar `james_device/plataforma/__init__.py` e `plataforma/windows.py`, `tela.py`, `captura.py`, criar `tests/test_plataforma.py`

- [ ] **Passo 1: Teste.** `tests/test_plataforma.py`:

```python
import subprocess
import sys
from pathlib import Path

RAIZ_DISPOSITIVO = Path(__file__).resolve().parents[1]


def _importa_limpo(modulo: str) -> subprocess.CompletedProcess:
    """Importa num processo novo: sys.modules deste processo já pode ter o
    módulo (ou o Win32) carregado por outro teste."""
    return subprocess.run(
        [sys.executable, "-c", f"import {modulo}"],
        cwd=RAIZ_DISPOSITIVO, capture_output=True, text=True,
    )


def test_superficies_importa_em_qualquer_sistema():
    r = _importa_limpo("james_device.superficies")
    assert r.returncode == 0, r.stderr
```

- [ ] **Passo 2: Rodar e ver falhar** (no Linux): `AttributeError ... WINFUNCTYPE` no stderr.

```bash
dispositivo/.venv/bin/python -m pytest -q dispositivo/tests/test_plataforma.py
```

- [ ] **Passo 3: Mover o Win32.**
  - Criar `plataforma/__init__.py` vazio por enquanto (só uma docstring; a seleção é a Task 11).
  - Criar `plataforma/windows.py` e **recortar** de `superficies.py`, sem mudar o código: `import ctypes`, `from ctypes import wintypes`, as constantes `_MONITORINFOF_PRIMARY`, `_MDT_EFFECTIVE_DPI`, `_DPI_PADRAO`, `_PROCESS_PER_MONITOR_DPI_AWARE`, `ativar_dpi_awareness` e a chamada de módulo `ativar_dpi_awareness()` com o comentário dela, `_RECT`, `_MONITORINFOEXW`, `_MONITORENUMPROC`, os `argtypes`/`restype`, `_escala_de`, `listar` (renomeada `listar_superficies`) e `retangulo_da_janela_em_foco`. Importar `Superficie` de `james_device.superficies`.
  - Acrescentar em `windows.py` o `handle_janela_em_foco()` que hoje é `tela._handle_janela_em_foco` (`tela.py:157-170`), com a docstring ajustada: agora o handle cru mora junto com o retângulo, e a duplicação acabou.
  - Em `superficies.py`, no lugar do que saiu, deixar as duas funções públicas como delegação tardia. Os nomes ficam iguais, assim `tela.py` e os `monkeypatch` de `test_tela.py` não mudam:

```python
def listar() -> list[Superficie]:
    """Monitores do sistema. Fala com o SO, que hoje só é o Windows; o Win32
    mora em plataforma/windows.py e só é importado aqui, sob demanda, pra
    este módulo (puro) importar em qualquer sistema."""
    from james_device.plataforma import windows

    return windows.listar_superficies()


def retangulo_da_janela_em_foco() -> tuple[int, int, int, int] | None:
    from james_device.plataforma import windows

    return windows.retangulo_da_janela_em_foco()
```

  - O docstring do módulo `superficies.py` passa a dizer que ele é puro e que o Win32 está em `plataforma/windows.py`.
  - `tela.py` e `captura.py`: o efeito de import que ativa o DPI awareness **antes** do `pyautogui` agora vem de `plataforma.windows`. Trocar a linha `from james_device import superficies` que fica antes do `import pyautogui` por:

```python
from james_device.plataforma import windows as _windows  # noqa: F401 — efeito de import: DPI awareness antes do pyautogui
from james_device import superficies
```

  Ajustar o comentário acima para dizer `plataforma/windows.py`.
  - `tela._handle_janela_em_foco` passa a ser `return _windows.handle_janela_em_foco()`.

- [ ] **Passo 4: Rodar e ver passar.** Agora `test_superficies.py` e `test_elementos.py` também rodam no Linux. Só `test_tela.py` continua com erro de coleta, porque o `pyautogui` não está instalado no Linux; a Task 12 resolve.

```bash
dispositivo/.venv/bin/python -m pytest -q dispositivo --ignore=dispositivo/tests/test_tela.py 2>&1 | tail -3
```

- [ ] **Passo 5: Commit.**

```bash
git add dispositivo/ && git commit -m "refactor(device): Win32 sai de superficies.py e vai para plataforma/windows.py, carregado sob demanda"
```

> **Risco Windows desta task:** a ordem DPI → `pyautogui` agora depende de `tela.py` e `captura.py` importarem `plataforma.windows` primeiro. O `test_tela.py` no Windows cobre o import; o DPI só se prova ao vivo. Isso entra no CA3 (Arthur, no Windows): `pytest` verde **e** um clique numa tela a 150%.

---

## Task 11: `plataforma`: som e ponte de tela por sistema

**Arquivos:** `plataforma/__init__.py`, criar `plataforma/linux.py`, `plataforma/windows.py`, `tests/test_plataforma.py`

- [ ] **Passo 1: Testes.** Acrescentar a `tests/test_plataforma.py`:

```python
from james_device import plataforma
from james_device.plataforma import linux


def test_escolhe_o_modulo_pelo_sistema():
    assert plataforma.nome_do_modulo("win32") == "james_device.plataforma.windows"
    assert plataforma.nome_do_modulo("linux") == "james_device.plataforma.linux"


def test_sistema_desconhecido_cai_no_linux():
    # Mac e afins: sem Win32, o que sobra de portável é o caminho Linux
    # (tela desligada). Falhar no import seria pior que voz sem tela.
    assert plataforma.nome_do_modulo("darwin") == "james_device.plataforma.linux"


def test_linux_toca_com_pw_play(monkeypatch):
    chamadas = []
    monkeypatch.setattr(linux.subprocess, "run", lambda args, **kw: chamadas.append(args))
    linux.tocar_som("dispositivo/assets/entrada.mp3")
    assert chamadas == [["pw-play", "dispositivo/assets/entrada.mp3"]]


def test_linux_sem_pw_play_nao_levanta(monkeypatch, caplog):
    def sem_binario(args, **kw):
        raise FileNotFoundError("pw-play")

    monkeypatch.setattr(linux.subprocess, "run", sem_binario)
    linux.tocar_som("x.wav")  # não pode levantar: som é enfeite, não pode derrubar a escuta
    assert "pw-play" in caplog.text


def test_linux_ponte_de_tela_recusa_sem_levantar():
    ponte = linux.ponte_de_tela()
    resposta = ponte({"acao": "click", "x": 1, "y": 1})
    assert set(resposta) == {"erro"}
    assert "indisponível" in resposta["erro"]


def test_linux_declara_tela_indisponivel():
    assert linux.TELA_DISPONIVEL is False
```

- [ ] **Passo 2: Rodar e ver falhar.** Esperado: `ImportError` (`linux` e `nome_do_modulo` não existem).

- [ ] **Passo 3: Implementar.**

`plataforma/linux.py`:

```python
"""Linux (Fedora, KDE, Wayland): voz sim, tela não.

Controle de tela no Wayland é a fatia B2, com spec própria. Até lá, a ponte
de tela responde com erro no mesmo formato de tela.executar_acao, e o jamesd
repassa isso pro modelo como faz quando o device cai.
"""

import logging
import subprocess

logger = logging.getLogger(__name__)

TELA_DISPONIVEL = False
MENSAGEM_TELA_INDISPONIVEL = (
    "tela indisponível neste sistema (Linux): controle de tela ainda não portado"
)


def tocar_som(caminho: str) -> None:
    """Bloqueia até o som acabar; quem chama já roda numa thread
    (main.tocar_em_paralelo). pw-play lê via libsndfile, que toca WAV e MP3."""
    try:
        subprocess.run(["pw-play", caminho], check=False)
    except FileNotFoundError:
        logger.warning("pw-play não encontrado, som %s não tocou (pipewire-utils instalado?)", caminho)


def ponte_de_tela():
    def recusar(dados: dict) -> dict:
        return {"erro": MENSAGEM_TELA_INDISPONIVEL}

    return recusar
```

Em `plataforma/windows.py`, acrescentar no fim:

```python
TELA_DISPONIVEL = True


def tocar_som(caminho: str) -> None:
    from playsound import playsound

    playsound(caminho)


def ponte_de_tela():
    # Import tardio: tela importa pyautogui, que só existe no Windows.
    from james_device.tela import executar_acao

    return executar_acao
```

`plataforma/__init__.py`:

```python
"""Ponto único de escolha de plataforma do device.

Cada sistema tem um módulo com as mesmas funções (tocar_som, ponte_de_tela,
TELA_DISPONIVEL). Só o módulo do sistema atual é importado: windows.py roda
Win32 no import e derrubaria o device no Linux.
"""

import importlib
import sys


def nome_do_modulo(sistema: str) -> str:
    if sistema == "win32":
        return "james_device.plataforma.windows"
    return "james_device.plataforma.linux"


atual = importlib.import_module(nome_do_modulo(sys.platform))
```

- [ ] **Passo 4: Rodar e ver passar** (com o mesmo `--ignore` da Task 10).

- [ ] **Passo 5: Commit.**

```bash
git add dispositivo/ && git commit -m "feat(device): plataforma escolhe som e ponte de tela, Linux toca com pw-play e recusa tela"
```

---

## Task 12: `main.py` usa a plataforma; `test_tela.py` vira skip fora do Windows

**Arquivos:** `james_device/main.py`, `tests/test_plataforma.py`, `tests/test_tela.py`

- [ ] **Passo 1: Testes.** Acrescentar a `tests/test_plataforma.py`:

```python
def test_main_importa_sem_puxar_tela_nem_playsound():
    """main não pode depender de tela/pyautogui/playsound no import: no Linux
    eles não existem, e o device inteiro (voz inclusa) morria antes do laço."""
    r = subprocess.run(
        [sys.executable, "-c",
         "import sys, james_device.main; "
         "print(sorted(m for m in ('james_device.tela', 'playsound', 'pyautogui') if m in sys.modules))"],
        cwd=RAIZ_DISPOSITIVO, capture_output=True, text=True,
    )
    assert r.returncode == 0, r.stderr
    assert r.stdout.strip() == "[]"
```

E no topo de `tests/test_tela.py`, **antes** de qualquer import de `james_device`:

```python
import sys

import pytest

if sys.platform != "win32":
    # tela.py é Windows (pyautogui + UI Automation). No Linux, controle de
    # tela é a fatia B2. Skip visível, não sumiço: o relatório mostra que
    # estes testes existem e não rodaram.
    pytest.skip("controle de tela só no Windows até a fatia B2", allow_module_level=True)
```

- [ ] **Passo 2: Rodar e ver falhar.** O teste de `main` falha com `ModuleNotFoundError: No module named 'playsound'`.

```bash
dispositivo/.venv/bin/python -m pytest -q dispositivo 2>&1 | tail -6
```

- [ ] **Passo 3: Implementar.** Em `main.py`:
  - Remover `from playsound import playsound` e `from james_device.tela import executar_acao`.
  - Acrescentar `from james_device.plataforma import atual as plataforma`.
  - `tocar_em_paralelo` passa a usar `target=plataforma.tocar_som`.
  - Em `main()`:

```python
def main():
    if not plataforma.TELA_DISPONIVEL:
        logger.warning("ponte de tela desligada neste sistema (%s); voz segue normal", sys.platform)
    conexao = ConexaoJames(URL_JAMESD, manipulador_acao=plataforma.ponte_de_tela())
```

  (com `import sys`). O comentário do topo de `main.py` que fala de dependências ganha uma linha apontando para `plataforma/`.

- [ ] **Passo 4: Rodar e ver passar. Suíte inteira do device verde no Linux, `test_tela.py` como skip.**

```bash
dispositivo/.venv/bin/python -m pytest -q -rs dispositivo 2>&1 | tail -4
```

Esperado: `N passed, 1 skipped`, com o motivo do skip listado.

- [ ] **Passo 5: Commit.**

```bash
git add dispositivo/ && git commit -m "feat(device): main usa a plataforma e sobe no Linux so com voz"
```

---

## Task 13: Verificação ao vivo e critérios de aceite

Sem código novo. Cada CA com a prova colada no relato. O que falhar vira task de correção **antes** do merge.

- [ ] **CA1, CA2:** `go vet ./... && GOOS=windows go vet ./... && GOOS=windows go build ./... && go test -count=1 ./...`
- [ ] **CA4:** `go test -count=1 -v -run 'Bash|Independentes|ExecutarBash' ./internal/ferramentas/ 2>&1 | tail -5`
- [ ] **CA5:** o teste da Task 3 (`TestRegistrarTerminalNaoRegistraQuandoNaoSuportado`) passando. O log de boot só existe em plataforma sem suporte; no Fedora, depois da Task 6, conferir que `executar_comando_sistema` **aparece** e pede confirmação para `rm` numa conversa real.
- [ ] **CA6:**
  - [ ] `./executar.sh` do zero (`docker compose down` antes, **sem** `-v`).
  - [ ] No navegador: mandar "que dia é hoje?" e receber resposta.
  - [ ] `docker exec james-postgres psql -U james -d james -c "select papel, left(conteudo,60) from mensagens order by id desc limit 2"` mostra o par.
  - [ ] Um `.mp3` novo em `audios/`, e ele toca.
- [ ] **CA7:** `dispositivo/.venv/bin/python -m pytest -q -rs dispositivo`.
- [ ] **CA8:** com o `executar.sh` de pé: "James" → som de entrada → "que horas são" → som de fechamento → resposta falada. Trecho de `.executar/device.log` com `comando entendido`.
- [ ] **CA9:** "James, clica no botão iniciar". O James diz que a tela está indisponível, e `.executar/device.log` continua recebendo comandos depois disso.
- [ ] **`pw-play` com MP3** (spec §9): `pw-play dispositivo/assets/entrada.mp3; echo $?` toca e devolve `0`.
- [ ] **CA3 (Arthur, no Windows):** `go test ./...`, `pytest` do device, `executar.bat` subindo e um clique por voz numa tela escalada (o risco da Task 10).
- [ ] Atualizar a memória do projeto e o HANDOFF com o estado e com o que ficou para B2 e Parte A.

---

## Task 14 (condicional): shell Wails no Linux

Só depois da Task 0 (os `-devel` instalados). Responde B-d de vez.

- [ ] `cd shell && wails build -tags webkit2_41`. Se passar: abrir o binário e conferir que a janela carrega `http://127.0.0.1:8080`. O `executar.sh` já usa `wails dev` quando o webkit existe.
- [ ] Se o `internal error: package "context" without types` continuar com os `-devel` instalados, é o CLI do Wails contra o Go 1.27 (spec §9). Registrar a saída e propor ao Arthur: fixar o toolchain para o `shell` ou compilar com `go build -tags desktop,production,webkit2_41`. **Não** decidir sozinho.

---

## Fora deste plano

- Corrida de migração no banco virgem (spec §9): task separada, advisory lock em `migrar`.
- `stop-process`/`spps` na denylist do Windows (spec §4.3): mudança separada.
- `shell/shell.exe` versionado.
- B2 (tela no Wayland) e Parte A (STT): a Parte A começa em `feat/stt-whisper` **depois** do merge desta branch em `main`.
