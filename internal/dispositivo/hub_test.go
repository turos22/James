package dispositivo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

// conexaoFalsa é lida pelo goroutine de teste (via primeiroEnviado) e escrita
// pelo Hub (via Enviar) concorrentemente — o mutex só protege esse acesso
// simultâneo à mesma struct; a espera continua sendo por polling, como o
// endpoint WS real faria ao aguardar a mensagem chegar pela rede.
type conexaoFalsa struct {
	mu       sync.Mutex
	enviados [][]byte
	erro     error
}

func (c *conexaoFalsa) Enviar(envelope []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enviados = append(c.enviados, envelope)
	return c.erro
}

// limpar esquece o que foi enviado, para o próximo esperarEnvelopeID pegar
// o envelope da próxima ação.
func (c *conexaoFalsa) limpar() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.enviados = nil
}

// primeiroEnviado devolve o primeiro envelope enviado e true, ou nil e false
// se ainda não chegou nada — usado pelo polling que simula o device.
func (c *conexaoFalsa) primeiroEnviado() ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.enviados) == 0 {
		return nil, false
	}
	return c.enviados[0], true
}

// esperarEnvelopeID espera o primeiro envelope enviado pela conexao falsa e
// devolve o id dele. Poll em vez de sleep fixo — sleep fixo é exatamente o
// que faz um teste flakar sob máquina carregada, e some silenciosamente em
// vez de falhar alto (quem chama trava no Timeout inteiro).
func esperarEnvelopeID(t *testing.T, conexao *conexaoFalsa) string {
	t.Helper()
	for i := 0; i < 1000; i++ {
		envelope, ok := conexao.primeiroEnviado()
		if ok {
			var recebido struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(envelope, &recebido); err != nil {
				t.Fatalf("envelope não decodifica: %v", err)
			}
			return recebido.ID
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("nenhum envelope chegou depois de 1s de polling")
	return ""
}

func TestEnviarAcaoSemDispositivoDevolveErroDedicado(t *testing.T) {
	h := NovoHub()
	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click", X: 10, Y: 20})
	if !errors.Is(err, ErroDispositivoAusente) {
		t.Fatalf("erro = %v, queria ErroDispositivoAusente", err)
	}
}

func TestEnviarAcaoComDispositivoConectadoDevolveResultado(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	go func() {
		// Simula o endpoint WS do device recebendo o envelope e respondendo.
		id := esperarEnvelopeID(t, conexao)
		h.ResolverResultado(id, Resposta{Saida: "clique feito"})
	}()

	resultado, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click", X: 10, Y: 20})
	if err != nil {
		t.Fatalf("EnviarAcao devolveu erro: %v", err)
	}
	if resultado.Saida != "clique feito" {
		t.Errorf("saida = %q, queria clique feito", resultado.Saida)
	}
}

func TestEnviarAcaoPropagaErroDoDevice(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	go func() {
		id := esperarEnvelopeID(t, conexao)
		h.ResolverResultado(id, Resposta{Erro: "tecla inválida"})
	}()

	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "tecla", Texto: "xyz"})
	if err == nil {
		t.Fatal("esperava erro propagado do device, veio nil")
	}
}

func TestEnviarAcaoEstouraTimeoutSemResposta(t *testing.T) {
	h := NovoHub()
	h.Timeout = 20 * time.Millisecond
	h.Registrar(&conexaoFalsa{})

	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click"})
	if err == nil {
		t.Fatal("esperava timeout (R4), veio nil")
	}
}

