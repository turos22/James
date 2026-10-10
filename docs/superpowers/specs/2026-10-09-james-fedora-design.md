# James — porte para o Fedora, fatias B0 e B1

**Data:** 2026-10-09
**Autor:** Arthur Orosco (design colaborativo)
**Estado:** aprovado pelo Arthur em 2026-10-09
**Projeto:** `/home/arthurorosco/SI/Projetos/Athena(Go)/meu-app-ts`
**Base:** [`plans/2026-10-09-james-analise-stt-e-fedora.md`](../plans/2026-10-09-james-analise-stt-e-fedora.md), Parte B e seção "Conferência" (C1 a C13). Modelo de permissão: [`decisoes/2026-08-11-permissao-denylist-e-origem.md`](../decisoes/2026-08-11-permissao-denylist-e-origem.md).

Este documento cobre B0 (backend no Fedora) e B1 (device só com voz). B2 (tela no Wayland) tem spec própria depois dos spikes.

---

## 1. Respostas da análise (B-a a B-d)

Todas levantadas no Fedora 44 (KDE Plasma, Wayland), commit `7a0d4fc`, Go 1.27.1, Python 3.13.16.

### B-a. `go test ./...` no Linux

Sem banco: 3 pacotes falham (`internal/api`, `internal/armazenamento/postgres`, `internal/ferramentas`). Os dois primeiros só por `connection refused` na 5434.

Com o Postgres de pé (`docker compose up -d`), `go test -count=1 ./...` em 3 rodadas seguidas:

```
FAIL	james/internal/ferramentas
--- FAIL: TestExecutarPowerShellDeVerdadeComAcentoEAspas
--- FAIL: TestExecutarPowerShellDeVerdadeRespeitaOTimeout
--- FAIL: TestExecutarPowerShellDeVerdadeCapturaExitCode
--- FAIL: TestExecutarPowerShellDeVerdadeNaoDeixaBOM
    exec: "powershell": executable file not found in $PATH
```

Todos os outros pacotes passam, incluindo `james/shell` (sem a tag `desktop` ele não precisa de cgo).

Achado à parte, que não é de Linux: na **primeira** rodada contra o banco virgem, `internal/api` falhou 2 testes com `duplicate key value violates unique constraint "pg_type_typname_nsp_index"`. São dois pacotes de teste aplicando as migrações ao mesmo tempo. Fica registrado em §9, fora do escopo.

O device (Python), num venv descartável sem PyAudio: `test_elementos.py`, `test_superficies.py` e `test_tela.py` **nem são coletados** (`AttributeError: module 'ctypes' has no attribute 'WINFUNCTYPE'`, em `superficies.py:102`). Os outros 63 testes passam.

### B-b. Docker ou Podman

**Docker.** `docker` 29.8.2 (moby-engine) com o plugin compose v5.6.0, daemon ativo, usuário no grupo `docker`. O `docker compose up -d` do repo subiu o `james-postgres` publicando `127.0.0.1:5434->5432` sem nenhuma mudança no `docker-compose.yml`. O `podman` 5.8.7 também está instalado, mas `podman compose` delega para o mesmo plugin do Docker. Não há motivo para suportar os dois no launcher.

### B-c. Denylist Linux

Respondida em §4.3.

### B-d. Wails v2.11 com `webkit2gtk-4.1`

**Ainda sem resposta. Está bloqueada por pacote de sistema.**

- `wails` CLI v2.11.0 foi instalado com `go install` (não havia nenhum).
- `go build -tags desktop,production,webkit2_41 ./shell` falha no cgo com `Package 'webkit2gtk-4.1' not found` e `Package 'gtk+-3.0' not found`. Os runtimes `webkit2gtk4.1` e `gtk3` estão instalados; os `-devel` não estão.
- `wails build -tags webkit2_41` falha antes, com `internal error: package "context" without types was imported from "james/shell"`. Provavelmente é o mesmo erro de cgo mascarado pelo carregador de pacotes do CLI, mas **não verifiquei**.

