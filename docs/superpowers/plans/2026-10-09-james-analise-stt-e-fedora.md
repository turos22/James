# Plano de análise: troca do STT do James e porte para Fedora

> Data: 2026-10-09
> Destino final: `docs/superpowers/plans/2026-10-09-james-analise-stt-e-fedora.md` no repo `meu-app-ts`.
> Saída deste plano: **duas specs** (A: STT, B: Fedora) e um plano de implementação para cada.
> Base: snapshot `James.zip`, commit `7a0d4fc` em `main`.
> Conferido contra o repo no Fedora em 2026-10-09, mesmo commit `7a0d4fc`. As correções estão na seção "Conferência" logo abaixo e marcadas com **[conferido]** no corpo.

## Conferência contra o repo (2026-10-09)

O repo no Fedora está no mesmo commit do snapshot. Cada referência de arquivo, linha e função foi lida de novo. O que bateu não está listado. O que mudou ou faltava:

| # | O plano dizia | O repo mostra |
|---|---|---|
| C1 | Janela em foco só em `tela.py:170` | Também em `superficies.py:112` e `:175` (`retangulo_da_janela_em_foco`). São dois pontos Win32, não um. |
| C2 | B1 = "ponte de tela desligada" basta | `main.py:28` importa `tela`, que importa `superficies`, que roda Win32 **no import** (`ctypes.WINFUNCTYPE` em `:102`, `ativar_dpi_awareness()` em nível de módulo, `windll.*.argtypes` em `:112-115`). No Linux o device morre antes do laço de voz. Prova: `pytest` falha na coleta de `test_elementos.py`, `test_superficies.py` e `test_tela.py` com `AttributeError: module 'ctypes' has no attribute 'WINFUNCTYPE'`. O B1 exige isolar o import, não só desligar a ponte. |
| C3 | TTS no Fedora: "igual" | O código é igual, mas `sintetizador.go:30` chama o binário `edge-tts`, que **não está instalado** no Fedora. |
| C4 | (ausente) | `config.toml:44` (versionado) tem `claude_code.diretorio = "C:\\Users\\Arthur\\..."`. No Linux esse caminho não existe. |
| C5 | (ausente) | A descrição da ferramenta para o modelo diz "PowerShell" (`terminal.go:86` e `:88`). No Linux, o modelo geraria PowerShell. |
| C6 | Classificador: denylist nova resolve | `classificador.go` faz `TrimLeft(token, "&.\\/-")`: `/usr/bin/rm` vira `usr/bin/rm` e **não casa** com `rm`. A versão Linux precisa comparar pelo basename. |
| C7 | B-c: Linux bloqueia `curl \| sh` | A versão Windows já pede confirmação para **`curl` e `wget` sozinhos** (e `iwr`, `irm`). Espelhar é bloquear o verbo, não só o pipe. |
| C8 | "Testes com caminho/comando Windows ganham build tag" | São exatamente 4: `TestExecutarPowerShellDeVerdade*` em `terminal_test.go:113-175`. |
| C9 | (ausente) | Achado não ligado a Linux: com o banco **virgem**, `internal/api` falhou 2 testes com `duplicate key ... pg_type_typname_nsp_index` (dois pacotes migrando ao mesmo tempo). Com o banco já migrado, 3 rodadas seguidas sem a falha. |
| C10 | Shell: módulo próprio | `shell/` não tem `go.mod`; é o pacote `james/shell` do módulo raiz. `go.mod:73` tem um `replace` comentado com caminho Windows (inofensivo). `shell/shell.exe` está versionado. |
| C11 | Launcher "sobe o shell" | `executar.bat` usa `go run .\cmd\jamesd` e `wails dev`, não binários compilados. |
| C12 | "Transcrição vazia: hoje só loga 'não entendi'" | São dois logs: `main.py:102` (`UnknownValueError`) e `escuta.py:186` ("janela fechou sem comando entendido"). |
| C13 | Dependências do Fedora (implícito: presentes) | Ausentes: `webkit2gtk4.1-devel`, `gtk3-devel`, `portaudio-devel`, `wails` CLI, `edge-tts`. Presentes: `docker` 29.8.2 + compose v5.6.0, `podman` 5.8.7, `pw-play`, `kscreen-doctor`, `spectacle`, `claude`. |