// TestResolverResultadoDuploMesmoIdNaoTrava cobre o cenário de um device com
// bug (ou frame duplicado na rede) mandando dois resultado_acao pro mesmo
// id: o primeiro resolve, o segundo tem que ser um no-op — jamais travar.
// No endpoint WS real (Task 18), ResolverResultado roda dentro da única
// goroutine que lê a conexão; travar aqui travaria a leitura inteira.
//
// Deliberadamente não usa EnviarAcao/goroutine concorrente pra montar o
// canal pendente — depender de duas goroutines correndo pra forçar a janela
// de corrida é não-determinístico (o agendador do Go costuma drenar o
// primeiro valor rápido demais pro segundo send nunca chegar a bloquear,
// mesmo contra a versão com bug). Em vez disso, o teste é a própria
// package `dispositivo`, então monta o estado "pendente" direto no mapa
// interno — reproduz o cenário de forma determinística: ninguém nunca vai
// ler do canal, então uma segunda chamada que tente mandar pra ele sem
// checar se já foi resolvido trava pra sempre, sem depender de sorte de
// escalonamento.
func TestResolverResultadoDuploMesmoIdNaoTrava(t *testing.T) {
	h := NovoHub()
	const id = "id-de-teste"
	espera := make(chan Resposta, 1)
	h.mu.Lock()
	h.pendentes[id] = espera
	h.mu.Unlock()

	feito := make(chan struct{})
	go func() {
		h.ResolverResultado(id, Resposta{Saida: "primeiro"})
		h.ResolverResultado(id, Resposta{Saida: "segundo"}) // pendente já devia ter sido removido — tem que ser no-op
		close(feito)
	}()

	select {
	case <-feito:
	case <-time.After(2 * time.Second):
		t.Fatal("segunda chamada a ResolverResultado travou (deveria ser no-op não-bloqueante, já que o pendente já foi resolvido)")
	}

	select {
	case r := <-espera:
		if r.Saida != "primeiro" {
			t.Errorf("valor entregue no canal = %+v, queria saida=primeiro (só a primeira resolução deve valer)", r)
		}
	default:
		t.Fatal("esperava que a primeira chamada a ResolverResultado tivesse mandado o resultado pro canal")
	}
}

func TestDesregistrarVoltaAErroDispositivoAusente(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)
	h.Desregistrar(conexao)

	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click"})
	if !errors.Is(err, ErroDispositivoAusente) {
		t.Fatalf("erro = %v, queria ErroDispositivoAusente após Desregistrar", err)
	}
}

// TestDesregistrarConexaoVelhaNaoDerrubaANova é a versão unitária do bug que
// TestReconexaoDoDeviceNaoDesregistraAConexaoNova (internal/api) exercita pelo
// WS real: quem chega depois manda, e quem já foi substituído não tem mais
// nada pra desregistrar.
func TestDesregistrarConexaoVelhaNaoDerrubaANova(t *testing.T) {
	h := NovoHub()
	velha := &conexaoFalsa{}
	nova := &conexaoFalsa{}
	h.Registrar(velha)
	h.Registrar(nova)

	h.Desregistrar(velha)

	// Contexto curto: ninguém vai resolver este id, então o desfecho esperado
	// é o prazo estourar. O que o teste afirma é que NÃO foi
	// ErroDispositivoAusente — ou seja, o Hub ainda tem device.
	ctx, cancelar := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelar()

	_, err := h.EnviarAcao(ctx, AcaoTela{Acao: "click"})
	if errors.Is(err, ErroDispositivoAusente) {
		t.Fatal("o Hub esqueceu a conexão nova quando a velha se desregistrou")
	}
	if _, ok := nova.primeiroEnviado(); !ok {
		t.Fatal("a ação não chegou na conexão nova")
	}
	if _, ok := velha.primeiroEnviado(); ok {
		t.Fatal("a ação foi parar na conexão velha")
	}
}

func TestEnviarAcaoDevolveImagemDoResultado(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	imagem := []byte{0x89, 'P', 'N', 'G'}
	go func() {
		id := esperarEnvelopeID(t, conexao)
		h.ResolverResultado(id, Resposta{Saida: "e1 Button \"Salvar\" (820,455)", Imagem: imagem})
	}()

	resultado, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "ver_tela"})
	if err != nil {
		t.Fatalf("EnviarAcao devolveu erro: %v", err)
	}
	if resultado.Saida == "" {
		t.Error("Saida veio vazia")
	}
	if !bytes.Equal(resultado.Imagem, imagem) {
		t.Errorf("Imagem = %v, queria %v", resultado.Imagem, imagem)
	}
}

