# Fase 6 — Front-end novo: plano de implementação

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Substituir a interface do James por uma que mostre o que o sistema está fazendo — ferramenta em voo, tela capturada, estado de conexão, sessões retomáveis — para que a verificação deixe de precisar ler o Postgres direto.

**Architecture:** A SPA em `web/src` é reescrita no lugar, com o orbe como estado recolhido e um painel expandido de trilha vertical (cinco abas) mais visualizador reativo. O visualizador não tem estado próprio: deriva do turno. O Go recebe três mudanças pontuais — um endpoint novo para a memória, retomada de sessão no WebSocket, e a remoção de um evento duplicado.

**Tech Stack:** Go 1.x (`net/http`, `gorilla/websocket`, `pgx`), React 18, TypeScript, Vite, Tailwind, vitest, `@testing-library/react`, Playwright.

**Spec:** [`2026-08-09-james-fase-6-frontend-novo-design.md`](../specs/2026-08-09-james-fase-6-frontend-novo-design.md)

---

## Estrutura de arquivos

**Go — criados:**
- nenhum. As três mudanças são em arquivos existentes.

**Go — modificados:**
- `internal/memoria/vault.go` — ganha `NotaAssunto` e `Listar()`
- `internal/api/rest.go` — ganha a rota e o handler `GET /api/memoria`
- `internal/api/ws.go` — ganha `reconstruirHistorico` e o tipo de mensagem `retomar`; perde a emissão de `token`

**Front-end — criados:**

| Arquivo | Responsabilidade |
|---|---|
| `web/src/lib/eventos.ts` | tipos dos eventos WS, um lugar só |
| `web/src/estado/turno.ts` | redutor puro: eventos → estado do turno |
| `web/src/shell/orbe.tsx` | arraste e resize da janela nativa |
| `web/src/abas/conversa.tsx` | lista de mensagens e campo de envio |
| `web/src/abas/tela.tsx` | última captura de `ver_tela` |
| `web/src/abas/atividade.tsx` | histórico de ferramentas do turno |
| `web/src/abas/memoria.tsx` | notas do vault |
| `web/src/abas/sessoes.tsx` | lista e retomada de sessões |
| `web/src/visualizador.tsx` | painel direito derivado do turno |
| `web/src/trilha.tsx` | trilha vertical de abas |

**Front-end — modificados:**
- `web/src/lib/ws.ts` — reconexão, fila, estado de conexão
- `web/src/lib/api.ts` — ganha `lerMemoria`
- `web/src/App.tsx` — vira composição, perde as cinco responsabilidades
- `web/vite.config.ts`, `web/src/style.css` — Tailwind

**Front-end — removidos ao final:**
- `web/src/App.css`, `web/src/components/CardAprovacao.tsx` (absorvido pelo visualizador)

---

## Task 1: `Vault.Listar()` — notas estruturadas

**Files:**
- Modify: `internal/memoria/vault.go`
- Test: `internal/memoria/vault_test.go`

- [ ] **Step 1: Escrever o teste que falha**

Acrescente em `internal/memoria/vault_test.go`:

```go
func TestListarDevolveNotasPorAssuntoEmOrdem(t *testing.T) {
	raiz := t.TempDir()
	v := NovoVault(raiz, 2000)

	if err := v.Acrescentar("projetos", "James está na Fase 6"); err != nil {
		t.Fatalf("erro ao acrescentar: %v", err)
	}
	if err := v.Acrescentar("comida", "não gosta de coentro"); err != nil {
		t.Fatalf("erro ao acrescentar: %v", err)
	}

	notas, err := v.Listar()
	if err != nil {
		t.Fatalf("erro ao listar: %v", err)
	}
	if len(notas) != 2 {
		t.Fatalf("esperava 2 notas, veio %d", len(notas))
	}
	if notas[0].Assunto != "comida" || notas[1].Assunto != "projetos" {
		t.Errorf("ordem alfabética quebrada: %q, %q", notas[0].Assunto, notas[1].Assunto)
	}
	if !strings.Contains(notas[1].Conteudo, "Fase 6") {
		t.Errorf("conteúdo não veio: %q", notas[1].Conteudo)
	}
}

func TestListarComVaultAusenteNaoEhErro(t *testing.T) {
	v := NovoVault(filepath.Join(t.TempDir(), "nao-existe"), 2000)

	notas, err := v.Listar()
	if err != nil {
		t.Fatalf("vault ausente é primeiro boot, não erro: %v", err)
	}
	if len(notas) != 0 {
		t.Errorf("esperava lista vazia, veio %d", len(notas))
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/memoria/ -run TestListar -v`
Expected: FAIL — `v.Listar undefined (type *Vault has no field or method Listar)`

- [ ] **Step 3: Implementar**

Em `internal/memoria/vault.go`, logo depois de `Renderizar`:

```go
// NotaAssunto é uma nota do vault exposta para leitura externa (a aba
// Memória da Fase 6). Diferente de Renderizar, não aplica orçamento: o
// orçamento existe para caber no system prompt, e a interface mostra tudo.
type NotaAssunto struct {
	Assunto    string    `json:"assunto"`
	Conteudo   string    `json:"conteudo"`
	Modificado time.Time `json:"modificado"`
}

// Listar devolve todas as notas em ordem alfabética de assunto. Reusa
// carregarNotas para não duplicar a leitura de disco que Renderizar já faz.
func (v *Vault) Listar() ([]NotaAssunto, error) {
	notas, err := v.carregarNotas()
	if err != nil {
		return nil, err
	}
	sort.Slice(notas, func(i, j int) bool { return notas[i].assunto < notas[j].assunto })

	saida := make([]NotaAssunto, 0, len(notas))
	for _, n := range notas {
		saida = append(saida, NotaAssunto{
			Assunto:    n.assunto,
			Conteudo:   n.conteudo,
			Modificado: n.modificado,
		})
	}
	return saida, nil
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/memoria/ -v`
Expected: PASS, incluindo os testes antigos de `Renderizar`

- [ ] **Step 5: Commit**

```bash
git add internal/memoria/vault.go internal/memoria/vault_test.go
git commit -m "feat(memoria): Listar devolve notas estruturadas por assunto"
```

---

## Task 2: `GET /api/memoria`

**Files:**
- Modify: `internal/api/rest.go`
- Test: `internal/api/rest_test.go`

- [ ] **Step 1: Escrever o teste que falha**

Acrescente em `internal/api/rest_test.go`:

```go
func TestGetMemoriaDevolveNotas(t *testing.T) {
	raiz := t.TempDir()
	vault := memoria.NovoVault(raiz, 2000)
	if err := vault.Acrescentar("projetos", "James está na Fase 6"); err != nil {
		t.Fatalf("erro ao preparar vault: %v", err)
	}

	s := &Servidor{Notas: vault}
	req := httptest.NewRequest("GET", "/api/memoria", nil)
	rec := httptest.NewRecorder()
	s.Rotas().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	var corpo struct {
		Notas []memoria.NotaAssunto `json:"notas"`
		Aviso string                `json:"aviso"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatalf("resposta não é JSON válido: %v", err)
	}
	if len(corpo.Notas) != 1 || corpo.Notas[0].Assunto != "projetos" {
		t.Errorf("notas vieram erradas: %+v", corpo.Notas)
	}
}