## Ordem de execução (resumo)

```
B0  jamesd + web + Postgres rodando no Fedora           ─┐ merge em main
B1  device só com VOZ no Linux (tela desligada)         ─┘ (não muda nada no Windows)
A   troca de STT (feat/stt-whisper), A/B via JAMES_STT  ← testada no Fedora
B2  device com TELA no Wayland/KDE (spec própria, a maior)
```

Motivo da ordem: o A/B do STT só tem valor com o device rodando na máquina onde ele vai viver (o Fedora). B0 e B1 são o mínimo para isso. B2 é independente do STT e é o trabalho mais pesado, então vem por último e não bloqueia a troca.

---

# Parte A: troca do STT (Google → Whisper local)

## A.0 Contexto levantado

| Item | Onde | Situação hoje |
|---|---|---|
| Wake word | `dispositivo/james_device/main.py` + `gramatica.py` | Vosk offline, `vosk-model-small-pt-0.3`, com gramática. **Fora do escopo.** |
| Transcrição do comando | `main.py:87` `criar_transcritor()` | `sr.Recognizer().recognize_google(..., language="pt-BR")`, áudio 16 kHz mono 16-bit |
| Contrato | `escuta.py:118` `escutar(transcrever_comando=...)` | Recebe `bytes` e devolve `str`. Já injetável. |
| Testes | `tests/test_escuta.py` | Usam transcritor falso. Não tocam no Google. |
| Tratamento de falha | `escuta.py:183` `_sem_derrubar` | Exceção na transcrição não derruba o laço |
| Janela de comando | `JANELA_PADRAO_SEGUNDOS = 8.0` + palavra de parada | O áudio inteiro da janela vai de uma vez para o transcritor |
| Dependências | `dispositivo/pyproject.toml` | `SpeechRecognition` (usa a API web gratuita do Google, sem SLA) |

**Conclusão:** a troca cabe numa fronteira só, a `criar_transcritor()`. O `escuta.py` e os testes da máquina de estados não mudam.

## A.1 Decisões já tomadas (2026-10-09)

| Tema | Decisão |
|---|---|
| Máquina | Device roda no **Fedora** (Ryzen 5 9600X, 12 threads, 14,7 GiB, RX 6650 XT) |
| Prioridade | **Precisão acima de latência.** Padrão `large-v3`; `turbo` só se empatar no WER |
| Backend padrão | **faster-whisper na CPU, int8**: empata com a GPU no `large-v3`, é Python puro, sem toolchain Vulkan |
| Alternância | `JAMES_STT=whisper\|google` (padrão `whisper`), mais a branch `feat/stt-whisper` |
| Falha | Sem fallback para o Google. Log com exceção + device de pé + **pedir para repetir** |
| Transcrição vazia | Mesmo caminho da falha (hoje só loga: `main.py:102` e `escuta.py:186`) **[conferido, C12]** |

**Benchmark no Fedora** (áudio sintético pt-BR de 9,3 s, modelo já carregado):

| Backend | Modelo | Tempo |
|---|---|---|
| faster-whisper, CPU int8 | large-v3 | **5,9 s** |
| whisper.cpp, Vulkan, RX 6650 XT | large-v3 q8_0 | ~6,3 s |
| whisper.cpp, Vulkan, RX 6650 XT | large-v3 f16 | ~10,4 s |
| faster-whisper, CPU int8 | large-v3-turbo | 3,7 s |
| whisper.cpp, Vulkan, RX 6650 XT | large-v3-turbo q8_0 | **~2,4 s** |

Os cinco acertaram a frase (incluindo "James", "Florianópolis", "Guilherme"). Como o áudio é limpo, isso **não mede precisão**. Achados técnicos:
- `faster-whisper 1.2.1` quebra com PyAV 19 (`metadata_errors`), então é preciso fixar `av<19`.
- No whisper.cpp, a RX 6650 XT não tem matrix cores, e o decode do `large-v3` custa 104–170 ms/token. Por isso a GPU só compensa com o `turbo`.
- Não testado: `-ac` (audio_ctx reduzido) no whisper.cpp, que evita codificar 30 s de padding.

