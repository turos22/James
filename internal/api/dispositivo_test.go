package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/dispositivo"
	"james/internal/ferramentas"
	"james/internal/memoria"
)

func servidorComDispositivoDeTeste(t *testing.T) *httptest.Server {
	t.Helper()
	url := postgres.URLDoAmbiente()
	if url == "" {
		t.Skip("DATABASE_URL não definida; suba o docker compose e preencha o .env")
	}
	pool, err := postgres.Abrir(context.Background(), url)
	if err != nil {
		t.Fatalf("não consegui conectar: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.AplicarMigracoes(context.Background(), pool); err != nil {
		t.Fatalf("erro ao aplicar migrações: %v", err)
	}

	s := &Servidor{
		Sessoes:    postgres.NovoSessaoRepositorio(pool),
		Mensagens:  postgres.NovoMensagemRepositorio(pool),
		Aprovacoes: postgres.NovoAprovacaoRepositorio(pool),
		Loop: &agente.Loop{
			Provider:     &providerFixo{texto: "resposta pra voz"},
			Ferramentas:  ferramentas.NovoRegistro(),
			MaxIteracoes: 5,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: 8000},
		SistemaPrompt:       "sistema de teste",
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
		TimeoutAprovacao:    time.Second,
		DispositivoHub:      dispositivo.NovoHub(),
	}
	return httptest.NewServer(s.Rotas())
}

func TestWSDispositivoResultadoAcaoResolveHub(t *testing.T) {
	servidor := servidorComDispositivoDeTeste(t)
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws/dispositivo"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS do device: %v", err)
	}
	defer conn.Close()

	go func() {
		var recebido Envelope
		if err := conn.ReadJSON(&recebido); err != nil {
			return
		}
		conn.WriteJSON(Envelope{
			Tipo: "resultado_acao", ID: recebido.ID,
			Dados: codificarDados(map[string]string{"saida": "clique feito"}),
		})
	}()

	time.Sleep(100 * time.Millisecond)
}

func TestResultadoAcaoDecodificaImagemBase64(t *testing.T) {
	imagem := []byte{0x89, 'P', 'N', 'G', 0x0D}
	dados := map[string]string{
		"saida":      "e1 Button \"Salvar\" (820,455)",
		"imagem_b64": base64.StdEncoding.EncodeToString(imagem),
	}
	bruto, err := json.Marshal(dados)
	if err != nil {
		t.Fatalf("erro ao montar dados: %v", err)
	}

	saida, erro, decodificada := decodificarResultadoAcao(bruto)

	if erro != "" {
		t.Errorf("erro = %q, queria vazio", erro)
	}
	if saida == "" {
		t.Error("saida veio vazia")
	}
	if string(decodificada) != string(imagem) {
		t.Errorf("imagem = %v, queria %v", decodificada, imagem)
	}
}

func TestResultadoAcaoComBase64InvalidaSegueSemImagem(t *testing.T) {
	bruto := []byte(`{"saida":"ok","imagem_b64":"não é base64!!!"}`)

	saida, erro, imagem := decodificarResultadoAcao(bruto)

	// Imagem quebrada não pode derrubar o turno: o texto ainda serve.
	if saida != "ok" {
		t.Errorf("saida = %q, queria ok", saida)
	}
	if erro != "" {
		t.Errorf("erro = %q, queria vazio", erro)
	}
	if imagem != nil {
		t.Error("base64 inválida virou imagem")
	}
}

func TestWSDispositivoEntradaDeVozGeraRespostaParaPrivilegiados(t *testing.T) {
	servidor := servidorComDispositivoDeTeste(t)
	defer servidor.Close()

	urlBase := "ws" + strings.TrimPrefix(servidor.URL, "http")

	connCliente, _, err := websocket.DefaultDialer.Dial(urlBase+"/ws", nil)
	if err != nil {
		t.Fatalf("erro ao conectar cliente: %v", err)
	}
	defer connCliente.Close()

	connDevice, _, err := websocket.DefaultDialer.Dial(urlBase+"/ws/dispositivo", nil)
	if err != nil {
		t.Fatalf("erro ao conectar device: %v", err)
	}
	defer connDevice.Close()

	if err := connDevice.WriteJSON(Envelope{Tipo: "entrada_voz", ID: "v1", Dados: codificarDados(map[string]string{"texto": "oi James"})}); err != nil {
		t.Fatalf("erro ao enviar entrada_voz: %v", err)
	}

	connCliente.SetReadDeadline(time.Now().Add(5 * time.Second))
	achouRespostaFinal := false
	for i := 0; i < 5; i++ {
		var recebido Envelope
		if err := connCliente.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler no cliente: %v", err)
		}
		if recebido.Tipo == "resposta_final" {
			achouRespostaFinal = true
			break
		}
	}
	if !achouRespostaFinal {
		t.Fatal("cliente privilegiado deveria ter recebido resposta_final originada da entrada_voz do device")
	}
}
