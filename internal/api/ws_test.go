package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/ferramentas"
	"james/internal/memoria"
)

type providerFixo struct{ texto string }

func (p *providerFixo) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	return agente.Resposta{Texto: p.texto}, nil
}
func (p *providerFixo) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

func servidorWSDeTeste(t *testing.T) *httptest.Server {
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
			Provider:     &providerFixo{texto: "resposta de teste"},
			Ferramentas:  ferramentas.NovoRegistro(),
			MaxIteracoes: 5,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: 8000},
		SistemaPrompt:       "sistema de teste",
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
		TimeoutAprovacao:    time.Second,
	}
	return httptest.NewServer(s.Rotas())
}

// ferramentaSempreExigeConfirmacao reproduz o desenho de ClaudeCode.ConfirmacaoNecessaria
// (decisão D10): NivelSeguro, então qualquer perfil passa por Pode(), mas
// exige confirmação humana sempre, mesmo do Privilegiado.
type ferramentaSempreExigeConfirmacao struct {
	mu        sync.Mutex
	executada bool
}

func (f *ferramentaSempreExigeConfirmacao) Declaracao() ferramentas.Declaracao {
	return ferramentas.Declaracao{Nome: "sempre_confirma"}
}
func (f *ferramentaSempreExigeConfirmacao) Nivel() ferramentas.Nivel                  { return ferramentas.NivelSeguro }
func (f *ferramentaSempreExigeConfirmacao) ConfirmacaoNecessaria(map[string]any) bool { return true }
func (f *ferramentaSempreExigeConfirmacao) Executar(ctx context.Context, args map[string]any) (string, error) {
	f.mu.Lock()
	f.executada = true
	f.mu.Unlock()
	return "executou", nil
}
func (f *ferramentaSempreExigeConfirmacao) foiExecutada() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.executada
}

// providerChamaFerramentaUmaVez chama sempre_confirma na primeira iteração,
// e devolve uma resposta final na segunda (depois da ferramenta resolvida).
type providerChamaFerramentaUmaVez struct{ chamou bool }

func (p *providerChamaFerramentaUmaVez) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	if !p.chamou {
		p.chamou = true
		return agente.Resposta{Chamadas: []agente.ChamadaFerramenta{{ID: "1", Nome: "sempre_confirma"}}}, nil
	}
	return agente.Resposta{Texto: "concluido"}, nil
}
func (p *providerChamaFerramentaUmaVez) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

// TestWSMesmaConexaoConsegueAprovarSuaPropriaAcao trava um bug real,
// descoberto na verificação ao vivo da Fase 3 em 2026-08-06: perguntar_ao_claude_code
// passou a exigir confirmação SEMPRE, inclusive do Privilegiado (D10) — e o
// desktop usa uma única conexão WS pra mandar mensagem e pra aprovar. O laço
// de leitura de ws() processa "mensagem" de forma síncrona dentro do mesmo
// goroutine que devia ler o "aprovar" seguinte — a conexão fica travada
// esperando aprovar a si mesma, e só destrava (negando) depois do timeout de
// aprovação inteiro. Verificado ao vivo: exatos 5 minutos, batendo com o
// default de 300s.
func TestWSMesmaConexaoConsegueAprovarSuaPropriaAcao(t *testing.T) {
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

	registro := ferramentas.NovoRegistro()
	tool := &ferramentaSempreExigeConfirmacao{}
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("erro ao registrar ferramenta: %v", err)
	}

	s := &Servidor{
		Sessoes:    postgres.NovoSessaoRepositorio(pool),
		Mensagens:  postgres.NovoMensagemRepositorio(pool),
		Aprovacoes: postgres.NovoAprovacaoRepositorio(pool),
		Loop: &agente.Loop{
			Provider:     &providerChamaFerramentaUmaVez{},
			Ferramentas:  registro,
			MaxIteracoes: 5,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: 8000},
		SistemaPrompt:       "sistema de teste",
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
		// Curto de propósito: se o deadlock não estiver corrigido, o teste
		// nega rápido em vez de travar 300s — mas o cenário corrigido
		// resolve em milissegundos, bem abaixo disso.
		TimeoutAprovacao: 2 * time.Second,
	}
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	inicio := time.Now()
	if err := conn.WriteJSON(Envelope{Tipo: "mensagem", ID: "1", Dados: codificarDados(map[string]string{"texto": "faz algo"})}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	achouRespostaFinal := false
	for i := 0; i < 10; i++ {
		var recebido Envelope
		if err := conn.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler do WS: %v", err)
		}
		if recebido.Tipo == "aprovacao_requerida" {
			var dados map[string]any
			if err := json.Unmarshal(recebido.Dados, &dados); err != nil {
				t.Fatalf("erro ao decodificar dados de aprovacao_requerida: %v", err)
			}
			idAprovacao, _ := dados["id"].(string)
			if idAprovacao == "" {
				t.Fatal("dados.id veio vazio no evento aprovacao_requerida")
			}
			// A MESMA conexão que disparou a ação tenta aprová-la — é
			// exatamente o cenário do desktop usando perguntar_ao_claude_code.
			if err := conn.WriteJSON(Envelope{Tipo: "aprovar", ID: idAprovacao, Dados: codificarDados(map[string]string{})}); err != nil {
				t.Fatalf("erro ao enviar aprovar: %v", err)
			}
		}
		if recebido.Tipo == "resposta_final" {
			achouRespostaFinal = true
			break
		}
	}
	decorrido := time.Since(inicio)

	if !achouRespostaFinal {
		t.Fatal("esperava receber um envelope resposta_final")
	}
	// A asserção real é sobre a ferramenta ter executado, não sobre o texto
	// final: o provider de teste devolve "concluido" na segunda chamada
	// independente do resultado da ferramenta, então só o texto não prova
	// nada — se o deadlock estiver presente, o loop continua depois da
	// negação por timeout e chegaria ao mesmo texto sem a ferramenta rodar.
	if !tool.foiExecutada() {
		t.Error("a ferramenta nunca executou — a mesma conexão não conseguiu aprovar sua própria ação (deadlock)")
	}
	// Corrigido, isso resolve em milissegundos. Travado, resolve só depois do
	// TimeoutAprovacao inteiro (2s neste teste — 300s por padrão em produção,
	// confirmado ao vivo).
	if decorrido >= s.TimeoutAprovacao {
		t.Errorf("levou %s, >= TimeoutAprovacao (%s) — sinal de que caiu no timeout em vez de aprovar de verdade", decorrido, s.TimeoutAprovacao)
	}
}

