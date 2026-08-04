package dispositivo

import (
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
		var primeiro []byte
		for {
			var ok bool
			primeiro, ok = conexao.primeiroEnviado()
			if ok {
				break
			}
			time.Sleep(time.Millisecond)
		}
		var envelope map[string]any
		_ = json.Unmarshal(primeiro, &envelope)
		h.ResolverResultado(envelope["id"].(string), "clique feito", "")
	}()

	saida, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click", X: 10, Y: 20})
	if err != nil {
		t.Fatalf("EnviarAcao devolveu erro: %v", err)
	}
	if saida != "clique feito" {
		t.Errorf("saida = %q, queria clique feito", saida)
	}
}

func TestEnviarAcaoPropagaErroDoDevice(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	go func() {
		var primeiro []byte
		for {
			var ok bool
			primeiro, ok = conexao.primeiroEnviado()
			if ok {
				break
			}
			time.Sleep(time.Millisecond)
		}
		var envelope map[string]any
		_ = json.Unmarshal(primeiro, &envelope)
		h.ResolverResultado(envelope["id"].(string), "", "tecla inválida")
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
// id: o primeiro resolve EnviarAcao, o segundo tem que ser um no-op —
// jamais travar. No endpoint WS real (Task 18), ResolverResultado roda
// dentro da única goroutine que lê a conexão; travar aqui travaria a
// leitura da conexão inteira.
func TestResolverResultadoDuploMesmoIdNaoTrava(t *testing.T) {
	h := NovoHub()
	conexao := &conexaoFalsa{}
	h.Registrar(conexao)

	type resultado struct {
		saida string
		err   error
	}
	saidaCh := make(chan resultado, 1)
	go func() {
		saida, err := h.EnviarAcao(context.Background(), AcaoTela{Acao: "click"})
		saidaCh <- resultado{saida, err}
	}()

	var id string
	for {
		primeiro, ok := conexao.primeiroEnviado()
		if ok {
			var envelope map[string]any
			_ = json.Unmarshal(primeiro, &envelope)
			id = envelope["id"].(string)
			break
		}
		time.Sleep(time.Millisecond)
	}

	feito := make(chan struct{})
	go func() {
		h.ResolverResultado(id, "primeiro", "")
		h.ResolverResultado(id, "segundo", "") // frame duplicado — tem que ser no-op
		close(feito)
	}()

	select {
	case <-feito:
	case <-time.After(2 * time.Second):
		t.Fatal("ResolverResultado duplicado travou (deveria ser no-op não-bloqueante)")
	}

	select {
	case r := <-saidaCh:
		if r.err != nil {
			t.Fatalf("EnviarAcao devolveu erro: %v", r.err)
		}
		if r.saida != "primeiro" {
			t.Errorf("saida = %q, queria %q (só a primeira resolução deve valer)", r.saida, "primeiro")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("EnviarAcao não terminou a tempo")
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
