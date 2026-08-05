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
	chamouDefinirTamanho          bool
	chamouDefinirPosicao          bool
}

func (c *chamadasJanelaFalsa) DefinirTamanho(_ context.Context, largura, altura int) {
	c.chamouDefinirTamanho = true
	c.tamanhoLargura, c.tamanhoAltura = largura, altura
}

func (c *chamadasJanelaFalsa) ObterPosicao(_ context.Context) (int, int) {
	return c.posX, c.posY
}

func (c *chamadasJanelaFalsa) DefinirPosicao(_ context.Context, x, y int) {
	c.chamouDefinirPosicao = true
	c.posX, c.posY = x, y
}

func TestPostTamanhoChamaDefinirTamanhoComOsValoresDaQuery(t *testing.T) {
	falsa := &chamadasJanelaFalsa{}
	mux := montarMuxControle(context.Background(), falsa)
	servidor := httptest.NewServer(comCORS(mux))
	defer servidor.Close()

	resp, err := servidor.Client().Post(servidor.URL+"/tamanho?largura=800&altura=900", "", nil)
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	if !falsa.chamouDefinirTamanho {
		t.Fatal("esperava DefinirTamanho chamado")
	}
	if falsa.tamanhoLargura != 800 || falsa.tamanhoAltura != 900 {
		t.Errorf("largura/altura = %d/%d, queria 800/900", falsa.tamanhoLargura, falsa.tamanhoAltura)
	}
}

func TestPostTamanhoComQueryInvalidaDevolve400(t *testing.T) {
	falsa := &chamadasJanelaFalsa{}
	mux := montarMuxControle(context.Background(), falsa)
	servidor := httptest.NewServer(comCORS(mux))
	defer servidor.Close()

	resp, err := servidor.Client().Post(servidor.URL+"/tamanho?largura=abc&altura=900", "", nil)
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
