# James · [English](README.en.md)

Assistente pessoal por voz e texto que age no computador. Você diz "James", fala o pedido, e ele
conversa com o Claude, decide quais ferramentas usar (terminal, arquivos, janelas, tela, mídia, busca
na web, Claude Code) e executa. Ações destrutivas e alterações de arquivo passam por aprovação explícita.

| Sistema | Situação |
|---|---|
| Fedora 44, KDE Plasma (Wayland) | plataforma atual; voz, tela, janelas e arquivos passaram por aceite ao vivo (as pendências estão registradas nos planos) |
| Windows 10/11 | onde o projeto nasceu (fases 0 a 7A); depois do porte para o Linux, compila (`GOOS=windows go vet`), mas não teve verificação funcional |

## O que ele faz

**Voz quase em tempo real.** Wake word offline (Vosk), fim de fala detectado por VAD (Silero em ONNX),
transcrição local com whisper.cpp na GPU (Vulkan) e voz local com Piper. Depois de uma resposta, há uma
janela de 6 s para continuar sem repetir "James". No spike de latência, com áudio sintético, a
mediana do fim da fala ao primeiro som foi **1,44 s** (Whisper turbo + Claude Haiku 5.5 + Piper). A
medição com fala real, em 20 turnos, ainda está pendente. Sem o whisper-server, o device cai para o
STT do Google.

**Agente com ferramentas.** O loop chama o modelo por uma interface `Provider` (Anthropic por padrão,
Gemini como alternativa), executa as chamadas de ferramenta e devolve o resultado ao modelo, com
limite de iterações por turno. A interface torna o loop testável sem gastar token.

| Ferramenta | O que faz | Aprovação no desktop | No celular |
|---|---|---|---|
| `executar_comando_sistema` | bash (Linux) ou PowerShell (Windows), com timeout e limite de saída | comando que apaga, cria ou altera arquivo, ou lê caminho sensível (`.ssh/`, `.env`…) | sempre |
| `arquivos` | lista pastas e lê arquivos, com os valores secretos mascarados | só para revelar um valor | sempre |
| `alterar_arquivos` | cria, edita, move, copia ou apaga (apagar vai para a lixeira) | sempre | sempre |
| `janelas` | lista, traz para frente ou fecha janelas | só fechar | sempre |
| `abrir` | abre app, arquivo, pasta ou URL no programa padrão | não | sempre |
| `manipular_interface` | clica em elemento ou coordenada, digita, aperta teclas | elemento de nome destrutivo e `Delete` | sempre |
| `perguntar_ao_claude_code` | delega uma tarefa de desenvolvimento ao Claude Code e devolve a resposta | sempre | sempre |
| `ver_tela` | captura a tela e lista os elementos clicáveis com id, nome, tipo e posição | não | não |
| `midia` | controla o player que estiver tocando (Spotify, navegador…) | não | não |
| `pesquisar_internet` | busca na web pela Tavily | não | não |
| `salvar_na_memoria` | grava uma nota que volta ao prompt nas próximas conversas | não | não |

**Permissão por cliente.** Conexão por loopback (o desktop) recebe o perfil *privilegiado*; qualquer
outra origem, como o celular pela VPN, recebe o perfil *restrito*, que só roda sem aprovação as
ferramentas de leitura e consulta. Assim o celular continua inofensivo se for perdido. Algumas
chamadas pedem aprovação até no desktop, decididas pelos argumentos (`ExigeConfirmacao` em
`internal/ferramentas/`). O pedido aparece como cartão no app e expira em 5 min. Em turno de voz, o
James avisa em voz alta que está esperando.

**Credenciais mascaradas.** O que uma ferramenta devolve vai para o modelo, para o Postgres, para a
janela de contexto e para o app. Por isso a leitura de arquivo mascara os valores antes de devolver:
pares `CHAVE=valor`, YAML, JSON, `.netrc` e blocos PEM saem como marcador, e o nome da chave continua
visível. O James responde se o `.env` tem a chave preenchida sem o valor sair da máquina.

**Memória.** Notas em Markdown (`Vault/Memoria/`) voltam ao prompt com orçamento próprio de tokens,
separado do orçamento da conversa e do resumo rolante da sessão. O histórico completo fica no Postgres.

**App.** SPA em React e TypeScript, servida pelo próprio backend (`go:embed`), com abas James,
Mensagens, Histórico, Memória e Dashboard (ainda sem métricas: a instrumentação de uso fica para a
Fase 7B). Abre como janela do Chrome em modo app, como shell nativo Wails ou no navegador.

## Arquitetura

```
 microfone, alto-falante                               celular (Tailscale, HTTPS)
          │                                                       │
 james-device (Python) ──── WebSocket ────► jamesd (Go) ◄──── REST + WS ──── app web (React)
   wake word, VAD, STT,                       agente, ferramentas,
   TTS, tela e janelas                        permissão, memória, API
   (AT-SPI, KWin, portal)                            │
                                               Postgres 17
```

- **`jamesd`** escuta só em `127.0.0.1:8080` e, para o celular, na porta 8443 com o certificado do
  Tailscale. Nunca em `0.0.0.0`: quem dá identidade de dispositivo e cifra o transporte é a VPN, e
  nenhuma porta é aberta no roteador.
