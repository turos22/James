package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/ferramentas"
	"james/internal/voz"
)

// providerDeFala roteiriza o que o modelo "diz": a cada chamada emite os
// deltas da rodada e devolve a resposta dela. O canal opcional segura a rodada
// até o teste liberar, o que prova que a frase chegou ANTES do fim do turno.
type providerDeFala struct {
	rodadas []rodadaDeFala
	erro    error
	mu      sync.Mutex
	chamada int
}

type rodadaDeFala struct {
	deltas   []string
	resposta agente.Resposta
	segura   <-chan struct{} // se não-nil, espera isto depois do primeiro delta
}

func (p *providerDeFala) Gerar(ctx context.Context, req agente.Requisicao) (agente.Resposta, error) {
	return p.GerarStream(ctx, req, nil)
}

func (p *providerDeFala) GerarStream(ctx context.Context, req agente.Requisicao, emitir func(string)) (agente.Resposta, error) {
	if p.erro != nil {
		return agente.Resposta{}, p.erro
	}
	p.mu.Lock()
	i := p.chamada
	p.chamada++
	p.mu.Unlock()
	if i >= len(p.rodadas) {
		return agente.Resposta{}, errors.New("sem mais rodadas")
	}
	r := p.rodadas[i]
	for n, d := range r.deltas {
		if emitir != nil {
			emitir(d)
		}
		if n == 0 && r.segura != nil {
			select {
			case <-r.segura:
			case <-ctx.Done():
			}
		}
	}
	return r.resposta, nil
}

// ferramentaLenta bloqueia até o teste liberar: é o "tempo de ferramenta"
// durante o qual a voz não pode ficar calada.
type ferramentaLenta struct{ liberar <-chan struct{} }

func (f *ferramentaLenta) Declaracao() ferramentas.Declaracao { return ferramentas.Declaracao{Nome: "lenta"} }
func (f *ferramentaLenta) Nivel() ferramentas.Nivel          { return ferramentas.NivelSeguro }
func (f *ferramentaLenta) Executar(ctx context.Context, _ map[string]any) (string, error) {
	select {
	case <-f.liberar:
	case <-ctx.Done():
	}
	return "feito", nil
}

type falaRecebida struct {
	Tipo     string
	Texto    string
	Indice   int
	Mensagem string
}

func conectarDevice(t *testing.T, servidor *httptest.Server) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws/dispositivo"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("erro ao conectar o device: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func falarPeloDevice(t *testing.T, conn *websocket.Conn, texto string) {
	t.Helper()
	err := conn.WriteJSON(Envelope{Tipo: "entrada_voz", ID: "v1", Dados: codificarDados(map[string]string{"texto": texto})})
	if err != nil {
		t.Fatalf("erro ao enviar entrada_voz: %v", err)
	}
}

func lerFala(t *testing.T, conn *websocket.Conn) falaRecebida {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var env Envelope
	if err := conn.ReadJSON(&env); err != nil {
		t.Fatalf("erro ao ler do device: %v", err)
	}
	f := falaRecebida{Tipo: env.Tipo}
	var dados struct {
		Texto    string `json:"texto"`
		Indice   int    `json:"indice"`
		Mensagem string `json:"mensagem"`
	}
	_ = json.Unmarshal(env.Dados, &dados)
	f.Texto, f.Indice, f.Mensagem = dados.Texto, dados.Indice, dados.Mensagem
	return f
}

// lerFalaAteOFim lê até fala_fim ou fala_erro, e devolve tudo em ordem.
func lerFalaAteOFim(t *testing.T, conn *websocket.Conn) []falaRecebida {
	t.Helper()
	var todas []falaRecebida
	for i := 0; i < 20; i++ {
		f := lerFala(t, conn)
		todas = append(todas, f)
		if f.Tipo == "fala_fim" || f.Tipo == "fala_erro" {
			return todas
		}
	}
	t.Fatalf("fala_fim nunca chegou: %+v", todas)
	return nil
}

