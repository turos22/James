package main

import (
	"time"

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
