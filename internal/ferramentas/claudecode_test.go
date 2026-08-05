package ferramentas

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestClaudeCodeDevolveOResultadoReal(t *testing.T) {
	c := &ClaudeCode{
		Diretorio: "C:\\projetos\\alvo",
		Executor: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			return []byte(`{"result":"refatorei o main.go e os testes passam","is_error":false}`), nil
		},
	}

	saida, err := c.Executar(context.Background(), map[string]any{"instrucao": "refatora o main.go"})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !strings.Contains(saida, "refatorei o main.go e os testes passam") {
		t.Errorf("saida = %q, deveria trazer o resultado real do Claude Code", saida)
	}
	if strings.Contains(saida, "Tarefa enviada") {
		t.Errorf("a confirmação fixa de disparo não pode mais existir: %q", saida)
	}
}

func TestClaudeCodeRepassaInstrucaoEDiretorio(t *testing.T) {
	var dirRecebido string
	var argsRecebidos []string
	c := &ClaudeCode{
		Diretorio: "C:\\projetos\\alvo",
		Executor: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			dirRecebido = dir
			argsRecebidos = args
			return []byte(`{"result":"ok","is_error":false}`), nil
		},
	}

	if _, err := c.Executar(context.Background(), map[string]any{"instrucao": "faz X"}); err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if dirRecebido != "C:\\projetos\\alvo" {
		t.Errorf("diretório = %q, queria o configurado", dirRecebido)
	}
	if !slices.Contains(argsRecebidos, "faz X") {
		t.Errorf("args = %v, deveriam conter a instrução", argsRecebidos)
	}
}

// TestClaudeCodeFalhaSemDiretorioConfigurado trava a Decisão D6: o cwd nunca
// é herdado do jamesd, porque isso significaria o James editando o próprio
// código-fonte.
func TestClaudeCodeFalhaSemDiretorioConfigurado(t *testing.T) {
	c := &ClaudeCode{
		Executor: func(context.Context, string, []string) ([]byte, error) {
			t.Fatal("não deveria chegar a executar sem diretório configurado")
			return nil, nil
		},
	}
	_, err := c.Executar(context.Background(), map[string]any{"instrucao": "x"})
	if err == nil {
		t.Fatal("esperava erro por diretório não configurado, veio nil")
	}
	if !strings.Contains(err.Error(), "diretório") {
		t.Errorf("erro deveria explicar que falta configurar o diretório, veio %q", err.Error())
	}
}

func TestClaudeCodeRejeitaArgumentoAusente(t *testing.T) {
	c := &ClaudeCode{Diretorio: "C:\\x", Executor: func(context.Context, string, []string) ([]byte, error) {
		return nil, nil
	}}
	if _, err := c.Executar(context.Background(), map[string]any{}); err == nil {
		t.Fatal("esperava erro por parâmetro instrucao ausente, veio nil")
	}
}

func TestClaudeCodePropagaErroDoExecutor(t *testing.T) {
	c := &ClaudeCode{Diretorio: "C:\\x", Executor: func(context.Context, string, []string) ([]byte, error) {
		return nil, errors.New("claude não encontrado no PATH")
	}}
	_, err := c.Executar(context.Background(), map[string]any{"instrucao": "x"})
	if err == nil {
		t.Fatal("esperava erro propagado, veio nil")
	}
	if !strings.Contains(err.Error(), "claude não encontrado") {
		t.Errorf("erro deveria carregar a causa real, veio %q", err.Error())
	}
}

func TestClaudeCodeTrataErroReportadoNoJSON(t *testing.T) {
	c := &ClaudeCode{
		Diretorio: "C:\\x",
		Executor: func(context.Context, string, []string) ([]byte, error) {
			return []byte(`{"result":"não consegui abrir o arquivo","is_error":true}`), nil
		},
	}
	_, err := c.Executar(context.Background(), map[string]any{"instrucao": "x"})
	if err == nil {
		t.Fatal("is_error true deveria virar erro, veio nil")
	}
	if !strings.Contains(err.Error(), "não consegui abrir o arquivo") {
		t.Errorf("erro deveria carregar a mensagem do Claude Code, veio %q", err.Error())
	}
}

func TestClaudeCodeRejeitaJSONInesperado(t *testing.T) {
	c := &ClaudeCode{
		Diretorio: "C:\\x",
		Executor: func(context.Context, string, []string) ([]byte, error) {
			return []byte(`isto não é json`), nil
		},
	}
	if _, err := c.Executar(context.Background(), map[string]any{"instrucao": "x"}); err == nil {
		t.Fatal("saída não-JSON deveria virar erro explícito, veio nil")
	}
}