func TestReconstruirHistoricoDesambiguaPeloPapel(t *testing.T) {
	chamadas, err := json.Marshal([]agente.ChamadaFerramenta{{ID: "c1", Nome: "ver_tela"}})
	if err != nil {
		t.Fatalf("erro ao preparar chamadas: %v", err)
	}
	resultados, err := json.Marshal([]agente.ResultadoFerramenta{{ID: "c1", Nome: "ver_tela", Saida: "superficie: DISPLAY1"}})
	if err != nil {
		t.Fatalf("erro ao preparar resultados: %v", err)
	}

	linhas := []postgres.Mensagem{
		{Papel: string(agente.PapelUsuario), Conteudo: "o que tem na tela?"},
		{Papel: string(agente.PapelModelo), Chamadas: chamadas},
		{Papel: string(agente.PapelFerramenta), Chamadas: resultados},
		{Papel: string(agente.PapelModelo), Conteudo: "tem a calculadora aberta"},
	}

	historico, err := reconstruirHistorico(linhas)
	if err != nil {
		t.Fatalf("erro ao reconstruir: %v", err)
	}
	if len(historico) != 4 {
		t.Fatalf("esperava 4 mensagens, veio %d", len(historico))
	}

	// A mensagem do modelo com a coluna preenchida tem que virar Chamadas,
	// não Resultados — e vice-versa. Trocar os dois produz histórico que o
	// Gemini aceita com semântica errada, que é o modo de falha silencioso.
	if len(historico[1].Chamadas) != 1 || historico[1].Chamadas[0].Nome != "ver_tela" {
		t.Errorf("papel modelo devia virar Chamadas: %+v", historico[1])
	}
	if len(historico[1].Resultados) != 0 {
		t.Errorf("papel modelo não pode produzir Resultados: %+v", historico[1])
	}
	if len(historico[2].Resultados) != 1 || historico[2].Resultados[0].Saida != "superficie: DISPLAY1" {
		t.Errorf("papel ferramenta devia virar Resultados: %+v", historico[2])
	}
	if len(historico[2].Chamadas) != 0 {
		t.Errorf("papel ferramenta não pode produzir Chamadas: %+v", historico[2])
	}
	if historico[0].Texto != "o que tem na tela?" || historico[0].Papel != agente.PapelUsuario {
		t.Errorf("mensagem de usuário veio errada: %+v", historico[0])
	}
}

func TestReconstruirHistoricoRejeitaColunaCorrompida(t *testing.T) {
	linhas := []postgres.Mensagem{
		{Papel: string(agente.PapelModelo), Chamadas: []byte("{isso não é json")},
	}

	if _, err := reconstruirHistorico(linhas); err == nil {
		t.Fatal("esperava erro, veio nil — coluna ilegível não pode virar histórico mudo")
	}
}

