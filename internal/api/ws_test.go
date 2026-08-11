package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	"james/internal/dispositivo"
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

// providerChamaFerramentaUmaVez chama Nome na primeira iteração, e devolve
// uma resposta final na segunda (depois da ferramenta resolvida).
type providerChamaFerramentaUmaVez struct {
	Nome   string
	chamou bool
}

func (p *providerChamaFerramentaUmaVez) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	if !p.chamou {
		p.chamou = true
		return agente.Resposta{Chamadas: []agente.ChamadaFerramenta{{ID: "1", Nome: p.Nome}}}, nil
	}
	return agente.Resposta{Texto: "concluido"}, nil
}
func (p *providerChamaFerramentaUmaVez) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

// ferramentaFalsaComSaida devolve uma saída fixa sem efeito colateral —
// usada pelos testes que precisam de um ferramenta_concluida com Saida
// preenchida de verdade (Task 18), sem depender de nenhuma ferramenta real
// nem do device.
type ferramentaFalsaComSaida struct {
	nome  string
	saida string
}

func (f *ferramentaFalsaComSaida) Declaracao() ferramentas.Declaracao {
	return ferramentas.Declaracao{Nome: f.nome}
}
func (f *ferramentaFalsaComSaida) Nivel() ferramentas.Nivel { return ferramentas.NivelSeguro }
func (f *ferramentaFalsaComSaida) Executar(ctx context.Context, args map[string]any) (string, error) {
	return f.saida, nil
}

