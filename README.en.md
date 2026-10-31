# James · [Português](README.md)

A personal voice and text assistant that acts on the computer. You say "James", speak the request, and
it talks to Claude, decides which tools to use (terminal, files, windows, screen, media, web search,
Claude Code) and runs them. Destructive actions and file changes require explicit approval.

The code, comments and docs are in Portuguese.

| System | Status |
|---|---|
| Fedora 44, KDE Plasma (Wayland) | current platform; voice, screen, windows and files went through live acceptance (open items are logged in the plans) |
| Windows 10/11 | where the project started (phases 0 to 7A); after the Linux port it builds (`GOOS=windows go vet`) but has not been functionally verified |

## What it does

**Near real-time voice.** Offline wake word (Vosk), end of speech detected by VAD (Silero on ONNX),
local transcription with whisper.cpp on the GPU (Vulkan) and local speech with Piper. After an answer
there is a 6 s window to follow up without saying "James" again. In the latency spike, with synthetic
audio, the median from end of speech to first sound was **1.44 s** (Whisper turbo + Claude Haiku 5.5 +
Piper). The measurement with real speech, over 20 turns, is still pending. Without the whisper-server,
the device falls back to Google STT.

**Tool-using agent.** The loop calls the model through a `Provider` interface (Anthropic by default,
Gemini as an alternative), runs the tool calls and feeds the results back, with an iteration cap per
turn. The interface makes the loop testable without spending tokens.

| Tool | What it does | Approval on desktop | On phone |
|---|---|---|---|
| `executar_comando_sistema` | bash (Linux) or PowerShell (Windows), with timeout and output cap | commands that delete, create or change files, or read sensitive paths (`.ssh/`, `.env`…) | always |
| `arquivos` | lists folders and reads files, with secret values masked | only to reveal a value | always |
| `alterar_arquivos` | creates, edits, moves, copies or deletes (deleting goes to the trash) | always | always |
| `janelas` | lists, focuses or closes windows | closing only | always |
| `abrir` | opens an app, file, folder or URL in the default program | no | always |
| `manipular_interface` | clicks an element or coordinate, types, presses keys | destructive-named elements and `Delete` | always |
| `perguntar_ao_claude_code` | hands a development task to Claude Code and returns its answer | always | always |
| `ver_tela` | captures the screen and lists clickable elements with id, name, type and position | no | no |
| `midia` | controls whatever player is playing (Spotify, browser…) | no | no |
| `pesquisar_internet` | web search through Tavily | no | no |
| `salvar_na_memoria` | saves a note that comes back into the prompt in later conversations | no | no |

**Per-client permission.** A loopback connection (the desktop) gets the *privileged* profile; any other
origin, such as the phone over the VPN, gets the *restricted* profile, which only runs read and query
tools without approval. That keeps the phone harmless if it is lost. Some calls ask for approval even
on the desktop, decided by their arguments (`ExigeConfirmacao` in `internal/ferramentas/`). The request
shows up as a card in the app and expires after 5 min. In a voice turn, James says out loud that it is
waiting.

**Masked credentials.** Whatever a tool returns goes to the model, to Postgres, to the context window
and to the app. So file reads mask values before returning them: `KEY=value` pairs, YAML, JSON,
`.netrc` and PEM blocks come out as a marker, and the key name stays visible. James can tell whether
`.env` has the key filled in without the value leaving the machine.

**Memory.** Markdown notes (`Vault/Memoria/`) come back into the prompt with their own token budget,
separate from the conversation budget and from the rolling session summary. The full history lives in
Postgres.

**App.** React and TypeScript SPA served by the backend itself (`go:embed`), with James, Messages,
History, Memory and Dashboard tabs (no metrics yet: usage instrumentation is phase 7B). It opens as a
Chrome app window, as a native Wails shell or in the browser.

## Architecture

```
 microphone, speaker                                   phone (Tailscale, HTTPS)
          │                                                       │
 james-device (Python) ──── WebSocket ────► jamesd (Go) ◄──── REST + WS ──── web app (React)
   wake word, VAD, STT,                       agent, tools,
   TTS, screen and windows                    permission, memory, API
   (AT-SPI, KWin, portal)                            │
                                               Postgres 17
```

- **`jamesd`** listens only on `127.0.0.1:8080` and, for the phone, on port 8443 with the Tailscale
  certificate. Never on `0.0.0.0`: the VPN provides device identity and transport encryption, and no
  port is opened on the router.
