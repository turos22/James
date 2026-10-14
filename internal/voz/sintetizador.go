// Package voz gera áudio por resposta (Piper local ou edge-tts), um arquivo por resposta. O servidor
// nunca toca o áudio — só gera e entrega por URL; quem toca é o cliente
// (spec 4.7, decisão D1).
package voz

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unicode/utf8"
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

// DuracaoDoWAV lê a duração de um WAV PCM pelo cabeçalho (taxa de bytes e
// tamanho do chunk data). Serve para dizer ao device quanto tempo o app vai
// falar, e o James não escutar a si mesmo.
func DuracaoDoWAV(caminho string) (time.Duration, error) {
	arquivo, err := os.Open(caminho)
	if err != nil {
		return 0, err
	}
	defer arquivo.Close()

	cabecalho := make([]byte, 12)
	if _, err := io.ReadFull(arquivo, cabecalho); err != nil || string(cabecalho[0:4]) != "RIFF" || string(cabecalho[8:12]) != "WAVE" {
		return 0, errors.New("não é um WAV válido")
	}
	var taxaDeBytes uint32
	for {
		chunk := make([]byte, 8)
		if _, err := io.ReadFull(arquivo, chunk); err != nil {
			return 0, errors.New("WAV sem chunk data")
		}
		tamanho := binary.LittleEndian.Uint32(chunk[4:8])
		switch string(chunk[0:4]) {
		case "fmt ":
			fmtChunk := make([]byte, tamanho)
			if _, err := io.ReadFull(arquivo, fmtChunk); err != nil || len(fmtChunk) < 12 {
				return 0, errors.New("chunk fmt ilegível")
			}
			taxaDeBytes = binary.LittleEndian.Uint32(fmtChunk[8:12])
		case "data":
			if taxaDeBytes == 0 {
				return 0, errors.New("WAV com data antes de fmt")
			}
			return time.Duration(float64(tamanho) / float64(taxaDeBytes) * float64(time.Second)), nil
		default:
			if _, err := arquivo.Seek(int64(tamanho)+int64(tamanho%2), io.SeekCurrent); err != nil {
				return 0, err
			}
		}
	}
}

// EstimarDuracao é o plano B quando o cabeçalho não serve: a voz do Piper
// fala uns 12 caracteres por segundo.
func EstimarDuracao(texto string) time.Duration {
	return time.Duration(float64(utf8.RuneCountInString(texto)) / 12 * float64(time.Second))
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
