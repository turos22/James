// Package voz gera áudio por resposta (Piper local ou edge-tts), um arquivo por resposta. O servidor
// nunca toca o áudio — só gera e entrega por URL; quem toca é o cliente
// (spec 4.7, decisão D1).
package voz

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type Sintetizador struct {
	Executor       func(ctx context.Context, texto, voz, arquivoSaida string) error
	DiretorioSaida string
	Timeout        time.Duration

	// Extensao do arquivo gerado, com o ponto. Vazia é ".mp3" (o edge-tts).
	Extensao string
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
	// O texto na tela tem Markdown; a voz não pode ler os asteriscos. Vale
	// pros dois motores.
	texto = LimparParaFala(texto)
	if texto == "" {
		return "", errors.New("nada para falar depois de limpar o texto")
	}

	ctx, cancelar := context.WithTimeout(ctx, s.Timeout)
	defer cancelar()

	if err := os.MkdirAll(s.DiretorioSaida, 0o755); err != nil {
		return "", fmt.Errorf("erro ao preparar diretório de áudio: %w", err)
	}

	extensao := s.Extensao
	if extensao == "" {
		extensao = ".mp3"
	}
	arquivo := filepath.Join(s.DiretorioSaida, id+extensao)
	if err := s.Executor(ctx, texto, voz, arquivo); err != nil {
		return "", fmt.Errorf("erro ao gerar áudio: %w", err)
	}
	return arquivo, nil
}

// NovoSintetizadorPiper gera a voz do turno de texto no servidor de TTS local
// do Piper (device/servidor_tts.py), a mesma voz que o device fala. A voz é a
// do servidor; o parâmetro voz de Gerar é ignorado.
func NovoSintetizadorPiper(diretorioSaida, urlServidor string) *Sintetizador {
	return &Sintetizador{
		Executor:       executarPiper(urlServidor),
		DiretorioSaida: diretorioSaida,
		Timeout:        15 * time.Second,
		Extensao:       ".wav",
	}
}

// tamanhoMaximoDeAudio protege o disco de um servidor de TTS defeituoso.
var tamanhoMaximoDeAudio int64 = 64 << 20

func executarPiper(urlServidor string) func(ctx context.Context, texto, voz, arquivoSaida string) error {
	cliente := &http.Client{Timeout: 10 * time.Second}
	return func(ctx context.Context, texto, _, arquivoSaida string) error {
		corpo, err := json.Marshal(map[string]string{"texto": texto})
		if err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlServidor+"/sintetizar", bytes.NewReader(corpo))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := cliente.Do(req)
		if err != nil {
			return fmt.Errorf("servidor de TTS: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("servidor de TTS devolveu HTTP %d", resp.StatusCode)
		}

		// Lê um byte a mais que o limite: só assim dá pra distinguir "cabe
		// exatamente" de "foi cortado". Cortar em silêncio gravaria um WAV com
		// cabeçalho inconsistente.
		audio, err := io.ReadAll(io.LimitReader(resp.Body, tamanhoMaximoDeAudio+1))
		if err != nil {
			return fmt.Errorf("lendo o áudio do servidor de TTS: %w", err)
		}
		if int64(len(audio)) > tamanhoMaximoDeAudio {
			return fmt.Errorf("servidor de TTS devolveu mais de %d bytes de áudio", tamanhoMaximoDeAudio)
		}
		if len(audio) == 0 {
			return errors.New("servidor de TTS devolveu áudio vazio")
		}
		return os.WriteFile(arquivoSaida, audio, 0o600)
	}
}