- **`james-device`** runs on the physical machine: microphone, sounds, screen capture and UI actions.
  `jamesd` asks for a screen action and the device returns the real outcome (failures included), so the
  model can correct its own course.

## Running it (Linux)

Requires Go 1.26+, Node LTS, Python 3.11+ and Docker with Compose.

```bash
cp .env.example .env                 # fill in CLAUDE_API_KEY, POSTGRES_PASSWORD and DATABASE_URL
python3 -m venv dispositivo/.venv
dispositivo/.venv/bin/pip install -e "dispositivo[dev]"
./executar.sh
```

`executar.sh` starts Postgres, builds the SPA and the backend, starts the local STT and TTS, `jamesd`
and `james-device`, and opens the interface. `Ctrl+C` stops everything. Logs go to `.executar/`.

Models downloaded separately (not versioned):

- **Vosk** (wake word): `vosk/vosk-model-small-pt-0.3` at the repo root, or `JAMES_VOSK_MODELO`.
- **Piper** (voice): `vozes/pt_BR-faber-medium.onnx` and its `.onnx.json`, from `rhasspy/piper-voices` on Hugging Face.
- **whisper.cpp** (optional): `whisper-server` and `ggml-large-v3-turbo-q8_0.bin` under
  `~/.local/share/james/`, or `JAMES_WHISPER_SERVIDOR` and `JAMES_WHISPER_MODELO`.

`TAVILY_API_KEY` is only needed for `pesquisar_internet`, and `GEMINI_API_KEY` only with
`provedor = "gemini"` in `config.toml`. On Windows, use `executar.bat`.

## Tests

```bash
npm install --prefix web
npm run build --prefix web           # before Go: web/embed.go embeds web/dist
go test ./...                        # database tests skip without DATABASE_URL
dispositivo/.venv/bin/python -m pytest -q dispositivo
npm test --prefix web                # vitest
npm run e2e --prefix web             # Playwright
```

Without `npm run build`, `cmd/jamesd` and `web` do not compile (`pattern all:dist: no matching files found`).

## How it was built

I built James with **Claude Code** as a pair programmer, and James uses Claude at runtime too: the
agent's brain is Claude Haiku 5.5 (`anthropic-sdk-go`), and development tasks are handed to Claude Code
through the `perguntar_ao_claude_code` tool.

Every phase followed the same process:

1. **Brainstorming** with Claude until the decisions were locked; anything only I could decide (network
   topology, permission model, database) was asked, not assumed.
2. **Spec** written and approved, with the current state read from the code, never guessed.
3. **Plan** in small tasks, each with the test written before the code (TDD).
4. **Execution** by subagents: one implements, another checks the code against the spec, another
   reviews quality. Important findings are fixed before moving on.
5. **Live acceptance** with me, `/code-review` and `/security-review` on the diff, and merge only with
   my OK.

All of it is versioned in [`docs/superpowers/`](docs/superpowers/): specs, plans, spikes with
measurements, decisions and handoffs between sessions. Of the 399 commits, 224 are co-authored by
Claude.

## Layout

```
cmd/jamesd/          backend: wires config, agent, tools and API
cmd/migrar-historico one-off migration of the old history (SQLite) to Postgres
internal/
  agente/            tool loop, Provider interface, anthropic and gemini providers
  api/               REST, WebSocket for the app and the device, approval cards
  ferramentas/       the tools, command classifiers, secret masking, trash
  permissao/         privileged and restricted profiles
  memoria/           context window, budgets, rolling summary, notes
  armazenamento/     Postgres (pgx) and migrations
  higiene/           guard against committed credentials
  voz/               speech synthesis for text turns
  config/            config.toml and .env, failing fast
dispositivo/         james-device in Python: listening, VAD, STT, TTS, screen and windows
web/                 SPA (React, TypeScript, Vite, Tailwind), vitest and Playwright
shell/               native Wails window
prompts/sistema.md   James's system prompt
docs/superpowers/    specs, plans, spikes and decisions
```

## Known limitations

- Latency with real speech not measured yet (only the spike with synthetic audio).
- Windows not functionally verified after the Linux port.
- The voice turn runs in a goroutine without `recover`: a panic there takes down the whole `jamesd`.
- The versioned `config.toml` holds the Windows path for Claude Code; on Linux, override it with
  `JAMES_CLAUDE_CODE_DIRETORIO` in `.env`.

## License

[MIT](LICENSE). The fonts in `web/src/assets/fonts/` are under the SIL Open Font License (`OFL.txt`).

Go • TypeScript • React • Python • PostgreSQL • Claude API • whisper.cpp • Piper • Wails