// TestClaudeCodeUsaOFixtureReal roda o parser contra a saída de verdade
// capturada no spike da Task 13.
func TestClaudeCodeUsaOFixtureReal(t *testing.T) {
	bruto, err := os.ReadFile("testdata/claude_code_resposta.json")
	if err != nil {
		t.Skipf("fixture do spike ausente: %v", err)
	}
	c := &ClaudeCode{
		Diretorio: "C:\\x",
		Executor:  func(context.Context, string, []string) ([]byte, error) { return bruto, nil },
	}
	saida, err := c.Executar(context.Background(), map[string]any{"instrucao": "x"})
	if err != nil {
		t.Fatalf("parser falhou contra a saída real do Claude Code: %v", err)
	}
	if strings.TrimSpace(saida) == "" {
		t.Error("parser devolveu texto vazio para a saída real")
	}
}

// TestClaudeCodeSempreExigeConfirmacao trava a decisão D10. O spike da Task 13
// provou que acceptEdits executa shell em modo headless; sem esta confirmação,
// perguntar_ao_claude_code vira um caminho indireto pra shell arbitrário que
// contorna o gate do executar_comando_sistema.
func TestClaudeCodeSempreExigeConfirmacao(t *testing.T) {
	var _ ExigeConfirmacao = NovoClaudeCode()

	c := NovoClaudeCode()
	casos := []map[string]any{
		{"instrucao": "lista os arquivos"},
		{"instrucao": "apaga tudo"},
		{},
		nil,
	}
	for _, args := range casos {
		if !c.ConfirmacaoNecessaria(args) {
			t.Errorf("ConfirmacaoNecessaria(%v) = false; o Claude Code deve confirmar SEMPRE", args)
		}
	}
}

func TestClaudeCodeNivelPerigoso(t *testing.T) {
	c := NovoClaudeCode()
	if c.Nivel() != NivelPerigoso {
		t.Error("perguntar_ao_claude_code deve ser NivelPerigoso")
	}
}

// TestAmbienteSemAnthropic é o Anexo A #17 virando código testável: variável
// de ambiente da Anthropic herdada do shell sequestraria a sessão do Claude
// Code pra outro provider em vez da assinatura Pro via OAuth.
func TestAmbienteSemAnthropic(t *testing.T) {
	entrada := []string{
		"PATH=/usr/bin",
		"ANTHROPIC_BASE_URL=https://openrouter.ai/api/v1",
		"ANTHROPIC_API_KEY=sk-falsa",
		"ANTHROPIC_AUTH_TOKEN=token-falso",
		"HOME=/home/arthur",
	}
	got := AmbienteSemAnthropic(entrada)

	for _, v := range got {
		if strings.HasPrefix(v, "ANTHROPIC_BASE_URL=") ||
			strings.HasPrefix(v, "ANTHROPIC_API_KEY=") ||
			strings.HasPrefix(v, "ANTHROPIC_AUTH_TOKEN=") {
			t.Errorf("variável da Anthropic sobreviveu: %q", v)
		}
	}
	if len(got) != 2 {
		t.Errorf("ambiente resultante = %v, queria só PATH e HOME", got)
	}
}

func TestAmbienteSemAnthropicPreservaOutrasVariaveis(t *testing.T) {
	got := AmbienteSemAnthropic([]string{"ANTHROPIC_MODEL=algo", "PATH=/bin"})
	if len(got) != 2 {
		t.Errorf("ANTHROPIC_MODEL não está na lista de remoção e deveria sobreviver: %v", got)
	}
}

func TestMontarArgsPrimeiraChamadaUsaSessionID(t *testing.T) {
	got := MontarArgs("refatora o main.go", "11111111-2222-3333-4444-555555555555", false)
	querido := []string{
		"-p", "refatora o main.go",
		"--output-format", "json",
		"--permission-mode", "acceptEdits",
		"--session-id", "11111111-2222-3333-4444-555555555555",
	}
	if !slices.Equal(got, querido) {
		t.Errorf("MontarArgs = %v, queria %v", got, querido)
	}
}

func TestMontarArgsRetomadaUsaResume(t *testing.T) {
	got := MontarArgs("agora ajusta o teste", "11111111-2222-3333-4444-555555555555", true)
	querido := []string{
		"-p", "agora ajusta o teste",
		"--output-format", "json",
		"--permission-mode", "acceptEdits",
		"--resume", "11111111-2222-3333-4444-555555555555",
	}
	if !slices.Equal(got, querido) {
		t.Errorf("MontarArgs = %v, queria %v", got, querido)
	}
}

func TestMontarArgsSemSessaoOmiteAsDuasFlags(t *testing.T) {
	got := MontarArgs("faz algo", "", false)
	for _, arg := range got {
		if arg == "--session-id" || arg == "--resume" {
			t.Errorf("sem sessão, não deveria haver %q: %v", arg, got)
		}
	}
}

// sessaoDeTeste monta um ctx com id de sessão, que é como o id chega à
// ferramenta em produção (posto pelo internal/api antes do loop rodar).
func sessaoDeTeste(id string) context.Context {
	return ComSessao(context.Background(), id)
}

