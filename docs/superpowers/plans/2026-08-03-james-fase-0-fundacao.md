# James — Fase 0: Fundação — Plano de Implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deixar o repositório sem nenhuma credencial, versionado em git, com suíte de teste verde em Go, Python e TypeScript, Postgres de pé em Docker, e o risco do Tailscale respondido por spike — de modo que a Fase 1 possa ser executada com TDD de verdade.

**Architecture:** Nada é reestruturado nesta fase. O código legado (`app.go`, `main.go`, `database.go`, `frontend/`) permanece onde está e continua funcionando. O que se adiciona é um esqueleto novo (`internal/`, `dispositivo/`) que a Fase 1 vai preencher, mais a higiene que torna o `git init` seguro. O módulo Go é renomeado de `meu-app-ts` para `james` — verificado como seguro, já que nenhum arquivo importa o próprio módulo.

**Tech Stack:** Go 1.26.1, Python 3.13.9, Node 24.12 / npm 11.6, Docker 29.1, PostgreSQL 17, `pgx/v5`, `godotenv`, `pytest`, `vitest`.

**Spec de origem:** [`docs/superpowers/specs/2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md`](../specs/2026-08-03-james-fase-0-1-fundacao-e-nucleo-design.md)

---

## Gate — pré-requisitos do Arthur

**Nenhuma task começa antes disto.** São ações que só o Arthur pode executar.

- [ ] **G1. Revogar/rotacionar as quatro credenciais**

  Elas estão em texto puro no fonte **e compiladas dentro de três `.exe`**. Apagar do código não basta — quem já tem o binário tem a chave.

  | Chave | Onde | Ação | Onde fazer |
  |---|---|---|---|
  | Gemini | `app.go:458` | rotacionar (segue em uso) | https://aistudio.google.com/apikey |
  | Tavily | `app.go:229` | rotacionar (segue em uso) | https://app.tavily.com |
  | NVIDIA | `app.go:385` | **revogar sem repor** — o código que a usa é morto e será deletado | https://build.nvidia.com |
  | OpenRouter | `rodar_claude.ps1:6` | **revogar sem repor** — o Claude Code passa a usar a assinatura Pro | https://openrouter.ai/settings/keys |

- [ ] **G2. Subir o Docker Desktop**

  Verificado em 2026-08-03: Docker 29.1.2 instalado, daemon parado.
  Confirmar com:

```bash
docker ps
```

  Esperado: cabeçalho de tabela, sem erro de pipe.

- [ ] **G3. Instalar o Tailscale no PC e no celular**

  Mesma conta nos dois. https://tailscale.com/download
  Confirmar com (o binário não está no PATH do Git Bash):

```bash
"/c/Program Files/Tailscale/tailscale.exe" status
```

  Esperado: lista de dispositivos do tailnet, incluindo o celular.

**✅ Gate concluído e verificado em 2026-08-03:** as quatro chaves foram
rotacionadas/revogadas pelo Arthur, `docker ps` responde, e o tailnet lista
`desktop-dc358rs` (100.96.219.29) e `s21-fe-de-arthur` (100.83.30.121).

---

## Estrutura de arquivos desta fase

| Arquivo | Responsabilidade |
|---|---|
| `internal/higiene/higiene.go` | Varre a árvore em busca de credenciais conhecidas |
| `internal/higiene/higiene_test.go` | Prova a varredura + guarda permanente do repositório |
| `internal/armazenamento/postgres/conexao.go` | Abre e valida o pool de conexões |
| `internal/armazenamento/postgres/conexao_test.go` | Recusa de URL vazia + conexão real |
| `dispositivo/james_device/wake.py` | Detecção da wake word "James" |
| `dispositivo/tests/test_wake.py` | Casos de variante de transcrição |
| `frontend/src/lib/markdown.ts` | Separa resposta em texto e blocos de código |
| `frontend/src/lib/markdown.test.ts` | Casos do parser |
| `.gitignore`, `.env.example` | Higiene de versionamento |
| `docker-compose.yml` | Postgres local |
| `docs/superpowers/spikes/2026-08-03-tailscale.md` | Respostas do spike |

**Ordem obrigatória:** Tasks 1 e 2 **não têm commit** — o repositório git ainda não existe, e criá-lo antes da limpeza colocaria as credenciais no primeiro commit. A Task 3 é o `git init` e o commit inicial de tudo que veio antes.

---

## Task 1: Guarda automatizada contra credenciais

Escrever a varredura **antes** de limpar, para que a limpeza seja verificável em vez de confiada à leitura humana. O teste falha agora e passa na Task 2.

**Files:**
- Modify: `go.mod:1`
- Create: `internal/higiene/higiene.go`
- Create: `internal/higiene/higiene_test.go`

- [ ] **Step 1: Renomear o módulo Go**

Trocar a linha 1 de `go.mod`:

```
module james
```

Nenhum outro arquivo precisa mudar. Verificado: `grep -rn "meu-app-ts" --include="*.go" .` só encontra uma string de caminho de sistema de arquivos em `app.go:150`, não um import.