## A.2 Branches

```
main                 → Google (referência estável)
feat/stt-whisper     → a partir de main (já com B0+B1); só a troca de STT
```

- Criar: `git switch main && git pull && git switch -c feat/stt-whisper`
- Dentro da branch, a troca de motor é via `JAMES_STT` + reiniciar o device. `git switch main` continua sendo o "Google puro" de referência.
- **Venv único** com as dependências das duas (`faster-whisper`, `av<19`, `SpeechRecognition`), assim trocar de branch não exige `pip install`.
- Modelo **fora do repo**, com caminho via `JAMES_WHISPER_MODELO` e entrada no `.gitignore`, igual ao Vosk.
- `git rebase main` sempre que `main` andar. O conflito esperado é pequeno.

## A.3 Análise: o que falta responder para a spec

### A2. Corpus real (base de toda a comparação)
- **30 comandos reais**, em WAV 16 kHz mono, com o microfone de uso diário: 10 perto, 10 a mais de 3 m, 10 com ruído (TV, ventilador).
- Pelo menos 10 com **nomes próprios** e termos técnicos.
- Transcrição correta escrita à mão em `corpus/esperado.tsv`.
- **Fora do git** (é gravação de voz do Arthur, mesma regra do `wake_candidatos.log`). `audios/*.mp3` são saída do TTS e não servem.

### A3. Benchmark Google × Whisper
- Script roda os 30 áudios e mede **WER** (`jiwer`), **acerto de nomes próprios** e **latência p50/p95**.
- Variações: `large-v3` e `large-v3-turbo`, com e sem `initial_prompt`. Opcional: `turbo` no whisper.cpp + Vulkan.
- **Critério:** tabela com números reais. Sem tabela, a spec não sai.

### A5. Viés de vocabulário (o motivo da troca)
- Medir o ganho de `initial_prompt`/`hotwords` com "James", os nomes do corpus e os nomes das ferramentas.
- Lista no `config.toml`, seção `[voz]` (recomendado).

### A6. Recursos
- Carregar o modelo **uma vez** no boot do device, nunca por comando.
- Medir a RAM com Vosk + Whisper `large-v3` carregados (~3–4 GB estimado, não medido) contra os 14,7 GiB totais e o resto do sistema.

### A7. Forma do "pede para repetir"
- Recomendação: um **som novo** em `dispositivo/assets/`, offline, que não depende do `jamesd` estar de pé.
- Alternativa: fala via TTS do `jamesd`, que exige um evento novo no WS.

### A8. Privacidade
- Com Whisper local, o áudio do comando não sai mais da máquina. Registrar como ganho e como restrição para não regredir.

## A.4 Spec A: conteúdo

Arquivo: `docs/superpowers/specs/2026-10-xx-james-stt-whisper-design.md`

1. Problema, com evidência do A3
2. Decisão: backend, modelo, `compute_type`, `initial_prompt`
3. Contrato: `criar_transcritor() -> Callable[[bytes], str]` não muda
4. Configuração: `JAMES_STT`, `JAMES_WHISPER_MODELO`, vocabulário em `[voz]`
5. Branches e alternância (A.2)
6. Falha, vazio e o aviso para repetir (A.1 + A7)
7. Critérios de aceite sobre o corpus:
   - WER Whisper < WER Google
   - nomes próprios: Whisper ≥ Google
   - latência p50/p95 registrada
   - `pytest` do `dispositivo/` verde
8. Fora do escopo: wake word, TTS, `jamesd`, frontend

## A.5 Plano de implementação A: esboço (TDD por tarefa)

1. `feat/stt-whisper` a partir de `main` com B0 e B1 já mesclados
2. Extrair `criar_transcritor` para `james_device/transcricao.py`, com fábrica por backend. Teste: escolhe pela env, e valor inválido gera erro claro no boot.
3. Transcritor Whisper: PCM16 `bytes` → `float32` numpy → `transcribe(language="pt", vad_filter=True, initial_prompt=...)`. Teste com modelo falso: junta os segmentos, e vazio vira `""`.
4. Carga única no boot, com log de tempo e de caminho. Falha de carga **não derruba** a ponte de tela.
5. Vazio ou falha → log + som de "repete". Teste no `test_escuta.py`.
6. `pyproject.toml`: `faster-whisper`, `av<19`, com comentário do motivo
7. `.gitignore`: modelo Whisper e corpus
8. `dispositivo/ferramentas/benchmark_stt.py` versionado (sem o corpus)
9. Rodar o benchmark no Fedora, anexar a tabela na spec e decidir o merge

