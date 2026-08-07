package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestAoVivoMemoriaAtravessaConexao é o CA3 da spec, e só roda com o jamesd
// real no ar: JAMES_AO_VIVO=1 go test ./internal/api/ -run TestAoVivo -v
//
// Prova o que nenhum teste de unidade prova: que a memória sobrevive ao fim
// da conexão. Antes desta fase ele falha por construção — historico é por
// conexão e começa vazio (ws.go:37).
func TestAoVivoMemoriaAtravessaConexao(t *testing.T) {
	if os.Getenv("JAMES_AO_VIVO") == "" {
		t.Skip("defina JAMES_AO_VIVO=1 e suba o jamesd para rodar a verificação ao vivo")
	}

	primeira := perguntarEmConexaoNova(t, "Meu carro é um Fusca 1971 azul. Guarde isso na sua memória.")
	t.Logf("conexão 1 respondeu: %s", primeira)

	segunda := perguntarEmConexaoNova(t, "Que carro eu tenho?")
	t.Logf("conexão 2 respondeu: %s", segunda)

	if !contemQualquer(segunda, "Fusca", "fusca") {
		t.Fatalf("o James não lembrou do carro na conexão nova; respondeu: %s", segunda)
	}
}

// perguntarEmConexaoNova abre uma conexão, manda uma mensagem, espera a
// resposta final e FECHA. Cada chamada é uma conexão nova de propósito: é o
// fechamento que garante sessão nova e histórico vazio do outro lado.
func perguntarEmConexaoNova(t *testing.T, texto string) string {
	t.Helper()

	conn, _, err := websocket.DefaultDialer.Dial("ws://127.0.0.1:8080/ws", nil)
	if err != nil {
		t.Fatalf("não consegui conectar no jamesd: %v (ele está no ar?)", err)
	}
	defer conn.Close()

	envio := Envelope{Tipo: "mensagem", ID: "1", Dados: codificarDados(map[string]string{"texto": texto})}
	if err := conn.WriteJSON(envio); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Minute))
	for {
		var resposta Envelope
		if err := conn.ReadJSON(&resposta); err != nil {
			t.Fatalf("erro ao ler resposta: %v", err)
		}
		if resposta.Tipo == "ferramenta_iniciada" {
			t.Logf("ferramenta chamada: %s", resposta.Dados)
		}
		if resposta.Tipo == "erro" {
			t.Fatalf("o jamesd devolveu erro: %s", resposta.Dados)
		}
		if resposta.Tipo == "resposta_final" {
			var dados struct {
				Texto string `json:"texto"`
			}
			if err := json.Unmarshal(resposta.Dados, &dados); err != nil {
				t.Fatalf("resposta_final em formato inesperado: %v", err)
			}
			return dados.Texto
		}
	}
}

func contemQualquer(texto string, agulhas ...string) bool {
	for _, a := range agulhas {
		if strings.Contains(texto, a) {
			return true
		}
	}
	return false
}