- [ ] **Step 2: Escrever o teste que falha**

Criar `internal/higiene/higiene_test.go`:

```go
package higiene

import (
	"os"
	"path/filepath"
	"testing"
)

// chaveFalsa monta um literal de credencial em tempo de execução para que
// este próprio arquivo de teste não seja acusado pela varredura.
func chaveFalsa() string {
	return "AIza" + "Sy" + "0000000000000000000000000000000000"
}

func TestVarrerAcusaCredencialEmArquivoTexto(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "config.go")
	if err := os.WriteFile(arquivo, []byte(`const chave = "`+chaveFalsa()+`"`), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("esperava 1 achado, vieram %d: %+v", len(achados), achados)
	}
	if achados[0].Arquivo != arquivo {
		t.Errorf("arquivo do achado = %q, queria %q", achados[0].Arquivo, arquivo)
	}
}

func TestVarrerIgnoraDiretoriosDeTerceiros(t *testing.T) {
	raiz := t.TempDir()
	pasta := filepath.Join(raiz, "node_modules")
	if err := os.Mkdir(pasta, 0o755); err != nil {
		t.Fatalf("não consegui criar node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pasta, "vendido.js"), []byte(chaveFalsa()), 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 0 {
		t.Errorf("node_modules deveria ser ignorado, vieram %d achados: %+v", len(achados), achados)
	}
}

func TestVarrerAcusaCredencialDentroDeBinario(t *testing.T) {
	raiz := t.TempDir()
	arquivo := filepath.Join(raiz, "app.exe")
	conteudo := append([]byte{0x4D, 0x5A, 0x90, 0x00}, []byte(chaveFalsa())...)
	if err := os.WriteFile(arquivo, conteudo, 0o600); err != nil {
		t.Fatalf("não consegui escrever a fixture: %v", err)
	}

	achados, err := Varrer(raiz)
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	if len(achados) != 1 {
		t.Fatalf("credencial dentro de binário deveria ser acusada, vieram %d: %+v", len(achados), achados)
	}
}

// TestRepositorioLimpo é a guarda de verdade: falha enquanto existir
// qualquer credencial conhecida na árvore do projeto.
func TestRepositorioLimpo(t *testing.T) {
	achados, err := Varrer("../..")
	if err != nil {
		t.Fatalf("Varrer devolveu erro: %v", err)
	}
	for _, a := range achados {
		t.Errorf("credencial encontrada: %s (padrão %q)", a.Arquivo, a.Padrao)
	}
}
```

- [ ] **Step 3: Rodar e confirmar que falha por não compilar**

```bash
go test ./internal/higiene/ -v
```

Esperado: falha de compilação, `undefined: Varrer`.

- [ ] **Step 4: Escrever a implementação mínima**

Criar `internal/higiene/higiene.go`:

```go
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

// diretoriosIgnorados nunca entram na varredura: são dependências de
// terceiros ou artefatos de build que não versionamos.
var diretoriosIgnorados = map[string]bool{
	".git":         true,
	"node_modules": true,
	"TTs":          true,
	"vosk":         true,
	"dist":         true,
}

// extensoesIgnoradas pula bibliotecas nativas de terceiros. São 68 MB de
// .dll e .lib que jamais conteriam uma credencial nossa, e lê-los a cada
// `go test ./...` só custa tempo. Binários que nós compilamos (.exe)
// continuam sendo varridos — são justamente o caso perigoso.
var extensoesIgnoradas = map[string]bool{
	".dll": true,
	".lib": true,
}

// Varrer percorre raiz recursivamente e devolve toda ocorrência de
// credencial conhecida, inclusive dentro de binários compilados.
func Varrer(raiz string) ([]Achado, error) {
	var achados []Achado

	err := filepath.WalkDir(raiz, func(caminho string, entrada fs.DirEntry, err error) error {
		if err != nil {
			return err
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
		return nil
	})

	return achados, err
}
```

- [ ] **Step 5: Rodar e confirmar o resultado esperado**

```bash
go test ./internal/higiene/ -v
```

Esperado: os três primeiros testes **passam**; `TestRepositorioLimpo` **falha**, listando `app.go`, `rodar_claude.ps1`, `meu-app-ts.exe`, `build/bin/meu-app-ts.exe` e `build/bin/meu-app-ts-dev.exe`.

**Essa falha é o objetivo da task.** Anotar a lista exata — é o roteiro da Task 2.

- [ ] **Step 6: Sem commit**

Repositório git ainda não existe. Seguir para a Task 2.

---

## Task 2: Expurgo das credenciais

Zerar o `TestRepositorioLimpo`.

**Files:**
- Delete: `meu-app-ts.exe`, `james.exe`, `build/bin/meu-app-ts.exe`
- Modify: `app.go` (chaves Gemini e Tavily; remoção do bloco NVIDIA morto)
- Modify: `rodar_claude.ps1`
- Modify: `main.go` (carga do `.env`)
- Create: `.env.example`, `.env`, `.gitignore`

