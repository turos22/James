package ferramentas

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestClaudeCodeDisparaComInstrucao(t *testing.T) {
	var instrucaoRecebida string
	c := &ClaudeCode{Executor: func(instrucao string) error {
		instrucaoRecebida = instrucao
		return nil
	}}

	saida, err := c.Executar(context.Background(), map[string]any{"instrucao": "refatora o main.go"})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if instrucaoRecebida != "refatora o main.go" {
		t.Errorf("instrucao repassada = %q", instrucaoRecebida)
	}
	if saida == "" {
		t.Error("esperava confirmação de disparo, veio vazio")
	}
}

func TestClaudeCodeRejeitaArgumentoAusente(t *testing.T) {
	c := &ClaudeCode{Executor: func(string) error { return nil }}
	if _, err := c.Executar(context.Background(), map[string]any{}); err == nil {
		t.Fatal("esperava erro por parâmetro instrucao ausente, veio nil")
	}
}

func TestClaudeCodePropagaErroDoExecutor(t *testing.T) {
	c := &ClaudeCode{Executor: func(string) error { return errors.New("falhou ao abrir") }}
	if _, err := c.Executar(context.Background(), map[string]any{"instrucao": "x"}); err == nil {
		t.Fatal("esperava erro propagado, veio nil")
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