Para fechar B-d: `sudo dnf install webkit2gtk4.1-devel gtk3-devel` e rodar de novo. O B0 **não depende** disso (§3, D4).

---

## 2. Escopo

### 2.1 Entra

- jamesd, SPA e Postgres rodando no Fedora, conversa por texto e TTS.
- `go test ./...` verde no Linux **e** no Windows.
- Ferramenta de terminal no Linux: primeiro desligada; depois `bash` com denylist Linux testada.
- `executar.sh` equivalente ao `executar.bat`.
- Device no Linux: wake word (Vosk), comando de voz (Google, que é o atual), sons, conexão com o jamesd. A ponte de tela responde "indisponível".

### 2.2 Não entra

- Tela no Wayland (B2).
- Troca de STT (Parte A, branch própria depois deste merge).
- Corrigir a corrida de migração (§9).
- Empacotar o James (RPM, Flatpak, systemd unit).

---

## 3. Decisões

| # | Decisão | Motivo |
|---|---|---|
| D1 | Uma base de código. Cada ponto de plataforma vira interface com duas implementações: build tags no Go, `james_device/plataforma/` no Python. | Regra do Arthur. O Windows não pode regredir. |
| D2 | **Os dois classificadores compilam em todas as plataformas.** Só a escolha de qual usar tem build tag. | O classificador é lógica pura de string. Assim, `go test` no Linux continua testando a denylist do Windows, e vice-versa. Uma denylist que só é testada na plataforma dela apodrece sem ninguém ver. |
| D3 | No Linux, o terminal **não é registrado** até a denylist Linux existir com testes. A ordem das tarefas do plano garante isso: a tarefa que registra o executor `bash` depende da tarefa da denylist. | Regra do Arthur. Sem denylist, `rm -rf ~` passaria sem aprovação. |
| D4 | O aceite do B0 é com a SPA no navegador (`http://127.0.0.1:8080`). O shell Wails no Linux é tarefa separada, condicionada a B-d. | B-d está bloqueada por pacote de sistema e não deve travar o resto. |
| D5 | Docker, não Podman (B-b). | É o que está instalado e funcionando. |
| D6 | `JAMES_CLAUDE_CODE_DIRETORIO` (env) sobrescreve `[ferramentas.claude_code].diretorio`. O `config.toml` versionado continua com o caminho do Windows. | C4. O arquivo é versionado e compartilhado pelas duas máquinas. Override por env é o padrão que o projeto já usa para segredos. |
| D7 | `superficies.py` é dividido: o que é puro (dataclass `Superficie` e conversões, linhas 184 em diante) fica; o Win32 (linhas 40–182) vai para `plataforma/windows.py`, carregado sob demanda. | C2. É o que impede o device de subir no Linux. De quebra, `test_superficies.py` e `test_elementos.py` voltam a rodar no Linux. |
| D8 | Sons no Linux via `pw-play` em subprocess. `playsound` e `uiautomation` passam a ter marcador `sys_platform == "win32"` no `pyproject.toml`. | `pw-play` está instalado e `libsndfile` 1.2.2 suporta MP3 desde a 1.1.0. **Não verifiquei tocando**: a tarefa do plano verifica. |

---

## 4. Go (jamesd)

### 4.1 Terminal

| Arquivo | Conteúdo |
|---|---|
| `terminal.go` (sem tag) | `ComandoSistema`, truncagem, `ConfirmacaoNecessaria`, tudo o que não depende de shell |
| `terminal_windows.go` | `executarPowerShell`, `codificarComando`, `prefixoUTF8`, descrição "PowerShell" para o modelo, `classificar = ComandoPowerShellPrecisaConfirmacao` |
| `terminal_linux.go` | `executarBash`, descrição "bash no Fedora Linux", `classificar = ComandoBashPrecisaConfirmacao` |
| `terminal_windows_test.go` | os 4 `TestExecutarPowerShellDeVerdade*` (C8) |
| `terminal_linux_test.go` | os mesmos 4 cenários contra `bash`: acento e aspas, timeout, exit code, saída sem lixo |

