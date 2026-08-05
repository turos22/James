// Comando jamesd: back-end de serviço do James. Sobe HTTP em loopback
// (shell desktop) e HTTPS na interface do Tailscale (celular), nunca em
// 0.0.0.0 (invariantes I1/I2 da spec).
package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"time"

	"james/internal/agente"
	"james/internal/agente/gemini"
	"james/internal/api"
	"james/internal/armazenamento/postgres"
	"james/internal/config"
	"james/internal/dispositivo"
	"james/internal/ferramentas"
	"james/internal/memoria"
	"james/internal/voz"
	"james/web"
)

func main() {
	if err := executar(); err != nil {
		slog.Error("jamesd não subiu", "erro", err)
		os.Exit(1)
	}
}

func executar() error {
	ctx := context.Background()

	cfg, err := config.Carregar("config.toml")
	if err != nil {
		return fmt.Errorf("configuração: %w", err)
	}

	pool, err := postgres.Abrir(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("banco: %w", err)
	}
	defer pool.Close()

	if err := postgres.AplicarMigracoes(ctx, pool); err != nil {
		return fmt.Errorf("migrações: %w", err)
	}

	provider, err := gemini.Novo(ctx, cfg.GeminiAPIKey, cfg.Modelo)
	if err != nil {
		return fmt.Errorf("gemini: %w", err)
	}

	sistemaPrompt, err := os.ReadFile("prompts/sistema.md")
	if err != nil {
		return fmt.Errorf("system prompt: %w", err)
	}

	dispositivoHub := dispositivo.NovoHub()

	registro := ferramentas.NovoRegistro()
	comandoSistema := ferramentas.NovoComandoSistema()
	comandoSistema.Timeout = time.Duration(cfg.TerminalTimeoutSegundos) * time.Second
	comandoSistema.MaxBytesSaida = cfg.TerminalMaxBytesSaida
	if err := registro.Registrar(comandoSistema); err != nil {
		return fmt.Errorf("registrar executar_comando_sistema: %w", err)
	}
	if err := registro.Registrar(ferramentas.NovoClaudeCode()); err != nil {
		return fmt.Errorf("registrar perguntar_ao_claude_code: %w", err)
	}
	if err := registro.Registrar(ferramentas.NovaBusca(cfg.TavilyAPIKey)); err != nil {
		return fmt.Errorf("registrar pesquisar_internet: %w", err)
	}
	if err := registro.Registrar(ferramentas.NovoManipularInterface(dispositivoHub)); err != nil {
		return fmt.Errorf("registrar manipular_interface: %w", err)
	}

	raizSPA, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return fmt.Errorf("assets da SPA: %w (rodou \"npm run build\" em web/ antes do go build?)", err)
	}

	servidor := &api.Servidor{
		Sessoes:        postgres.NovoSessaoRepositorio(pool),
		Mensagens:      postgres.NovoMensagemRepositorio(pool),
		Aprovacoes:     postgres.NovoAprovacaoRepositorio(pool),
		DiretorioAudio: "audios",
		EmbutidosSPA:   raizSPA,
		Loop: &agente.Loop{
			Provider:     provider,
			Ferramentas:  registro,
			MaxIteracoes: cfg.MaxIteracoes,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: cfg.OrcamentoTokens},
		SistemaPrompt:       string(sistemaPrompt),
		Clientes:            api.NovoClientesHub(),
		AprovacoesPendentes: api.NovoRegistroAprovacoesPendentes(),
		TimeoutAprovacao:    time.Duration(cfg.TimeoutAprovacaoSegundos) * time.Second,
		DispositivoHub:      dispositivoHub,
		Sintetizador:        voz.NovoSintetizador("audios"),
		VozTTS:              cfg.VozTTS,
		TimeoutTurno:        time.Duration(cfg.TimeoutSegundos) * time.Second,
	}

	// Só o loopback é fatal pro processo — é o serviço mínimo que o shell
	// desktop depende. A VPN (Tailscale) é sempre melhor-esforço: sem
	// interface, sem certificado válido, ou caindo depois de já ter subido,
	// o celular fica sem alcance mas o loopback segue de pé. R1 é sobre
	// segredo/config ausente, não sobre rede externa instável ou cert
	// ainda não emitido (primeiro deploy, por exemplo).
	errosServidor := make(chan error, 1)

	go func() {
		slog.Info("servindo em loopback", "endereco", cfg.BindLoopback)
		errosServidor <- http.ListenAndServe(cfg.BindLoopback, servidor.Rotas())
	}()

	ipVPN, err := resolverIPTailscale()
	if err != nil {
		slog.Error("interface do Tailscale não encontrada — servindo só em loopback; celular não vai alcançar o jamesd até isso ser corrigido", "erro", err)
	} else {
		enderecoVPN := fmt.Sprintf("%s:%d", ipVPN, cfg.BindVPNPorta)
		go func() {
			slog.Info("servindo na VPN com TLS", "endereco", enderecoVPN)
			if err := http.ListenAndServeTLS(enderecoVPN, cfg.CertArquivo, cfg.ChaveArquivo, servidor.Rotas()); err != nil {
				slog.Error("servidor da VPN caiu — celular sem alcance até corrigir; loopback continua de pé", "erro", err)
			}
		}()
	}

	return <-errosServidor
}
