package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"james/internal/agente"
	"james/internal/agente/anthropic"
	"james/internal/agente/gemini"
	"james/internal/config"
	"james/internal/ferramentas"
)

// construirComandoSistema aplica timeout e limite de saída vindos do
// config.toml na ferramenta de terminal. Extraída de executar() pro teste
// poder verificar a injeção sem subir o jamesd inteiro — mesmo motivo de
// resolverIPTailscale, em tailscale.go: executar() abre Postgres de verdade,
// usa caminho relativo pra config.toml/.env, e termina bloqueada num canal
// que só retorna quando o servidor cai.
func construirComandoSistema(cfg *config.Config) *ferramentas.ComandoSistema {
	c := ferramentas.NovoComandoSistema()
	c.Timeout = time.Duration(cfg.TerminalTimeoutSegundos) * time.Second
	c.MaxBytesSaida = cfg.TerminalMaxBytesSaida
	return c
}

// registrarTerminal registra executar_comando_sistema só onde a plataforma
// tem denylist própria (ferramentas.TerminalSuportado). Recebe o booleano
// por parâmetro pro teste cobrir os dois lados em qualquer sistema.
func registrarTerminal(registro ferramentas.Registro, cfg *config.Config, suportado bool) error {
	if !suportado {
		slog.Warn("terminal desligado nesta plataforma: sem denylist própria")
		return nil
	}
	return registro.Registrar(construirComandoSistema(cfg))
}

// construirBusca aplica timeout e limite de resultados vindos do config.toml
// na ferramenta de busca — mesmo motivo de construirComandoSistema: nunca o
// cliente HTTP default sem timeout, e nunca a ferramenta funcionando "quase
// certo" (sem limite) sem que um teste acuse.
func construirBusca(cfg *config.Config) *ferramentas.Busca {
	b := ferramentas.NovaBusca(cfg.TavilyAPIKey,
		time.Duration(cfg.BuscaTimeoutSegundos)*time.Second)
	b.MaxResultados = cfg.BuscaMaxResultados
	return b
}

// construirClaudeCode aplica diretório e timeout vindos do config.toml.
// Diretorio nunca é herdado do cwd do jamesd — sem isso, o James poderia
// mandar o Claude Code editar o próprio código-fonte do James (decisão D6).
func construirClaudeCode(cfg *config.Config) *ferramentas.ClaudeCode {
	c := ferramentas.NovoClaudeCode()
	c.Diretorio = cfg.ClaudeCodeDiretorio
	c.Timeout = time.Duration(cfg.ClaudeCodeTimeoutSegundos) * time.Second
	return c
}

// construirProvedor escolhe quem responde pelo [agente] provedor do
// config.toml. Extraída de executar() pelo mesmo motivo das outras (abrir
// Postgres de verdade e bloquear num canal): assim dá pra testar a escolha.
func construirProvedor(ctx context.Context, cfg *config.Config) (agente.Provider, error) {
	switch cfg.Provedor {
	case "anthropic":
		return anthropic.Novo(cfg.ClaudeAPIKey, cfg.Modelo), nil
	case "gemini":
		p, err := gemini.Novo(ctx, cfg.GeminiAPIKey, cfg.Modelo)
		if err != nil {
			return nil, fmt.Errorf("gemini: %w", err)
		}
		return p, nil
	default:
		return nil, fmt.Errorf("provedor %q desconhecido", cfg.Provedor)
	}
}