O executor `bash`:
- `bash --noprofile --norc -c <comando>`, com `LC_ALL=C.UTF-8` no ambiente. O UTF-8 é nativo, então não precisa do truque do codepage.
- Timeout que **mata o grupo de processos**: `SysProcAttr.Setpgid = true`, `cmd.Cancel` envia `SIGKILL` para `-pgid`, e `cmd.WaitDelay` é limitado. Sem isso, um filho em segundo plano segura o pipe de stdout e o `Wait` não volta, o que faz o teste de timeout passar no Windows e travar no Linux.

O `cmd/jamesd/main.go` registra o terminal só se `ferramentas.TerminalSuportado` for `true`. Essa constante tem build tag e fica `false` no Linux até a tarefa da denylist (D3). Quando o terminal não é registrado, o jamesd loga no boot:

```
terminal desligado neste sistema: denylist linux ausente
```

### 4.2 Classificador

`classificador.go` é dividido em núcleo e tabelas, **todos sem build tag** (D2):

- **Núcleo:** fail closed para vazio, marcas de ofuscação por substring, caminhos sensíveis por substring, tokenização, verbo por mapa e por prefixo. Recebe uma `denylist` (struct com as quatro listas e os separadores).
- **`ComandoPowerShellPrecisaConfirmacao`:** as tabelas atuais, sem mudar uma entrada. O `classificador_test.go` atual passa a testar esta função e continua cobrindo tudo o que cobre hoje.
- **`ComandoBashPrecisaConfirmacao`:** a tabela de §4.3, com uma diferença de tokenização: o verbo é comparado pelo **basename** (`/usr/bin/rm` → `rm`). Isso fecha o C6. Para os testes não se chocarem com o nome antigo, `ComandoPrecisaConfirmacao` deixa de existir; quem a chama passa a chamar a do terminal da plataforma.

### 4.3 Denylist Linux (resposta de B-c)

Mesmo modelo de ameaça da versão Windows, nas palavras dela: **erro honesto do James, não um adversário**. Mesmas três famílias: o que **apaga**, o que **abre a máquina** e o que **executa código de procedência desconhecida**.

| Família (como no Windows) | Verbos (token, comparado por basename) |
|---|---|
| Apagar e truncar | `rm` `rmdir` `shred` `unlink` `truncate` `srm` |
| Disco e partição | `dd` `fdisk` `sfdisk` `gdisk` `parted` `wipefs` `mkswap` `swapoff` `mount` `umount` `cryptsetup` `lvremove` `vgremove` `pvremove`; prefixo `mkfs` |
| Contas, permissões, posse | `sudo` `su` `doas` `pkexec` `run0` `chmod` `chown` `chgrp` `setfacl` `chattr` `useradd` `userdel` `usermod` `groupadd` `groupdel` `gpasswd` `passwd` `chpasswd` `visudo` |
| "Registro e boot" | `grubby` `grub2-mkconfig` `grub2-install` `efibootmgr` `sysctl` `setenforce` `semanage` `bootctl` |
| Firewall e rede | `firewall-cmd` `iptables` `ip6tables` `nft` `ufw` `nmcli` |
| Persistência e serviços | `systemctl` `service` `crontab` `at` `loginctl` |
| Energia e sessão | `shutdown` `reboot` `poweroff` `halt` `kill` `pkill` `killall` |
| Pacotes (análogo ao `msiexec`) | `dnf` `dnf5` `yum` `rpm` `flatpak` |
| Baixar e executar | `curl` `wget` (C7: o verbo, como no Windows) `eval` `source` `exec` `xdg-open` `gio` `bash` `sh` `zsh` `dash` |

**Marcas de ofuscação (substring):** `` ` `` (no bash é substituição de comando), `base64`, `xxd -r`, `$'\`, `${ifs}`, `\x`.