- [ ] **Step 1: Apagar os binários contaminados**

Lista corrigida contra o disco real na Task 1. `build/bin/meu-app-ts-dev.exe`,
que a versão anterior deste plano mandava apagar, **não existe**. Em compensação
`james.exe` apareceu: depois do rename do módulo, qualquer `go build .` sem
`-o` passa a gerar esse nome na raiz, com as chaves de `app.go` compiladas dentro.

```bash
rm -f meu-app-ts.exe james.exe build/bin/meu-app-ts.exe
```

> **Sempre use `go build -o /dev/null .` para checar compilação.** Um `go build .`
> pelado deposita um binário de 28 MB na raiz com todo segredo que o fonte tiver.

- [ ] **Step 2: Substituir a chave do Gemini**

Em `app.go`, dentro de `PerguntaAi`, trocar:

```go
    client, err := genai.NewClient(a.ctx, &genai.ClientConfig{
        APIKey:  os.Getenv("GEMINI_API_KEY"),
        Backend: genai.BackendGeminiAPI,
    })
```

- [ ] **Step 3: Substituir a chave da Tavily**

Em `app.go`, dentro de `BuscarNoTavily`, trocar a linha do literal por:

```go
    apiKey := os.Getenv("TAVILY_API_KEY")
```

- [ ] **Step 4: Remover o bloco morto da NVIDIA**

Deletar de `app.go` a função `chamadaResumo` inteira (classificador WEB/LOCAL/CHAT, nunca chamada por ninguém) e o tipo `NVIDIAApiResponse` que só ela usa. Isso elimina a terceira credencial em vez de parametrizá-la.

Confirmar que ninguém a chamava:

```bash
grep -n "chamadaResumo\|NVIDIAApiResponse" *.go
```

Esperado após a remoção: nenhuma linha.

- [ ] **Step 5: Limpar o `rodar_claude.ps1`**

Substituir o arquivo inteiro por:

```powershell
# rodar_claude.ps1
param([string]$Instrucao)

# O Claude Code autentica pela assinatura Pro via OAuth. Variáveis de ambiente
# da Anthropic herdadas do shell sequestrariam a sessão para outro provider —
# era exatamente o que este script fazia antes, apontando para o OpenRouter.
Remove-Item Env:\ANTHROPIC_BASE_URL, Env:\ANTHROPIC_API_KEY, Env:\ANTHROPIC_AUTH_TOKEN -ErrorAction SilentlyContinue

claude $Instrucao

Read-Host "Tarefa finalizada. Pressione Enter para fechar"
```

- [ ] **Step 6: Adicionar o carregamento do `.env`**

```bash
go get github.com/joho/godotenv
```

Em `main.go`, adicionar o import e a carga como primeira instrução de `main()`:

```go
import (
	"context"
	"embed"

	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

func main() {
	// Ausência de .env não é erro: em produção as variáveis vêm do ambiente.
	_ = godotenv.Load()

	app := NewApp()
	// ... resto inalterado
```

O pacote `os` já está nos imports de `app.go` (linha 14) — nada a acrescentar lá.

- [ ] **Step 7: Criar o `.env.example`**

```bash
# Credenciais do James. Copie para .env e preencha.
# O .env NUNCA é versionado.

# Google AI Studio — https://aistudio.google.com/apikey
GEMINI_API_KEY=

# Tavily — https://app.tavily.com
TAVILY_API_KEY=

# Postgres local (docker-compose.yml lê estas duas)
POSTGRES_PASSWORD=
DATABASE_URL=postgres://james:SENHA@localhost:5433/james?sslmode=disable
```

- [ ] **Step 8: Criar o `.env` real**

Copiar `.env.example` para `.env` e preencher com as chaves **novas**, rotacionadas no gate G1. Escolher uma senha qualquer para o Postgres local e refletir em `DATABASE_URL`.

- [ ] **Step 9: Criar o `.gitignore`**

```gitignore
# Segredos
.env

# Binários e artefatos de build
*.exe
build/bin/
frontend/dist/
frontend/package.json.md5

# Dependências
node_modules/
__pycache__/
.venv/
*.egg-info/
.pytest_cache/

# Bibliotecas nativas pesadas — baixadas à parte, não versionadas
*.dll
*.lib
vosk/
TTs/

# Banco legado (migrado para Postgres na Fase 1)
Athena.db
Athena.db-shm
Athena.db-wal

# Áudio gerado pelo TTS. Os assets do agente de dispositivo
# (entrada.mp3, faaah.wav, um_minuto.mp3) continuam versionados.
audios/audio_jarvis.mp3

# Material de referência de terceiros (obsidian.py é insumo da Fase 4)
arquivos do Gui/
```

- [ ] **Step 10: Rodar a guarda e confirmar que passa**

```bash
go test ./internal/higiene/ -v
```

Esperado: **os quatro testes passam**, incluindo `TestRepositorioLimpo`.