func TestReconstruirHistoricoIgnoraColunaVazia(t *testing.T) {
	linhas := []postgres.Mensagem{
		{Papel: string(agente.PapelModelo), Conteudo: "oi", Chamadas: nil},
		{Papel: string(agente.PapelModelo), Conteudo: "oi de novo", Chamadas: []byte{}},
	}

	historico, err := reconstruirHistorico(linhas)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	for i, m := range historico {
		if len(m.Chamadas) != 0 || len(m.Resultados) != 0 {
			t.Errorf("linha %d não devia ter chamadas nem resultados: %+v", i, m)
		}
	}
}

// TestReconstruirHistoricoRejeitaPapelInesperadoComColunaPreenchida cobre o
// outro eixo de corrupção: JSON válido, mas numa linha cujo papel não é
// nem PapelModelo nem PapelFerramenta. Migração de schema, bug futuro em
// persistirNovasMensagens ou edição manual do banco podem produzir esse
// estado — reconstruirHistorico não pode ignorá-lo em silêncio, porque
// silêncio aqui vira contexto errado indo pro Gemini.
func TestReconstruirHistoricoRejeitaPapelInesperadoComColunaPreenchida(t *testing.T) {
	chamadas, err := json.Marshal([]agente.ChamadaFerramenta{{ID: "c1", Nome: "ver_tela"}})
	if err != nil {
		t.Fatalf("erro ao preparar chamadas: %v", err)
	}

	linhas := []postgres.Mensagem{
		{ID: 42, Papel: string(agente.PapelUsuario), Chamadas: chamadas},
	}

	if _, err := reconstruirHistorico(linhas); err == nil {
		t.Fatal("esperava erro, veio nil — papel usuário não pode ter a coluna chamadas preenchida")
	}
}

// TestReconstruirHistoricoEhEspelhoDePersistirNovasMensagens prova que
// persistirNovasMensagens e reconstruirHistorico são de fato inversas uma da
// outra: grava um histórico com os três papéis num Postgres real, lê de
// volta pela mesma via que uma sessão retomada usaria (ListarPorSessao), e
// compara campo a campo com o original. Os testes acima montam
// postgres.Mensagem na mão — este é o que pegaria as duas funções
// divergindo com o tempo, já que vivem no mesmo arquivo.
func TestReconstruirHistoricoEhEspelhoDePersistirNovasMensagens(t *testing.T) {
	s := servidorDeTeste(t)
	ctx := context.Background()

	sessao, err := s.Sessoes.Criar(ctx, "sessão de ida e volta")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	original := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "o que tem na tela?"},
		{Papel: agente.PapelModelo, Chamadas: []agente.ChamadaFerramenta{{ID: "c1", Nome: "ver_tela"}}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{{ID: "c1", Nome: "ver_tela", Saida: "superficie: DISPLAY1"}}},
		{Papel: agente.PapelModelo, Texto: "tem a calculadora aberta"},
	}

	s.persistirNovasMensagens(ctx, sessao.ID, original, 0)

	linhas, err := s.Mensagens.ListarPorSessao(ctx, sessao.ID, 200)
	if err != nil {
		t.Fatalf("erro ao listar mensagens: %v", err)
	}
	if len(linhas) != len(original) {
		t.Fatalf("esperava %d linhas gravadas, veio %d", len(original), len(linhas))
	}

	reconstruido, err := reconstruirHistorico(linhas)
	if err != nil {
		t.Fatalf("erro ao reconstruir: %v", err)
	}
	if len(reconstruido) != len(original) {
		t.Fatalf("esperava %d mensagens reconstruídas, veio %d", len(original), len(reconstruido))
	}

	for i := range original {
		quero, tenho := original[i], reconstruido[i]
		// Imagem nunca é persistida (ver comentário de reconstruirHistorico)
		// — comparar esse campo faria o teste falhar por design, não por
		// bug. Já vem nil em ambos os lados aqui, mas zeramos explícito pra
		// deixar a intenção clara caso o teste ganhe um caso com imagem.
		quero.Imagem = nil
		tenho.Imagem = nil
		if !reflect.DeepEqual(quero, tenho) {
			t.Errorf("mensagem %d divergiu na ida e volta:\nquero: %+v\ntenho: %+v", i, quero, tenho)
		}
	}
}