---

# Parte B: porte do James para o Fedora (KDE Plasma, Wayland)

## B.0 Levantamento: o que é Windows-only hoje

| Camada | Arquivo | Dependência Windows | Equivalente no Fedora |
|---|---|---|---|
| Launcher | `executar.bat` | batch, Docker Desktop, janelas `start` | `executar.sh` + `docker compose` ou `podman compose` |
| jamesd: terminal | `internal/ferramentas/terminal.go` | `powershell -EncodedCommand`, codepage OEM | `bash -c` (UTF-8 nativo) |
| jamesd: permissão | `internal/ferramentas/classificador.go` | denylist de cmdlets/exe (`reg`, `netsh`, `sc.exe`, `Set-MpPreference`...) | **denylist nova** para Linux: `sudo`, `rm -rf /`, `dd`, `mkfs`, `systemctl`, `chmod -R`, `curl \| sh`, `crontab`, `firewall-cmd`... |
| jamesd: terminal (descrição) | `terminal.go:86`, `:88` | texto "PowerShell" para o modelo | descrição bash **[conferido, C5]** |
| jamesd: config | `config.toml:44` | `claude_code.diretorio` com caminho Windows | override por env **[conferido, C4]** |
| jamesd: TTS | `internal/voz/sintetizador.go:30` | nenhuma (binário `edge-tts`) | igual, mas **instalar** `edge-tts` (ausente) **[conferido, C3]** |
| jamesd: Claude Code | `internal/ferramentas/claudecode.go` | nenhuma (`claude` no PATH) | igual |
| Postgres | `docker-compose.yml` | Docker Desktop | igual, porta 5434 |
| Shell desktop | `shell/` (Wails v2.11) | build Windows (`shell.exe`) | build Linux: `webkit2gtk4.1-devel`, `gtk3-devel`, tag `webkit2_41` |
| Device: Vosk | `libvosk.dll` na raiz | DLL | wheel `vosk` do pip já traz `.so`; modelo é o mesmo |
| Device: áudio | `PyAudio` | nenhuma | `portaudio-devel` + PipeWire (funciona) |
| Device: sons | `playsound 1.2.2` | backend Windows | subprocess `pw-play` (PipeWire), sem dependência Python |
| Device: árvore de UI | `elementos.py`, `tela.py` (`uiautomation`) | **UI Automation do Windows** | **AT-SPI** (`pyatspi`); Qt/KDE expõe bem, Chromium/Electron precisam de flag de acessibilidade |
| Device: import | `superficies.py:80`, `:102`, `:112-115` | Win32 **no import** derruba o device inteiro **[conferido, C2]** | isolar atrás de `plataforma/` |
| Device: janela em foco | `tela.py:170` e `superficies.py:175` `windll.user32.GetForegroundWindow` **[conferido, C1]** | Win32 | KWin via D-Bus (script KWin) ou `kdotool` |
| Device: monitores/DPI | `superficies.py` (`ctypes.windll.shcore`) | Win32 | `kscreen-doctor -j` (geometria + escala por monitor) |
| Device: mouse/teclado | `tela.py`, `captura.py` (`pyautogui`) | funciona no X11, **não no Wayland** | portal **RemoteDesktop** (xdg-desktop-portal-kde, via libei) ou `ydotool` (uinput) |
| Device: screenshot | `captura.py` (`pyautogui`) | idem | `spectacle -b -n -f -o arquivo` ou portal Screenshot |
| Binários soltos | `verificar_tokens_imagem.exe`, DLLs na raiz | Windows | recompilar (Go) ou ignorar no Linux |

