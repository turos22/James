// Package config carrega segredos do .env e o resto da configuração do
// config.toml, com falha rápida: falta algo obrigatório, o jamesd não sobe.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/joho/godotenv"
)

type Config struct {
	GeminiAPIKey string
	ClaudeAPIKey string
	TavilyAPIKey string
	DatabaseURL  string

	BindLoopback string
	BindVPNPorta int
	CertArquivo  string
	ChaveArquivo string

	// Provedor é quem responde: "anthropic" (padrão) ou "gemini".
	Provedor        string
	Modelo          string
	MaxIteracoes    int
	TimeoutSegundos int

	OrcamentoTokens  int
	OrcamentoNotas   int
	OrcamentoResumo  int
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

	// MotorVoz é quem sintetiza a resposta do turno de texto: "piper" (padrão,
	// local) ou "edge". TTSURL é o servidor de TTS do Piper, sempre loopback.
	MotorVoz string
	TTSURL   string
}

type arquivoToml struct {
	Servidor struct {
		BindLoopback string `toml:"bind_loopback"`
		BindVPNPorta int    `toml:"bind_vpn_porta"`
		CertArquivo  string `toml:"cert_arquivo"`
		ChaveArquivo string `toml:"chave_arquivo"`
	} `toml:"servidor"`
	Agente struct {
		Provedor        string `toml:"provedor"`
		Modelo          string `toml:"modelo"`
		MaxIteracoes    int    `toml:"max_iteracoes"`
		TimeoutSegundos int    `toml:"timeout_segundos"`
	} `toml:"agente"`
	Memoria struct {
		OrcamentoTokens  int    `toml:"orcamento_tokens"`
		OrcamentoNotas   int    `toml:"orcamento_notas"`
		OrcamentoResumo  int    `toml:"orcamento_resumo"`
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
		Motor  string `toml:"motor"`
		TTSURL string `toml:"tts_url"`
	} `toml:"voz"`
}

// modeloDescartado é o único modelo proibido pela spec — o mais fraco da
// família Gemini, causa provável do planejamento ruim de tools no app antigo.
const modeloDescartado = "gemini-2.5-flash-lite"

const (
	provedorPadrao = "anthropic"
	motorVozPadrao = "piper"
	ttsURLPadrao   = "http://127.0.0.1:8180"
)

// exigirLoopback recusa qualquer URL cujo host não seja loopback. O servidor
// de TTS é local e não tem autenticação, e recebe o texto das respostas, que
// pode ter dado pessoal: apontá-lo pra fora da máquina é vazamento, e não
// existe motivo legítimo pra isso.
func exigirLoopback(bruta string) error {
	u, err := url.Parse(bruta)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("URL inválida %q", bruta)
	}
	host := u.Hostname()
	if host == "localhost" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return nil
	}
	return fmt.Errorf("%q não é loopback (use 127.0.0.1, localhost ou ::1)", host)
}

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
		ClaudeAPIKey: os.Getenv("CLAUDE_API_KEY"),
		TavilyAPIKey: os.Getenv("TAVILY_API_KEY"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),

		BindLoopback: valorOuDefault(arq.Servidor.BindLoopback, "127.0.0.1:8080"),
		BindVPNPorta: intOuDefault(arq.Servidor.BindVPNPorta, 8443),
		CertArquivo:  valorOuDefault(arq.Servidor.CertArquivo, "certs/tailscale.crt"),
		ChaveArquivo: valorOuDefault(arq.Servidor.ChaveArquivo, "certs/tailscale.key"),

		Provedor:        valorOuDefault(arq.Agente.Provedor, provedorPadrao),
		Modelo:          arq.Agente.Modelo, // o padrão depende do provedor, abaixo
		MaxIteracoes:    intOuDefault(arq.Agente.MaxIteracoes, 5),
		TimeoutSegundos: intOuDefault(arq.Agente.TimeoutSegundos, 30),

		OrcamentoTokens:  intOuDefault(arq.Memoria.OrcamentoTokens, 8000),
		OrcamentoNotas:   intOuDefault(arq.Memoria.OrcamentoNotas, 2000),
		OrcamentoResumo:  intOuDefault(arq.Memoria.OrcamentoResumo, 1000),
		MemoriaDiretorio: valorOuDefault(arq.Memoria.DiretorioMemoria, "Vault/Memoria"),

		TimeoutAprovacaoSegundos: intOuDefault(arq.Permissao.TimeoutAprovacaoSegundos, 300),

		TerminalTimeoutSegundos: intOuDefault(arq.Ferramentas.Terminal.TimeoutSegundos, 60),
		TerminalMaxBytesSaida:   intOuDefault(arq.Ferramentas.Terminal.MaxBytesSaida, 65536),

		ClaudeCodeDiretorio:       arq.Ferramentas.ClaudeCode.Diretorio,
		ClaudeCodeTimeoutSegundos: intOuDefault(arq.Ferramentas.ClaudeCode.TimeoutSegundos, 600),

		BuscaTimeoutSegundos: intOuDefault(arq.Ferramentas.Busca.TimeoutSegundos, 20),
		BuscaMaxResultados:   intOuDefault(arq.Ferramentas.Busca.MaxResultados, 5),

		VozTTS:   valorOuDefault(arq.Voz.VozTTS, "pt-BR-AntonioNeural"),
		MotorVoz: valorOuDefault(arq.Voz.Motor, motorVozPadrao),
		TTSURL:   valorOuDefault(arq.Voz.TTSURL, ttsURLPadrao),
	}

	// O config.toml é versionado e serve Windows e Fedora; o caminho do
	// workspace do Claude Code é da máquina. Env não vazia vence.
	if d := os.Getenv("JAMES_CLAUDE_CODE_DIRETORIO"); d != "" {
		cfg.ClaudeCodeDiretorio = d
	}

	// O modelo padrão acompanha o provedor: um único default global daria
	// "gemini-2.5-flash" mandado pra API da Anthropic.
	switch cfg.Provedor {
	case "anthropic":
		cfg.Modelo = valorOuDefault(cfg.Modelo, "claude-haiku-5-5")
	case "gemini":
		cfg.Modelo = valorOuDefault(cfg.Modelo, "gemini-2.5-flash")
	default:
		return nil, fmt.Errorf("provedor %q desconhecido em [agente] provedor (use \"anthropic\" ou \"gemini\")", cfg.Provedor)
	}

	var faltando []string
	if cfg.Provedor == "anthropic" && cfg.ClaudeAPIKey == "" {
		faltando = append(faltando, "CLAUDE_API_KEY")
	}
	if cfg.Provedor == "gemini" && cfg.GeminiAPIKey == "" {
		faltando = append(faltando, "GEMINI_API_KEY")
	}
	if cfg.DatabaseURL == "" {
		faltando = append(faltando, "DATABASE_URL")
	}
	if len(faltando) > 0 {
		return nil, fmt.Errorf("configuração incompleta, faltando: %s", strings.Join(faltando, ", "))
	}

	// A proibição do flash-lite é da família Gemini.
	if cfg.Provedor == "gemini" && cfg.Modelo == modeloDescartado {
		return nil, fmt.Errorf("modelo %q está descartado pela spec (o mais fraco da família Gemini)", modeloDescartado)
	}

	if cfg.MotorVoz != "piper" && cfg.MotorVoz != "edge" {
		return nil, fmt.Errorf("motor de voz %q desconhecido em [voz] motor (use \"piper\" ou \"edge\")", cfg.MotorVoz)
	}
	if err := exigirLoopback(cfg.TTSURL); err != nil {
		return nil, fmt.Errorf("[voz] tts_url: %w", err)
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