func TestGetMemoriaSemVaultDevolve200EListaVazia(t *testing.T) {
	// Notas nil é o estado de todo teste anterior e do primeiro boot. A aba
	// não pode ser a única parte do sistema que trata vault ausente como 500.
	s := &Servidor{}
	req := httptest.NewRequest("GET", "/api/memoria", nil)
	rec := httptest.NewRecorder()
	s.Rotas().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, veio %d", rec.Code)
	}
	var corpo struct {
		Notas []memoria.NotaAssunto `json:"notas"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &corpo); err != nil {
		t.Fatalf("resposta não é JSON válido: %v", err)
	}
	if len(corpo.Notas) != 0 {
		t.Errorf("esperava lista vazia, veio %d", len(corpo.Notas))
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/api/ -run TestGetMemoria -v`
Expected: FAIL com 404 — a rota não existe

- [ ] **Step 3: Implementar**

Em `internal/api/rest.go`, dentro de `Rotas()`, logo depois da linha de `GET /api/sessoes/{id}/mensagens`:

```go
	mux.HandleFunc("GET /api/memoria", s.listarMemoria)
```

E o handler, junto dos outros:

```go
// listarMemoria alimenta a aba Memória. Nunca devolve 500 por vault
// quebrado: memória é melhor-esforço em todo o resto do sistema (ver o
// tratamento em processarTurno), e a aba segue a mesma regra — devolve
// lista vazia com aviso, para o front conseguir mostrar o problema sem
// tratar a resposta como falha de rede.
func (s *Servidor) listarMemoria(w http.ResponseWriter, r *http.Request) {
	type resposta struct {
		Notas []memoria.NotaAssunto `json:"notas"`
		Aviso string                `json:"aviso,omitempty"`
	}

	if s.Notas == nil {
		responderJSON(w, http.StatusOK, resposta{Notas: []memoria.NotaAssunto{}})
		return
	}

	notas, err := s.Notas.Listar()
	if err != nil {
		slog.Warn("erro ao ler memória para a aba", "erro", err)
		responderJSON(w, http.StatusOK, resposta{Notas: []memoria.NotaAssunto{}, Aviso: err.Error()})
		return
	}
	if notas == nil {
		notas = []memoria.NotaAssunto{}
	}
	responderJSON(w, http.StatusOK, resposta{Notas: notas})
}
```

Se `log/slog` ainda não estiver importado em `rest.go`, acrescente ao bloco de import.

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/api/ -run TestGetMemoria -v`
Expected: PASS nos dois

- [ ] **Step 5: Rodar a suíte Go inteira**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: tudo verde

- [ ] **Step 6: Commit**

```bash
git add internal/api/rest.go internal/api/rest_test.go
git commit -m "feat(api): GET /api/memoria alimenta a aba Memoria"
```

---

## Task 3: Reconstrução de histórico a partir do Postgres

Esta é a task de maior risco do plano (R1 da spec). A coluna `chamadas` guarda **ou** `Chamadas` **ou** `Resultados`, escolhido por qual dos dois está preenchido em `persistirNovasMensagens`. A desambiguação na volta é pelo papel da mensagem, nunca por tentativa de decodificar.

**Files:**
- Modify: `internal/api/ws.go`
- Test: `internal/api/ws_test.go`

- [ ] **Step 1: Escrever o teste que falha**

Acrescente em `internal/api/ws_test.go`:

```go
func TestReconstruirHistoricoDesambiguaPeloPapel(t *testing.T) {
	chamadas, err := json.Marshal([]agente.ChamadaFerramenta{{ID: "c1", Nome: "ver_tela"}})
	if err != nil {
		t.Fatalf("erro ao preparar chamadas: %v", err)
	}
	resultados, err := json.Marshal([]agente.ResultadoFerramenta{{ID: "c1", Nome: "ver_tela", Saida: "superficie: DISPLAY1"}})
	if err != nil {
		t.Fatalf("erro ao preparar resultados: %v", err)
	}

	linhas := []postgres.Mensagem{
		{Papel: string(agente.PapelUsuario), Conteudo: "o que tem na tela?"},
		{Papel: string(agente.PapelModelo), Chamadas: chamadas},
		{Papel: string(agente.PapelFerramenta), Chamadas: resultados},
		{Papel: string(agente.PapelModelo), Conteudo: "tem a calculadora aberta"},
	}

	historico, err := reconstruirHistorico(linhas)
	if err != nil {
		t.Fatalf("erro ao reconstruir: %v", err)
	}
	if len(historico) != 4 {
		t.Fatalf("esperava 4 mensagens, veio %d", len(historico))
	}

	// A mensagem do modelo com a coluna preenchida tem que virar Chamadas,
	// não Resultados — e vice-versa. Trocar os dois produz histórico que o
	// Gemini aceita com semântica errada, que é o modo de falha silencioso.
	if len(historico[1].Chamadas) != 1 || historico[1].Chamadas[0].Nome != "ver_tela" {
		t.Errorf("papel modelo devia virar Chamadas: %+v", historico[1])
	}
	if len(historico[1].Resultados) != 0 {
		t.Errorf("papel modelo não pode produzir Resultados: %+v", historico[1])
	}
	if len(historico[2].Resultados) != 1 || historico[2].Resultados[0].Saida != "superficie: DISPLAY1" {
		t.Errorf("papel ferramenta devia virar Resultados: %+v", historico[2])
	}
	if len(historico[2].Chamadas) != 0 {
		t.Errorf("papel ferramenta não pode produzir Chamadas: %+v", historico[2])
	}
	if historico[0].Texto != "o que tem na tela?" || historico[0].Papel != agente.PapelUsuario {
		t.Errorf("mensagem de usuário veio errada: %+v", historico[0])
	}
}

func TestReconstruirHistoricoRejeitaColunaCorrompida(t *testing.T) {
	linhas := []postgres.Mensagem{
		{Papel: string(agente.PapelModelo), Chamadas: []byte("{isso não é json")},
	}

	if _, err := reconstruirHistorico(linhas); err == nil {
		t.Fatal("esperava erro, veio nil — coluna ilegível não pode virar histórico mudo")
	}
}

func TestReconstruirHistoricoIgnoraColunaVazia(t *testing.T) {
	linhas := []postgres.Mensagem{
		{Papel: string(agente.PapelModelo), Conteudo: "oi", Chamadas: nil},
		{Papel: string(agente.PapelModelo), Conteudo: "oi de novo", Chamadas: []byte{}},
	}

	historico, err := reconstruirHistorico(linhas)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	for i, m := range historico {
		if len(m.Chamadas) != 0 || len(m.Resultados) != 0 {
			t.Errorf("linha %d não devia ter chamadas nem resultados: %+v", i, m)
		}
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/api/ -run TestReconstruirHistorico -v`
Expected: FAIL — `undefined: reconstruirHistorico`

- [ ] **Step 3: Implementar**

Em `internal/api/ws.go`, junto de `persistirNovasMensagens`:

```go
// reconstruirHistorico faz o caminho de volta de persistirNovasMensagens:
// converte linhas da tabela `mensagens` em []agente.Mensagem para retomar
// uma sessão anterior.
//
// A coluna `chamadas` é polimórfica de propósito — persistirNovasMensagens
// grava ali OU m.Chamadas OU m.Resultados, o que estiver preenchido. Na
// volta, a desambiguação é pelo papel, nunca por tentativa de decodificar
// nos dois formatos: ChamadaFerramenta e ResultadoFerramenta compartilham
// ID e Nome, então uma tentativa de decodificar teria sucesso nos dois e
// escolheria o errado em silêncio.
func reconstruirHistorico(linhas []postgres.Mensagem) ([]agente.Mensagem, error) {
	historico := make([]agente.Mensagem, 0, len(linhas))

	for i, linha := range linhas {
		mensagem := agente.Mensagem{
			Papel: agente.Papel(linha.Papel),
			Texto: linha.Conteudo,
		}

		if len(linha.Chamadas) > 0 {
			switch mensagem.Papel {
			case agente.PapelFerramenta:
				if err := json.Unmarshal(linha.Chamadas, &mensagem.Resultados); err != nil {
					return nil, fmt.Errorf("linha %d (papel %q): resultados ilegíveis: %w", i, linha.Papel, err)
				}
			case agente.PapelModelo:
				if err := json.Unmarshal(linha.Chamadas, &mensagem.Chamadas); err != nil {
					return nil, fmt.Errorf("linha %d (papel %q): chamadas ilegíveis: %w", i, linha.Papel, err)
				}
			}
		}

		historico = append(historico, mensagem)
	}

	return historico, nil
}
```

Acrescente `"fmt"` e `"james/internal/armazenamento/postgres"` aos imports de `ws.go` se ainda não estiverem lá.

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/api/ -run TestReconstruirHistorico -v`
Expected: PASS nos três

- [ ] **Step 5: Commit**

```bash
git add internal/api/ws.go internal/api/ws_test.go
git commit -m "feat(api): reconstroi historico da sessao a partir do Postgres"
```

---

## Task 4: Retomada de sessão pelo WebSocket

**Files:**
- Modify: `internal/api/ws.go`
- Test: `internal/api/ws_test.go`

- [ ] **Step 1: Escrever o teste que falha**

```go
func TestRetomarCarregaSessaoEHistorico(t *testing.T) {
	sessao := uuid.New()
	linhas := []postgres.Mensagem{
		{Papel: string(agente.PapelUsuario), Conteudo: "lembra do bolo?"},
		{Papel: string(agente.PapelModelo), Conteudo: "lembro, de cenoura"},
	}

	var sessaoID uuid.UUID
	var historico []agente.Mensagem

	if err := aplicarRetomada(sessao, linhas, &sessaoID, &historico); err != nil {
		t.Fatalf("erro ao retomar: %v", err)
	}
	if sessaoID != sessao {
		t.Errorf("sessaoID não foi adotada: %v", sessaoID)
	}
	if len(historico) != 2 || historico[1].Texto != "lembro, de cenoura" {
		t.Errorf("histórico veio errado: %+v", historico)
	}
}

func TestRetomadaComColunaCorrompidaNaoSujaOEstado(t *testing.T) {
	// Se a reconstrução falha, a conexão precisa continuar utilizável como
	// sessão nova — não pode ficar com sessaoID adotada e histórico pela
	// metade, que é o estado que produziria contexto silenciosamente errado.
	linhas := []postgres.Mensagem{
		{Papel: string(agente.PapelModelo), Chamadas: []byte("{quebrado")},
	}

	var sessaoID uuid.UUID
	var historico []agente.Mensagem

	if err := aplicarRetomada(uuid.New(), linhas, &sessaoID, &historico); err == nil {
		t.Fatal("esperava erro")
	}
	if sessaoID != uuid.Nil {
		t.Errorf("sessaoID não podia ter sido adotada: %v", sessaoID)
	}
	if len(historico) != 0 {
		t.Errorf("histórico devia continuar vazio: %+v", historico)
	}
}
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/api/ -run TestRetom -v`
Expected: FAIL — `undefined: aplicarRetomada`

- [ ] **Step 3: Implementar a função pura**

Em `internal/api/ws.go`:

```go
// aplicarRetomada adota uma sessão existente na conexão. Só escreve nos
// ponteiros depois de a reconstrução ter dado certo: falhar no meio deixaria
// a conexão com sessão adotada e histórico parcial, que é pior que não
// retomar — o modelo receberia um contexto truncado sem ninguém saber.
func aplicarRetomada(sessao uuid.UUID, linhas []postgres.Mensagem, sessaoID *uuid.UUID, historico *[]agente.Mensagem) error {
	reconstruido, err := reconstruirHistorico(linhas)
	if err != nil {
		return err
	}
	*sessaoID = sessao
	*historico = reconstruido
	return nil
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `go test ./internal/api/ -run TestRetom -v`
Expected: PASS nos dois

- [ ] **Step 5: Ligar ao protocolo WS**

Em `internal/api/ws.go`, na goroutine de leitura, acrescente o caso ao `switch`:

```go
			case "retomar":
				mensagens <- envelope
```

E no laço sequencial, troque `s.tratarMensagem(...)` por um despacho por tipo:

```go
	for envelope := range mensagens {
		switch envelope.Tipo {
		case "retomar":
			s.tratarRetomar(r.Context(), cliente, envelope, &sessaoID, &historico)
		default:
			s.tratarMensagem(r.Context(), cliente, envelope, &sessaoID, &historico)
		}
	}
```

E o handler novo:

```go
// tratarRetomar roda no laço sequencial, junto com "mensagem", porque mexe
// em sessaoID e historico — o mesmo motivo que mantém "mensagem" fora da
// goroutine de leitura.
func (s *Servidor) tratarRetomar(ctx context.Context, cliente *conexaoCliente, envelope Envelope, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	var dados struct {
		SessaoID string `json:"sessao_id"`
	}
	if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": "corpo inválido"})
		return
	}

	sessao, err := uuid.Parse(dados.SessaoID)
	if err != nil {
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": "id de sessão inválido"})
		return
	}

	linhas, err := s.Mensagens.ListarPorSessao(ctx, sessao, 200)
	if err != nil {
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": err.Error()})
		return
	}

	if err := aplicarRetomada(sessao, linhas, sessaoID, historico); err != nil {
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": "não consegui reconstruir a conversa: " + err.Error()})
		return
	}

	cliente.Enviar("sessao_retomada", envelope.ID, map[string]any{
		"sessao_id": sessao.String(),
		"mensagens": len(linhas),
	})
}
```

- [ ] **Step 6: Rodar a suíte Go**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: verde

- [ ] **Step 7: Commit**

```bash
git add internal/api/ws.go internal/api/ws_test.go
git commit -m "feat(api): WS aceita retomar sessao existente"
```

---

## Task 5: Remover o evento `token`

`token` manda `resp.Texto` inteiro imediatamente antes de `resposta_final` mandar o mesmo conteúdo. Sem streaming (D4 da spec), é um evento duplicado com nome que promete o que não entrega.

**Files:**
- Modify: `internal/api/ws.go:101`
- Test: `internal/api/ws_test.go`

- [ ] **Step 1: Procurar quem depende do evento**

Run: `grep -rn '"token"' --include="*.go" --include="*.ts" --include="*.tsx" .`
Expected: só `internal/api/ws.go:101` e, possivelmente, uma asserção em `ws_test.go`. Se aparecer em teste, esse teste é atualizado no Step 2.

- [ ] **Step 2: Ajustar o teste que fixa a sequência de eventos**

Se houver teste afirmando que `token` é emitido, remova a asserção. Acrescente a asserção inversa:

```go
func TestTurnoNaoEmiteToken(t *testing.T) {
	// Sem streaming, "token" duplicava resposta_final com o mesmo conteúdo.
	// Este teste existe para o evento não voltar por engano quando alguém
	// implementar streaming de verdade sem revisar o formato.
	// Monte o servidor como os outros testes de turno deste arquivo fazem e
	// capture os eventos enviados ao cliente falso.
	eventos := capturarEventosDeUmTurno(t, "oi")
	for _, e := range eventos {
		if e.Tipo == "token" {
			t.Fatalf("evento token voltou: %+v", eventos)
		}
	}
}
```

Reuse o helper de cliente falso que `ws_test.go` já usa (`ClienteConectado` é interface, então o teste existente já tem um duplo). Se o nome do helper for outro, use o do arquivo.

- [ ] **Step 3: Rodar e ver falhar**

Run: `go test ./internal/api/ -run TestTurnoNaoEmiteToken -v`
Expected: FAIL — o evento ainda é emitido

- [ ] **Step 4: Remover a linha**

Em `internal/api/ws.go`, apague:

```go
	cliente.Enviar("token", envelope.ID, map[string]string{"texto": resp.Texto})
```

- [ ] **Step 5: Rodar e ver passar**

Run: `go test ./internal/api/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/api/ws.go internal/api/ws_test.go
git commit -m "refactor(api): remove evento token, que duplicava resposta_final"
```

---

## Task 6: Tailwind

**Files:**
- Modify: `web/package.json`, `web/vite.config.ts`, `web/src/style.css`

- [ ] **Step 1: Instalar**

```bash
npm install --prefix web -D tailwindcss @tailwindcss/vite
```

- [ ] **Step 2: Ligar o plugin**

`web/vite.config.ts` inteiro:

```ts
import {defineConfig} from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  test: {
    environment: 'jsdom',
    setupFiles: ['./vitest.setup.ts'],
  },
})
```

- [ ] **Step 3: Importar no CSS de entrada**

Acrescente como **primeira linha** de `web/src/style.css`:

```css
@import "tailwindcss";
```

`App.css` continua importado por `App.tsx` e não é tocado ainda — R3 da spec: o arquivo antigo só sai quando a última aba deixar de depender dele.

- [ ] **Step 4: Provar que a classe utilitária chega ao build**

Em `web/src/App.tsx`, no `div.container`, acrescente `data-tw-teste` e a classe `text-[13px]`:

```tsx
    <div className="container text-[13px]" data-tw-teste>