// servidorComFerramentaFalsa monta um *Servidor cujo provider chama `tool`
// (registrada sob o nome que ela mesma declara) na primeira iteração do
// loop, e devolve resposta final na segunda — usado pelos testes que
// precisam inspecionar o payload de ferramenta_concluida com Saida
// conhecida, nos dois caminhos que a Parte A da Task 18 unificou (texto e
// voz). Devolve o *Servidor, não o *httptest.Server: quem chama decide se
// precisa registrar DispositivoHub (caminho de voz) antes de expor as
// rotas.
func servidorComFerramentaFalsa(t *testing.T, tool ferramentas.Ferramenta, nomeFerramenta string) *Servidor {
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

	registro := ferramentas.NovoRegistro()
	if err := registro.Registrar(tool); err != nil {
		t.Fatalf("erro ao registrar ferramenta: %v", err)
	}

	return &Servidor{
		Sessoes:    postgres.NovoSessaoRepositorio(pool),
		Mensagens:  postgres.NovoMensagemRepositorio(pool),
		Aprovacoes: postgres.NovoAprovacaoRepositorio(pool),
		Loop: &agente.Loop{
			Provider:     &providerChamaFerramentaUmaVez{Nome: nomeFerramenta},
			Ferramentas:  registro,
			MaxIteracoes: 5,
		},
		Memoria:             memoria.Janela{OrcamentoTokens: 8000},
		SistemaPrompt:       "sistema de teste",
		Clientes:            NovoClientesHub(),
		AprovacoesPendentes: NovoRegistroAprovacoesPendentes(),
		TimeoutAprovacao:    time.Second,
	}
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
			Provider:     &providerChamaFerramentaUmaVez{Nome: "sempre_confirma"},
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

// capturarEventosDeUmTurno conecta no WS de teste, manda uma mensagem e
// devolve todos os envelopes recebidos até (e incluindo) resposta_final —
// usado por testes que precisam inspecionar a sequência inteira de eventos
// de um turno, não só o resultado final. Sobe o servidor WS fixo
// (servidorWSDeTeste): quem precisa de um provider ou registro de
// ferramentas customizado (ex.: TestFerramentaConcluidaCarregaSaida) monta o
// próprio *httptest.Server e chama capturarEventosDeUmTurnoNoServidor
// direto.
func capturarEventosDeUmTurno(t *testing.T, texto string) []Envelope {
	t.Helper()
	servidor := servidorWSDeTeste(t)
	defer servidor.Close()
	return capturarEventosDeUmTurnoNoServidor(t, servidor, texto)
}

func capturarEventosDeUmTurnoNoServidor(t *testing.T, servidor *httptest.Server, texto string) []Envelope {
	t.Helper()
	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(Envelope{Tipo: "mensagem", ID: "1", Dados: codificarDados(map[string]string{"texto": texto})}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var eventos []Envelope
	for i := 0; i < 10; i++ {
		var recebido Envelope
		if err := conn.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler do WS: %v", err)
		}
		eventos = append(eventos, recebido)
		if recebido.Tipo == "resposta_final" {
			break
		}
	}
	return eventos
}

// TestRetomarTrazACaudaDaConversaQuandoEstouraOLimite é o guarda que faltava.
//
// A auditoria da Fase 6 mostrou que trocar ListarRecentesPorSessao por
// ListarPorSessao em tratarRetomar deixava a suíte INTEIRA verde: o teste que
// existia (TestListarRecentesPorSessaoDivergeDeListarPorSessaoQuandoEstoraLimite,
// no pacote postgres) prova que os dois repositórios divergem, mas não olha
// para o ponto de chamada. E o plano da fase já tinha mandado a retomada usar
// o método do REST uma vez — é um erro que o projeto já cometeu.
//
// A divergência só aparece acima de limiteMensagensSessao: abaixo dele os
// dois métodos devolvem a conversa toda e qualquer um "funciona". Por isso
// este é também o primeiro exercício real do truncamento da retomada, que
// nenhuma sessão de verdade tinha alcançado.
//
// Limpa a sessão que criou: a suíte já enche o banco de trabalho sem limpar
// nada (1775 sessões na auditoria), e um teste que escreve 205 linhas por
// execução não pode piorar isso.
func TestRetomarTrazACaudaDaConversaQuandoEstouraOLimite(t *testing.T) {
	s := servidorComFerramentaFalsa(t, &ferramentaFalsaComSaida{nome: "nada"}, "nada")
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	ctx := context.Background()
	sessao, err := s.Sessoes.Criar(ctx, "conversa longa de teste")
	if err != nil {
		t.Fatalf("erro ao criar sessão: %v", err)
	}
	t.Cleanup(func() { apagarSessaoDeTeste(t, sessao.ID) })

	const excedente = 5
	total := limiteMensagensSessao + excedente
	for i := 0; i < total; i++ {
		papel := string(agente.PapelUsuario)
		if i%2 == 1 {
			papel = string(agente.PapelModelo)
		}
		if _, err := s.Mensagens.Salvar(ctx, sessao.ID, papel, fmt.Sprintf("msg-%d", i), nil); err != nil {
			t.Fatalf("erro ao salvar mensagem %d: %v", i, err)
		}
	}

	retomada := retomarSessaoNoServidor(t, servidor, sessao.ID.String())

	if len(retomada.Mensagens) != limiteMensagensSessao {
		t.Fatalf("retomada trouxe %d mensagens, queria o teto de %d", len(retomada.Mensagens), limiteMensagensSessao)
	}

	primeira := retomada.Mensagens[0].Texto
	ultima := retomada.Mensagens[len(retomada.Mensagens)-1].Texto
	if ultima != fmt.Sprintf("msg-%d", total-1) {
		t.Errorf("última mensagem retomada = %q, queria %q — a retomada não pegou o fim da conversa",
			ultima, fmt.Sprintf("msg-%d", total-1))
	}
	if primeira != fmt.Sprintf("msg-%d", excedente) {
		t.Errorf("primeira mensagem retomada = %q, queria %q — isto é o começo da conversa, não a cauda (ListarPorSessao no lugar de ListarRecentesPorSessao)",
			primeira, fmt.Sprintf("msg-%d", excedente))
	}
}

// retomarSessaoNoServidor manda "retomar" e devolve o payload de
// sessao_retomada decodificado.
func retomarSessaoNoServidor(t *testing.T, servidor *httptest.Server, sessaoID string) payloadSessaoRetomada {
	t.Helper()
	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(Envelope{Tipo: "retomar", ID: "r1", Dados: codificarDados(map[string]string{"sessao_id": sessaoID})}); err != nil {
		t.Fatalf("erro ao enviar retomar: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var recebido Envelope
	if err := conn.ReadJSON(&recebido); err != nil {
		t.Fatalf("erro ao ler do WS: %v", err)
	}
	if recebido.Tipo != "sessao_retomada" {
		t.Fatalf("esperava sessao_retomada, veio %q: %s", recebido.Tipo, recebido.Dados)
	}

	var payload payloadSessaoRetomada
	if err := json.Unmarshal(recebido.Dados, &payload); err != nil {
		t.Fatalf("payload de sessao_retomada ilegível: %v", err)
	}
	return payload
}

// apagarSessaoDeTeste remove a sessão criada pelo teste. As mensagens saem
// junto pela foreign key com ON DELETE CASCADE.
func apagarSessaoDeTeste(t *testing.T, id uuid.UUID) {
	t.Helper()
	pool, err := postgres.Abrir(context.Background(), postgres.URLDoAmbiente())
	if err != nil {
		t.Logf("não consegui abrir o banco para limpar a sessão %s: %v", id, err)
		return
	}
	defer pool.Close()
	if _, err := pool.Exec(context.Background(), "DELETE FROM sessoes WHERE id = $1", id); err != nil {
		t.Logf("não consegui apagar a sessão de teste %s: %v", id, err)
	}
}

// providerQueFalha simula qualquer falha de infraestrutura no meio do turno
// (Gemini fora, DNS, credencial) trazendo no texto do erro o tipo de detalhe
// que erros reais carregam: host, porta, usuário.
type providerQueFalha struct{}

func (p *providerQueFalha) Gerar(context.Context, agente.Requisicao) (agente.Resposta, error) {
	return agente.Resposta{}, errors.New("dial tcp 127.0.0.1:5433: user=james database=james recusou a conexão")
}
func (p *providerQueFalha) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	return p.Gerar(ctx, req)
}

// detalheDeInfraestrutura é o que não pode aparecer na tela. Esta lista é o
// contrato: qualquer um desses fragmentos vazando significa que o handler
// mandou err.Error() cru pro cliente.
var detalheDeInfraestrutura = []string{"127.0.0.1", "5433", "user=james", "dial tcp"}

// TestErroDeTurnoNaoVazaDetalheDeInfraestrutura trava um achado da auditoria
// da Fase 6, reproduzido ao vivo: derrubando o Postgres e mandando uma
// mensagem, a tela recebia
//
//	Erro: erro ao criar sessão: failed to connect to `user=james database=james`:
//	[::1]:5433 (localhost): dial error ...
//
// com usuário, banco, host e porta. O próprio ws.go já documenta por que isso
// não pode acontecer (o comentário em tratarRetomar diz que essa tela é a que
// o celular abre pela VPN) e tratarRetomar respeita a regra — loga completo e
// manda mensagem genérica. tratarMensagem e tratarEntradaDeVoz não respeitavam.
func TestErroDeTurnoNaoVazaDetalheDeInfraestrutura(t *testing.T) {
	casos := []struct {
		nome     string
		capturar func(*testing.T, *httptest.Server) []Envelope
		comVoz   bool
	}{
		{
			nome: "caminho de texto",
			capturar: func(t *testing.T, srv *httptest.Server) []Envelope {
				return capturarEventosAteErro(t, srv, false)
			},
		},
		{
			nome:   "caminho de voz",
			comVoz: true,
			capturar: func(t *testing.T, srv *httptest.Server) []Envelope {
				return capturarEventosAteErro(t, srv, true)
			},
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			s := servidorComFerramentaFalsa(t, &ferramentaFalsaComSaida{nome: "nada"}, "nada")
			s.Loop.Provider = &providerQueFalha{}
			if caso.comVoz {
				s.DispositivoHub = dispositivo.NovoHub()
			}
			servidor := httptest.NewServer(s.Rotas())
			defer servidor.Close()

			eventos := caso.capturar(t, servidor)

			dadosErro := dadosDoEvento(eventos, "erro")
			if dadosErro == nil {
				t.Fatalf("nenhum evento de erro capturado: %+v", eventos)
			}
			var payload payloadErro
			if err := json.Unmarshal(dadosErro, &payload); err != nil {
				t.Fatalf("payload de erro ilegível: %v", err)
			}
			if payload.Mensagem == "" {
				t.Fatal("mensagem de erro vazia: o usuário precisa saber que o turno falhou")
			}
			for _, fragmento := range detalheDeInfraestrutura {
				if strings.Contains(payload.Mensagem, fragmento) {
					t.Errorf("mensagem de erro vazou %q para o cliente: %q", fragmento, payload.Mensagem)
				}
			}
		})
	}
}

// capturarEventosAteErro manda um turno (por texto ou por voz) e lê até o
// evento "erro" chegar — os helpers existentes param em "resposta_final",
// que num turno que falha nunca vem.
func capturarEventosAteErro(t *testing.T, servidor *httptest.Server, porVoz bool) []Envelope {
	t.Helper()
	urlBase := "ws" + strings.TrimPrefix(servidor.URL, "http")

	conn, _, err := websocket.DefaultDialer.Dial(urlBase+"/ws", nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	if porVoz {
		connDevice, _, err := websocket.DefaultDialer.Dial(urlBase+"/ws/dispositivo", nil)
		if err != nil {
			t.Fatalf("erro ao conectar device: %v", err)
		}
		defer connDevice.Close()
		if err := connDevice.WriteJSON(Envelope{Tipo: "entrada_voz", ID: "v1", Dados: codificarDados(map[string]string{"texto": "oi"})}); err != nil {
			t.Fatalf("erro ao enviar entrada_voz: %v", err)
		}
	} else if err := conn.WriteJSON(Envelope{Tipo: "mensagem", ID: "1", Dados: codificarDados(map[string]string{"texto": "oi"})}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var eventos []Envelope
	for i := 0; i < 10; i++ {
		var recebido Envelope
		if err := conn.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler do WS: %v", err)
		}
		eventos = append(eventos, recebido)
		if recebido.Tipo == "erro" {
			break
		}
	}
	return eventos
}

// TestFerramentaConcluidaCarregaSaida trava a Task 18: a aba Tela da Fase 6
// depende da saída de ver_tela (superfície + lista de elementos), que até
// esta task só ia para o modelo — nunca para o cliente. Sem o campo
// "saida" em ferramenta_concluida, a aba fica permanentemente vazia. Usa
// payloadFerramentaConcluida (o mesmo tipo que a produção serializa em
// eventos.go) pra decodificar: um typo de chave no futuro vira erro de
// compilação aqui, não passa silencioso como passava com map literal.
func TestFerramentaConcluidaCarregaSaida(t *testing.T) {
	tool := &ferramentaFalsaComSaida{nome: "ver_tela", saida: "superficie: DISPLAY1\ne1 Button \"Salvar\" (820,455)"}
	s := servidorComFerramentaFalsa(t, tool, "ver_tela")
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	eventos := capturarEventosDeUmTurnoNoServidor(t, servidor, "olha a tela")

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
		t.Fatal("nenhum ferramenta_concluida no turno")
	}
}

// TestTurnoNaoEmiteToken trava a Task 5: sem streaming (decisão D4 da spec),
// "token" mandava resp.Texto inteiro um instante antes de resposta_final
// mandar o mesmo conteúdo de novo — evento duplicado prometendo streaming
// que não existe. Este teste existe pra "token" não voltar por engano
// quando alguém implementar streaming de verdade sem revisar o formato.
func TestTurnoNaoEmiteToken(t *testing.T) {
	eventos := capturarEventosDeUmTurno(t, "oi")
	for _, e := range eventos {
		if e.Tipo == "token" {
			t.Fatalf("evento token voltou: %+v", eventos)
		}
	}
}

// TestRespostaFinalCarregaSessaoIDDaSessaoCriada é a emenda da Task 5,
// Parte B: hoje sessaoID nasce dentro do servidor (processarTurno cria a
// sessão na primeira mensagem de uma conversa nova) e nunca chegava ao
// cliente — só descobria a sessão quem já tivesse pedido "retomar" alguma
// vez. Isso quebraria a aba Sessões (Task 15), que precisa destacar como
// "atual" a conversa que está rolando agora, não só as já retomadas. O
// teste confere não só que sessao_id vem preenchido, mas que é EXATAMENTE o
// id da sessão que processarTurno criou no Postgres — não um UUID qualquer.
func TestRespostaFinalCarregaSessaoIDDaSessaoCriada(t *testing.T) {
	s := servidorDeTeste(t)
	s.Loop = &agente.Loop{
		Provider:     &providerFixo{texto: "resposta de teste"},
		Ferramentas:  ferramentas.NovoRegistro(),
		MaxIteracoes: 5,
	}
	s.Memoria = memoria.Janela{OrcamentoTokens: 8000}
	s.SistemaPrompt = "sistema de teste"
	s.Clientes = NovoClientesHub()
	s.AprovacoesPendentes = NovoRegistroAprovacoesPendentes()
	s.TimeoutAprovacao = time.Second

	ctx := context.Background()
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	// Título único (inclui um UUID) pelo mesmo motivo de
	// TestWSRetomarSessaoInexistenteDevolveErro: os pacotes de teste rodam
	// em paralelo contra o mesmo banco, filtrar só por "sessão mais nova"
	// seria frágil.
	textoUnico := "teste sessao_id em resposta_final " + uuid.New().String()
	if err := conn.WriteJSON(Envelope{Tipo: "mensagem", ID: "1", Dados: codificarDados(map[string]string{"texto": textoUnico})}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var dadosRecebidos struct {
		Texto    string `json:"texto"`
		SessaoID string `json:"sessao_id"`
	}
	achouRespostaFinal := false
	for i := 0; i < 5; i++ {
		var recebido Envelope
		if err := conn.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler do WS: %v", err)
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
		t.Fatal("esperava receber um envelope resposta_final")
	}
	if dadosRecebidos.SessaoID == "" {
		t.Fatal("sessao_id veio vazio em resposta_final")
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
// de verdade: cria uma sessão com histórico no Postgres real (usuário,
// modelo com texto, ferramenta e modelo sem texto — só chamada), conecta,
// manda {tipo:"retomar"} com esse id e confirma que o evento
// "sessao_retomada" volta com o CONTEÚDO das mensagens exibíveis, na ordem
// certa — não mais a contagem (achado da verificação ao vivo da Fase 6: o
// Gemini lembrava do histórico, mas a tela ficava em branco). Os testes
// acima cobrem aplicarRetomada e mensagensExibiveis isoladas; este cobre o
// encanamento do protocolo — goroutine de leitura repassando "retomar" pro
// canal, laço sequencial despachando pro handler novo, e o handler batendo
// no repositório real.
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
		{Papel: agente.PapelModelo, Chamadas: []agente.ChamadaFerramenta{{ID: "c1", Nome: "ver_tela"}}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{{ID: "c1", Nome: "ver_tela", Saida: "superficie: DISPLAY1"}}},
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
		SessaoID  string                `json:"sessao_id"`
		Mensagens []mensagemDoHistorico `json:"mensagens"`
	}
	if err := json.Unmarshal(recebido.Dados, &dados); err != nil {
		t.Fatalf("erro ao decodificar dados de sessao_retomada: %v", err)
	}
	if dados.SessaoID != sessao.ID.String() {
		t.Errorf("sessao_id = %q, queria %q", dados.SessaoID, sessao.ID.String())
	}

	esperado := []mensagemDoHistorico{
		{Papel: "usuario", Texto: "lembra do bolo?"},
		{Papel: "modelo", Texto: "lembro, de cenoura"},
	}
	if len(dados.Mensagens) != len(esperado) {
		t.Fatalf("mensagens = %+v, queria %+v (ferramenta e modelo sem texto deviam ficar de fora)", dados.Mensagens, esperado)
	}
	for i := range esperado {
		if dados.Mensagens[i] != esperado[i] {
			t.Errorf("mensagem %d = %+v, queria %+v", i, dados.Mensagens[i], esperado[i])
		}
	}
}

// TestWSRetomarSessaoInexistenteDevolveErro cobre o achado da auto-revisão
// da Task 4: sem a checagem de existência, "retomar" com um sessao_id que
// não está em `sessoes` devolvia "sessao_retomada" com zero mensagem (sucesso
// enganoso) e deixava a conexão com sessaoID adotado — o turno seguinte
// tentava persistir sob esse id, estourava a foreign key
// mensagens_sessao_id_fkey, e persistirNovasMensagens só logava o erro: a
// conversa continuava rodando em memória sem gravar nada, em silêncio. Este
// teste prova três pontas: o evento vem "erro", não "sessao_retomada"; a
// mensagem é especificamente a de sessão inexistente, não a genérica de
// falha de banco (senão inverter a condição do errors.Is não quebraria
// nada); e a conexão segue limpa o bastante pra abrir e gravar uma sessão
// nova depois da recusa.
func TestWSRetomarSessaoInexistenteDevolveErro(t *testing.T) {
	s := servidorDeTeste(t)
	s.Loop = &agente.Loop{
		Provider:     &providerFixo{texto: "resposta de teste"},
		Ferramentas:  ferramentas.NovoRegistro(),
		MaxIteracoes: 5,
	}
	s.Memoria = memoria.Janela{OrcamentoTokens: 8000}
	s.SistemaPrompt = "sistema de teste"
	s.Clientes = NovoClientesHub()
	s.AprovacoesPendentes = NovoRegistroAprovacoesPendentes()
	s.TimeoutAprovacao = time.Second

	ctx := context.Background()

	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar no WS: %v", err)
	}
	defer conn.Close()

	inexistente := uuid.New()
	if err := conn.WriteJSON(Envelope{Tipo: "retomar", ID: "1", Dados: codificarDados(map[string]string{"sessao_id": inexistente.String()})}); err != nil {
		t.Fatalf("erro ao enviar retomar: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var recebido Envelope
	if err := conn.ReadJSON(&recebido); err != nil {
		t.Fatalf("erro ao ler do WS: %v", err)
	}
	if recebido.Tipo != "erro" {
		t.Fatalf("esperava erro, veio %q (dados=%s) — sessão inexistente não pode virar sessao_retomada", recebido.Tipo, recebido.Dados)
	}
	var dadosErro struct {
		Mensagem string `json:"mensagem"`
	}
	if err := json.Unmarshal(recebido.Dados, &dadosErro); err != nil {
		t.Fatalf("erro ao decodificar dados do erro: %v", err)
	}
	// Especificamente esta mensagem, não a genérica de falha de banco —
	// senão inverter a condição do errors.Is(err, pgx.ErrNoRows) em
	// tratarRetomar não quebraria teste nenhum.
	if dadosErro.Mensagem != "essa conversa não existe mais" {
		t.Errorf("mensagem = %q, queria %q", dadosErro.Mensagem, "essa conversa não existe mais")
	}

	// A conexão precisa continuar limpa: uma mensagem depois da recusa tem
	// que abrir sessão nova (sessaoID da retomada recusada não pode ter
	// ficado adotado) e gravar de verdade. Título improvável de colidir
	// (inclui um UUID) porque internal/armazenamento/postgres roda em
	// paralelo com internal/api sob `go test ./...`, gravando na mesma
	// tabela `sessoes` do mesmo banco — filtrar só por "sessão mais nova"
	// seria frágil.
	textoUnico := "teste retomada sessão inexistente " + uuid.New().String()
	if err := conn.WriteJSON(Envelope{Tipo: "mensagem", ID: "2", Dados: codificarDados(map[string]string{"texto": textoUnico})}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}
	achouRespostaFinal := false
	for i := 0; i < 5; i++ {
		var ev Envelope
		if err := conn.ReadJSON(&ev); err != nil {
			t.Fatalf("erro ao ler do WS: %v", err)
		}
		if ev.Tipo == "resposta_final" {
			achouRespostaFinal = true
			break
		}
	}
	if !achouRespostaFinal {
		t.Fatal("esperava receber um envelope resposta_final")
	}

	todas, err := s.Sessoes.Listar(ctx)
	if err != nil {
		t.Fatalf("erro ao listar sessões: %v", err)
	}
	var nova uuid.UUID
	for _, sess := range todas {
		if sess.Titulo == textoUnico {
			nova = sess.ID
			break
		}
	}
	if nova == uuid.Nil {
		t.Fatal("esperava achar a sessão nova pelo título único, não achei nenhuma")
	}
	if nova == inexistente {
		t.Fatal("a sessão nova não pode ser o id inexistente que a retomada recusou")
	}

	linhas, err := s.Mensagens.ListarPorSessao(ctx, nova, limiteMensagensSessao)
	if err != nil {
		t.Fatalf("erro ao listar mensagens da sessão nova: %v", err)
	}
	if len(linhas) == 0 {
		t.Error("a sessão nova devia ter gravado a mensagem enviada depois da recusa")
	}
}