- [ ] **Step 11: Confirmar que o projeto ainda compila**

```bash
go build -o /dev/null . && go vet ./...
```

Esperado: sem saída, exit 0 nos dois.

- [ ] **Step 12: Sem commit**

Ainda sem repositório. Seguir para a Task 3.

---

## Task 3: `git init` e commit inicial

Só agora é seguro: a árvore está limpa e o `.gitignore` existe.

**Files:**
- Create: repositório git

- [ ] **Step 1: Inicializar**

```bash
git init -b main
```

- [ ] **Step 2: Preparar tudo e inspecionar antes de commitar**

```bash
git add -A && git status --short
```

Ler a lista. Confirmar que **não** aparecem: `.env`, nenhum `.exe`, nenhum `.dll`, `Athena.db`, `node_modules/`, `TTs/`, `vosk/`.

- [ ] **Step 3: Verificação dupla — a guarda sobre o que o git vai versionar**

```bash
git ls-files | grep -cE "\.(exe|dll|lib)$"
```

Esperado: `0`.

```bash
git ls-files | grep -c "^\.env$"
```

Esperado: `0`.

- [ ] **Step 4: Commit inicial**

```bash
git commit -m "chore: commit inicial do James com árvore limpa de credenciais

Renomeia o módulo Go de meu-app-ts para james, remove as quatro credenciais
que estavam em texto puro no fonte e compiladas nos binários, e adiciona a
guarda automatizada que impede a reincidência.

Inclui a spec e o plano das Fases 0 e 1.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

- [ ] **Step 5: Confirmar que o histórico nasceu limpo**

Os padrões vão com classe de caractere de propósito: escritos por extenso, o
próprio comando viraria um achado da guarda quando este arquivo fosse varrido.

```bash
git log -p | grep -cE "AIza[S]y|tvly[-]|nvapi[-]|sk-or-[v]1-"
```

Esperado: `0`.

---

## Task 4: Corrigir a diretiva `//go:embed` ausente

Sem ela, `var assets embed.FS` fica vazio e `wails build` produz binário com tela em branco. Só `wails dev` funciona hoje, porque usa o servidor do Vite.

**Files:**
- Modify: `main.go:13-14`

- [ ] **Step 1: Confirmar o defeito**

```bash
grep -n -B2 "var assets embed.FS" main.go
```

Esperado: a linha `var assets embed.FS` sem nenhum comentário `//go:embed` acima.

- [ ] **Step 2: Garantir que o alvo do embed existe**

```bash
cd frontend && npm install && npm run build && cd ..
```

Esperado: `frontend/dist/` populado. `//go:embed` falha em tempo de compilação se o diretório não existir.

- [ ] **Step 3: Adicionar a diretiva**

Em `main.go`, imediatamente acima de `var assets embed.FS`:

```go
//go:embed all:frontend/dist
var assets embed.FS
```

- [ ] **Step 4: Confirmar que compila com o embed**

```bash
go build -o /dev/null .
```

Esperado: exit 0. Se `frontend/dist` estivesse faltando, aqui apareceria `pattern all:frontend/dist: no matching files found`.

- [ ] **Step 5: Commit**

```bash
git add main.go && git commit -m "fix: adiciona diretiva //go:embed ausente para os assets do frontend

Sem ela o embed.FS ficava vazio e o binário de release abria com tela branca.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 5: Harness Python — wake word do James

Primeiro código do agente de dispositivo. Lógica pura, sem hardware, então é honestamente testável — e já implementa parte do rename.

**Files:**
- Create: `dispositivo/pyproject.toml`
- Create: `dispositivo/james_device/__init__.py`
- Create: `dispositivo/james_device/wake.py`
- Create: `dispositivo/tests/__init__.py`
- Create: `dispositivo/tests/test_wake.py`

- [ ] **Step 1: Criar o pacote Python**

`dispositivo/pyproject.toml`:

```toml
[project]
name = "james-device"
version = "0.1.0"
description = "Agente de dispositivo do James: voz e controle de tela"
requires-python = ">=3.11"
dependencies = []

[project.optional-dependencies]
dev = ["pytest>=8.0"]

[build-system]
requires = ["setuptools>=68"]
build-backend = "setuptools.build_meta"

[tool.pytest.ini_options]
testpaths = ["tests"]
pythonpath = ["."]
```

Criar `dispositivo/james_device/__init__.py` e `dispositivo/tests/__init__.py` vazios.

- [ ] **Step 2: Instalar o pytest**

```bash
cd dispositivo && python -m pip install -e ".[dev]" && cd ..
```

- [ ] **Step 3: Escrever o teste que falha**

`dispositivo/tests/test_wake.py`:

```python
import pytest

from james_device.wake import contem_wake_word, normalizar


def test_normalizar_remove_acento_e_caixa():
    assert normalizar("Jaímes") == "jaimes"


@pytest.mark.parametrize(
    "ouvido",
    ["james", "James", "Jeimes, tudo bem?", "ei jaimes", "JEIMS me ajuda"],
)
def test_reconhece_variantes_da_wake_word(ouvido):
    assert contem_wake_word(ouvido) is True


