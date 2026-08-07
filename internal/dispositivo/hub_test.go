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
		h.ResolverResultado(id, "clique feito", "", nil)
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
		h.ResolverResultado(id, "", "tecla inválida", nil)
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
	espera := make(chan resultadoAcao, 1)
	h.mu.Lock()
	h.pendentes[id] = espera
	h.mu.Unlock()

	feito := make(chan struct{})
	go func() {
		h.ResolverResultado(id, "primeiro", "", nil)
		h.ResolverResultado(id, "segundo", "", nil) // pendente já devia ter sido removido — tem que ser no-op
		close(feito)
	}()

	select {
	case <-feito:
	case <-time.After(2 * time.Second):
		t.Fatal("segunda chamada a ResolverResultado travou (deveria ser no-op não-bloqueante, já que o pendente já foi resolvido)")
	}

	select {
	case r := <-espera:
		if r.saida != "primeiro" {
			t.Errorf("valor entregue no canal = %+v, queria saida=primeiro (só a primeira resolução deve valer)", r)
		}
	default:
		t.Fatal("esperava que a primeira chamada a ResolverResultado tivesse mandado o resultado pro canal")
	}
}

func TestDesregistrarVoltaAErroDispositivoAusente(t *testing.T) {
	h := NovoHub()
	h.Registrar(&conexaoFalsa{})
	h.Desregistrar()

	_, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click"})
	if !errors.Is(err, ErroDispositivoAusente) {
		t.Fatalf("erro = %v, queria ErroDispositivoAusente após Desregistrar", err)
	}
}

func TestEnviarAcaoDevolveImagemDoResultado(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	imagem := []byte{0x89, 'P', 'N', 'G'}
	go func() {
		id := esperarEnvelopeID(t, conexao)
		h.ResolverResultado(id, "e1 Button \"Salvar\" (820,455)", "", imagem)
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
		h.ResolverResultado(id, "ok", "", nil)
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