**Caminhos sensíveis (substring):** `/etc/`, `/boot/`, `/dev/sd`, `/dev/nvme`, `/sys/`, `/proc/sys/`, `.ssh/`, `.bashrc`, `.bash_profile`, `.profile`, `.zshrc`, `/autostart`, `/systemd/`, `/var/spool/cron`, `sudoers`. Leitura também reprova, com o mesmo argumento que o Windows usa para o registro: separar leitura de escrita exigiria analisar o verbo, e o saldo é ruim.

**Prefixos de família:** `mkfs` (`mkfs.ext4`, `mkfs.btrfs`...).

**Separadores:** os do Windows mais `` ` `` e `$`.

Divergências deliberadas em relação ao Windows, registradas para não parecerem esquecimento:

- `systemctl` pede confirmação **sempre**, inclusive `systemctl status`. No Windows `Get-Service` passa. Separar exigiria analisar o subcomando. É o mesmo argumento do registro.
- `kill`, `pkill` e `killall` entram; o `Stop-Process` do Windows não está na lista. Matar um editor aberto perde trabalho. Proponho incluir também `stop-process`/`spps` no Windows, mas em mudança separada (não muda a política do Windows dentro do porte).
- `ip` **não** entra: `ip a` é leitura corriqueira, e o equivalente destrutivo (`ip link set ... down`) é raro no uso do James.
- Redirecionamento (`> arquivo`) não reprova. O Windows também não reprova `Out-File`.

**Limite honesto,** escrito no arquivo como na versão Windows: `r''m`, `$(printf rm)` e `IFS` montado escapam. A lista fecha a evasão acidental, não a deliberada.

Os testes seguem o formato de tabela de `classificador_test.go`: para cada família, um caso que reprova, um que passa e um com caminho absoluto (`/usr/bin/rm -rf x`). Para cada marca e cada caminho, um caso. E o par de controle que motivou a denylist: `ls -la`, `cat arquivo.txt`, `grep -r foo .`, `git status` e `go test ./...` **passam**.

### 4.4 Configuração

`internal/config`: `JAMES_CLAUDE_CODE_DIRETORIO`, quando definida e não vazia, vence o toml (D6). Teste: env vence, env vazia não vence.

---

## 5. Python (device)

### 5.1 `james_device/plataforma/`

```
plataforma/__init__.py   escolhe por sys.platform; exporta as funções abaixo
plataforma/windows.py    Win32 que hoje vive em superficies.py (D7) + playsound + ponte de tela real
plataforma/linux.py      pw-play + ponte de tela "indisponível"
```

| Função | Windows | Linux |
|---|---|---|
| `tocar_som(caminho)` | `playsound` (como hoje) | `subprocess.Popen(["pw-play", caminho])`, sem esperar |
| `ponte_de_tela()` → `Callable[[dict], dict]` | `tela.executar_acao`, com import tardio | devolve `{"erro": "tela indisponível neste sistema (Linux): controle de tela ainda não portado"}` |
| `listar_superficies()`, `retangulo_da_janela_em_foco()`, `handle_janela_em_foco()` | Win32 (movido de `superficies.py` e `tela.py:170`) | não existem no B1 (B2) |

O `main.py` deixa de importar `tela` e `playsound` no topo e passa a chamar `plataforma.ponte_de_tela()` e `plataforma.tocar_som()`. A forma `{"erro": ...}` é o contrato que `tela.executar_acao` já usa (`tela.py:57-88`), então o jamesd não muda nada.

Log no boot do device no Linux:

```
ponte de tela desligada neste sistema (linux); voz segue normal
```

### 5.2 Testes

- `test_superficies.py` e `test_elementos.py`: depois do D7, rodam no Linux sem mudança.
- `test_tela.py`: `pytest.skip(..., allow_module_level=True)` fora do `win32`, **antes** dos imports, para aparecer como skip e não sumir.
- Novos: `test_plataforma.py`. A escolha por `sys.platform` (monkeypatch), e no Linux: o `pw-play` é chamado com o caminho (subprocess falso) e a ponte devolve `{"erro": ...}` sem levantar.

### 5.3 `pyproject.toml`

- `uiautomation`, `playsound` e `pyautogui` com `; sys_platform == "win32"`.
- `PyAudio` continua em todas; no Fedora ela exige `portaudio-devel` para compilar.

---

## 6. `executar.sh`

Espelho do `executar.bat`, nos mesmos 5 passos e com as mesmas armadilhas comentadas:

1. Pré-requisitos: `.env` existe; `docker info` responde.
2. `docker compose up -d`; espera `pg_isready` (60 s); confere se o `james-postgres` publica a 5434 (a mesma checagem de porta roubada do `.bat`).
3. `npm install` se faltar `web/node_modules`; `npm run build --prefix web`; `go build ./...`.
4. `jamesd` e device em segundo plano, com logs em `.executar/jamesd.log` e `.executar/device.log` (pasta no `.gitignore`); espera `/api/saude`. Um `trap` em `EXIT`/`INT` derruba os dois.
5. Se `wails` existir **e** `pkg-config --exists webkit2gtk-4.1`, roda `wails dev -tags webkit2_41` em `shell/`. Senão, `xdg-open http://127.0.0.1:8080` e espera Ctrl+C.

