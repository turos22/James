package agente

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"james/internal/ferramentas"
	"james/internal/permissao"
)

// providerQueDistingue, ao contrário do providerFalso, sabe QUAL método o loop
// chamou, o que o caminho de streaming precisa provar.
type providerQueDistingue struct {
	mu          sync.Mutex
	respostas   []Resposta
	deltas      [][]string // o que GerarStream emite, por chamada
	usouGerar   int
	usouStream  int
	modos       []Modo
	temPrazoNoS []bool
}

func (p *providerQueDistingue) proxima(req Requisicao) Resposta {
	i := len(p.modos)
	p.modos = append(p.modos, req.Modo)
	if i >= len(p.respostas) {
		return Resposta{}
	}
	return p.respostas[i]
}

func (p *providerQueDistingue) Gerar(ctx context.Context, req Requisicao) (Resposta, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usouGerar++
	return p.proxima(req), nil
}

func (p *providerQueDistingue) GerarStream(ctx context.Context, req Requisicao, emitir func(string)) (Resposta, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usouStream++
	_, tem := ctx.Deadline()
	p.temPrazoNoS = append(p.temPrazoNoS, tem)
	i := len(p.modos)
	if i < len(p.deltas) {
		for _, d := range p.deltas[i] {
			emitir(d)
		}
	}
	return p.proxima(req), nil
}

func executar(t *testing.T, l *Loop) (Resposta, error) {
	t.Helper()
	resp, _, err := l.Executar(context.Background(), "sistema", nil,
		Mensagem{Papel: PapelUsuario, Texto: "oi"}, permissao.Privilegiado, nil)
	return resp, err
}

func TestLoopComAoDeltaUsaStreamEPassaOsDeltasDeTodasAsRodadas(t *testing.T) {
	eco := &ferramentaFalsa{nome: "eco", nivel: ferramentas.NivelSeguro, saida: "ok"}
	p := &providerQueDistingue{
		respostas: []Resposta{
			{Texto: "Vou ver. ", Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "eco"}}},
			{Texto: "Pronto."},
		},
		deltas: [][]string{{"Vou ", "ver. "}, {"Pronto."}},
	}
	var recebidos []string
	l := &Loop{Provider: p, Ferramentas: novoRegistroCom(eco), MaxIteracoes: 5,
		AoDelta: func(d string) { recebidos = append(recebidos, d) }}

	if _, err := executar(t, l); err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}

	// O texto falado ANTES da ferramenta ("Vou ver.") também tem que sair: é o
	// que o James diz enquanto a ferramenta roda.
	if want := []string{"Vou ", "ver. ", "Pronto."}; !reflect.DeepEqual(recebidos, want) {
		t.Errorf("deltas = %v, queria %v", recebidos, want)
	}
	if p.usouStream != 2 || p.usouGerar != 0 {
		t.Errorf("stream=%d gerar=%d, queria stream=2 gerar=0", p.usouStream, p.usouGerar)
	}
}

func TestLoopSemAoDeltaUsaGerar(t *testing.T) {
	p := &providerQueDistingue{respostas: []Resposta{{Texto: "oi"}}}
	l := &Loop{Provider: p, Ferramentas: ferramentas.NovoRegistro(), MaxIteracoes: 5}

	if _, err := executar(t, l); err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}

	if p.usouGerar != 1 || p.usouStream != 0 {
		t.Errorf("gerar=%d stream=%d, queria gerar=1 stream=0", p.usouGerar, p.usouStream)
	}
}

func TestLoopPropagaOModoAoProvider(t *testing.T) {
	for _, comStream := range []bool{false, true} {
		for _, modo := range []Modo{ModoTexto, ModoVoz} {
			p := &providerQueDistingue{respostas: []Resposta{{Texto: "oi"}}}
			l := &Loop{Provider: p, Ferramentas: ferramentas.NovoRegistro(), MaxIteracoes: 5, Modo: modo}
			if comStream {
				l.AoDelta = func(string) {}
			}

			if _, err := executar(t, l); err != nil {
				t.Fatalf("Executar devolveu erro: %v", err)
			}

			if len(p.modos) != 1 || p.modos[0] != modo {
				t.Errorf("stream=%v: o provider recebeu modo %v, queria %v", comStream, p.modos, modo)
			}
		}
	}
}

func TestLoopSomaOUsoDeTodasAsRodadas(t *testing.T) {
	eco := &ferramentaFalsa{nome: "eco", nivel: ferramentas.NivelSeguro, saida: "ok"}
	p := &providerQueDistingue{respostas: []Resposta{
		{Chamadas: []ChamadaFerramenta{{ID: "1", Nome: "eco"}}, Uso: Uso{Entrada: 100, Saida: 10}},
		{Texto: "feito", Uso: Uso{Entrada: 150, Saida: 20, CacheLeitura: 90}},
	}}
	l := &Loop{Provider: p, Ferramentas: novoRegistroCom(eco), MaxIteracoes: 5}

	resp, err := executar(t, l)
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}

	if want := (Uso{Entrada: 250, Saida: 30, CacheLeitura: 90}); resp.Uso != want {
		t.Errorf("Uso = %+v, queria o total do turno %+v", resp.Uso, want)
	}
}

// O teto por chamada vale também no streaming: sem ele, um stream pendurado
// seguraria o turno de voz pra sempre.
func TestTimeoutDoProviderTambemLimitaOStream(t *testing.T) {
	p := &providerQueDistingue{respostas: []Resposta{{Texto: "oi"}}}
	l := &Loop{Provider: p, Ferramentas: ferramentas.NovoRegistro(), MaxIteracoes: 5,
		TimeoutProvider: time.Minute, AoDelta: func(string) {}}

	if _, err := executar(t, l); err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}

	if len(p.temPrazoNoS) != 1 || !p.temPrazoNoS[0] {
		t.Errorf("o ctx do GerarStream não tinha prazo: %v", p.temPrazoNoS)
	}
}