func servidorDeFala(t *testing.T, p *providerDeFala) (*httptest.Server, *Servidor) {
	t.Helper()
	servidor, s := servidorComDispositivoDeTeste(t)
	t.Cleanup(servidor.Close)
	s.Loop.Provider = p
	return servidor, s
}

func TestTurnoDeVozMandaFrasesLimpasEmOrdemEFim(t *testing.T) {
	servidor, _ := servidorDeFala(t, &providerDeFala{rodadas: []rodadaDeFala{{
		deltas:   []string{"**Canberra** é a capital. ", "Tudo certo? ", "Sim."},
		resposta: agente.Resposta{Texto: "**Canberra** é a capital. Tudo certo? Sim."},
	}}})
	conn := conectarDevice(t, servidor)

	falarPeloDevice(t, conn, "qual a capital")
	got := lerFalaAteOFim(t, conn)

	esperado := []falaRecebida{
		{Tipo: "fala_frase", Texto: "Canberra é a capital.", Indice: 0},
		{Tipo: "fala_frase", Texto: "Tudo certo?", Indice: 1},
		{Tipo: "fala_frase", Texto: "Sim.", Indice: 2},
		{Tipo: "fala_fim"},
	}
	if len(got) != len(esperado) {
		t.Fatalf("recebido %+v, queria %+v", got, esperado)
	}
	for i := range esperado {
		if got[i] != esperado[i] {
			t.Errorf("evento %d = %+v, queria %+v", i, got[i], esperado[i])
		}
	}
}

// O ponto da spec inteira: o primeiro som sai na primeira frase, não no fim
// da resposta. O provedor fica parado depois do primeiro delta; se o jamesd
// esperasse o turno acabar, o teste travaria aqui.
func TestPrimeiraFraseChegaAntesDoFimDoTurno(t *testing.T) {
	liberar := make(chan struct{})
	servidor, _ := servidorDeFala(t, &providerDeFala{rodadas: []rodadaDeFala{{
		deltas:   []string{"Primeira frase. Segunda", " frase."},
		resposta: agente.Resposta{Texto: "Primeira frase. Segunda frase."},
		segura:   liberar,
	}}})
	conn := conectarDevice(t, servidor)

	falarPeloDevice(t, conn, "oi")
	primeira := lerFala(t, conn) // o provedor ainda está seguro aqui

	if primeira.Tipo != "fala_frase" || primeira.Texto != "Primeira frase." {
		t.Fatalf("primeiro evento = %+v, queria a primeira frase", primeira)
	}
	close(liberar)
	resto := lerFalaAteOFim(t, conn)
	if len(resto) != 2 || resto[0].Texto != "Segunda frase." || resto[1].Tipo != "fala_fim" {
		t.Errorf("resto = %+v", resto)
	}
}

// "Vou olhar a tela." termina com ponto no fim do buffer, e o quebrador espera
// o próximo caractere. Com a ferramenta rodando esse caractere só viria depois
// dela, e o James ficaria calado justo quando devia dizer o que está fazendo.
func TestFraseAntesDaFerramentaSaiNaHoraSemEsperarAFerramenta(t *testing.T) {
	liberar := make(chan struct{})
	servidor, s := servidorDeFala(t, &providerDeFala{rodadas: []rodadaDeFala{
		{deltas: []string{"Vou ver."}, resposta: agente.Resposta{
			Texto: "Vou ver.", Chamadas: []agente.ChamadaFerramenta{{ID: "1", Nome: "lenta"}}}},
		{deltas: []string{"Pronto."}, resposta: agente.Resposta{Texto: "Pronto."}},
	}})
	reg := ferramentas.NovoRegistro()
	_ = reg.Registrar(&ferramentaLenta{liberar: liberar})
	s.Loop.Ferramentas = reg
	conn := conectarDevice(t, servidor)

	falarPeloDevice(t, conn, "olha a tela")
	antes := lerFala(t, conn) // a ferramenta ainda está presa

	if antes.Tipo != "fala_frase" || antes.Texto != "Vou ver." || antes.Indice != 0 {
		t.Fatalf("evento antes da ferramenta = %+v, queria a frase 'Vou ver.'", antes)
	}
	close(liberar)
	resto := lerFalaAteOFim(t, conn)
	if len(resto) != 2 || resto[0].Texto != "Pronto." || resto[0].Indice != 1 || resto[1].Tipo != "fala_fim" {
		t.Errorf("depois da ferramenta = %+v", resto)
	}
}

