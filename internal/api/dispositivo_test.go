package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
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

	r := decodificarResultadoAcao(bruto)
	saida, erro, decodificada := r.Saida, r.Erro, r.Imagem

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

	r := decodificarResultadoAcao(bruto)
	saida, erro, imagem := r.Saida, r.Erro, r.Imagem

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

	r := decodificarResultadoAcao(bruto)
	saida, erro, imagem := r.Saida, r.Erro, r.Imagem

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
// caminho de voz em vez da resposta direta a um cliente WS. Sobe o servidor
// de device fixo (servidorComDispositivoDeTeste); quem precisa de provider
// ou registro de ferramentas customizado chama
// capturarEventosDeEntradaDeVozNoServidor direto.
func capturarEventosDeEntradaDeVoz(t *testing.T, texto string) []Envelope {
	t.Helper()
	servidor, _ := servidorComDispositivoDeTeste(t)
	defer servidor.Close()
	return capturarEventosDeEntradaDeVozNoServidor(t, servidor, texto)
}

func capturarEventosDeEntradaDeVozNoServidor(t *testing.T, servidor *httptest.Server, texto string) []Envelope {
	t.Helper()
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

// TestEntradaDeVozAnunciaTranscricaoAntesDeProcessar trava o achado da
// auditoria da Fase 6: um turno falado chegava na tela só com a resposta do
// James, sem nenhum balão do que foi dito. O caminho de texto monta o balão
// do usuário localmente (envio_local, no próprio front), mas a voz não passa
// por cliente WS nenhum — quem fala é o device, e o texto transcrito morria
// dentro do servidor. A resposta aparecia do nada.
//
// A transcrição precisa ser emitida ANTES de processarTurno, não junto com o
// desfecho: o turno pode levar minutos (ferramenta, aprovação humana), e a
// tela tem que mostrar o que foi entendido no ato — inclusive porque é assim
// que se percebe que o Vosk ouviu errado.
func TestEntradaDeVozAnunciaTranscricaoAntesDeProcessar(t *testing.T) {
	eventos := capturarEventosDeEntradaDeVoz(t, "que horas são")

	if len(eventos) == 0 {
		t.Fatal("nenhum evento capturado")
	}
	if eventos[0].Tipo != "transcricao" {
		tipos := make([]string, 0, len(eventos))
		for _, e := range eventos {
			tipos = append(tipos, e.Tipo)
		}
		t.Fatalf("primeiro evento devia ser transcricao, veio %q (sequência: %v)", eventos[0].Tipo, tipos)
	}

	var dados payloadTranscricao
	if err := json.Unmarshal(eventos[0].Dados, &dados); err != nil {
		t.Fatalf("dados de transcricao ilegíveis: %v", err)
	}
	if dados.Texto != "que horas são" {
		t.Errorf("transcricao veio com texto %q, queria %q", dados.Texto, "que horas são")
	}
}

// TestFerramentaConcluidaCarregaSaidaViaVoz estende pro caminho de voz o que
// TestFerramentaConcluidaCarregaSaida (ws_test.go) já trava pro caminho de
// texto: a Parte A da Task 18 unificou a montagem do payload de
// ferramenta_concluida entre os dois caminhos, então a Saida acrescentada
// na Parte B precisa valer aqui também — sem duplicar a fixture, só o
// registro de DispositivoHub, que o caminho de voz exige e o de texto não.
func TestFerramentaConcluidaCarregaSaidaViaVoz(t *testing.T) {
	tool := &ferramentaFalsaComSaida{nome: "ver_tela", saida: "superficie: DISPLAY1\ne1 Button \"Salvar\" (820,455)"}
	s := servidorComFerramentaFalsa(t, tool, "ver_tela")
	s.DispositivoHub = dispositivo.NovoHub()
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	eventos := capturarEventosDeEntradaDeVozNoServidor(t, servidor, "olha a tela")

	var achou bool
	for _, e := range eventos {
		if e.Tipo != "ferramenta_concluida" {
			continue
		}
		var dados payloadFerramentaConcluida
		if err := json.Unmarshal(e.Dados, &dados); err != nil {
			t.Fatalf("dados ilegíveis: %v", err)
		}
		if dados.Saida == "" {
			t.Errorf("ferramenta_concluida sem saida: %+v", dados)
		}
		achou = true
	}
	if !achou {
		t.Fatal("nenhum ferramenta_concluida no turno de voz")
	}
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

	todas, err := s.Sessoes.ListarPaginado(ctx, postgres.FiltroSessoes{Busca: textoUnico})
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

// providerQueAnotaOModo guarda o Modo de cada requisição que chega ao modelo.
type providerQueAnotaOModo struct {
	mu    sync.Mutex
	modos []agente.Modo
}

func (p *providerQueAnotaOModo) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	p.mu.Lock()
	p.modos = append(p.modos, req.Modo)
	p.mu.Unlock()
	return agente.Resposta{Texto: "ok"}, nil
}

func (p *providerQueAnotaOModo) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

func (p *providerQueAnotaOModo) ultimoModo(t *testing.T) agente.Modo {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.modos) == 0 {
		t.Fatal("o provider não recebeu nenhuma requisição")
	}
	return p.modos[len(p.modos)-1]
}

// Voz desliga o pensamento (rápido), texto pensa (correto). Quem decide é o
// canal por onde o turno entrou, não o conteúdo.
func TestTurnoDeVozPassaModoVozETurnoDeTextoPassaModoTexto(t *testing.T) {
	servidor, s := servidorComDispositivoDeTeste(t)
	defer servidor.Close()
	anota := &providerQueAnotaOModo{}
	s.Loop.Provider = anota

	capturarEventosDeEntradaDeVozNoServidor(t, servidor, "que horas são")
	if got := anota.ultimoModo(t); got != agente.ModoVoz {
		t.Errorf("turno de voz: modo = %v, queria ModoVoz", got)
	}

	capturarEventosDeUmTurnoNoServidor(t, servidor, "que horas são")
	if got := anota.ultimoModo(t); got != agente.ModoTexto {
		t.Errorf("turno de texto: modo = %v, queria ModoTexto", got)
	}
}

func TestResultadoAcaoLeOsElementosDoVerTela(t *testing.T) {
	r := decodificarResultadoAcao([]byte(`{"saida":"lista","elementos":[{"id":"e1","nome":"Excluir","tipo":"Button"}]}`))

	if len(r.Elementos) != 1 || r.Elementos[0] != (dispositivo.Elemento{ID: "e1", Nome: "Excluir", Tipo: "Button"}) {
		t.Errorf("Elementos = %+v", r.Elementos)
	}
}

func TestResultadoAcaoSemElementosDaListaVazia(t *testing.T) {
	if r := decodificarResultadoAcao([]byte(`{"saida":"cliquei"}`)); len(r.Elementos) != 0 || r.Saida != "cliquei" {
		t.Errorf("resposta = %+v", r)
	}
}

func TestResultadoAcaoComElementosQuebradosSegueSemElementos(t *testing.T) {
	// Mesma tolerância da imagem: a lista quebrada não derruba o texto.
	r := decodificarResultadoAcao([]byte(`{"saida":"lista","elementos":"não é lista"}`))

	if r.Saida != "lista" || r.Erro != "" || len(r.Elementos) != 0 {
		t.Errorf("resposta = %+v", r)
	}
}
