package ferramentas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// ClaudeCode roda o agente Claude Code headless e devolve ao modelo a
// resposta real. A versão anterior era fire-and-forget: abria uma janela e
// devolvia uma string fixa de confirmação, então o agente nunca sabia se a
// tarefa tinha dado certo (Anexo A #18).
type ClaudeCode struct {
	// Executor recebe o diretório de trabalho e os argumentos já montados, e
	// devolve a stdout crua do processo.
	Executor func(ctx context.Context, diretorio string, args []string) ([]byte, error)

	// Diretorio vem do config.toml e nunca é herdado do cwd do jamesd — sem
	// isso, o James mandaria o Claude Code editar o próprio código do James.
	Diretorio string

	Timeout time.Duration

	// mu protege sessoesIniciadas: sessões diferentes despacham em paralelo, e
	// uma mesma sessão pode ter mais de uma chamada em voo.
	mu               sync.Mutex
	sessoesIniciadas map[string]bool
}

func (c *ClaudeCode) jaIniciou(sessaoID string) bool {
	if sessaoID == "" {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessoesIniciadas[sessaoID]
}

func (c *ClaudeCode) marcarIniciada(sessaoID string) {
	if sessaoID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sessoesIniciadas == nil {
		c.sessoesIniciadas = make(map[string]bool)
	}
	c.sessoesIniciadas[sessaoID] = true
}

func NovoClaudeCode() *ClaudeCode {
	return &ClaudeCode{Executor: rodarClaudeCode}
}

// resultadoClaudeCode é o recorte tipado do JSON de --output-format json.
type resultadoClaudeCode struct {
	Result    string `json:"result"`
	IsError   bool   `json:"is_error"`
	SessionID string `json:"session_id"`
}

func rodarClaudeCode(ctx context.Context, diretorio string, args []string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = diretorio
	cmd.Env = AmbienteSemAnthropic(os.Environ())

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

func (c *ClaudeCode) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "perguntar_ao_claude_code",
		Descricao: "Usa o agente Claude Code pra interagir com o ambiente de desenvolvimento. Devolve a resposta do agente.",
		Parametros: map[string]Parametro{
			"instrucao": {Tipo: "string", Descricao: "instrução ou mensagem pro Claude Code"},
		},
		Obrigatorios: []string{"instrucao"},
	}
}

func (c *ClaudeCode) Nivel() Nivel { return NivelPerigoso }

// ConfirmacaoNecessaria implementa ExigeConfirmacao e devolve sempre true
// (decisão D10). Um spike mostrou que --permission-mode acceptEdits EXECUTA
// comando de shell em modo headless, sem recusa — a premissa contrária, que
// motivou a escolha original de acceptEdits, era falsa. Sem esta confirmação,
// o modelo teria pelo Claude Code um caminho indireto pra shell arbitrário,
// contornando o classificador e o gate que esta mesma fase construiu pro
// executar_comando_sistema.
//
// Sempre true, e não condicional aos args: a instrução é texto livre em
// linguagem natural, e não há como classificar com honestidade o que o Claude
// Code vai decidir fazer com ela.
func (c *ClaudeCode) ConfirmacaoNecessaria(map[string]any) bool { return true }

func (c *ClaudeCode) Executar(ctx context.Context, args map[string]any) (string, error) {
	instrucao, ok := args["instrucao"].(string)
	if !ok || instrucao == "" {
		return "", fmt.Errorf("parâmetro instrucao ausente ou vazio")
	}
	if c.Diretorio == "" {
		return "", fmt.Errorf("diretório de trabalho do Claude Code não configurado " +
			"(ferramentas.claude_code.diretorio no config.toml)")
	}

	ctxClaude := ctx
	if c.Timeout > 0 {
		var cancelar context.CancelFunc
		ctxClaude, cancelar = context.WithTimeout(ctx, c.Timeout)
		defer cancelar()
	}

	sessaoID := SessaoDo(ctx)
	bruto, err := c.Executor(ctxClaude, c.Diretorio,
		MontarArgs(instrucao, sessaoID, c.jaIniciou(sessaoID)))
	if errors.Is(ctxClaude.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("Claude Code excedeu o tempo limite de %s e foi encerrado", c.Timeout)
	}
	if err != nil {
		return "", fmt.Errorf("erro ao rodar Claude Code: %w", err)
	}

	// Só no caminho de sucesso: se a primeira chamada morreu antes de a sessão
	// existir, a próxima precisa tentar criar de novo, não retomar uma sessão
	// que nunca existiu.
	c.marcarIniciada(sessaoID)

	var res resultadoClaudeCode
	if err := json.Unmarshal(bruto, &res); err != nil {
		return "", fmt.Errorf("saída do Claude Code em formato inesperado: %w", err)
	}
	if res.IsError {
		return "", fmt.Errorf("Claude Code reportou erro: %s", res.Result)
	}
	return res.Result, nil
}

// variaveisQueSequestram são as variáveis de ambiente que apontariam o Claude
// Code pra outro provider em vez da assinatura Pro via OAuth. O script
// rodar_claude.ps1 fazia essa limpeza em PowerShell, onde nada era testado;
// aqui vira código com teste (Anexo A #17).
var variaveisQueSequestram = []string{
	"ANTHROPIC_BASE_URL=",
	"ANTHROPIC_API_KEY=",
	"ANTHROPIC_AUTH_TOKEN=",
}

// AmbienteSemAnthropic devolve o ambiente recebido sem as variáveis que
// sequestrariam a autenticação. Compara pelo prefixo com "=" pra não remover
// variável de nome parecido (ANTHROPIC_MODEL, por exemplo, sobrevive).
func AmbienteSemAnthropic(ambiente []string) []string {
	limpo := make([]string, 0, len(ambiente))
	for _, v := range ambiente {
		sequestra := false
		for _, proibida := range variaveisQueSequestram {
			if strings.HasPrefix(v, proibida) {
				sequestra = true
				break
			}
		}
		if !sequestra {
			limpo = append(limpo, v)
		}
	}
	return limpo
}

// MontarArgs monta a linha de comando do Claude Code headless. sessaoID vazio
// omite as flags de sessão. retomar escolhe entre --session-id (primeira
// chamada, cria a sessão) e --resume (chamadas seguintes, continuam nela) —
// verificado por execução real (spike da Task 13): reusar --session-id no
// mesmo id dá erro "Session ID already in use", e --resume no mesmo id
// preserva o contexto.
func MontarArgs(instrucao, sessaoID string, retomar bool) []string {
	args := []string{
		"-p", instrucao,
		"--output-format", "json",
		"--permission-mode", "acceptEdits",
	}
	if sessaoID == "" {
		return args
	}
	if retomar {
		return append(args, "--resume", sessaoID)
	}
	return append(args, "--session-id", sessaoID)
}