func TestErroDoTurnoDeVozChegaAoDeviceComMensagemSegura(t *testing.T) {
	servidor, _ := servidorDeFala(t, &providerDeFala{erro: errors.New("postgres://james:senha-secreta@127.0.0.1:5434 recusou")})
	conn := conectarDevice(t, servidor)

	falarPeloDevice(t, conn, "oi")
	got := lerFalaAteOFim(t, conn)

	ultimo := got[len(got)-1]
	if ultimo.Tipo != "fala_erro" {
		t.Fatalf("último evento = %+v, queria fala_erro", ultimo)
	}
	if ultimo.Mensagem != "não consegui completar esse turno" {
		t.Errorf("mensagem = %q", ultimo.Mensagem)
	}
	if strings.Contains(ultimo.Mensagem, "senha") || strings.Contains(ultimo.Mensagem, "5434") {
		t.Errorf("a mensagem vazou detalhe interno: %q", ultimo.Mensagem)
	}
}

// leitorDoApp lê o /ws numa goroutine e entrega por canal. Não dá pra usar
// prazo de leitura na conexão pra detectar silêncio: no gorilla/websocket uma
// leitura que estoura o prazo deixa a conexão quebrada pra sempre.
func leitorDoApp(conn *websocket.Conn) <-chan Envelope {
	canal := make(chan Envelope, 64)
	go func() {
		defer close(canal)
		for {
			var e Envelope
			if err := conn.ReadJSON(&e); err != nil {
				return
			}
			canal <- e
		}
	}()
	return canal
}

// coletarEventosDoApp junta o que o app recebe até resposta_final e mais um
// instante. Os helpers que já existem param em resposta_final, que sai ANTES
// do audio_pronto, e por isso nunca enxergariam o áudio.
func coletarEventosDoApp(t *testing.T, eventos <-chan Envelope) []Envelope {
	t.Helper()
	var todos []Envelope
	limite := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-eventos:
			if !ok {
				return todos
			}
			todos = append(todos, e)
			if e.Tipo == "resposta_final" {
				// Um instante a mais, pra pegar o que vem depois.
				quieto := time.After(500 * time.Millisecond)
				for {
					select {
					case e2, ok2 := <-eventos:
						if !ok2 {
							return todos
						}
						todos = append(todos, e2)
					case <-quieto:
						return todos
					}
				}
			}
		case <-limite:
			t.Fatalf("resposta_final não chegou ao app: %+v", todos)
			return todos
		}
	}
}

func temEvento(eventos []Envelope, tipo string) bool {
	for _, e := range eventos {
		if e.Tipo == tipo {
			return true
		}
	}
	return false
}

