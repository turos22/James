package voz

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestGerarChamaExecutorEDevolveArquivoUnicoPorID(t *testing.T) {
	var vozRecebida, arquivoRecebido string
	s := &Sintetizador{
		Executor: func(ctx context.Context, texto, voz, arquivoSaida string) error {
			vozRecebida = voz
			arquivoRecebido = arquivoSaida
			return nil
		},
		DiretorioSaida: t.TempDir(),
		Timeout:        time.Second,
	}

	caminho, err := s.Gerar(context.Background(), "olá Arthur", "pt-BR-AntonioNeural", "resposta-123")
	if err != nil {
		t.Fatalf("Gerar devolveu erro: %v", err)
	}
	if vozRecebida != "pt-BR-AntonioNeural" {
		t.Errorf("voz repassada = %q", vozRecebida)
	}
	if filepath.Base(caminho) != "resposta-123.mp3" {
		t.Errorf("caminho = %q, queria terminar em resposta-123.mp3 (nome único por resposta, corrige bug #7)", caminho)
	}
	if arquivoRecebido != caminho {
		t.Errorf("arquivo passado ao Executor = %q, deveria bater com o devolvido = %q", arquivoRecebido, caminho)
	}
}

func TestGerarPropagaErroDoExecutor(t *testing.T) {
	s := &Sintetizador{
		Executor:       func(ctx context.Context, texto, voz, arquivoSaida string) error { return errors.New("edge-tts não encontrado") },
		DiretorioSaida: t.TempDir(),
		Timeout:        time.Second,
	}

	_, err := s.Gerar(context.Background(), "teste", "pt-BR-AntonioNeural", "id-1")
	if err == nil {
		t.Fatal("esperava erro propagado do Executor (corrige bug #7: erro nunca ignorado), veio nil")
	}
}

func TestGerarRespeitaTimeout(t *testing.T) {
	s := &Sintetizador{
		Executor: func(ctx context.Context, texto, voz, arquivoSaida string) error {
			select {
			case <-time.After(200 * time.Millisecond):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		DiretorioSaida: t.TempDir(),
		Timeout:        20 * time.Millisecond,
	}

	_, err := s.Gerar(context.Background(), "teste", "pt-BR-AntonioNeural", "id-2")
	if err == nil {
		t.Fatal("esperava erro de timeout (R4), veio nil")
	}
}

func TestGerarIDsDiferentesGeramArquivosDiferentes(t *testing.T) {
	s := &Sintetizador{
		Executor:       func(ctx context.Context, texto, voz, arquivoSaida string) error { return nil },
		DiretorioSaida: t.TempDir(),
		Timeout:        time.Second,
	}

	a, _ := s.Gerar(context.Background(), "um", "pt-BR-AntonioNeural", "id-a")
	b, _ := s.Gerar(context.Background(), "dois", "pt-BR-AntonioNeural", "id-b")
	if a == b {
		t.Error("respostas diferentes não podem colidir no mesmo arquivo (corrige bug #7: arquivo de saída fixo)")
	}
}