@pytest.mark.parametrize("ouvido", ["athena", "bom dia", "", "jazz", "gosto de games"])
def test_ignora_texto_sem_wake_word(ouvido):
    assert contem_wake_word(ouvido) is False


def test_aceita_lista_de_variantes_customizada():
    assert contem_wake_word("chapolin", ["chapolin"]) is True
    assert contem_wake_word("james", ["chapolin"]) is False
```

- [ ] **Step 4: Rodar e confirmar que falha**

```bash
cd dispositivo && python -m pytest -v; cd ..
```

Esperado: `ModuleNotFoundError: No module named 'james_device.wake'`.

- [ ] **Step 5: Escrever a implementação mínima**

`dispositivo/james_device/wake.py`:

```python
"""Detecção da palavra de ativação do James.

O STT em português transcreve "James" de várias formas. Esta função
normaliza o que foi ouvido e compara contra a lista de variantes.
"""

import unicodedata

# Variantes com que o STT em português costuma transcrever "James".
# Deliberadamente conservadora: palavras comuns do português que soam
# parecido (games, jaime) ficam de fora para não disparar ativação à toa.
# A calibração com gravações reais do Arthur é tarefa da Fase 5.
VARIANTES_PADRAO = [
    "james",
    "jeimes",
    "jaimes",
    "jeims",
    "geimes",
]


def normalizar(texto: str) -> str:
    """Devolve o texto em minúsculas e sem acentos."""
    decomposto = unicodedata.normalize("NFD", texto.lower())
    return "".join(c for c in decomposto if unicodedata.category(c) != "Mn")


def contem_wake_word(texto: str, variantes: list[str] | None = None) -> bool:
    """Informa se o texto ouvido contém alguma variante da wake word."""
    if variantes is None:
        variantes = VARIANTES_PADRAO
    normalizado = normalizar(texto)
    return any(normalizar(v) in normalizado for v in variantes)
```

- [ ] **Step 6: Rodar e confirmar que passa**

```bash
cd dispositivo && python -m pytest -v; cd ..
```

Esperado: 12 testes passando (1 + 5 + 5 + 1).

- [ ] **Step 7: Commit**

```bash
git add dispositivo/ && git commit -m "feat: adiciona detecção da wake word do James com suíte pytest

Primeiro código do agente de dispositivo e primeiro harness Python do projeto.
Lista de variantes conservadora; calibração com áudio real fica para a Fase 5.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 6: Harness web — parser de blocos de código

O `renderMessageContent` em `App.tsx:21-51` mistura parsing com renderização, o que o torna intestável. Extrair o parsing para função pura estabelece o harness TypeScript e melhora a fronteira.

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/src/lib/markdown.ts`
- Create: `frontend/src/lib/markdown.test.ts`
- Modify: `frontend/src/App.tsx:21-51`

- [ ] **Step 1: Instalar o vitest e atualizar a toolchain**

O Vite instalado é o 3.0.7, de 2022, e o vitest atual exige Vite 5+. Instalar as versões correntes:

```bash
cd frontend && npm install -D vitest@latest vite@latest @vitejs/plugin-react@latest && cd ..
```

Anotar as versões que caíram (`npm list vite vitest @vitejs/plugin-react`) — se a Task falhar adiante, é a primeira suspeita.

- [ ] **Step 2: Adicionar o script de teste**

Em `frontend/package.json`, o bloco `scripts` passa a:

```json
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "test": "vitest run"
  },
```

- [ ] **Step 3: Confirmar que o build ainda funciona depois do upgrade**

```bash
cd frontend && npm run build && cd ..
```

Esperado: build conclui e `frontend/dist/` é regenerado. **Se falhar, parar e resolver aqui** — a Task 4 depende de `frontend/dist` existir.

- [ ] **Step 4: Escrever o teste que falha**

`frontend/src/lib/markdown.test.ts`:

```ts
import { describe, expect, it } from "vitest";

import { separarSegmentos } from "./markdown";

describe("separarSegmentos", () => {
  it("devolve um único segmento de texto quando não há bloco de código", () => {
    expect(separarSegmentos("bom dia")).toEqual([
      { tipo: "texto", conteudo: "bom dia" },
    ]);
  });

  it("descarta os segmentos de texto vazios ao redor de um bloco", () => {
    expect(separarSegmentos("```py\nx = 1\n```")).toEqual([
      { tipo: "codigo", linguagem: "py", conteudo: "x = 1\n" },
    ]);
  });

  it("extrai a linguagem da primeira linha do bloco", () => {
    expect(separarSegmentos("veja:\n```go\nfmt.Println()\n```")).toEqual([
      { tipo: "texto", conteudo: "veja:\n" },
      { tipo: "codigo", linguagem: "go", conteudo: "fmt.Println()\n" },
    ]);
  });

  it("não trata a primeira linha como linguagem quando ela contém espaços", () => {
    expect(separarSegmentos("```isto é código\nmais\n```")).toEqual([
      { tipo: "codigo", linguagem: "", conteudo: "isto é código\nmais\n" },
    ]);
  });
});
```

- [ ] **Step 5: Rodar e confirmar que falha**

```bash
cd frontend && npm test; cd ..
```

Esperado: falha de resolução, `Failed to resolve import "./markdown"`.

- [ ] **Step 6: Escrever a implementação mínima**

`frontend/src/lib/markdown.ts`:

```ts
export type Segmento =
  | { tipo: "texto"; conteudo: string }
  | { tipo: "codigo"; linguagem: string; conteudo: string };