// Voz fala pelo device; o app não toca de novo. E o texto digitado continua
// tocando pelo app, uma vez.
func TestVozNaoGeraAudioProntoETextoGera(t *testing.T) {
	servidor, s := servidorDeFala(t, &providerDeFala{rodadas: []rodadaDeFala{
		{deltas: []string{"Resposta da voz."}, resposta: agente.Resposta{Texto: "Resposta da voz."}},
		{deltas: []string{"Resposta do texto."}, resposta: agente.Resposta{Texto: "Resposta do texto."}},
	}})
	var geradas []string
	var mu sync.Mutex
	s.DiretorioAudio = t.TempDir()
	s.Sintetizador = &voz.Sintetizador{
		DiretorioSaida: s.DiretorioAudio,
		Timeout:        time.Second,
		Executor: func(_ context.Context, texto, _, arquivo string) error {
			mu.Lock()
			geradas = append(geradas, texto)
			mu.Unlock()
			return os.WriteFile(filepath.Clean(arquivo), []byte("RIFF"), 0o600)
		},
	}
	urlBase := "ws" + strings.TrimPrefix(servidor.URL, "http")
	app, _, err := websocket.DefaultDialer.Dial(urlBase+"/ws", nil)
	if err != nil {
		t.Fatalf("erro ao conectar o app: %v", err)
	}
	defer app.Close()
	doApp := leitorDoApp(app)
	device := conectarDevice(t, servidor)

	// Voz: o app recebe transcricao e resposta_final, mas NÃO audio_pronto.
	falarPeloDevice(t, device, "fala")
	eventosDaVoz := coletarEventosDoApp(t, doApp)
	if !temEvento(eventosDaVoz, "resposta_final") {
		t.Fatalf("o app não recebeu resposta_final: %+v", eventosDaVoz)
	}
	if temEvento(eventosDaVoz, "audio_pronto") {
		t.Error("turno de voz emitiu audio_pronto: voz dupla (device + app)")
	}
	mu.Lock()
	aposAVoz := len(geradas)
	mu.Unlock()
	if aposAVoz != 0 {
		t.Errorf("turno de voz sintetizou %d áudio(s) no jamesd, queria 0", aposAVoz)
	}

	// Texto: o app recebe audio_pronto, uma vez.
	if err := app.WriteJSON(Envelope{Tipo: "mensagem", ID: "t1", Dados: codificarDados(map[string]string{"texto": "digitado"})}); err != nil {
		t.Fatalf("erro ao enviar mensagem: %v", err)
	}
	eventosDoTexto := coletarEventosDoApp(t, doApp)
	n := 0
	for _, e := range eventosDoTexto {
		if e.Tipo == "audio_pronto" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("turno de texto emitiu %d audio_pronto, queria 1", n)
	}
}

// O tempo até o primeiro delta é o que o jamesd contribui pra latência da
// voz: sem ele no log, "o James está lento" não diz de quem é a culpa.
func TestLogDoTurnoTrazOTempoAtePrimeiroDelta(t *testing.T) {
	var saida strings.Builder
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&saida, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(anterior) })

	servidor, _ := servidorDeFala(t, &providerDeFala{rodadas: []rodadaDeFala{{
		deltas:   []string{"Oi. ", "Tudo bem."},
		resposta: agente.Resposta{Texto: "Oi. Tudo bem."},
	}}})
	conn := conectarDevice(t, servidor)

	falarPeloDevice(t, conn, "oi")
	lerFalaAteOFim(t, conn)

	var linha string
	for _, l := range strings.Split(saida.String(), "\n") {
		if strings.Contains(l, "msg=turno") {
			linha = l
		}
	}
	if linha == "" {
		t.Fatalf("nenhuma linha 'turno' no log: %q", saida.String())
	}
	if !strings.Contains(linha, "primeiro_delta_ms=") || strings.Contains(linha, "primeiro_delta_ms=-1") {
		t.Errorf("a linha do turno não traz o tempo até o primeiro delta: %s", linha)
	}
	if !strings.Contains(linha, "modo=voz") {
		t.Errorf("a linha do turno devia dizer modo=voz: %s", linha)
	}
}

func TestLogDoTurnoDeTextoSemStreamingDizMenosUm(t *testing.T) {
	var saida strings.Builder
	anterior := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&saida, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() { slog.SetDefault(anterior) })

	servidor, _ := servidorDeFala(t, &providerDeFala{rodadas: []rodadaDeFala{{
		deltas: []string{"Oi."}, resposta: agente.Resposta{Texto: "Oi."},
	}}})
	capturarEventosDeUmTurnoNoServidor(t, servidor, "oi")

	if !strings.Contains(saida.String(), "primeiro_delta_ms=-1") {
		t.Errorf("o turno de texto não usa streaming e devia logar -1: %s", saida.String())
	}
}
