package ferramentas_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"james/internal/ferramentas"
	"james/internal/memoria"
)

// Spec 2026-10-14 arquivos, D11: a memória não guarda o que veio de arquivo
// secreto, nem credencial reconhecível.

func salvar(t *testing.T, s *ferramentas.SegredosLidos, texto string) (string, error, string) {
	t.Helper()
	raiz := filepath.Join(t.TempDir(), "Memoria")
	f := ferramentas.NovoSalvarNaMemoria(memoria.NovoVault(raiz, 2000))
	f.Segredos = s
	saida, err := f.Executar(context.Background(), map[string]any{"assunto": "projeto", "texto": texto})
	gravado, _ := os.ReadFile(filepath.Join(raiz, "projeto.md"))
	return saida, err, string(gravado)
}

func TestMemoriaRecusaValorQueVeioDeArquivoSecreto(t *testing.T) {
	segredos := ferramentas.NovoSegredosLidos()
	segredos.Registrar([]string{"abcdef123456"})

	_, err, gravado := salvar(t, segredos, "a chave do projeto é abcdef123456")

	if err == nil {
		t.Fatal("a nota com o valor secreto foi aceita")
	}
	if !strings.Contains(err.Error(), "valor secreto") {
		t.Errorf("mensagem = %q", err)
	}
	if strings.Contains(err.Error(), "abcdef123456") {
		t.Error("a mensagem de erro repete o valor secreto")
	}
	if gravado != "" {
		t.Errorf("algo foi gravado mesmo assim: %q", gravado)
	}
}

func TestMemoriaAceitaAConclusaoSemOValor(t *testing.T) {
	segredos := ferramentas.NovoSegredosLidos()
	segredos.Registrar([]string{"abcdef123456"})

	_, err, gravado := salvar(t, segredos, "o projeto usa uma chave de API")

	if err != nil {
		t.Fatalf("a conclusão sem o valor foi recusada: %v", err)
	}
	if !strings.Contains(gravado, "o projeto usa uma chave de API") {
		t.Errorf("gravado = %q", gravado)
	}
}

func TestMemoriaRecusaCredencialReconhecivelMesmoSemLeituraPrevia(t *testing.T) {
	chave := "sk-" + "ant-" + "api03-" + "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"

	_, err, gravado := salvar(t, ferramentas.NovoSegredosLidos(), "minha chave é "+chave)

	if err == nil || !strings.Contains(err.Error(), "credencial") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(err.Error(), chave) {
		t.Error("a mensagem de erro repete a credencial")
	}
	if gravado != "" {
		t.Errorf("gravou: %q", gravado)
	}
}

func TestMemoriaSemSegredosSeguePeloCaminhoDeSempre(t *testing.T) {
	_, err, gravado := salvar(t, nil, "o Arthur prefere Go a Python")

	if err != nil || !strings.Contains(gravado, "o Arthur prefere Go a Python") {
		t.Errorf("err = %v, gravado = %q", err, gravado)
	}
}

func TestMemoriaSemSegredosAindaRecusaCredencialReconhecivel(t *testing.T) {
	// A checagem da credencial não depende de haver um SegredosLidos.
	chave := "sk-" + "ant-" + "api03-" + "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789"

	if _, err, _ := salvar(t, nil, chave); err == nil {
		t.Error("credencial reconhecível aceita quando Segredos é nil")
	}
}