func TestWSMensagemDevolveRespostaFinal(t *testing.T) {
	servidor := servidorWSDeTeste(t)
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(Envelope{Tipo: "mensagem", ID: "1", Dados: codificarDados(map[string]string{"texto": "oi"})}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	achouRespostaFinal := false
	for i := 0; i < 5; i++ {
		var recebido Envelope
		if err := conn.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler do WS: %v", err)
		}
		if recebido.Tipo == "resposta_final" {
			achouRespostaFinal = true
			break
		}
	}
	if !achouRespostaFinal {
		t.Fatal("esperava receber um envelope resposta_final")
	}
}

func TestRetomarCarregaSessaoEHistorico(t *testing.T) {
	sessao := uuid.New()
	linhas := []postgres.Mensagem{
		{Papel: string(agente.PapelUsuario), Conteudo: "lembra do bolo?"},
		{Papel: string(agente.PapelModelo), Conteudo: "lembro, de cenoura"},
	}

	var sessaoID uuid.UUID
	var historico []agente.Mensagem

	if err := aplicarRetomada(sessao, linhas, &sessaoID, &historico); err != nil {
		t.Fatalf("erro ao retomar: %v", err)
	}
	if sessaoID != sessao {
		t.Errorf("sessaoID não foi adotada: %v", sessaoID)
	}
	if len(historico) != 2 || historico[1].Texto != "lembro, de cenoura" {
		t.Errorf("histórico veio errado: %+v", historico)
	}
}

func TestRetomadaComColunaCorrompidaNaoSujaOEstado(t *testing.T) {
	// Se a reconstrução falha, a conexão precisa continuar utilizável como
	// sessão nova — não pode ficar com sessaoID adotada e histórico pela
	// metade, que é o estado que produziria contexto silenciosamente errado.
	linhas := []postgres.Mensagem{
		{Papel: string(agente.PapelModelo), Chamadas: []byte("{quebrado")},
	}

	var sessaoID uuid.UUID
	var historico []agente.Mensagem

	if err := aplicarRetomada(uuid.New(), linhas, &sessaoID, &historico); err == nil {
		t.Fatal("esperava erro")
	}
	if sessaoID != uuid.Nil {
		t.Errorf("sessaoID não podia ter sido adotada: %v", sessaoID)
	}
	if len(historico) != 0 {
		t.Errorf("histórico devia continuar vazio: %+v", historico)
	}
}

// TestWSRetomarDevolveSessaoRetomada é o teste de integração pelo WebSocket
// de verdade: cria uma sessão com histórico no Postgres real, conecta,
// manda {tipo:"retomar"} com esse id e confirma que o evento
// "sessao_retomada" volta com a contagem certa de mensagens. Os testes
// acima cobrem aplicarRetomada isolada; este cobre o encanamento do
// protocolo — goroutine de leitura repassando "retomar" pro canal, laço
// sequencial despachando pro handler novo, e o handler batendo no
// repositório real.
func TestWSRetomarDevolveSessaoRetomada(t *testing.T) {
	s := servidorDeTeste(t)
	ctx := context.Background()

	sessao, err := s.Sessoes.Criar(ctx, "sessão pra retomar")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}

	original := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "lembra do bolo?"},
		{Papel: agente.PapelModelo, Texto: "lembro, de cenoura"},
	}
	s.persistirNovasMensagens(ctx, sessao.ID, original, 0)

	s.Clientes = NovoClientesHub()
	s.AprovacoesPendentes = NovoRegistroAprovacoesPendentes()
	s.TimeoutAprovacao = time.Second
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(Envelope{Tipo: "retomar", ID: "1", Dados: codificarDados(map[string]string{"sessao_id": sessao.ID.String()})}); err != nil {
		t.Fatalf("erro ao enviar retomar: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var recebido Envelope
	if err := conn.ReadJSON(&recebido); err != nil {
		t.Fatalf("erro ao ler do WS: %v", err)
	}
	if recebido.Tipo != "sessao_retomada" {
		t.Fatalf("esperava sessao_retomada, veio %q (dados=%s)", recebido.Tipo, recebido.Dados)
	}

	var dados struct {
		SessaoID  string `json:"sessao_id"`
		Mensagens int    `json:"mensagens"`
	}
	if err := json.Unmarshal(recebido.Dados, &dados); err != nil {
		t.Fatalf("erro ao decodificar dados de sessao_retomada: %v", err)
	}
	if dados.SessaoID != sessao.ID.String() {
		t.Errorf("sessao_id = %q, queria %q", dados.SessaoID, sessao.ID.String())
	}
	if dados.Mensagens != len(original) {
		t.Errorf("mensagens = %d, queria %d", dados.Mensagens, len(original))
	}
}
