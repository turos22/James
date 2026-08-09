package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/dispositivo"
	"james/internal/ferramentas"
	"james/internal/memoria"
)

// servidorComDispositivoDeTeste devolve o *httptest.Server (pra dial WS e
// Close) e também o *Servidor por trás dele — testes que precisam conferir
// estado gravado no Postgres (ex.: TestWSDispositivoEntradaDeVozRespostaFinalCarregaSessaoID)
// usam o segundo pra chamar s.Sessoes.Listar diretamente; quem não precisa,
// descarta com `_`.
func servidorComDispositivoDeTeste(t *testing.T) (*httptest.Server, *Servidor) {
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
	return httptest.NewServer(s.Rotas()), s
}

func TestWSDispositivoResultadoAcaoResolveHub(t *testing.T) {
	servidor, _ := servidorComDispositivoDeTeste(t)
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
	if saida != "e1 Button \"Salvar\" (820,455)" {
		t.Errorf("saida = %q, queria o texto exato do fixture", saida)
	}
	if string(decodificada) != string(imagem) {
		t.Errorf("imagem = %v, queria %v", decodificada, imagem)
	}
}

func TestResultadoAcaoComJSONDeTopoQuebradoVideraErro(t *testing.T) {
	bruto := []byte(`{"saida": "texto", "erro": [1,2]}`) // erro não é string, unmarshal falha

	saida, erro, imagem := decodificarResultadoAcao(bruto)

	if saida != "" {
		t.Errorf("saida = %q, queria vazio (payload não confiável)", saida)
	}
	if erro == "" {
		t.Error("erro veio vazio — JSON de topo quebrado devia virar erro explícito, não sucesso silencioso")
	}
	if imagem != nil {
		t.Error("payload quebrado não devia produzir imagem")
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
	servidor, _ := servidorComDispositivoDeTeste(t)
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

// capturarEventosDeEntradaDeVoz conecta um cliente e um device no WS de
// teste, manda "entrada_voz" pelo device e devolve todos os envelopes que o
// cliente Privilegiado recebeu até (e incluindo) resposta_final — molde de
// capturarEventosDeUmTurno (ws_test.go), mas pro broadcast que sai do
// caminho de voz em vez da resposta direta a um cliente WS.
func capturarEventosDeEntradaDeVoz(t *testing.T, texto string) []Envelope {
	t.Helper()
	servidor, _ := servidorComDispositivoDeTeste(t)
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

	if err := connDevice.WriteJSON(Envelope{Tipo: "entrada_voz", ID: "v1", Dados: codificarDados(map[string]string{"texto": texto})}); err != nil {
		t.Fatalf("erro ao enviar entrada_voz: %v", err)
	}

	connCliente.SetReadDeadline(time.Now().Add(5 * time.Second))
	var eventos []Envelope
	for i := 0; i < 10; i++ {
		var recebido Envelope
		if err := connCliente.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler no cliente: %v", err)
		}
		eventos = append(eventos, recebido)
		if recebido.Tipo == "resposta_final" {
			break
		}
	}
	return eventos
}

// TestEntradaDeVozNaoEmiteToken estende pro caminho de voz o que
// TestTurnoNaoEmiteToken (ws_test.go) já trava pro caminho de texto: sem
// streaming (D8/D4 da spec), "token" duplicava resposta_final com o mesmo
// conteúdo. Ficar vivo só no broadcast de voz mantinha a mesma mentira num
// canto que ninguém olha — e o ClienteWS novo (Task 8) rejeita evento
// desconhecido com console.warn, então token sobrevivendo aqui vira ruído
// de console em todo turno falado.
func TestEntradaDeVozNaoEmiteToken(t *testing.T) {
	eventos := capturarEventosDeEntradaDeVoz(t, "oi James")
	for _, e := range eventos {
		if e.Tipo == "token" {
			t.Fatalf("evento token voltou no broadcast de voz: %+v", eventos)
		}
	}
}

// TestWSDispositivoEntradaDeVozRespostaFinalCarregaSessaoID confere que a
// emenda da Task 5 (Parte B) também vale pro caminho de voz: sessaoID já
// era parâmetro de tratarEntradaDeVoz (preenchido por processarTurno) antes
// desta mudança — não precisou de refatoração de assinatura pra ficar
// disponível aqui, então não há motivo pra esse caminho ficar sem o campo
// que o caminho do cliente WS ganhou.
//
// A asserção final compara contra s.Sessoes.Listar (Postgres real), não só
// "não vazio" — mesmo padrão de TestRespostaFinalCarregaSessaoIDDaSessaoCriada
// (ws_test.go). "Não vazio" passaria até com um UUID trocado ou o de outra
// sessão; a rede de segurança que essa task promete só existe comparando
// UUID com UUID contra o que foi de fato gravado.
func TestWSDispositivoEntradaDeVozRespostaFinalCarregaSessaoID(t *testing.T) {
	servidor, s := servidorComDispositivoDeTeste(t)
	defer servidor.Close()
	ctx := context.Background()

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

	// Título único (inclui um UUID), mesmo motivo de
	// TestWSRetomarSessaoInexistenteDevolveErro (ws_test.go): os pacotes de
	// teste rodam em paralelo contra o mesmo banco, filtrar só por "sessão
	// mais nova" seria frágil.
	textoUnico := "oi James, sessao_id por voz " + uuid.New().String()
	if err := connDevice.WriteJSON(Envelope{Tipo: "entrada_voz", ID: "v1", Dados: codificarDados(map[string]string{"texto": textoUnico})}); err != nil {
		t.Fatalf("erro ao enviar entrada_voz: %v", err)
	}

	connCliente.SetReadDeadline(time.Now().Add(5 * time.Second))
	var dadosRecebidos struct {
		Texto    string `json:"texto"`
		SessaoID string `json:"sessao_id"`
	}
	achouRespostaFinal := false
	for i := 0; i < 5; i++ {
		var recebido Envelope
		if err := connCliente.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler no cliente: %v", err)
		}
		if recebido.Tipo == "resposta_final" {
			if err := json.Unmarshal(recebido.Dados, &dadosRecebidos); err != nil {
				t.Fatalf("erro ao decodificar dados de resposta_final: %v", err)
			}
			achouRespostaFinal = true
			break
		}
	}
	if !achouRespostaFinal {
		t.Fatal("cliente privilegiado deveria ter recebido resposta_final originada da entrada_voz do device")
	}
	if dadosRecebidos.SessaoID == "" {
		t.Fatal("sessao_id veio vazio em resposta_final originada de entrada_voz")
	}

	todas, err := s.Sessoes.Listar(ctx)
	if err != nil {
		t.Fatalf("erro ao listar sessões: %v", err)
	}
	var criada uuid.UUID
	for _, sess := range todas {
		if sess.Titulo == textoUnico {
			criada = sess.ID
			break
		}
	}
	if criada == uuid.Nil {
		t.Fatal("não achei no banco a sessão criada pelo título único")
	}
	if dadosRecebidos.SessaoID != criada.String() {
		t.Errorf("sessao_id em resposta_final = %q, queria o id da sessão criada no banco: %q", dadosRecebidos.SessaoID, criada.String())
	}
}