func TestAcaoTelaSerializaCamposDeElementoESuperficie(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	go func() {
		id := esperarEnvelopeID(t, conexao)
		h.ResolverResultado(id, Resposta{Saida: "ok"})
	}()

	_, err := h.EnviarAcao(context.Background(), AcaoTela{
		Acao: "clicar_elemento", Superficie: "DISPLAY7", ElementoID: "e7", Nome: "Salvar", Tipo: "Button",
	})
	if err != nil {
		t.Fatalf("EnviarAcao devolveu erro: %v", err)
	}

	envelope, _ := conexao.primeiroEnviado()
	var enviado struct {
		Dados struct {
			Acao       string `json:"acao"`
			Superficie string `json:"superficie"`
			ElementoID string `json:"elemento_id"`
			Nome       string `json:"nome"`
			Tipo       string `json:"tipo"`
		} `json:"dados"`
	}
	if err := json.Unmarshal(envelope, &enviado); err != nil {
		t.Fatalf("envelope não decodifica: %v", err)
	}
	if enviado.Dados.Superficie != "DISPLAY7" || enviado.Dados.ElementoID != "e7" {
		t.Errorf("dados chegaram incompletos ao device: %+v", enviado.Dados)
	}
}

func TestTimeoutPorAcaoSobrepoeOGlobal(t *testing.T) {
	h := NovoHub()
	h.Timeout = 10 * time.Millisecond
	h.TimeoutPorAcao = map[string]time.Duration{"ver_tela": 300 * time.Millisecond}
	h.Registrar(&conexaoFalsa{})

	inicio := time.Now()
	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "ver_tela"})
	decorrido := time.Since(inicio)

	if err == nil {
		t.Fatal("esperava timeout, veio nil")
	}
	if decorrido < 250*time.Millisecond {
		t.Errorf("desistiu em %v — usou o Timeout global em vez do da ação", decorrido)
	}
}

// --- B2: protocolo aditivo e último mapa (spec 2026-10-13, 4.1) -------------

func TestAcaoTelaSemCamposNovosSerializaIgualAntes(t *testing.T) {
	bruto, err := json.Marshal(AcaoTela{Acao: "click", X: 1, Y: 2})
	if err != nil {
		t.Fatal(err)
	}
	if string(bruto) != `{"acao":"click","x":1,"y":2}` {
		t.Errorf("JSON mudou para ações antigas: %s", bruto)
	}
	bruto, _ = json.Marshal(AcaoTela{Acao: "abrir", Alvo: "main.go", Linha: 12})
	if string(bruto) != `{"acao":"abrir","alvo":"main.go","linha":12}` {
		t.Errorf("JSON com alvo e linha: %s", bruto)
	}
}

func verTela(t *testing.T, h *Hub, conexao *conexaoFalsa, resposta Resposta) Resultado {
	t.Helper()
	conexao.limpar()
	go func() {
		id := esperarEnvelopeID(t, conexao)
		h.ResolverResultado(id, resposta)
	}()
	r, _ := h.EnviarAcao(context.Background(), AcaoTela{Acao: "ver_tela"})
	return r
}

func TestResultadoTrazOsElementosDoVerTela(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)
	elementos := []Elemento{{ID: "e1", Nome: "Excluir", Tipo: "Button"}}

	r := verTela(t, h, conexao, Resposta{Saida: "lista", Elementos: elementos})

	if len(r.Elementos) != 1 || r.Elementos[0] != elementos[0] {
		t.Errorf("Elementos = %+v", r.Elementos)
	}
}

