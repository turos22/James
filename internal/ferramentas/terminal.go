package ferramentas

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ComandoSistema executa comandos no shell da plataforma. Não há allowlist de comando — a
// contenção é confirmação humana pro que destrói, abre a máquina ou executa
// código baixado (ver classificador.go), mais timeout e limite de saída.
type ComandoSistema struct {
	// Executor recebe o ctx pra que o timeout mate o processo de verdade, e
	// não só abandone a espera.
	Executor func(ctx context.Context, comando string) (ResultadoComando, error)

	// Timeout zero desliga o limite de tempo. MaxBytesSaida zero desliga a
	// truncagem. Ambos vêm do config.toml (Task 8).
	Timeout       time.Duration
	MaxBytesSaida int

	// Classificador decide se o comando precisa de confirmação humana. É
	// injetado por plataforma (plataforma_*.go), como o Executor. Nil é fail
	// closed: tudo pede confirmação.
	Classificador func(comando string) bool

	// NomeShell aparece na descrição que o modelo lê. Sem ele o modelo não
	// sabe se escreve PowerShell ou bash.
	NomeShell string
}

func (c *ComandoSistema) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "executar_comando_sistema",
		Descricao: fmt.Sprintf("Executa comandos %s. Se não souber o comando pra uma tarefa, use pesquisar_internet primeiro.", c.NomeShell),
		Parametros: map[string]Parametro{
			"comando": {Tipo: "string", Descricao: fmt.Sprintf("comando %s a executar", c.NomeShell)},
		},
		Obrigatorios: []string{"comando"},
	}
}

func (c *ComandoSistema) Nivel() Nivel { return NivelPerigoso }

// ConfirmacaoNecessaria implementa ExigeConfirmacao: comando destrutivo pede
// confirmação humana mesmo do perfil Privilegiado. Comando comum — inclusive
// de escrita — passa direto. Argumento ausente ou de tipo errado devolve true
// (fail closed); a validação de argumento em si continua em Executar.
func (c *ComandoSistema) ConfirmacaoNecessaria(args map[string]any) bool {
	comando, ok := args["comando"].(string)
	if !ok {
		return true
	}
	if c.Classificador == nil {
		return true
	}
	return c.Classificador(comando)
}

func (c *ComandoSistema) Executar(ctx context.Context, args map[string]any) (string, error) {
	comando, ok := args["comando"].(string)
	if !ok || comando == "" {
		return "", fmt.Errorf("parâmetro comando ausente ou vazio")
	}

	ctxComando := ctx
	if c.Timeout > 0 {
		var cancelar context.CancelFunc
		ctxComando, cancelar = context.WithTimeout(ctx, c.Timeout)
		defer cancelar()
	}

	res, err := c.Executor(ctxComando, comando)

	// Timeout é reportado COMO timeout, distinguível de erro de execução: o
	// modelo precisa saber que o comando não terminou, e não que ele falhou.
	if errors.Is(ctxComando.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("comando excedeu o tempo limite de %s e foi encerrado", c.Timeout)
	}
	if err != nil {
		return "", fmt.Errorf("erro ao executar: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("comando terminou com exit code %d: %s",
			res.ExitCode, truncar(res.Stderr, c.MaxBytesSaida))
	}
	return formatarResultadoComando(res, c.MaxBytesSaida), nil
}