/**
 * Divide uma resposta em segmentos de texto e blocos de código delimitados
 * por crase tripla. A primeira linha do bloco vira a linguagem quando é uma
 * única palavra curta.
 */
export function separarSegmentos(conteudo: string): Segmento[] {
  const partes = conteudo.split("```");
  const segmentos: Segmento[] = [];

  partes.forEach((parte, indice) => {
    const ehCodigo = indice % 2 === 1;

    if (!ehCodigo) {
      if (parte.trim() !== "") {
        segmentos.push({ tipo: "texto", conteudo: parte });
      }
      return;
    }

    const linhas = parte.split("\n");
    const primeira = linhas[0].trim();
    const ehLinguagem =
      linhas.length > 1 &&
      primeira !== "" &&
      !primeira.includes(" ") &&
      primeira.length < 20;

    segmentos.push({
      tipo: "codigo",
      linguagem: ehLinguagem ? primeira : "",
      conteudo: ehLinguagem ? linhas.slice(1).join("\n") : parte,
    });
  });

  return segmentos;
}
```

- [ ] **Step 7: Rodar e confirmar que passa**

```bash
cd frontend && npm test; cd ..
```

Esperado: 4 testes passando.

- [ ] **Step 8: Fazer o `App.tsx` usar a função extraída**

Substituir todo o corpo de `renderMessageContent` (linhas 21-51 do arquivo original) por:

```tsx
  const renderMessageContent = (content: string) =>
    separarSegmentos(content).map((segmento, indice) =>
      segmento.tipo === "texto" ? (
        <span key={indice}>{segmento.conteudo}</span>
      ) : (
        <pre className="code-block" key={indice}>
          <code className={segmento.linguagem}>{segmento.conteudo}</code>
        </pre>
      )
    );
```

O texto vai dentro de um `<span>` com `key` de propósito: devolver string crua
num array faz o React reclamar de chave ausente no console a cada render.

E adicionar o import no topo do arquivo:

```tsx
import { separarSegmentos } from "./lib/markdown";
```

- [ ] **Step 9: Confirmar que o build continua verde**

```bash
cd frontend && npm run build && npm test; cd ..
```

Esperado: build conclui e os 4 testes passam.

- [ ] **Step 10: Commit**

```bash
git add frontend/ && git commit -m "refactor: extrai o parser de blocos de código do App.tsx e adiciona vitest

Separa parsing de renderização, o que torna a lógica testável, e estabelece
o harness TypeScript do projeto. Atualiza Vite e plugin-react, que estavam
em versões de 2022 e incompatíveis com o vitest atual.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 7: PostgreSQL em Docker

**Files:**
- Create: `docker-compose.yml`
- Create: `internal/armazenamento/postgres/conexao.go`
- Create: `internal/armazenamento/postgres/conexao_test.go`

- [ ] **Step 1: Criar o `docker-compose.yml`**

Porta 5433 no host de propósito, para não colidir com um Postgres já instalado na 5432. A senha vem do `.env`, não fica no arquivo versionado.

```yaml
services:
  postgres:
    image: postgres:17-alpine
    container_name: james-postgres
    restart: unless-stopped
    environment:
      POSTGRES_USER: james
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      POSTGRES_DB: james
    ports:
      - "127.0.0.1:5433:5432"
    volumes:
      - james_pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U james -d james"]
      interval: 5s
      timeout: 3s
      retries: 10

volumes:
  james_pgdata:
```

- [ ] **Step 2: Subir o banco**

```bash
docker compose up -d
```

Confirmar que ficou saudável:

```bash
docker compose ps
```

Esperado: `james-postgres` com status `Up` e `(healthy)`.

- [ ] **Step 3: Escrever o teste que falha**

`internal/armazenamento/postgres/conexao_test.go`:

```go
package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
)

// TestMain carrega o .env da raiz para que os testes de integração enxerguem
// DATABASE_URL sem depender de o shell já ter exportado a variável.
func TestMain(m *testing.M) {
	_ = godotenv.Load("../../../.env")
	os.Exit(m.Run())
}

func TestAbrirRecusaURLVazia(t *testing.T) {
	if _, err := Abrir(context.Background(), ""); err == nil {
		t.Fatal("esperava erro para URL vazia, veio nil")
	}
}

func TestAbrirConectaNoPostgresLocal(t *testing.T) {
	url := URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}

	pool, err := Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	defer pool.Close()

	var um int
	if err := pool.QueryRow(context.Background(), "SELECT 1").Scan(&um); err != nil {
		t.Fatalf("query de sanidade falhou: %v", err)
	}
	if um != 1 {
		t.Errorf("SELECT 1 devolveu %d, queria 1", um)
	}
}
```