func TestVerTelaGuardaOUltimoMapaETrocaACadaVarredura(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	if len(h.UltimoMapa()) != 0 {
		t.Fatal("Hub novo já tem mapa")
	}
	verTela(t, h, conexao, Resposta{Saida: "a", Elementos: []Elemento{{ID: "e1", Nome: "Novo", Tipo: "Button"}}})
	verTela(t, h, conexao, Resposta{Saida: "b", Elementos: []Elemento{{ID: "e1", Nome: "Excluir", Tipo: "Button"}}})

	mapa := h.UltimoMapa()
	if len(mapa) != 1 || mapa[0].Nome != "Excluir" {
		t.Errorf("UltimoMapa = %+v, queria o da segunda varredura", mapa)
	}
	mapa[0].Nome = "mexido por fora"
	if h.UltimoMapa()[0].Nome != "Excluir" {
		t.Error("UltimoMapa devolveu a fatia interna: quem chama consegue alterar o mapa do Hub")
	}
}

func TestVerTelaComErroZeraOMapa(t *testing.T) {
	// O device pode ter varrido de novo (ids renumerados) mesmo quando o
	// resultado não chegou: manter o mapa velho faria "e7" apontar para o
	// elemento errado na hora de pedir aprovação.
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)
	verTela(t, h, conexao, Resposta{Saida: "a", Elementos: []Elemento{{ID: "e1", Nome: "Novo"}}})

	verTela(t, h, conexao, Resposta{Erro: "sem permissão"})

	if mapa := h.UltimoMapa(); len(mapa) != 0 {
		t.Errorf("UltimoMapa = %+v, queria vazio depois de um ver_tela com erro", mapa)
	}
}

func TestVerTelaComTimeoutZeraOMapa(t *testing.T) {
	h := NovoHub()
	h.Timeout = 20 * time.Millisecond
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)
	verTela(t, h, conexao, Resposta{Saida: "a", Elementos: []Elemento{{ID: "e1", Nome: "Novo"}}})

	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "ver_tela"}) // ninguém responde
	if err == nil {
		t.Fatal("esperava timeout")
	}
	if len(h.UltimoMapa()) != 0 {
		t.Error("o mapa velho sobreviveu ao timeout")
	}
}

func TestOutraAcaoNaoTrocaOMapa(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)
	verTela(t, h, conexao, Resposta{Saida: "a", Elementos: []Elemento{{ID: "e1", Nome: "Novo"}}})

	conexao.limpar()
	go func() {
		id := esperarEnvelopeID(t, conexao)
		h.ResolverResultado(id, Resposta{Saida: "cliquei", Elementos: []Elemento{{ID: "x", Nome: "outro"}}})
	}()
	_, _ = h.EnviarAcao(context.Background(), AcaoTela{Acao: "clicar_elemento", ElementoID: "e1"})

	if mapa := h.UltimoMapa(); len(mapa) != 1 || mapa[0].Nome != "Novo" {
		t.Errorf("UltimoMapa = %+v", mapa)
	}
}

func TestDeviceNovoNaoHerdaOMapaDoVelho(t *testing.T) {
	h := NovoHub()
	velha := &conexaoFalsa{}
	h.Registrar(velha)
	verTela(t, h, velha, Resposta{Saida: "a", Elementos: []Elemento{{ID: "e1", Nome: "Excluir"}}})

	h.Registrar(&conexaoFalsa{})
	if len(h.UltimoMapa()) != 0 {
		t.Error("a conexão nova herdou o mapa da velha")
	}

	h2 := NovoHub()
	unica := &conexaoFalsa{}
	h2.Registrar(unica)
	verTela(t, h2, unica, Resposta{Saida: "a", Elementos: []Elemento{{ID: "e1", Nome: "Excluir"}}})
	h2.Desregistrar(unica)
	if len(h2.UltimoMapa()) != 0 {
		t.Error("o mapa sobreviveu à saída do device")
	}
}
