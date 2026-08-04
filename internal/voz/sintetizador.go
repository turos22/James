// Package voz gera áudio via edge-tts, um arquivo por resposta. O servidor
// nunca toca o áudio — só gera e entrega por URL; quem toca é o cliente
// (spec 4.7, decisão D1).
package voz

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Sintetizador struct {
	Executor       func(ctx context.Context, texto, voz, arquivoSaida string) error
	DiretorioSaida string
	Timeout        time.Duration
}

func NovoSintetizador(diretorioSaida string) *Sintetizador {
	return &Sintetizador{
		Executor:       executarEdgeTTS,
		DiretorioSaida: diretorioSaida,
		Timeout:        30 * time.Second,
	}
}

func executarEdgeTTS(ctx context.Context, texto, voz, arquivoSaida string) error {
	cmd := exec.CommandContext(ctx, "edge-tts", "--voice", voz, "--text", texto, "--write-media", arquivoSaida)
	return cmd.Run()
}

// Gerar sintetiza texto em áudio e devolve o caminho do arquivo — nome
// único por id de resposta, nunca um arquivo fixo compartilhado (bug #7).
// Erro do Executor sempre propaga (nunca é engolido, R4 + bug #7).
func (s *Sintetizador) Gerar(ctx context.Context, texto, voz, id string) (string, error) {
	ctx, cancelar := context.WithTimeout(ctx, s.Timeout)
	defer cancelar()

	if err := os.MkdirAll(s.DiretorioSaida, 0o755); err != nil {
		return "", fmt.Errorf("erro ao preparar diretório de áudio: %w", err)
	}

	arquivo := filepath.Join(s.DiretorioSaida, id+".mp3")
	if err := s.Executor(ctx, texto, voz, arquivo); err != nil {
		return "", fmt.Errorf("erro ao gerar áudio: %w", err)
	}
	return arquivo, nil
}