- **`james-device`** roda na máquina física: microfone, sons, captura de tela e ações de interface. O
  `jamesd` pede uma ação de tela e o device devolve o resultado real (inclusive a falha), para o modelo
  poder corrigir a própria trajetória.

## Como rodar (Linux)

Precisa de Go 1.26+, Node LTS, Python 3.11+ e Docker com Compose.

```bash
cp .env.example .env                 # preencha CLAUDE_API_KEY, POSTGRES_PASSWORD e DATABASE_URL
python3 -m venv dispositivo/.venv
dispositivo/.venv/bin/pip install -e "dispositivo[dev]"
./executar.sh
```

O `executar.sh` sobe o Postgres, compila a SPA e o backend, inicia o STT e o TTS locais, o `jamesd` e
o `james-device`, e abre a interface. `Ctrl+C` encerra tudo. Os logs ficam em `.executar/`.

Modelos baixados à parte (fora do versionamento):

- **Vosk** (wake word): `vosk/vosk-model-small-pt-0.3` na raiz, ou `JAMES_VOSK_MODELO`.
- **Piper** (voz): `vozes/pt_BR-faber-medium.onnx` e o `.onnx.json`, de `rhasspy/piper-voices` no Hugging Face.
- **whisper.cpp** (opcional): `whisper-server` e `ggml-large-v3-turbo-q8_0.bin` em
  `~/.local/share/james/`, ou `JAMES_WHISPER_SERVIDOR` e `JAMES_WHISPER_MODELO`.

`TAVILY_API_KEY` é necessária só para `pesquisar_internet`, e `GEMINI_API_KEY` só com
`provedor = "gemini"` no `config.toml`. No Windows, o equivalente é o `executar.bat`.

## Testes

```bash
npm install --prefix web
npm run build --prefix web           # antes do Go: web/embed.go embute web/dist
go test ./...                        # os testes de banco pulam sem DATABASE_URL
dispositivo/.venv/bin/python -m pytest -q dispositivo
npm test --prefix web                # vitest
npm run e2e --prefix web             # Playwright
```

Sem o `npm run build`, `cmd/jamesd` e `web` não compilam (`pattern all:dist: no matching files found`).

## Como foi desenvolvido

O James foi construído por mim com o **Claude Code** como par de programação, e usa o Claude também
em tempo de execução: o cérebro do agente é o Claude Haiku 5.5 (`anthropic-sdk-go`), e tarefas de
desenvolvimento são delegadas ao Claude Code pela ferramenta `perguntar_ao_claude_code`.

O processo foi o mesmo em todas as fases:

1. **Brainstorming** com o Claude até as decisões estarem travadas; o que só eu podia decidir
   (topologia de rede, modelo de permissão, banco) foi perguntado, não presumido.
2. **Spec** escrita e aprovada, com o estado atual lido no código, nunca suposto.
3. **Plano** em tarefas pequenas, cada uma com teste antes do código (TDD).
4. **Execução** por subagentes: um implementa, outro confere o código contra a spec, outro revisa a
   qualidade. Achado importante é corrigido antes de seguir.
5. **Aceite ao vivo** comigo, `/code-review` e `/security-review` no diff, e merge só com o meu OK.

Tudo isso está versionado em [`docs/superpowers/`](docs/superpowers/): specs, planos, spikes com
medições, decisões e handoffs entre sessões. Dos 399 commits, 224 têm co-autoria do Claude.

## Estrutura

```
cmd/jamesd/          backend: monta config, agente, ferramentas e API
cmd/migrar-historico migração avulsa do histórico antigo (SQLite) para o Postgres
internal/
  agente/            loop de ferramentas, interface Provider, provedores anthropic e gemini
  api/               REST, WebSocket do app e do device, cartões de aprovação
  ferramentas/       as ferramentas, classificadores de comando, máscara de segredos, lixeira
  permissao/         perfis privilegiado e restrito
  memoria/           janela de contexto, orçamentos, resumo rolante, notas
  armazenamento/     Postgres (pgx) e migrações
  higiene/           guarda contra credencial versionada
  voz/               síntese de voz para o turno de texto
  config/            config.toml e .env, com falha rápida
dispositivo/         james-device em Python: escuta, VAD, STT, TTS, tela e janelas
web/                 SPA (React, TypeScript, Vite, Tailwind), vitest e Playwright
shell/               janela nativa Wails
prompts/sistema.md   prompt de sistema do James
docs/superpowers/    specs, planos, spikes e decisões
```

## Limitações conhecidas

- Latência com fala real ainda não medida (só o spike com áudio sintético).
- Windows sem verificação funcional depois do porte para o Linux.
- O turno de voz roda numa goroutine sem `recover`: um panic ali derruba o `jamesd` inteiro.
- O `config.toml` versionado traz o caminho do Claude Code no Windows; no Linux, sobrescreva com
  `JAMES_CLAUDE_CODE_DIRETORIO` no `.env`.

## Licença

[MIT](LICENSE). As fontes em `web/src/assets/fonts/` seguem a SIL Open Font License (`OFL.txt`).

Go • TypeScript • React • Python • PostgreSQL • Claude API • whisper.cpp • Piper • Wails