- [ ] **Step 4: Rodar e confirmar que falha**

```bash
go test ./internal/armazenamento/postgres/ -v
```

Esperado: falha de compilação, `undefined: Abrir` e `undefined: URLDoAmbiente`.

- [ ] **Step 5: Escrever a implementação mínima**

```bash
go get github.com/jackc/pgx/v5/pgxpool
```

`internal/armazenamento/postgres/conexao.go`:

```go
// Package postgres concentra o acesso ao banco do James.
package postgres

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Abrir cria um pool de conexões a partir da URL informada e confirma que o
// banco responde antes de devolver. Falhar aqui é melhor que falhar na
// primeira query, no meio de uma conversa.
func Abrir(ctx context.Context, url string) (*pgxpool.Pool, error) {
	if url == "" {
		return nil, fmt.Errorf("URL do banco vazia: defina DATABASE_URL")
	}

	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("erro ao criar o pool: %w", err)
	}

	ctxPing, cancelar := context.WithTimeout(ctx, 5*time.Second)
	defer cancelar()

	if err := pool.Ping(ctxPing); err != nil {
		pool.Close()
		return nil, fmt.Errorf("banco não respondeu: %w", err)
	}

	return pool, nil
}

// URLDoAmbiente devolve a URL de conexão configurada em DATABASE_URL.
func URLDoAmbiente() string {
	return os.Getenv("DATABASE_URL")
}
```

- [ ] **Step 6: Rodar e confirmar que passa**

```bash
go test ./internal/armazenamento/postgres/ -v
```

Esperado: os dois testes passam. Se `TestAbrirConectaNoPostgresLocal` aparecer como `SKIP`, o `.env` não tem `DATABASE_URL` ou a senha não bate com a do container — corrigir antes de seguir, um skip aqui não conta como verde.

- [ ] **Step 7: Commit**

```bash
git add docker-compose.yml internal/armazenamento/ go.mod go.sum && git commit -m "feat: adiciona PostgreSQL local e a camada de conexão com pgx

Compose expõe a 5433 no loopback para não colidir com um Postgres nativo.
Senha vem do .env, não do arquivo versionado.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 8: Spike do Tailscale

**Não é TDD** — é investigação para derrubar o maior risco da Fase 1 antes de escrever código que dependa dele. Produz um documento, não uma feature.

**Files:**
- Create: `docs/superpowers/spikes/2026-08-03-tailscale.md`
- Create (temporário): `spike/bind/main.go`

> **O binário do Tailscale não está no PATH do Git Bash.** Ele fica em
> `C:\Program Files\Tailscale\tailscale.exe`. Todos os comandos desta task usam
> o caminho completo. Para encurtar, exportar no início da sessão:
>
> ```bash
> export TS="/c/Program Files/Tailscale/tailscale.exe"
> ```

- [ ] **Step 1: Confirmar o endereço do tailnet**

```bash
"$TS" status
```

```bash
"$TS" ip -4
```

Já levantado em 2026-08-03 — confirmar que segue valendo:

| Dispositivo | Nome no tailnet | IPv4 | Sistema |
|---|---|---|---|
| PC | `desktop-dc358rs` | `100.96.219.29` | windows |
| Celular | `s21-fe-de-arthur` | `100.83.30.121` | android |

Anotar também o nome DNS completo do PC (`desktop-dc358rs.<tailnet>.ts.net`),
necessário no Step 3.

- [ ] **Step 2: Provar que dá para fazer bind só na interface da VPN**

Criar `spike/bind/main.go`:

```go
// Comando spike: confirma que é possível escutar apenas no endereço do
// tailnet, sem expor a porta na LAN. Descartado depois do spike.
package main

import (
	"fmt"
	"net/http"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("uso: go run ./spike/bind <ip-do-tailnet>")
		os.Exit(1)
	}
	endereco := os.Args[1] + ":8099"

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "james vivo; visto de %s\n", r.RemoteAddr)
	})

	fmt.Println("escutando em", endereco)
	if err := http.ListenAndServe(endereco, nil); err != nil {
		fmt.Println("erro:", err)
		os.Exit(1)
	}
}
```

Rodar com o IP confirmado no Step 1:

```bash
go run ./spike/bind 100.96.219.29
```

Do **celular** (`s21-fe-de-arthur`), abrir `http://100.96.219.29:8099`.
Anotar se respondeu e qual `RemoteAddr` a página mostrou — deve ser
`100.83.30.121`, provando que o tráfego veio pelo tailnet.

Depois, de outra máquina na LAN de casa que **não** esteja no tailnet, tentar o IP local do PC na porta 8099. **Esperado: recusa de conexão** — é isso que prova que o bind não vazou para a LAN.

