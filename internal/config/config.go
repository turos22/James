// Package config carrega segredos do .env e o resto da configuração do
// config.toml, com falha rápida: falta algo obrigatório, o jamesd não sobe.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
)

type Config struct {
	GeminiAPIKey string
	TavilyAPIKey string
	DatabaseURL  string

	BindLoopback string
	BindVPNPorta int
	CertArquivo  string
	ChaveArquivo string

	Modelo          string
	MaxIteracoes    int
	TimeoutSegundos int

	OrcamentoTokens  int
	OrcamentoNotas   int
	MemoriaDiretorio string

	TimeoutAprovacaoSegundos int

	TerminalTimeoutSegundos int
	TerminalMaxBytesSaida   int

	// ClaudeCodeDiretorio é o único campo sem default. Sem ele o jamesd sobe
	// normalmente e a ferramenta é que falha na chamada — o James funciona
	// sem nunca invocar o Claude Code, então campo não preenchido não pode
	// impedir o serviço de subir.
	ClaudeCodeDiretorio       string
	ClaudeCodeTimeoutSegundos int

	BuscaTimeoutSegundos int
	BuscaMaxResultados   int

	VozTTS string
}

type arquivoToml struct {
	Servidor struct {
		BindLoopback string `toml:"bind_loopback"`
		BindVPNPorta int    `toml:"bind_vpn_porta"`
		CertArquivo  string `toml:"cert_arquivo"`
		ChaveArquivo string `toml:"chave_arquivo"`
	} `toml:"servidor"`
	Agente struct {
		Modelo          string `toml:"modelo"`
		MaxIteracoes    int    `toml:"max_iteracoes"`
		TimeoutSegundos int    `toml:"timeout_segundos"`
	} `toml:"agente"`
	Memoria struct {
		OrcamentoTokens  int    `toml:"orcamento_tokens"`
		OrcamentoNotas   int    `toml:"orcamento_notas"`
		DiretorioMemoria string `toml:"diretorio_memoria"`
	} `toml:"memoria"`
	Permissao struct {
		TimeoutAprovacaoSegundos int `toml:"timeout_aprovacao_segundos"`
	} `toml:"permissao"`
	Ferramentas struct {
		Terminal struct {
			TimeoutSegundos int `toml:"timeout_segundos"`
			MaxBytesSaida   int `toml:"max_bytes_saida"`
		} `toml:"terminal"`
		ClaudeCode struct {
			Diretorio       string `toml:"diretorio"`
			TimeoutSegundos int    `toml:"timeout_segundos"`
		} `toml:"claude_code"`
		Busca struct {
			TimeoutSegundos int `toml:"timeout_segundos"`
			MaxResultados   int `toml:"max_resultados"`
		} `toml:"busca"`
	} `toml:"ferramentas"`
	Voz struct {
		VozTTS string `toml:"voz_tts"`
	} `toml:"voz"`
}

// modeloDescartado é o único modelo proibido pela spec — o mais fraco da
// família Gemini, causa provável do planejamento ruim de tools no app antigo.
const modeloDescartado = "gemini-2.5-flash-lite"

// Carregar lê o .env (mesmo diretório do config.toml, se existir) e o
// config.toml em caminhoToml, aplica defaults e falha rápido se faltar
// segredo obrigatório ou se o modelo estiver descartado.
func Carregar(caminhoToml string) (*Config, error) {
	_ = godotenv.Load(".env")

	var arq arquivoToml
	if _, err := os.Stat(caminhoToml); err == nil {
		if _, err := toml.DecodeFile(caminhoToml, &arq); err != nil {
			return nil, fmt.Errorf("erro ao ler %s: %w", caminhoToml, err)
		}
	}

	cfg := &Config{
		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		TavilyAPIKey: os.Getenv("TAVILY_API_KEY"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),

		BindLoopback: valorOuDefault(arq.Servidor.BindLoopback, "127.0.0.1:8080"),
		BindVPNPorta: intOuDefault(arq.Servidor.BindVPNPorta, 8443),
		CertArquivo:  valorOuDefault(arq.Servidor.CertArquivo, "certs/tailscale.crt"),
		ChaveArquivo: valorOuDefault(arq.Servidor.ChaveArquivo, "certs/tailscale.key"),

		Modelo:          valorOuDefault(arq.Agente.Modelo, "gemini-2.5-flash"),
		MaxIteracoes:    intOuDefault(arq.Agente.MaxIteracoes, 5),
		TimeoutSegundos: intOuDefault(arq.Agente.TimeoutSegundos, 30),

		OrcamentoTokens:  intOuDefault(arq.Memoria.OrcamentoTokens, 8000),
		OrcamentoNotas:   intOuDefault(arq.Memoria.OrcamentoNotas, 2000),
		MemoriaDiretorio: valorOuDefault(arq.Memoria.DiretorioMemoria, "Vault/Memoria"),

		TimeoutAprovacaoSegundos: intOuDefault(arq.Permissao.TimeoutAprovacaoSegundos, 300),

		TerminalTimeoutSegundos: intOuDefault(arq.Ferramentas.Terminal.TimeoutSegundos, 60),
		TerminalMaxBytesSaida:   intOuDefault(arq.Ferramentas.Terminal.MaxBytesSaida, 65536),

		ClaudeCodeDiretorio:       arq.Ferramentas.ClaudeCode.Diretorio,
		ClaudeCodeTimeoutSegundos: intOuDefault(arq.Ferramentas.ClaudeCode.TimeoutSegundos, 600),

		BuscaTimeoutSegundos: intOuDefault(arq.Ferramentas.Busca.TimeoutSegundos, 20),
		BuscaMaxResultados:   intOuDefault(arq.Ferramentas.Busca.MaxResultados, 5),

		VozTTS: valorOuDefault(arq.Voz.VozTTS, "pt-BR-AntonioNeural"),
	}

	var faltando []string
	if cfg.GeminiAPIKey == "" {
		faltando = append(faltando, "GEMINI_API_KEY")
	}
	if cfg.DatabaseURL == "" {
		faltando = append(faltando, "DATABASE_URL")
	}
	if len(faltando) > 0 {
		return nil, fmt.Errorf("configuração incompleta, faltando: %s", strings.Join(faltando, ", "))
	}

	if cfg.Modelo == modeloDescartado {
		return nil, fmt.Errorf("modelo %q está descartado pela spec (o mais fraco da família Gemini)", modeloDescartado)
	}

	return cfg, nil
}

func valorOuDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func intOuDefault(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}
