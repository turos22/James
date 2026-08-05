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

// construirClaudeCode ainda não aplica nenhuma configuração: cfg.ClaudeCodeDiretorio
// e cfg.ClaudeCodeTimeoutSegundos existem desde a Task 7, mas ferramentas.ClaudeCode
// ainda não tem campo pra eles. A Task 15 preenche esta função.
func construirClaudeCode(cfg *config.Config) *ferramentas.ClaudeCode {
	return ferramentas.NovoClaudeCode()
}