O `executar.bat` não muda.

---

## 7. Dependências do Fedora

| Pacote | Para quê | Situação | Quem instala |
|---|---|---|---|
| `portaudio-devel` | compilar `PyAudio` (B1) | ausente | Arthur (`sudo dnf`) |
| `webkit2gtk4.1-devel`, `gtk3-devel` | shell Wails (B-d) | ausentes | Arthur (`sudo dnf`) |
| `edge-tts` | TTS do jamesd (C3) | ausente | `pipx install edge-tts` |
| `wails` v2.11.0 | shell | instalado nesta sessão | — |
| docker + compose, `pw-play` | Postgres, sons | presentes | — |

O venv do device fica em `dispositivo/.venv`, fora do git, e é criado pelo Arthur ou por tarefa do plano.

---

## 8. Critérios de aceite

| # | Critério | Prova |
|---|---|---|
| CA1 | `go test -count=1 ./...` verde no Fedora, com o Postgres de pé | saída do comando |
| CA2 | `go vet ./...` e `GOOS=windows go build ./...` no Fedora | saída; garante que o lado Windows compila |
| CA3 | `go test ./...` verde no Windows | rodado pelo Arthur na máquina Windows, saída colada (não tenho como rodar) |
| CA4 | Denylist Linux: todos os casos de §4.3 testados, inclusive os de controle que passam | `go test -run Bash ./internal/ferramentas -v` |
| CA5 | Antes da denylist, o terminal não aparece na lista de ferramentas no Linux | teste em `cmd/jamesd` + log de boot |
| CA6 | `./executar.sh` sobe tudo do zero; conversa por texto no navegador; mensagem gravada no Postgres; áudio TTS gerado e tocado | sessão ao vivo, query no `james-postgres`, arquivo em `audios/` |
| CA7 | `pytest` do device no Fedora: verde, com `test_tela.py` como skip | saída do comando |
| CA8 | Device no Fedora: wake word → som de entrada → comando → som de fechamento → resposta falada | sessão ao vivo, log do device |
| CA9 | Pedido de tela por voz no Fedora: o James responde que a tela está indisponível, sem derrubar o device | sessão ao vivo |

---

## 9. Riscos e achados fora do escopo

- **Corrida de migração (C9).** Dois pacotes de teste migram o mesmo banco virgem em paralelo. Workaround: `go test -p 1` na primeira vez. Correção: advisory lock no `migrar`. Proponho como tarefa separada.
- **`shell/shell.exe` versionado (C10).** Binário no git. Fora deste escopo.
- **Wails CLI com Go 1.27.** O `internal error` do `wails build` pode ser incompatibilidade do CLI com o Go 1.27, e não só o cgo. Só dá para saber depois de instalar os `-devel`. Se for o Go, a saída é fixar o toolchain do `shell` ou usar só `go build` com as tags. Não bloqueia o B0 (D4).
- **`pw-play` com MP3.** Suportado pelo `libsndfile` 1.2.2 segundo a documentação. Não verificado tocando.
