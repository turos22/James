package main

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
)

type chamadasJanelaFalsa struct {
	tamanhoLargura, tamanhoAltura int
	posX, posY                    int
	telaLargura, telaAltura       int
	chamouDefinirTamanho          bool
	chamouDefinirPosicao          bool
}

func (c *chamadasJanelaFalsa) DefinirTamanho(_ context.Context, largura, altura int) {
	c.chamouDefinirTamanho = true
	c.tamanhoLargura, c.tamanhoAltura = largura, altura
}

func (c *chamadasJanelaFalsa) ObterTamanho(_ context.Context) (int, int) {
	return c.tamanhoLargura, c.tamanhoAltura
}

func (c *chamadasJanelaFalsa) ObterPosicao(_ context.Context) (int, int) {
	return c.posX, c.posY
}

func (c *chamadasJanelaFalsa) DefinirPosicao(_ context.Context, x, y int) {
	c.chamouDefinirPosicao = true
	c.posX, c.posY = x, y
}

func (c *chamadasJanelaFalsa) TamanhoTelaAtual(_ context.Context) (int, int) {
	return c.telaLargura, c.telaAltura
}

func TestPostRedimensionarAncoraNoCantoInferiorDireitoAtual(t *testing.T) {
	// Bolinha 100x100 em (700,700) — canto inferior-direito em (800,800).
	// Expandindo pra 800x900, o canto inferior-direito tem que continuar
	// em (800,800): novo topo-esquerda = (800-800, 800-900) = (0,-100).
	// Sem tela cadastrada (0,0), não há clamp — confere só a ancoragem.
	falsa := &chamadasJanelaFalsa{posX: 700, posY: 700, tamanhoLargura: 100, tamanhoAltura: 100}
	mux := montarMuxControle(context.Background(), falsa)
	servidor := httptest.NewServer(comCORS(mux))
	defer servidor.Close()

	resp, err := servidor.Client().Post(servidor.URL+"/redimensionar?largura=800&altura=900", "", nil)
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if !falsa.chamouDefinirTamanho || falsa.tamanhoLargura != 800 || falsa.tamanhoAltura != 900 {
		t.Errorf("tamanho = %d/%d, queria 800/900", falsa.tamanhoLargura, falsa.tamanhoAltura)
	}
	if !falsa.chamouDefinirPosicao || falsa.posX != 0 || falsa.posY != -100 {
		t.Errorf("posição = (%d,%d), queria (0,-100) — canto inferior-direito devia continuar em (800,800)", falsa.posX, falsa.posY)
	}
}

func TestPostRedimensionarLimitaContraATela(t *testing.T) {
	// Mesma expansão do teste acima, mas agora com uma tela de 1920x1080
	// cadastrada — sem clamp o Y ficaria negativo (-100), o que já é
	// fora da tela por cima; com clamp deve ficar em 0.
	falsa := &chamadasJanelaFalsa{
		posX: 700, posY: 700, tamanhoLargura: 100, tamanhoAltura: 100,
		telaLargura: 1920, telaAltura: 1080,
	}
	mux := montarMuxControle(context.Background(), falsa)
	servidor := httptest.NewServer(comCORS(mux))
	defer servidor.Close()

	resp, err := servidor.Client().Post(servidor.URL+"/redimensionar?largura=800&altura=900", "", nil)
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if falsa.posY != 0 {
		t.Errorf("posY = %d, queria 0 (sem clamp ficaria negativo, fora da tela por cima)", falsa.posY)
	}
	if falsa.posX != 0 {
		t.Errorf("posX = %d, queria 0", falsa.posX)
	}
}

func TestPostRedimensionarComQueryInvalidaDevolve400(t *testing.T) {
	falsa := &chamadasJanelaFalsa{}
	mux := montarMuxControle(context.Background(), falsa)
	servidor := httptest.NewServer(comCORS(mux))
	defer servidor.Close()

	resp, err := servidor.Client().Post(servidor.URL+"/redimensionar?largura=abc&altura=900", "", nil)
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 400 {
		t.Errorf("status = %d, queria 400", resp.StatusCode)
	}
	if falsa.chamouDefinirTamanho {
		t.Error("não deveria ter chamado DefinirTamanho com query inválida")
	}
}