- [ ] **Step 3: Verificar a emissão de certificado TLS**

```bash
"$TS" cert --help
```

Depois, com o nome DNS obtido no Step 1:

```bash
"$TS" cert desktop-dc358rs.<tailnet>.ts.net
```

Anotar: emitiu? Quais arquivos gerou? Deu erro pedindo habilitar HTTPS no painel do tailnet?

- [ ] **Step 4: Verificar o WebView2 contra HTTPS do tailnet**

Com o `wails dev` rodando, abrir o DevTools do WebView2 e navegar até a URL HTTPS do tailnet. Anotar se houve aviso de certificado ou bloqueio de conteúdo misto.

- [ ] **Step 5: Escrever as conclusões**

Criar `docs/superpowers/spikes/2026-08-03-tailscale.md` com uma seção por pergunta e a resposta observada:

1. Qual o IPv4 e o nome DNS do PC no tailnet?
2. O bind exclusivo na interface do tailnet funciona? O celular alcança? A LAN é recusada?
3. `tailscale cert` emite certificado válido? Que arquivos, em que caminho?
4. O WebView2 carrega HTTPS do tailnet sem aviso?
5. **Veredito:** o desenho da Fase 1 se sustenta, ou precisa de plano B?

Cada resposta acompanhada da saída real do comando. Sem "deve funcionar".

- [ ] **Step 6: Remover o código do spike**

```bash
rm -rf spike/
```

- [ ] **Step 7: Commit**

```bash
git add docs/superpowers/spikes/ && git commit -m "docs: registra o spike do Tailscale com bind, TLS e WebView2

Derruba o maior risco da Fase 1 antes de escrever código que dependa dele.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 9: Verificação final da Fase 0

Nada é declarado pronto sem esta task rodar inteira e a saída ser colada.

- [ ] **Step 1: Suíte Go completa**

```bash
go vet ./... && go test ./... -v
```

Esperado: exit 0. `internal/higiene` com 4 testes verdes, `internal/armazenamento/postgres` com 2 verdes e nenhum SKIP.

- [ ] **Step 2: Suíte Python**

```bash
cd dispositivo && python -m pytest -v; cd ..
```

Esperado: 12 testes verdes.

- [ ] **Step 3: Suíte TypeScript**

```bash
cd frontend && npm test; cd ..
```

Esperado: 4 testes verdes.

- [ ] **Step 4: O app legado ainda sobe**

```bash
go build -o /dev/null .
```

Esperado: exit 0.

- [ ] **Step 5: Histórico do git limpo**

```bash
git log -p | grep -cE "AIza[S]y|tvly[-]|nvapi[-]|sk-or-[v]1-"
```

Esperado: `0`.

- [ ] **Step 6: Revisão do diff**

Rodar `/code-review` e `/security-review` sobre o branch. Tratar os achados antes de declarar a fase encerrada.

- [ ] **Step 7: Atualizar o plano**

Marcar as checkboxes concluídas neste arquivo, anotar o que ficou aberto, e commitar.

---

## Critérios de conclusão

Espelham a Seção 9 da spec, restritos ao que a Fase 0 alcança:

| # | Critério | Como provar |
|---|---|---|
| 1 | Suíte Go verde | `go vet ./... && go test ./...` |
| 2 | Suíte Python verde | `python -m pytest` em `dispositivo/` |
| 3 | Suíte TypeScript verde | `npm test` em `frontend/` |
| 4 | Postgres de pé e alcançável | `docker compose ps` + teste de conexão sem SKIP |
| 5 | Nenhuma credencial na árvore | `TestRepositorioLimpo` verde |
| 6 | Nenhuma credencial no histórico | `git log -p \| grep -cE "AIza[S]y\|tvly[-]\|nvapi[-]\|sk-or-[v]1-"` devolve 0 |
| 7 | App legado ainda compila | `go build` exit 0 |
| 8 | Risco do Tailscale respondido | Documento de spike com saídas reais |

Os critérios 5, 6, 7, 8 e 9 da spec que dependem do `jamesd` rodando pertencem à Fase 1.

---

## O que esta fase deliberadamente não faz

- Não reestrutura o código legado. `app.go` continua com 574 linhas e sete responsabilidades — a cirurgia é a Fase 1.
- Não melhora comportamento nenhum. Busca, clique e Claude Code seguem exatamente como estão.
- Não cria o `jamesd`. O único binário continua sendo o app Wails atual.
- Não renomeia pastas nem troca a persona. O rename completo acontece na Fase 1; a Fase 0 só renomeia o módulo Go e cria `dispositivo/` já com o nome novo.

**Desvio consciente da spec:** a Seção 8 da spec lista `config.toml` na Fase 0.
Ele foi movido para a Fase 1. Criá-lo agora produziria um arquivo de configuração
que nenhum código lê — placeholder disfarçado. Ele nasce junto com o pacote
`internal/config`, que é quem o consome.
