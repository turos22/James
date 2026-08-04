package ferramentas

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBuscaEnviaQueryEDevolveRespostaBruta(t *testing.T) {
	var corpoRecebido map[string]any
	servidor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		corpo, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(corpo, &corpoRecebido)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"results":[{"title":"exemplo"}]}`))
	}))
	defer servidor.Close()

	b := &Busca{APIKey: "chave-de-teste", Cliente: servidor.Client(), URL: servidor.URL}
	saida, err := b.Executar(context.Background(), map[string]any{"query": "clima em são paulo"})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if saida != `{"results":[{"title":"exemplo"}]}` {
		t.Errorf("saida = %q, deveria devolver o JSON bruto da Tavily", saida)
	}
	if corpoRecebido["query"] != "clima em são paulo" {
		t.Errorf("query enviada = %v, queria clima em são paulo", corpoRecebido["query"])
	}
	if corpoRecebido["search_depth"] != "basic" {
		t.Errorf("search_depth = %v, queria basic (comportamento portado como está)", corpoRecebido["search_depth"])
	}
}

func TestBuscaRejeitaArgumentoAusente(t *testing.T) {
	b := NovaBusca("chave")
	if _, err := b.Executar(context.Background(), map[string]any{}); err == nil {
		t.Fatal("esperava erro por parâmetro query ausente, veio nil")
	}
}

func TestBuscaNivelSeguro(t *testing.T) {
	b := NovaBusca("chave")
	if b.Nivel() != NivelSeguro {
		t.Error("pesquisar_internet deve ser NivelSeguro")
	}
}
