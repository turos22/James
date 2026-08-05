// Comando jamesd: back-end de serviço do James. Sobe HTTP em loopback
// (shell desktop) e HTTPS na interface do Tailscale (celular), nunca em
// 0.0.0.0 (invariantes I1/I2 da spec).
package main

import (
	"context"
	"fmt"
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
	if err := registro.Registrar(ferramentas.NovoComandoSistema()); err != nil {
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

	servidor := &api.Servidor{
		Sessoes:        postgres.NovoSessaoRepositorio(pool),
		Mensagens:      postgres.NovoMensagemRepositorio(pool),
		Aprovacoes:     postgres.NovoAprovacaoRepositorio(pool),
		DiretorioAudio: "audios",
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

	errosServidor := make(chan error, 2)

	go func() {
		slog.Info("servindo em loopback", "endereco", cfg.BindLoopback)
		errosServidor <- http.ListenAndServe(cfg.BindLoopback, servidor.Rotas())
	}()

	ipVPN, err := resolverIPTailscale()
	if err != nil {
		// Não é fatal: o shell desktop (loopback) continua funcionando.
		// Só o celular fica sem alcance até a VPN voltar — R1 é sobre
		// segredo/config ausente, não sobre uma rede externa instável.
		slog.Error("interface do Tailscale não encontrada — servindo só em loopback; celular não vai alcançar o jamesd até isso ser corrigido", "erro", err)
	} else {
		enderecoVPN := fmt.Sprintf("%s:%d", ipVPN, cfg.BindVPNPorta)
		go func() {
			slog.Info("servindo na VPN com TLS", "endereco", enderecoVPN)
			errosServidor <- http.ListenAndServeTLS(enderecoVPN, cfg.CertArquivo, cfg.ChaveArquivo, servidor.Rotas())
		}()
	}

	return <-errosServidor
}