**Achado de segurança:** o classificador de permissão conhece **só comandos de PowerShell e Windows**. Se a ferramenta de terminal for portada para `bash` sem uma denylist Linux, comandos destrutivos passam **sem aprovação**. Regra para o B0: no Linux, o terminal fica **desligado** até existir a denylist Linux com testes.

## B.1 Estratégia

- **Uma base de código para as duas plataformas.** O Windows continua funcionando, e cada ponto da tabela vira uma interface com duas implementações.
  - Go: build tags (`terminal_windows.go` / `terminal_linux.go`, `classificador_windows.go` / `classificador_linux.go`).
  - Python: um módulo `plataforma/` com `windows.py` / `linux.py`, escolhido por `sys.platform`, e as mesmas fronteiras que o código já usa.
- Branch `feat/fedora` a partir de `main`, separada da `feat/stt-whisper`. B0 e B1 são mesclados em `main` antes da Parte A.

## B.2 Fatias

### B0. Backend no Fedora (sem device)
- `executar.sh` equivalente ao `.bat`: confere `.env`, sobe o Postgres, builda a SPA, sobe o `jamesd` e o shell.
- `go test ./...` verde no Linux. Testes com caminho/comando Windows ganham build tag (são 4, ver C8).
- Terminal: implementação `bash` + **denylist Linux com testes**. Até lá, a ferramenta fica desligada no Linux.
- Shell Wails compilado para Linux (ou só a web no navegador, como primeiro passo).
- **Aceite:** conversa por texto funcionando no Fedora, histórico no Postgres, TTS tocando.

### B1. Device só com voz
- Vosk via pip, `PyAudio` + PipeWire, sons via `pw-play`.
- Import de `tela`/`superficies` isolado: hoje ele roda Win32 no import e derruba o device (C2).
- Ponte de tela **desligada** no Linux, com log claro. O `jamesd` responde "tela indisponível", como já faz quando o device cai.
- **Aceite:** wake word + comando de voz + resposta falada, no Fedora, ponta a ponta.

### B2. Device com tela no Wayland (spec própria)
- Pontos a provar **antes** de escrever a spec (spikes de 1 dia cada):
  1. AT-SPI lista os elementos de Dolphin, Firefox/Chrome e VSCode com nome, tipo e retângulo.
  2. Clique e digitação via portal RemoteDesktop no KDE 6 (pede permissão uma vez, guarda o token) contra `ydotool`.
  3. Janela em foco via KWin.
  4. Coordenadas: AT-SPI e o portal no mesmo sistema de coordenadas, com escala fracionária.
- Resultado dos spikes → spec B2 → plano B2.

## B.3 Análise: perguntas da spec B

| # | Pergunta | Como responder |
|---|---|---|
| B-a | O `go test ./...` passa no Linux hoje? Quais testes dependem de Windows? | rodar e listar as falhas |
| B-b | `docker` ou `podman` no Fedora? | o que estiver instalado; o compose funciona com os dois |
| B-c | Denylist Linux: quais comandos? | espelhar as categorias da versão Windows (registro→`/etc`, serviços→`systemctl`, firewall→`firewall-cmd`, baixar-e-executar→`curl`/`wget` como verbo, ver C7), comparando por basename (C6) |
| B-d | Wails v2.11 builda com `webkit2gtk-4.1` no Fedora 44? | `wails build -tags webkit2_41` |
| B-e | AT-SPI cobre os apps de uso diário? | spike B2.1 |
| B-f | Portal RemoteDesktop ou `ydotool`? | spike B2.2 (o portal é o caminho certo no Wayland; o `ydotool` precisa de daemon root e de uinput) |

## B.4 Spec B e planos

- `docs/superpowers/specs/2026-10-xx-james-fedora-design.md`: B0 + B1, com a tabela B.0 como contrato
- `docs/superpowers/plans/2026-10-xx-james-fedora-b0-b1.md`: plano TDD
- `docs/superpowers/specs/2026-10-xx-james-tela-wayland-design.md`: só depois dos spikes do B2

---

## Pendências

- [ ] A2: gravar o corpus de 30 comandos (só o Arthur pode fazer)
- [ ] B-e / B-f: spikes do B2
- [x] Máquina, prioridade, backend, alternância e falha: decididos (A.1)