func TestGetPosicaoDevolveJSONComAPosicaoAtual(t *testing.T) {
	falsa := &chamadasJanelaFalsa{posX: 123, posY: 456}
	mux := montarMuxControle(context.Background(), falsa)
	servidor := httptest.NewServer(comCORS(mux))
	defer servidor.Close()

	resp, err := servidor.Client().Get(servidor.URL + "/posicao")
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	var corpo map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&corpo); err != nil {
		t.Fatalf("erro ao decodificar: %v", err)
	}
	if corpo["x"] != 123 || corpo["y"] != 456 {
		t.Errorf("posição = %v, queria {x:123,y:456}", corpo)
	}
}

func TestPostPosicaoChamaDefinirPosicao(t *testing.T) {
	falsa := &chamadasJanelaFalsa{}
	mux := montarMuxControle(context.Background(), falsa)
	servidor := httptest.NewServer(comCORS(mux))
	defer servidor.Close()

	resp, err := servidor.Client().Post(servidor.URL+"/posicao?x=10&y=20", "", nil)
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if !falsa.chamouDefinirPosicao || falsa.posX != 10 || falsa.posY != 20 {
		t.Errorf("esperava DefinirPosicao(10,20), veio chamou=%v pos=(%d,%d)", falsa.chamouDefinirPosicao, falsa.posX, falsa.posY)
	}
}

func TestCORSLiberaOrigemDiferente(t *testing.T) {
	falsa := &chamadasJanelaFalsa{}
	mux := montarMuxControle(context.Background(), falsa)
	servidor := httptest.NewServer(comCORS(mux))
	defer servidor.Close()

	resp, err := servidor.Client().Get(servidor.URL + "/posicao")
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, queria \"*\"", got)
	}
}

func TestCalcularRedimensionamentoAncoradoSemTela(t *testing.T) {
	// Bolinha 100x100 em (700,700), expande pra 800x900: canto
	// inferior-direito (800,800) tem que se manter fixo.
	x, y := calcularRedimensionamentoAncorado(700, 700, 100, 100, 800, 900, 0, 0)
	if x != 0 || y != -100 {
		t.Errorf("posição = (%d,%d), queria (0,-100)", x, y)
	}
}

func TestCalcularRedimensionamentoAncoradoEncolhendoDeVolta(t *testing.T) {
	// Chat 800x900 em (0,-100) — canto inferior-direito em (800,800).
	// Encolhendo de volta pra 100x100, o canto inferior-direito continua
	// em (800,800): novo topo-esquerda = (700,700), voltando exatamente
	// pra onde a bolinha estava antes de abrir.
	x, y := calcularRedimensionamentoAncorado(0, -100, 800, 900, 100, 100, 0, 0)
	if x != 700 || y != 700 {
		t.Errorf("posição = (%d,%d), queria (700,700)", x, y)
	}
}

func TestCalcularRedimensionamentoAncoradoNaoDeixaSairDaTelaPelaDireita(t *testing.T) {
	// Bolinha em (1850,50) — canto inferior-direito em (1950,150), já
	// além dos 1920 de largura da tela (a bolinha nasceu ali antes de
	// qualquer clamp existir, ou a tela mudou de resolução). Expandindo
	// pra 800 de largura, o clamp tem que garantir x+largura <= 1920.
	x, _ := calcularRedimensionamentoAncorado(1850, 50, 100, 100, 800, 900, 1920, 1080)
	if x+800 > 1920 {
		t.Errorf("x+largura = %d, deveria ser <= 1920 (largura da tela)", x+800)
	}
	if x < 0 {
		t.Errorf("x = %d, não deveria ficar negativo", x)
	}
}