```

Run: `npm run build --prefix web && grep -c "text-\[13px\]" web/dist/assets/*.css`
Expected: pelo menos 1 — se der 0, o Tailwind não está varrendo os arquivos; consulte a documentação da versão instalada pelo context7 (MCP) antes de seguir.

- [ ] **Step 5: Desfazer a marcação de teste**

Volte `App.tsx` para `<div className="container">`, sem `data-tw-teste`.

- [ ] **Step 6: Rodar os testes do front**

Run: `npm test --prefix web`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add web/package.json web/package-lock.json web/vite.config.ts web/src/style.css
git commit -m "chore(web): instala Tailwind e liga o plugin do Vite"
```

---

## Task 7: `lib/eventos.ts` — tipos dos eventos

**Files:**
- Create: `web/src/lib/eventos.ts`
- Test: `web/src/lib/eventos.test.ts`

- [ ] **Step 1: Escrever o teste que falha**

```ts
import { describe, expect, it } from "vitest";
import { EVENTOS_CONHECIDOS, ehEventoConhecido } from "./eventos";

describe("eventos", () => {
  it("conhece exatamente os eventos que o backend emite hoje", () => {
    // Se o backend ganhar ou perder um evento, este teste é o lugar onde a
    // divergência aparece antes de virar tela em branco. `token` foi
    // removido do ws.go na Fase 6 e não pode voltar para cá sem decisão.
    expect([...EVENTOS_CONHECIDOS].sort()).toEqual([
      "aprovacao_requerida",
      "audio_pronto",
      "erro",
      "ferramenta_concluida",
      "ferramenta_iniciada",
      "resposta_final",
      "sessao_retomada",
    ]);
  });

  it("reconhece evento conhecido e rejeita desconhecido", () => {
    expect(ehEventoConhecido("resposta_final")).toBe(true);
    expect(ehEventoConhecido("token")).toBe(false);
    expect(ehEventoConhecido("qualquer_coisa")).toBe(false);
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `npm test --prefix web -- eventos`
Expected: FAIL — módulo não existe

- [ ] **Step 3: Implementar**

`web/src/lib/eventos.ts`:

```ts
// Tipos dos eventos que o jamesd emite pelo WebSocket. Um lugar só: antes
// da Fase 6 cada handler inventava sua própria forma inline em App.tsx, e
// três dos sete eventos simplesmente não tinham tratamento nenhum.

export interface DadosRespostaFinal {
  texto: string;
}

export interface DadosErro {
  mensagem: string;
}

export interface DadosAudioPronto {
  id: string;
}

export interface DadosFerramentaIniciada {
  nome: string;
}

export interface DadosFerramentaConcluida {
  nome: string;
  erro: string;
}

export interface DadosAprovacaoRequerida {
  // O id vem duplicado dentro de `dados` de propósito: o cliente WS só
  // repassa `dados` aos manipuladores, então o id do envelope não chega
  // sozinho ao handler. Ver o comentário em internal/api/ws.go.
  id: string;
  ferramenta: string;
  argumentos: Record<string, unknown>;
}

export interface DadosSessaoRetomada {
  sessao_id: string;
  mensagens: number;
}

export const EVENTOS_CONHECIDOS = [
  "resposta_final",
  "erro",
  "audio_pronto",
  "ferramenta_iniciada",
  "ferramenta_concluida",
  "aprovacao_requerida",
  "sessao_retomada",
] as const;

export type TipoEvento = (typeof EVENTOS_CONHECIDOS)[number];

export function ehEventoConhecido(tipo: string): tipo is TipoEvento {
  return (EVENTOS_CONHECIDOS as readonly string[]).includes(tipo);
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `npm test --prefix web -- eventos`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/eventos.ts web/src/lib/eventos.test.ts
git commit -m "feat(web): tipos dos eventos WS num lugar so"
```

---

## Task 8: `lib/ws.ts` — reconexão, fila e estado de conexão

É esta task que ataca 1.3 da spec: hoje o socket cai e a página fica muda.

**Files:**
- Modify: `web/src/lib/ws.ts`
- Test: `web/src/lib/ws.test.ts`

- [ ] **Step 1: Escrever os testes que falham**

Acrescente em `web/src/lib/ws.test.ts` (mantenha os testes existentes de despacho por tipo):

```ts
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ClienteWS } from "./ws";

class SocketFalso {
  static ultimo: SocketFalso | null = null;
  static abertos = 0;

  onopen: (() => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  onmessage: ((e: { data: string }) => void) | null = null;
  enviados: string[] = [];
  fechado = false;

  constructor(public url: string) {
    SocketFalso.ultimo = this;
    SocketFalso.abertos++;
  }
  send(dado: string) {
    if (this.fechado) throw new Error("socket fechado");
    this.enviados.push(dado);
  }
  close() {
    this.fechado = true;
    this.onclose?.();
  }
  abrir() {
    this.onopen?.();
  }
}

describe("ClienteWS resiliente", () => {
  beforeEach(() => {
    SocketFalso.ultimo = null;
    SocketFalso.abertos = 0;
    vi.stubGlobal("WebSocket", SocketFalso as unknown as typeof WebSocket);
    vi.useFakeTimers();
  });

  it("enfileira envio feito antes de abrir e descarrega ao abrir", () => {
    const cliente = new ClienteWS("ws://teste/ws");
    cliente.enviar("mensagem", "id1", { texto: "oi" });

    expect(SocketFalso.ultimo!.enviados).toHaveLength(0);

    SocketFalso.ultimo!.abrir();

    expect(SocketFalso.ultimo!.enviados).toHaveLength(1);
    expect(JSON.parse(SocketFalso.ultimo!.enviados[0])).toEqual({
      tipo: "mensagem",
      id: "id1",
      dados: { texto: "oi" },
    });
  });

  it("reconecta com backoff depois de uma queda", () => {
    const cliente = new ClienteWS("ws://teste/ws");
    SocketFalso.ultimo!.abrir();
    expect(SocketFalso.abertos).toBe(1);

    SocketFalso.ultimo!.close();
    expect(SocketFalso.abertos).toBe(1);

    vi.advanceTimersByTime(1000);
    expect(SocketFalso.abertos).toBe(2);

    cliente.fechar();
  });

  it("publica o estado da conexão", () => {
    const estados: string[] = [];
    const cliente = new ClienteWS("ws://teste/ws");
    cliente.aoMudarEstado((e) => estados.push(e));

    SocketFalso.ultimo!.abrir();
    SocketFalso.ultimo!.close();

    expect(estados).toContain("conectado");
    expect(estados).toContain("reconectando");

    cliente.fechar();
  });

  it("fechar de propósito não reconecta", () => {
    const cliente = new ClienteWS("ws://teste/ws");
    SocketFalso.ultimo!.abrir();
    cliente.fechar();

    vi.advanceTimersByTime(30000);

    expect(SocketFalso.abertos).toBe(1);
  });

  it("ignora evento desconhecido sem lançar", () => {
    const cliente = new ClienteWS("ws://teste/ws");
    SocketFalso.ultimo!.abrir();

    expect(() =>
      SocketFalso.ultimo!.onmessage!({ data: JSON.stringify({ tipo: "inventado", id: "x", dados: {} }) })
    ).not.toThrow();

    cliente.fechar();
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `npm test --prefix web -- ws`
Expected: FAIL — não há fila, nem reconexão, nem `aoMudarEstado`

- [ ] **Step 3: Implementar**

`web/src/lib/ws.ts` inteiro:

```ts
import { ehEventoConhecido } from "./eventos";

export interface Envelope {
  tipo: string;
  id: string;
  dados: unknown;
}

export type EstadoConexao = "conectando" | "conectado" | "reconectando" | "fechado";

type Manipulador = (dados: any) => void;
type ObservadorEstado = (estado: EstadoConexao, tentativa: number) => void;

const ESPERA_INICIAL_MS = 1000;
const ESPERA_MAXIMA_MS = 15000;

// ClienteWS existia com 35 linhas e sem onclose, onerror ou reconexão: o
// socket caía e a página ficava muda, sem nada na tela. Era o candidato
// direto para as respostas que completavam no backend e nunca apareciam.
export class ClienteWS {
  private socket: WebSocket | null = null;
  private manipuladores = new Map<string, Manipulador[]>();
  private observadores: ObservadorEstado[] = [];
  private fila: Envelope[] = [];
  private tentativa = 0;
  private encerrado = false;
  private timer: ReturnType<typeof setTimeout> | null = null;

  constructor(private url: string) {
    this.conectar();
  }

  private conectar() {
    this.publicar(this.tentativa === 0 ? "conectando" : "reconectando");
    const socket = new WebSocket(this.url);
    this.socket = socket;

    socket.onopen = () => {
      this.tentativa = 0;
      this.publicar("conectado");
      this.descarregarFila();
    };

    socket.onmessage = (evento: MessageEvent) => {
      let envelope: Envelope;
      try {
        envelope = JSON.parse(evento.data);
      } catch {
        console.warn("envelope ilegível do jamesd", evento.data);
        return;
      }
      // Evento desconhecido nunca derruba a tela: o backend pode ganhar um
      // evento novo antes de a SPA ser atualizada.
      if (!ehEventoConhecido(envelope.tipo)) {
        console.warn("evento desconhecido, ignorado:", envelope.tipo);
        return;
      }
      for (const fn of this.manipuladores.get(envelope.tipo) ?? []) fn(envelope.dados);
    };

    // onerror sempre é seguido de onclose. A reconexão é agendada só lá,
    // para não haver dois caminhos agendando o mesmo timer.
    socket.onerror = () => {};

    socket.onclose = () => {
      if (this.encerrado) return;
      this.agendarReconexao();
    };
  }

  private agendarReconexao() {
    this.tentativa++;
    const espera = Math.min(ESPERA_INICIAL_MS * 2 ** (this.tentativa - 1), ESPERA_MAXIMA_MS);
    this.publicar("reconectando");
    this.timer = setTimeout(() => this.conectar(), espera);
  }

  private descarregarFila() {
    const pendentes = this.fila;
    this.fila = [];
    for (const envelope of pendentes) this.escrever(envelope);
  }

  private escrever(envelope: Envelope) {
    if (this.socket?.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify(envelope));
      return;
    }
    this.fila.push(envelope);
  }

  private publicar(estado: EstadoConexao) {
    for (const fn of this.observadores) fn(estado, this.tentativa);
  }

  on(tipo: string, manipulador: Manipulador) {
    const lista = this.manipuladores.get(tipo) ?? [];
    lista.push(manipulador);
    this.manipuladores.set(tipo, lista);
  }

  aoMudarEstado(observador: ObservadorEstado) {
    this.observadores.push(observador);
  }

  // enviar nunca lança: mensagem feita com o socket fechado espera a
  // reconexão em vez de virar exceção não tratada, que era o que acontecia
  // antes e deixava o campo de texto aparentemente sem efeito.
  enviar(tipo: string, id: string, dados: unknown) {
    this.escrever({ tipo, id, dados });
  }

  fechar() {
    this.encerrado = true;
    if (this.timer) clearTimeout(this.timer);
    this.publicar("fechado");
    this.socket?.close();
  }
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `npm test --prefix web -- ws`
Expected: PASS em todos

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/ws.ts web/src/lib/ws.test.ts
git commit -m "feat(web): ClienteWS com reconexao, fila de envio e estado"
```

---

## Task 9: `estado/turno.ts` — redutor puro

**Files:**
- Create: `web/src/estado/turno.ts`
- Test: `web/src/estado/turno.test.ts`

- [ ] **Step 1: Escrever o teste que falha**

```ts
import { describe, expect, it } from "vitest";
import { estadoInicial, reduzir } from "./turno";

describe("estado do turno", () => {
  it("registra a mensagem enviada e marca o turno em andamento", () => {
    const estado = reduzir(estadoInicial, { tipo: "envio_local", texto: "abre a calculadora" });

    expect(estado.mensagens).toHaveLength(1);
    expect(estado.mensagens[0]).toMatchObject({ papel: "usuario", texto: "abre a calculadora" });
    expect(estado.emAndamento).toBe(true);
  });

  it("abre e fecha ferramenta em voo", () => {
    let estado = reduzir(estadoInicial, { tipo: "envio_local", texto: "olha a tela" });
    estado = reduzir(estado, { tipo: "ferramenta_iniciada", dados: { nome: "ver_tela" } });

    expect(estado.ferramentas).toHaveLength(1);
    expect(estado.ferramentas[0]).toMatchObject({ nome: "ver_tela", estado: "voando" });

    estado = reduzir(estado, { tipo: "ferramenta_concluida", dados: { nome: "ver_tela", erro: "" } });

    expect(estado.ferramentas[0].estado).toBe("concluida");
  });

  it("marca ferramenta com erro", () => {
    let estado = reduzir(estadoInicial, { tipo: "ferramenta_iniciada", dados: { nome: "executar_comando_sistema" } });
    estado = reduzir(estado, {
      tipo: "ferramenta_concluida",
      dados: { nome: "executar_comando_sistema", erro: "ação negada pelo usuário" },
    });

    expect(estado.ferramentas[0].estado).toBe("erro");
    expect(estado.ferramentas[0].erro).toBe("ação negada pelo usuário");
  });

  it("resposta final encerra o turno e acrescenta a mensagem", () => {
    let estado = reduzir(estadoInicial, { tipo: "envio_local", texto: "oi" });
    estado = reduzir(estado, { tipo: "resposta_final", dados: { texto: "oi, Arthur" } });

    expect(estado.emAndamento).toBe(false);
    expect(estado.mensagens[1]).toMatchObject({ papel: "james", texto: "oi, Arthur" });
  });

  it("erro encerra o turno e vira mensagem visível", () => {
    let estado = reduzir(estadoInicial, { tipo: "envio_local", texto: "oi" });
    estado = reduzir(estado, { tipo: "erro", dados: { mensagem: "deu ruim" } });

    expect(estado.emAndamento).toBe(false);
    expect(estado.mensagens[1].texto).toContain("deu ruim");
  });

  it("guarda a aprovação pendente e a limpa quando decidida", () => {
    let estado = reduzir(estadoInicial, {
      tipo: "aprovacao_requerida",
      dados: { id: "a1", ferramenta: "executar_comando_sistema", argumentos: { comando: "rm -rf" } },
    });

    expect(estado.aprovacao?.id).toBe("a1");

    estado = reduzir(estado, { tipo: "aprovacao_decidida" });

    expect(estado.aprovacao).toBeNull();
  });

  it("é puro: não muta o estado recebido", () => {
    const antes = estadoInicial;
    reduzir(antes, { tipo: "envio_local", texto: "oi" });

    expect(antes.mensagens).toHaveLength(0);
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `npm test --prefix web -- turno`
Expected: FAIL — módulo não existe

- [ ] **Step 3: Implementar**

`web/src/estado/turno.ts`:

```ts
import type {
  DadosAprovacaoRequerida,
  DadosErro,
  DadosFerramentaConcluida,
  DadosFerramentaIniciada,
  DadosRespostaFinal,
} from "../lib/eventos";

export interface Mensagem {
  papel: "usuario" | "james";
  texto: string;
}

export interface FerramentaNoTurno {
  nome: string;
  estado: "voando" | "concluida" | "erro";
  erro?: string;
  iniciadaEm: number;
}

export interface EstadoTurno {
  mensagens: Mensagem[];
  ferramentas: FerramentaNoTurno[];
  aprovacao: DadosAprovacaoRequerida | null;
  emAndamento: boolean;
}

export type Acao =
  | { tipo: "envio_local"; texto: string }
  | { tipo: "resposta_final"; dados: DadosRespostaFinal }
  | { tipo: "erro"; dados: DadosErro }
  | { tipo: "ferramenta_iniciada"; dados: DadosFerramentaIniciada }
  | { tipo: "ferramenta_concluida"; dados: DadosFerramentaConcluida }
  | { tipo: "aprovacao_requerida"; dados: DadosAprovacaoRequerida }
  | { tipo: "aprovacao_decidida" }
  | { tipo: "historico_carregado"; mensagens: Mensagem[] };

export const estadoInicial: EstadoTurno = {
  mensagens: [],
  ferramentas: [],
  aprovacao: null,
  emAndamento: false,
};

// reduzir é função pura de propósito: todo o comportamento de turno fica
// testável sem React, sem socket e sem DOM. O componente só liga eventos a
// esta função.
export function reduzir(estado: EstadoTurno, acao: Acao): EstadoTurno {
  switch (acao.tipo) {
    case "envio_local":
      return {
        ...estado,
        mensagens: [...estado.mensagens, { papel: "usuario", texto: acao.texto }],
        ferramentas: [],
        emAndamento: true,
      };

    case "resposta_final":
      return {
        ...estado,
        mensagens: [...estado.mensagens, { papel: "james", texto: acao.dados.texto }],
        emAndamento: false,
      };

    case "erro":
      return {
        ...estado,
        mensagens: [...estado.mensagens, { papel: "james", texto: `Erro: ${acao.dados.mensagem}` }],
        emAndamento: false,
      };

    case "ferramenta_iniciada":
      return {
        ...estado,
        ferramentas: [
          ...estado.ferramentas,
          { nome: acao.dados.nome, estado: "voando", iniciadaEm: Date.now() },
        ],
      };

    case "ferramenta_concluida": {
      let fechou = false;
      const ferramentas = estado.ferramentas.map((f) => {
        if (fechou || f.nome !== acao.dados.nome || f.estado !== "voando") return f;
        fechou = true;
        return {
          ...f,
          estado: acao.dados.erro ? ("erro" as const) : ("concluida" as const),
          erro: acao.dados.erro || undefined,
        };
      });
      return { ...estado, ferramentas };
    }

    case "aprovacao_requerida":
      return { ...estado, aprovacao: acao.dados };

    case "aprovacao_decidida":
      return { ...estado, aprovacao: null };

    case "historico_carregado":
      return { ...estado, mensagens: acao.mensagens, ferramentas: [], emAndamento: false };
  }
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `npm test --prefix web -- turno`
Expected: PASS em todos

- [ ] **Step 5: Commit**

```bash
git add web/src/estado/turno.ts web/src/estado/turno.test.ts
git commit -m "feat(web): estado do turno como redutor puro"
```

---

## Task 10: `shell/orbe.tsx` — mover o arraste sem reescrever

Os comentários em `App.tsx:35-93` documentam bugs já pagos: clique rápido deixando `arrastando` preso em true, e mouseup chegando antes da resposta de `posicaoJanela()`. **A lógica sai intacta.** Só muda de arquivo.

**Files:**
- Create: `web/src/shell/orbe.tsx`
- Test: `web/src/shell/orbe.test.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Criar o arquivo movendo o código**

Copie de `App.tsx` o bloco `LIMIAR_ARRASTE_PX` mais `useArrastarJanela` inteiro, **com todos os comentários**, para `web/src/shell/orbe.tsx`, e acrescente o componente:

```tsx
import { useEffect, useRef } from "react";
import qfLogo from "../assets/images/QF.png";
import { moverJanela, posicaoJanela, redimensionarJanela } from "../lib/janela";

// LIMIAR_ARRASTE_PX e useArrastarJanela vêm de App.tsx sem alteração de
// lógica. Os comentários longos abaixo documentam bugs reais já corrigidos
// na Fase 1 — reescrever do zero seria recomprá-los.
// [COLE AQUI o bloco LIMIAR_ARRASTE_PX + useArrastarJanela de App.tsx,
//  trocando apenas o caminho dos imports]

export const TAMANHO_RECOLHIDO = 100;
export const LARGURA_EXPANDIDA = 1100;
export const ALTURA_EXPANDIDA = 900;

interface Props {
  aberto: boolean;
  aoAlternar: (aberto: boolean) => void;
}

export function Orbe({ aberto, aoAlternar }: Props) {
  const { aoIniciarArraste, houveArraste } = useArrastarJanela();

  const alternar = () => {
    // Se acabou de arrastar a janela, esse clique é o mouseup do arraste,
    // não uma intenção de abrir/fechar o chat — ignora.
    if (houveArraste.current) return;
    const novoEstado = !aberto;
    aoAlternar(novoEstado);
    redimensionarJanela(
      novoEstado ? LARGURA_EXPANDIDA : TAMANHO_RECOLHIDO,
      novoEstado ? ALTURA_EXPANDIDA : TAMANHO_RECOLHIDO
    );
  };

  return (
    <div className="drag-handle" onMouseDown={aoIniciarArraste} onClick={alternar}>
      <div className="sphere-visual">
        <img src={qfLogo} className="img_qf" alt="James" />
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Escrever o teste**

`web/src/shell/orbe.test.tsx`:

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ALTURA_EXPANDIDA, LARGURA_EXPANDIDA, Orbe, TAMANHO_RECOLHIDO } from "./orbe";

vi.mock("../lib/janela", () => ({
  redimensionarJanela: vi.fn(),
  posicaoJanela: vi.fn().mockResolvedValue(null),
  moverJanela: vi.fn(),
}));

import { redimensionarJanela } from "../lib/janela";

describe("Orbe", () => {
  it("abre pedindo a janela expandida", () => {
    const aoAlternar = vi.fn();
    render(<Orbe aberto={false} aoAlternar={aoAlternar} />);

    fireEvent.click(screen.getByAltText("James"));

    expect(aoAlternar).toHaveBeenCalledWith(true);
    expect(redimensionarJanela).toHaveBeenCalledWith(LARGURA_EXPANDIDA, ALTURA_EXPANDIDA);
  });

  it("fecha pedindo a janela recolhida", () => {
    const aoAlternar = vi.fn();
    render(<Orbe aberto={true} aoAlternar={aoAlternar} />);

    fireEvent.click(screen.getByAltText("James"));

    expect(aoAlternar).toHaveBeenCalledWith(false);
    expect(redimensionarJanela).toHaveBeenCalledWith(TAMANHO_RECOLHIDO, TAMANHO_RECOLHIDO);
  });
});
```

- [ ] **Step 3: Rodar e ver passar**

Run: `npm test --prefix web -- orbe`
Expected: PASS

- [ ] **Step 4: Tirar o código antigo do `App.tsx`**

Remova `LIMIAR_ARRASTE_PX`, `useArrastarJanela`, `handleToggleChat` e o `div.drag-handle` de `App.tsx`, e use `<Orbe aberto={isOpen} aoAlternar={setIsOpen} />` no lugar. Remova os imports que ficaram órfãos (`moverJanela`, `posicaoJanela`, `redimensionarJanela`, `qfLogo`).

- [ ] **Step 5: Rodar tudo**

Run: `npm test --prefix web && npm run build --prefix web`
Expected: PASS e build limpo

- [ ] **Step 6: Commit**

```bash
git add web/src/shell/orbe.tsx web/src/shell/orbe.test.tsx web/src/App.tsx
git commit -m "refactor(web): move arraste e resize da janela para shell/orbe"
```

---

## Task 11: Trilha de abas e casca do painel

**Files:**
- Create: `web/src/trilha.tsx`, `web/src/trilha.test.tsx`

- [ ] **Step 1: Escrever o teste que falha**

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ABAS, Trilha } from "./trilha";

describe("Trilha", () => {
  it("mostra as cinco abas", () => {
    render(<Trilha ativa="conversa" aoTrocar={vi.fn()} />);

    expect(ABAS).toHaveLength(5);
    for (const aba of ABAS) {
      expect(screen.getByRole("tab", { name: aba.rotulo })).toBeTruthy();
    }
  });

  it("marca a aba ativa e troca ao clicar", () => {
    const aoTrocar = vi.fn();
    render(<Trilha ativa="conversa" aoTrocar={aoTrocar} />);

    expect(screen.getByRole("tab", { name: "Conversa" }).getAttribute("aria-selected")).toBe("true");

    fireEvent.click(screen.getByRole("tab", { name: "Sessões" }));

    expect(aoTrocar).toHaveBeenCalledWith("sessoes");
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `npm test --prefix web -- trilha`
Expected: FAIL — módulo não existe

- [ ] **Step 3: Implementar**

`web/src/trilha.tsx`:

```tsx
export type IdAba = "conversa" | "tela" | "atividade" | "memoria" | "sessoes";

export const ABAS: { id: IdAba; rotulo: string; icone: string }[] = [
  { id: "conversa", rotulo: "Conversa", icone: "💬" },
  { id: "tela", rotulo: "Tela", icone: "🖥" },
  { id: "atividade", rotulo: "Atividade", icone: "🔧" },
  { id: "memoria", rotulo: "Memória", icone: "🧠" },
  { id: "sessoes", rotulo: "Sessões", icone: "🕘" },
];

interface Props {
  ativa: IdAba;
  aoTrocar: (id: IdAba) => void;
}

// Trilha vertical no desktop, barra horizontal embaixo no celular — o
// colapso é só a direção do flex, sem componente separado (CA8).
export function Trilha({ ativa, aoTrocar }: Props) {
  return (
    <nav
      role="tablist"
      aria-label="Seções do James"
      className="flex flex-row justify-around border-t border-white/10 md:flex-col md:justify-start md:gap-1 md:border-t-0 md:border-r md:p-2"
    >
      {ABAS.map((aba) => (
        <button
          key={aba.id}
          role="tab"
          aria-selected={ativa === aba.id}
          onClick={() => aoTrocar(aba.id)}
          className={`flex flex-col items-center gap-0.5 rounded px-3 py-2 text-[10px] transition-opacity ${
            ativa === aba.id ? "opacity-100" : "opacity-50 hover:opacity-80"
          }`}
        >
          <span aria-hidden="true" className="text-base">
            {aba.icone}
          </span>
          {aba.rotulo}
        </button>
      ))}
    </nav>
  );
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `npm test --prefix web -- trilha`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/trilha.tsx web/src/trilha.test.tsx
git commit -m "feat(web): trilha vertical de abas, horizontal no celular"
```

---

## Task 12: Aba Conversa e faixa de conexão

**Files:**
- Create: `web/src/abas/conversa.tsx`, `web/src/abas/conversa.test.tsx`

- [ ] **Step 1: Escrever o teste que falha**

```tsx
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Conversa } from "./conversa";
import { estadoInicial } from "../estado/turno";

const estadoComConversa = {
  ...estadoInicial,
  mensagens: [
    { papel: "usuario" as const, texto: "abre a calculadora" },
    { papel: "james" as const, texto: "Feito." },
  ],
};

describe("Aba Conversa", () => {
  it("renderiza as mensagens dos dois lados", () => {
    render(<Conversa estado={estadoComConversa} conexao="conectado" aoEnviar={vi.fn()} />);

    expect(screen.getByText("abre a calculadora")).toBeTruthy();
    expect(screen.getByText("Feito.")).toBeTruthy();
  });

  it("envia com Enter e limpa o campo", () => {
    const aoEnviar = vi.fn();
    render(<Conversa estado={estadoInicial} conexao="conectado" aoEnviar={aoEnviar} />);

    const campo = screen.getByPlaceholderText("Comando de voz ou texto...") as HTMLInputElement;
    fireEvent.change(campo, { target: { value: "oi" } });
    fireEvent.keyDown(campo, { key: "Enter" });

    expect(aoEnviar).toHaveBeenCalledWith("oi");
    expect(campo.value).toBe("");
  });

  it("não envia texto em branco", () => {
    const aoEnviar = vi.fn();
    render(<Conversa estado={estadoInicial} conexao="conectado" aoEnviar={aoEnviar} />);

    const campo = screen.getByPlaceholderText("Comando de voz ou texto...");
    fireEvent.change(campo, { target: { value: "   " } });
    fireEvent.keyDown(campo, { key: "Enter" });

    expect(aoEnviar).not.toHaveBeenCalled();
  });

  it("mostra a faixa de reconexão e bloqueia o envio", () => {
    const aoEnviar = vi.fn();
    render(<Conversa estado={estadoInicial} conexao="reconectando" aoEnviar={aoEnviar} />);

    expect(screen.getByRole("status").textContent).toContain("Reconectando");

    const campo = screen.getByPlaceholderText("Comando de voz ou texto...");
    expect((campo as HTMLInputElement).disabled).toBe(true);
  });

  it("mostra ferramenta em voo durante o turno", () => {
    const estado = {
      ...estadoInicial,
      emAndamento: true,
      ferramentas: [{ nome: "ver_tela", estado: "voando" as const, iniciadaEm: Date.now() }],
    };
    render(<Conversa estado={estado} conexao="conectado" aoEnviar={vi.fn()} />);

    expect(screen.getByText(/ver_tela/)).toBeTruthy();
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `npm test --prefix web -- conversa`
Expected: FAIL — módulo não existe

- [ ] **Step 3: Implementar**

`web/src/abas/conversa.tsx`:

```tsx
import { useState } from "react";
import type { EstadoConexao } from "../lib/ws";
import type { EstadoTurno } from "../estado/turno";
import { separarSegmentos } from "../lib/markdown";

interface Props {
  estado: EstadoTurno;
  conexao: EstadoConexao;
  aoEnviar: (texto: string) => void;
}

function Corpo({ texto }: { texto: string }) {
  return (
    <>
      {separarSegmentos(texto).map((segmento, i) =>
        segmento.tipo === "texto" ? (
          <span key={i}>{segmento.conteudo}</span>
        ) : (
          <pre key={i} className="my-1 overflow-x-auto rounded bg-black/40 p-2 text-xs">
            <code className={segmento.linguagem}>{segmento.conteudo}</code>
          </pre>
        )
      )}
    </>
  );
}

export function Conversa({ estado, conexao, aoEnviar }: Props) {
  const [rascunho, setRascunho] = useState("");
  const bloqueado = conexao !== "conectado";

  const enviar = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== "Enter" || rascunho.trim() === "") return;
    aoEnviar(rascunho);
    setRascunho("");
  };

  return (
    <div className="flex h-full flex-col gap-2">
      {bloqueado && (
        <div role="status" className="rounded bg-amber-500/20 px-3 py-1 text-xs">
          {conexao === "reconectando" ? "Reconectando ao James..." : "Conectando..."}
        </div>
      )}

      <div className="flex-1 overflow-y-auto pr-1">
        {estado.mensagens.map((m, i) => (
          <div
            key={i}
            className={`mb-2 max-w-[85%] rounded-lg px-3 py-2 text-sm ${
              m.papel === "usuario" ? "ml-auto bg-white/10" : "bg-white/5"
            }`}
          >
            <div className="mb-0.5 text-[10px] opacity-50">{m.papel === "usuario" ? "Arthur" : "James"}</div>
            <Corpo texto={m.texto} />
          </div>
        ))}

        {estado.ferramentas.map((f, i) => (
          <div key={i} className="mb-1 font-mono text-[10px] opacity-60">
            {f.estado === "voando" ? "⚡" : f.estado === "erro" ? "✗" : "✓"} {f.nome}
            {f.erro ? ` — ${f.erro}` : ""}
          </div>
        ))}
      </div>

      <input
        className="w-full rounded border border-white/15 bg-transparent px-3 py-2 text-sm outline-none disabled:opacity-40"
        placeholder="Comando de voz ou texto..."
        value={rascunho}
        disabled={bloqueado}
        onChange={(e) => setRascunho(e.target.value)}
        onKeyDown={enviar}
        autoFocus
      />
    </div>
  );
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `npm test --prefix web -- conversa`
Expected: PASS em todos

- [ ] **Step 5: Commit**

```bash
git add web/src/abas/conversa.tsx web/src/abas/conversa.test.tsx
git commit -m "feat(web): aba Conversa com ferramenta em voo e faixa de conexao"
```

---

## Task 13: Aba Tela

**Files:**
- Create: `web/src/abas/tela.tsx`, `web/src/abas/tela.test.tsx`

> **Nota de dependência:** o backend hoje **não** entrega a imagem de `ver_tela` ao cliente WS — ela vai do device ao modelo. Esta aba consome o que o visualizador recebe via `ferramenta_concluida` de `ver_tela`, cuja `saida` é o texto com superfície, escala e lista de elementos. A imagem em si fica registrada como lacuna: exibi-la exige o `jamesd` repassar `Imagem` ao cliente, o que **não** está nesta fase. A aba mostra a lista de elementos e a superfície, e um aviso explícito quando não há imagem.

- [ ] **Step 1: Escrever o teste que falha**

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { Tela, interpretarSaidaVerTela } from "./tela";

const SAIDA = `superficie: DISPLAY1 1920x1080 escala 100%
e3 Button "Um" (1163,761)
e4 Button "Dois" (1229,761)
e7 Edit "visor" (1100,300)`;

describe("interpretarSaidaVerTela", () => {
  it("separa a superfície dos elementos", () => {
    const lido = interpretarSaidaVerTela(SAIDA);

    expect(lido.superficie).toBe("DISPLAY1 1920x1080 escala 100%");
    expect(lido.elementos).toHaveLength(3);
    expect(lido.elementos[0]).toBe('e3 Button "Um" (1163,761)');
  });

  it("aguenta saída sem linha de superfície", () => {
    const lido = interpretarSaidaVerTela('e1 Button "OK" (10,10)');

    expect(lido.superficie).toBe("");
    expect(lido.elementos).toHaveLength(1);
  });
});

describe("Aba Tela", () => {
  it("mostra superfície e elementos", () => {
    render(<Tela saida={SAIDA} />);

    expect(screen.getByText(/DISPLAY1 1920x1080 escala 100%/)).toBeTruthy();
    expect(screen.getByText(/e3 Button "Um"/)).toBeTruthy();
  });

  it("explica quando nada foi capturado ainda", () => {
    render(<Tela saida={null} />);

    expect(screen.getByText(/Nenhuma captura/)).toBeTruthy();
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `npm test --prefix web -- tela`
Expected: FAIL — módulo não existe

- [ ] **Step 3: Implementar**

`web/src/abas/tela.tsx`:

```tsx
export interface LeituraTela {
  superficie: string;
  elementos: string[];
}

// interpretarSaidaVerTela lê o texto que a ferramenta ver_tela devolve. O
// formato é o definido na spec da Fase 5, seção 5.1: uma linha de
// superfície, depois uma linha por elemento.
export function interpretarSaidaVerTela(saida: string): LeituraTela {
  const linhas = saida.split("\n").map((l) => l.trim()).filter(Boolean);
  const primeira = linhas[0] ?? "";

  if (primeira.startsWith("superficie:")) {
    return {
      superficie: primeira.slice("superficie:".length).trim(),
      elementos: linhas.slice(1),
    };
  }
  return { superficie: "", elementos: linhas };
}

export function Tela({ saida }: { saida: string | null }) {
  if (!saida) {
    return (
      <p className="p-4 text-sm opacity-60">
        Nenhuma captura ainda. Peça ao James para olhar a tela.
      </p>
    );
  }

  const { superficie, elementos } = interpretarSaidaVerTela(saida);

  return (
    <div className="flex h-full flex-col gap-2 p-2">
      {superficie && <div className="text-[11px] opacity-60">{superficie}</div>}
      <div className="rounded border border-dashed border-white/20 p-3 text-[11px] opacity-50">
        A imagem capturada não é entregue ao cliente nesta fase — só a lista de elementos.
      </div>
      <ul className="flex-1 overflow-y-auto font-mono text-[11px] leading-relaxed">
        {elementos.map((e, i) => (
          <li key={i}>{e}</li>
        ))}
      </ul>
    </div>
  );
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `npm test --prefix web -- tela`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add web/src/abas/tela.tsx web/src/abas/tela.test.tsx
git commit -m "feat(web): aba Tela com superficie e lista de elementos"
```

---

## Task 14: Aba Memória

**Files:**
- Create: `web/src/abas/memoria.tsx`, `web/src/abas/memoria.test.tsx`
- Modify: `web/src/lib/api.ts`, `web/src/lib/api.test.ts`

- [ ] **Step 1: Escrever o teste que falha**

`web/src/abas/memoria.test.tsx`:

```tsx
import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Memoria } from "./memoria";

vi.mock("../lib/api", () => ({
  lerMemoria: vi.fn(),
}));

import { lerMemoria } from "../lib/api";

describe("Aba Memória", () => {
  it("lista as notas por assunto", async () => {
    vi.mocked(lerMemoria).mockResolvedValue({
      notas: [{ assunto: "projetos", conteudo: "James está na Fase 6", modificado: "2026-08-09T12:00:00Z" }],
      aviso: "",
    });

    render(<Memoria />);

    await waitFor(() => expect(screen.getByText("projetos")).toBeTruthy());
    expect(screen.getByText(/Fase 6/)).toBeTruthy();
  });

  it("mostra o aviso quando o vault falhou", async () => {
    vi.mocked(lerMemoria).mockResolvedValue({ notas: [], aviso: "diretório ilegível" });

    render(<Memoria />);

    await waitFor(() => expect(screen.getByRole("status").textContent).toContain("diretório ilegível"));
  });

  it("explica o vault vazio sem parecer erro", async () => {
    vi.mocked(lerMemoria).mockResolvedValue({ notas: [], aviso: "" });

    render(<Memoria />);

    await waitFor(() => expect(screen.getByText(/Nenhuma nota/)).toBeTruthy());
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `npm test --prefix web -- memoria`
Expected: FAIL — módulos não existem

- [ ] **Step 3: Acrescentar o cliente REST**

No fim de `web/src/lib/api.ts`:

```ts
export interface NotaAssunto {
  assunto: string;
  conteudo: string;
  modificado: string;
}

export interface RespostaMemoria {
  notas: NotaAssunto[];
  aviso?: string;
}

export async function lerMemoria(): Promise<RespostaMemoria> {
  const resp = await fetch("/api/memoria");
  if (!resp.ok) throw new Error(`erro ao ler memória: ${resp.status}`);
  return resp.json();
}
```

- [ ] **Step 4: Implementar a aba**

`web/src/abas/memoria.tsx`:

```tsx
import { useEffect, useState } from "react";
import { lerMemoria, type RespostaMemoria } from "../lib/api";

export function Memoria() {
  const [dados, setDados] = useState<RespostaMemoria | null>(null);
  const [falha, setFalha] = useState<string>("");

  // Relê ao montar, não a cada turno: a memória muda por escrita do modelo,
  // não pela conversa. Abrir a aba de novo é o gesto de atualizar.
  useEffect(() => {
    lerMemoria()
      .then(setDados)
      .catch((e: Error) => setFalha(e.message));
  }, []);

  if (falha) {
    return (
      <p role="status" className="p-4 text-sm text-amber-300">
        Não consegui ler a memória: {falha}
      </p>
    );
  }
  if (!dados) return <p className="p-4 text-sm opacity-60">Carregando...</p>;

  return (
    <div className="h-full overflow-y-auto p-3">
      {dados.aviso && (
        <p role="status" className="mb-3 rounded bg-amber-500/20 px-3 py-1 text-xs">
          {dados.aviso}
        </p>
      )}
      {dados.notas.length === 0 && !dados.aviso && (
        <p className="text-sm opacity-60">Nenhuma nota guardada ainda.</p>
      )}
      {dados.notas.map((nota) => (
        <section key={nota.assunto} className="mb-4">
          <h3 className="mb-1 text-xs uppercase tracking-wide opacity-60">{nota.assunto}</h3>
          <pre className="whitespace-pre-wrap text-sm">{nota.conteudo}</pre>
        </section>
      ))}
    </div>
  );
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `npm test --prefix web -- memoria`
Expected: PASS nos três

- [ ] **Step 6: Commit**

```bash
git add web/src/lib/api.ts web/src/abas/memoria.tsx web/src/abas/memoria.test.tsx
git commit -m "feat(web): aba Memoria lendo GET /api/memoria"
```

---

## Task 15: Aba Sessões com retomada

**Files:**
- Create: `web/src/abas/sessoes.tsx`, `web/src/abas/sessoes.test.tsx`

- [ ] **Step 1: Escrever o teste que falha**

```tsx
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Sessoes } from "./sessoes";

vi.mock("../lib/api", () => ({
  listarSessoes: vi.fn(),
}));

import { listarSessoes } from "../lib/api";

const SESSOES = [
  { ID: "11111111-1111-1111-1111-111111111111", Titulo: "abre a calculadora", CriadaEm: "2026-08-08T22:00:00Z", AtualizadaEm: "2026-08-08T22:05:00Z" },
  { ID: "22222222-2222-2222-2222-222222222222", Titulo: "pesquisa sobre Go", CriadaEm: "2026-08-07T10:00:00Z", AtualizadaEm: "2026-08-07T10:30:00Z" },
];

describe("Aba Sessões", () => {
  it("lista as sessões do banco", async () => {
    vi.mocked(listarSessoes).mockResolvedValue(SESSOES);

    render(<Sessoes aoRetomar={vi.fn()} sessaoAtual={null} />);

    await waitFor(() => expect(screen.getByText("abre a calculadora")).toBeTruthy());
    expect(screen.getByText("pesquisa sobre Go")).toBeTruthy();
  });

  it("retoma a sessão clicada", async () => {
    vi.mocked(listarSessoes).mockResolvedValue(SESSOES);
    const aoRetomar = vi.fn();

    render(<Sessoes aoRetomar={aoRetomar} sessaoAtual={null} />);

    await waitFor(() => screen.getByText("abre a calculadora"));
    fireEvent.click(screen.getByText("abre a calculadora"));

    expect(aoRetomar).toHaveBeenCalledWith("11111111-1111-1111-1111-111111111111");
  });

  it("marca a sessão atual", async () => {
    vi.mocked(listarSessoes).mockResolvedValue(SESSOES);

    render(<Sessoes aoRetomar={vi.fn()} sessaoAtual="22222222-2222-2222-2222-222222222222" />);

    await waitFor(() => screen.getByText("pesquisa sobre Go"));
    expect(screen.getByText("pesquisa sobre Go").closest("button")!.getAttribute("aria-current")).toBe("true");
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `npm test --prefix web -- sessoes`
Expected: FAIL — módulo não existe

- [ ] **Step 3: Implementar**

`web/src/abas/sessoes.tsx`:

```tsx
import { useEffect, useState } from "react";
import { listarSessoes, type Sessao } from "../lib/api";

interface Props {
  sessaoAtual: string | null;
  aoRetomar: (id: string) => void;
}

export function Sessoes({ sessaoAtual, aoRetomar }: Props) {
  const [sessoes, setSessoes] = useState<Sessao[] | null>(null);
  const [falha, setFalha] = useState("");

  useEffect(() => {
    listarSessoes()
      .then(setSessoes)
      .catch((e: Error) => setFalha(e.message));
  }, []);

  if (falha) return <p role="status" className="p-4 text-sm text-amber-300">{falha}</p>;
  if (!sessoes) return <p className="p-4 text-sm opacity-60">Carregando...</p>;
  if (sessoes.length === 0) return <p className="p-4 text-sm opacity-60">Nenhuma conversa guardada.</p>;

  return (
    <ul className="h-full overflow-y-auto p-2">
      {sessoes.map((s) => (
        <li key={s.ID}>
          <button
            aria-current={s.ID === sessaoAtual}
            onClick={() => aoRetomar(s.ID)}
            className={`mb-1 w-full rounded px-3 py-2 text-left text-sm hover:bg-white/10 ${
              s.ID === sessaoAtual ? "bg-white/10" : ""
            }`}
          >
            <div className="truncate">{s.Titulo}</div>
            <div className="text-[10px] opacity-50">{new Date(s.AtualizadaEm).toLocaleString("pt-BR")}</div>
          </button>
        </li>
      ))}
    </ul>
  );
}
```

- [ ] **Step 4: Rodar e ver passar**

Run: `npm test --prefix web -- sessoes`
Expected: PASS nos três

- [ ] **Step 5: Commit**

```bash
git add web/src/abas/sessoes.tsx web/src/abas/sessoes.test.tsx
git commit -m "feat(web): aba Sessoes lista e retoma conversa do Postgres"
```

---

## Task 16: Aba Atividade e visualizador reativo

O visualizador não tem estado próprio (D9): deriva do turno, com precedência fixa.

**Files:**
- Create: `web/src/abas/atividade.tsx`, `web/src/visualizador.tsx`, `web/src/visualizador.test.tsx`

- [ ] **Step 1: Escrever o teste que falha**

`web/src/visualizador.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { estadoInicial } from "./estado/turno";
import { escolherConteudo, Visualizador } from "./visualizador";

describe("escolherConteudo", () => {
  it("aprovação pendente vence tudo", () => {
    const estado = {
      ...estadoInicial,
      aprovacao: { id: "a1", ferramenta: "executar_comando_sistema", argumentos: {} },
      ferramentas: [{ nome: "ver_tela", estado: "concluida" as const, iniciadaEm: 0 }],
    };

    expect(escolherConteudo(estado, "saida qualquer")).toBe("aprovacao");
  });

  it("captura de tela vence saída de ferramenta comum", () => {
    const estado = {
      ...estadoInicial,
      ferramentas: [{ nome: "ver_tela", estado: "concluida" as const, iniciadaEm: 0 }],
    };

    expect(escolherConteudo(estado, "superficie: DISPLAY1")).toBe("tela");
  });

  it("sem nada relevante, fica vazio", () => {
    expect(escolherConteudo(estadoInicial, null)).toBe("vazio");
  });
});

describe("Visualizador", () => {
  it("mostra o pedido de aprovação com os dois botões", () => {
    const estado = {
      ...estadoInicial,
      aprovacao: { id: "a1", ferramenta: "executar_comando_sistema", argumentos: { comando: "del x" } },
    };
    const aoDecidir = vi.fn();

    render(<Visualizador estado={estado} saidaVerTela={null} aoDecidir={aoDecidir} />);

    expect(screen.getByText(/executar_comando_sistema/)).toBeTruthy();
    screen.getByRole("button", { name: "Aprovar" }).click();
    expect(aoDecidir).toHaveBeenCalledWith(true);
  });
});
```

- [ ] **Step 2: Rodar e ver falhar**

Run: `npm test --prefix web -- visualizador`
Expected: FAIL — módulo não existe

- [ ] **Step 3: Implementar a aba Atividade**

`web/src/abas/atividade.tsx`:

```tsx
import type { EstadoTurno } from "../estado/turno";

export function Atividade({ estado }: { estado: EstadoTurno }) {
  if (estado.ferramentas.length === 0) {
    return <p className="p-4 text-sm opacity-60">Nenhuma ferramenta neste turno.</p>;
  }

  return (
    <ul className="h-full overflow-y-auto p-3 font-mono text-xs">
      {estado.ferramentas.map((f, i) => (
        <li key={i} className="mb-1">
          <span className="mr-2">{f.estado === "voando" ? "⚡" : f.estado === "erro" ? "✗" : "✓"}</span>
          {f.nome}
          {f.erro && <span className="ml-2 text-amber-300">{f.erro}</span>}
        </li>
      ))}
    </ul>
  );
}
```

- [ ] **Step 4: Implementar o visualizador**

`web/src/visualizador.tsx`:

```tsx
import type { EstadoTurno } from "./estado/turno";
import { Tela } from "./abas/tela";

export type ConteudoVisualizador = "aprovacao" | "tela" | "ferramenta" | "vazio";

// Precedência fixa (D9 da spec). Estado derivado não dessincroniza: não há
// "estado do visualizador" para ficar velho.
export function escolherConteudo(estado: EstadoTurno, saidaVerTela: string | null): ConteudoVisualizador {
  if (estado.aprovacao) return "aprovacao";
  if (saidaVerTela) return "tela";
  if (estado.ferramentas.length > 0) return "ferramenta";
  return "vazio";
}

interface Props {
  estado: EstadoTurno;
  saidaVerTela: string | null;
  aoDecidir: (aprovado: boolean) => void;
}

export function Visualizador({ estado, saidaVerTela, aoDecidir }: Props) {
  const conteudo = escolherConteudo(estado, saidaVerTela);

  if (conteudo === "aprovacao" && estado.aprovacao) {
    return (
      <div className="p-4">
        <p className="mb-2 text-sm">
          O James quer executar <strong>{estado.aprovacao.ferramenta}</strong>
        </p>
        <pre className="mb-3 overflow-x-auto rounded bg-black/40 p-2 text-xs">
          {JSON.stringify(estado.aprovacao.argumentos, null, 2)}
        </pre>
        <div className="flex gap-2">
          <button onClick={() => aoDecidir(true)} className="rounded bg-emerald-600/70 px-3 py-1 text-sm">
            Aprovar
          </button>
          <button onClick={() => aoDecidir(false)} className="rounded bg-red-600/70 px-3 py-1 text-sm">
            Negar
          </button>
        </div>
      </div>
    );
  }

  if (conteudo === "tela") return <Tela saida={saidaVerTela} />;

  if (conteudo === "ferramenta") {
    const ultima = estado.ferramentas[estado.ferramentas.length - 1];
    return (
      <div className="p-4 font-mono text-xs">
        <div className="mb-1 opacity-60">última ferramenta</div>
        <div>{ultima.nome}</div>
        {ultima.erro && <div className="mt-1 text-amber-300">{ultima.erro}</div>}
      </div>
    );
  }

  return <p className="p-4 text-sm opacity-40">Nada para mostrar ainda.</p>;
}
```

- [ ] **Step 5: Rodar e ver passar**

Run: `npm test --prefix web -- visualizador`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add web/src/abas/atividade.tsx web/src/visualizador.tsx web/src/visualizador.test.tsx
git commit -m "feat(web): visualizador reativo derivado do turno"
```

---

## Task 17: Compor o `App.tsx` novo

**Files:**
- Modify: `web/src/App.tsx`
- Delete: `web/src/App.css`, `web/src/components/CardAprovacao.tsx`, `web/src/components/CardAprovacao.test.tsx`

- [ ] **Step 1: Reescrever o `App.tsx`**

```tsx
import { useEffect, useReducer, useRef, useState } from "react";
import { Atividade } from "./abas/atividade";
import { Conversa } from "./abas/conversa";
import { Memoria } from "./abas/memoria";
import { Sessoes } from "./abas/sessoes";
import { Tela } from "./abas/tela";
import { estadoInicial, reduzir } from "./estado/turno";
import { ClienteWS, type EstadoConexao } from "./lib/ws";
import { Orbe } from "./shell/orbe";
import { Trilha, type IdAba } from "./trilha";
import { Visualizador } from "./visualizador";

function urlWS(): string {
  const protocolo = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocolo}//${window.location.host}/ws`;
}

export default function App() {
  const [aberto, setAberto] = useState(false);
  const [aba, setAba] = useState<IdAba>("conversa");
  const [conexao, setConexao] = useState<EstadoConexao>("conectando");
  const [saidaVerTela, setSaidaVerTela] = useState<string | null>(null);
  const [sessaoAtual, setSessaoAtual] = useState<string | null>(null);
  const [estado, despachar] = useReducer(reduzir, estadoInicial);
  const clienteRef = useRef<ClienteWS | null>(null);

  useEffect(() => {
    const cliente = new ClienteWS(urlWS());
    clienteRef.current = cliente;

    cliente.aoMudarEstado((e) => setConexao(e));
    cliente.on("resposta_final", (dados) => despachar({ tipo: "resposta_final", dados }));
    cliente.on("erro", (dados) => despachar({ tipo: "erro", dados }));
    cliente.on("ferramenta_iniciada", (dados) => despachar({ tipo: "ferramenta_iniciada", dados }));
    cliente.on("aprovacao_requerida", (dados) => despachar({ tipo: "aprovacao_requerida", dados }));
    cliente.on("sessao_retomada", (dados: { sessao_id: string }) => setSessaoAtual(dados.sessao_id));

    cliente.on("ferramenta_concluida", (dados: { nome: string; erro: string }) => {
      despachar({ tipo: "ferramenta_concluida", dados });
    });

    cliente.on("audio_pronto", (dados: { id: string }) => {
      const excluirAudio = () => {
        fetch(`/api/audio/${dados.id}`, { method: "DELETE" }).catch(() => {});
      };
      const audio = new Audio(`/api/audio/${dados.id}`);
      audio.addEventListener("ended", excluirAudio);
      // Autoplay pode ser bloqueado sem gesto recente do usuário — se falhar,
      // o áudio só não toca, mas o arquivo ainda precisa sair do disco.
      audio.play().catch(excluirAudio);
    });

    return () => cliente.fechar();
  }, []);

  const enviar = (texto: string) => {
    despachar({ tipo: "envio_local", texto });
    clienteRef.current?.enviar("mensagem", crypto.randomUUID(), { texto });
  };

  const decidir = (aprovado: boolean) => {
    if (!estado.aprovacao) return;
    clienteRef.current?.enviar(aprovado ? "aprovar" : "negar", estado.aprovacao.id, {});
    despachar({ tipo: "aprovacao_decidida" });
  };

  const retomar = (id: string) => {
    clienteRef.current?.enviar("retomar", crypto.randomUUID(), { sessao_id: id });
    setAba("conversa");
  };

  return (
    <div className="h-screen w-screen text-white">
      <Orbe aberto={aberto} aoAlternar={setAberto} />

      {aberto && (
        <div className="flex h-full flex-col-reverse rounded-xl bg-black/70 backdrop-blur md:flex-row">
          <Trilha ativa={aba} aoTrocar={setAba} />

          <main className="min-w-0 flex-1 p-2">
            {aba === "conversa" && <Conversa estado={estado} conexao={conexao} aoEnviar={enviar} />}
            {aba === "tela" && <Tela saida={saidaVerTela} />}
            {aba === "atividade" && <Atividade estado={estado} />}
            {aba === "memoria" && <Memoria />}
            {aba === "sessoes" && <Sessoes sessaoAtual={sessaoAtual} aoRetomar={retomar} />}
          </main>

          <aside className="hidden min-w-0 flex-1 border-l border-white/10 md:block">
            <Visualizador estado={estado} saidaVerTela={saidaVerTela} aoDecidir={decidir} />
          </aside>
        </div>
      )}
    </div>
  );
}
```

Nota: `setSaidaVerTela` é alimentado na Task 18. Até lá, a aba Tela mostra o estado vazio, que é o comportamento correto.

- [ ] **Step 2: Remover o que morreu**

```bash
git rm web/src/App.css web/src/components/CardAprovacao.tsx web/src/components/CardAprovacao.test.tsx
```

Confirme que nada mais importa `App.css`:

Run: `grep -rn "App.css\|CardAprovacao" web/src/`
Expected: nenhuma saída

- [ ] **Step 3: Rodar tudo**

Run: `npm test --prefix web && npm run build --prefix web`
Expected: PASS e build limpo

- [ ] **Step 4: Commit**

```bash
git add -A web/src
git commit -m "feat(web): App.tsx vira composicao, sai o CSS antigo"
```

---

## Task 18: Levar a saída de `ver_tela` até a aba Tela

**Files:**
- Modify: `internal/api/ws.go`, `web/src/App.tsx`
- Test: `internal/api/ws_test.go`

Hoje `ferramenta_concluida` manda só `{nome, erro}`. A aba Tela precisa da saída.

- [ ] **Step 1: Escrever o teste que falha**

```go
func TestFerramentaConcluidaCarregaSaida(t *testing.T) {
	// A aba Tela da Fase 6 depende da saída de ver_tela, que hoje só vai
	// para o modelo. Sem isso, a aba fica permanentemente vazia.
	eventos := capturarEventosDeUmTurno(t, "olha a tela")

	var achou bool
	for _, e := range eventos {
		if e.Tipo != "ferramenta_concluida" {
			continue
		}
		var dados struct {
			Nome  string `json:"nome"`
			Erro  string `json:"erro"`
			Saida string `json:"saida"`
		}
		if err := json.Unmarshal(e.Dados, &dados); err != nil {
			t.Fatalf("dados ilegíveis: %v", err)
		}
		if dados.Saida == "" {
			t.Errorf("ferramenta_concluida sem saida: %+v", dados)
		}
		achou = true
	}
	if !achou {
		t.Fatal("nenhum ferramenta_concluida no turno")
	}
}
```

Use o mesmo helper de turno das tasks anteriores, com uma ferramenta falsa que devolve saída conhecida.

- [ ] **Step 2: Rodar e ver falhar**

Run: `go test ./internal/api/ -run TestFerramentaConcluidaCarregaSaida -v`
Expected: FAIL — `saida` vem vazia

- [ ] **Step 3: Implementar**

Em `internal/api/ws.go`, no callback `aoConcluir` de `tratarMensagem`:

```go
		func(rf agente.ResultadoFerramenta) {
			cliente.Enviar("ferramenta_concluida", envelope.ID, map[string]string{
				"nome":  rf.Nome,
				"erro":  rf.Erro,
				"saida": rf.Saida,
			})
		},
```

Faça o mesmo em `internal/api/dispositivo.go:125`, para a entrada por voz.

- [ ] **Step 4: Consumir no front**

Em `web/src/App.tsx`, troque o handler:

```tsx
    cliente.on("ferramenta_concluida", (dados: { nome: string; erro: string; saida?: string }) => {
      despachar({ tipo: "ferramenta_concluida", dados });
      if (dados.nome === "ver_tela" && dados.saida) setSaidaVerTela(dados.saida);
    });
```

- [ ] **Step 5: Rodar tudo**

Run: `go test ./... && npm test --prefix web`
Expected: verde nos dois

- [ ] **Step 6: Commit**

```bash
git add internal/api/ws.go internal/api/dispositivo.go internal/api/ws_test.go web/src/App.tsx
git commit -m "feat: ferramenta_concluida carrega a saida, alimentando a aba Tela"
```

---

## Task 19: Teste de ponta a ponta com Playwright

**Files:**
- Create: `web/e2e/fluxo.spec.ts`

- [ ] **Step 1: Instalar**

```bash
npm install --prefix web -D @playwright/test && npx --prefix web playwright install chromium
```

- [ ] **Step 2: Escrever o teste**

`web/e2e/fluxo.spec.ts`:

```ts
import { expect, test } from "@playwright/test";

// Este teste existe por causa da nota de metodologia da Fase 5: o SPA
// antigo não renderizava respostas de forma confiável, e a verificação
// precisou ler o Postgres direto. Aqui o critério é a tela.
test("mensagem enviada aparece e a resposta renderiza", async ({ page }) => {
  await page.goto("http://127.0.0.1:8080");
  await page.getByAltText("James").click();

  const campo = page.getByPlaceholder("Comando de voz ou texto...");
  await campo.fill("responda apenas com a palavra pong");
  await campo.press("Enter");

  await expect(page.getByText("responda apenas com a palavra pong")).toBeVisible();
  await expect(page.getByText(/pong/i)).toBeVisible({ timeout: 60000 });
});

test("queda do backend mostra a faixa de reconexão", async ({ page, context }) => {
  await page.goto("http://127.0.0.1:8080");
  await page.getByAltText("James").click();

  await context.setOffline(true);
  await expect(page.getByRole("status")).toContainText("Reconectando", { timeout: 15000 });

  await context.setOffline(false);
  await expect(page.getByPlaceholder("Comando de voz ou texto...")).toBeEnabled({ timeout: 30000 });
});
```

- [ ] **Step 3: Rodar com o `jamesd` de pé**

Suba Docker, `jamesd` e o device conforme o roteiro do handoff da Fase 5, depois:

Run: `npx --prefix web playwright test`
Expected: os dois testes passam

- [ ] **Step 4: Commit**

```bash
git add web/e2e web/package.json web/package-lock.json
git commit -m "test(web): fluxo de ponta a ponta com Playwright"
```

---

## Task 20: Verificação ao vivo dos critérios de aceitação

Nenhum critério é dado como cumprido sem o comando ter sido rodado e a saída lida. Vale a regra do CLAUDE.md do Arthur.

- [ ] **Step 1: Subir o ambiente**

```bash
docker compose up -d
```

Depois `go run ./cmd/jamesd` e `python -m dispositivo.james_device.main`, cada um em seu terminal, com o cwd na raiz do repositório.

- [ ] **Step 2: Percorrer CA1 a CA8**

Para cada critério da seção 10 da spec, execute e **anote a evidência real** (o que apareceu na tela, o que voltou no log). Ordem sugerida: CA1, CA2, CA3, CA4, CA6, CA5, CA7, CA8.

- [ ] **Step 3: Rodar a suíte Go com banco de verdade**

R2 da spec: sem `DATABASE_URL` a suíte pula os testes de banco e fecha verde sem nunca tê-lo tocado.

Run: `DATABASE_URL=<a do .env> go test ./internal/armazenamento/... -v`
Expected: os testes rodam de fato, sem `SKIP`

- [ ] **Step 4: Registrar o resultado na spec**

Acrescente a seção `10.1 Resultado da verificação ao vivo` na spec, no mesmo formato que a Fase 5 usou, com evidência por critério — inclusive o que falhou ou não pôde ser exercitado.

- [ ] **Step 5: Commit**

```bash
git add docs/superpowers/specs/2026-08-09-james-fase-6-frontend-novo-design.md
git commit -m "docs: fecha a Fase 6 com verificacao ao vivo do CA1 ao CA8"
```

---

## Auto-revisão do plano

**Cobertura da spec:**

| Item da spec | Task |
|---|---|
| 2.1 Reescrita de `web/src` na forma B | 10, 11, 17 |
| 2.1 Consumo dos eventos ignorados | 9, 12, 16, 18 |
| 2.1 Reconexão de WebSocket | 8 |
| 2.1 Aba Sessões com retomada | 4, 15 |
| 2.1 Aba Tela | 13, 18 |
| 2.1 Aba Memória | 1, 2, 14 |
| 2.1 Tailwind | 6 |
| 2.1 Remoção do `token` | 5 |
| 2.1 Testes vitest e Playwright | 7-16, 19 |
| 4.3 As três mudanças no Go | 2, 4, 5 |
| 5.2 Retomada e a coluna polimórfica | 3, 4 |
| 7 Estados de erro na tela | 8, 12 |
| 10 CA1 a CA8 | 20 |
| R2 Suíte com `DATABASE_URL` | 20 |

**Lacuna encontrada e corrigida durante a auto-revisão:** a spec assume que a aba Tela mostra a captura, mas o backend nunca entrega a imagem ao cliente — ela vai do device direto ao modelo. A Task 18 leva a **saída textual** (superfície e lista de elementos) até a aba; a imagem em si fica declarada como fora do alcance, com aviso na própria interface, e precisa de decisão em fase futura.

**Consistência de tipos:** `EstadoTurno`, `Acao`, `IdAba`, `EstadoConexao`, `NotaAssunto` e `RespostaMemoria` são definidos uma vez e referenciados com o mesmo nome em todas as tasks seguintes. `reduzir` e `escolherConteudo` mantêm assinatura estável entre as tasks 9, 16 e 17.