func TestClaudeCodePrimeiraChamadaCriaSessao(t *testing.T) {
	var argsRecebidos []string
	c := &ClaudeCode{
		Diretorio: "C:\\x",
		Executor: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			argsRecebidos = args
			return []byte(`{"result":"ok","is_error":false}`), nil
		},
	}

	ctx := sessaoDeTeste("11111111-2222-3333-4444-555555555555")
	if _, err := c.Executar(ctx, map[string]any{"instrucao": "faz X"}); err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !slices.Contains(argsRecebidos, "--session-id") {
		t.Errorf("primeira chamada deveria usar --session-id: %v", argsRecebidos)
	}
	if !slices.Contains(argsRecebidos, "11111111-2222-3333-4444-555555555555") {
		t.Errorf("o id da sessão do James deveria ir como id da sessão do Claude Code: %v", argsRecebidos)
	}
}

func TestClaudeCodeSegundaChamadaRetomaSessao(t *testing.T) {
	var ultimosArgs []string
	c := &ClaudeCode{
		Diretorio: "C:\\x",
		Executor: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			ultimosArgs = args
			return []byte(`{"result":"ok","is_error":false}`), nil
		},
	}

	ctx := sessaoDeTeste("11111111-2222-3333-4444-555555555555")
	if _, err := c.Executar(ctx, map[string]any{"instrucao": "faz X"}); err != nil {
		t.Fatalf("primeira chamada devolveu erro: %v", err)
	}
	if _, err := c.Executar(ctx, map[string]any{"instrucao": "agora ajusta"}); err != nil {
		t.Fatalf("segunda chamada devolveu erro: %v", err)
	}

	if !slices.Contains(ultimosArgs, "--resume") {
		t.Errorf("segunda chamada deveria retomar a sessão: %v", ultimosArgs)
	}
}

// TestClaudeCodeSessoesDiferentesNaoSeMisturam é o teste que o desenho antigo
// (campo no struct) não passaria: uma única instância da ferramenta atende
// todas as conversas, e a conversa B não pode retomar a sessão da conversa A.
func TestClaudeCodeSessoesDiferentesNaoSeMisturam(t *testing.T) {
	var ultimosArgs []string
	c := &ClaudeCode{
		Diretorio: "C:\\x",
		Executor: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			ultimosArgs = args
			return []byte(`{"result":"ok","is_error":false}`), nil
		},
	}

	if _, err := c.Executar(sessaoDeTeste("aaaa"), map[string]any{"instrucao": "faz X"}); err != nil {
		t.Fatalf("chamada da sessão A devolveu erro: %v", err)
	}
	if _, err := c.Executar(sessaoDeTeste("bbbb"), map[string]any{"instrucao": "faz Y"}); err != nil {
		t.Fatalf("chamada da sessão B devolveu erro: %v", err)
	}

	if slices.Contains(ultimosArgs, "--resume") {
		t.Errorf("a sessão B é nova e não pode retomar a de A: %v", ultimosArgs)
	}
}

// TestClaudeCodeFalhaNaoMarcaSessaoComoIniciada evita o estado preso: se a
// primeira chamada morreu antes de criar a sessão, a próxima precisa tentar
// criar de novo em vez de retomar uma sessão que nunca existiu.
func TestClaudeCodeFalhaNaoMarcaSessaoComoIniciada(t *testing.T) {
	var ultimosArgs []string
	falhar := true
	c := &ClaudeCode{
		Diretorio: "C:\\x",
		Executor: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			ultimosArgs = args
			if falhar {
				return nil, errors.New("claude sumiu")
			}
			return []byte(`{"result":"ok","is_error":false}`), nil
		},
	}

	ctx := sessaoDeTeste("11111111-2222-3333-4444-555555555555")
	if _, err := c.Executar(ctx, map[string]any{"instrucao": "faz X"}); err == nil {
		t.Fatal("esperava erro na primeira chamada")
	}
	falhar = false
	if _, err := c.Executar(ctx, map[string]any{"instrucao": "faz X de novo"}); err != nil {
		t.Fatalf("segunda chamada devolveu erro: %v", err)
	}
	if !slices.Contains(ultimosArgs, "--session-id") {
		t.Errorf("depois de falhar, deveria tentar criar a sessão de novo: %v", ultimosArgs)
	}
}

// TestClaudeCodeSemSessaoNoContextoNaoQuebra cobre o caminho dos testes e de
// qualquer chamador que não ponha sessão no ctx.
func TestClaudeCodeSemSessaoNoContextoNaoQuebra(t *testing.T) {
	var argsRecebidos []string
	c := &ClaudeCode{
		Diretorio: "C:\\x",
		Executor: func(ctx context.Context, dir string, args []string) ([]byte, error) {
			argsRecebidos = args
			return []byte(`{"result":"ok","is_error":false}`), nil
		},
	}

	if _, err := c.Executar(context.Background(), map[string]any{"instrucao": "faz X"}); err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	for _, arg := range argsRecebidos {
		if arg == "--session-id" || arg == "--resume" {
			t.Errorf("sem sessão no ctx, não deveria haver %q: %v", arg, argsRecebidos)
		}
	}
}
