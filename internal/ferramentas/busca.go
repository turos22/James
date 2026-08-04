package ferramentas

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Busca porta app.go:215 (BuscarNoTavily) como está: search_depth basic,
// devolve o JSON bruto pro modelo extrair o que precisa. Melhorar é Fase 3
// (Anexo A #16).
type Busca struct {
	APIKey  string
	Cliente *http.Client
	URL     string
}

func NovaBusca(apiKey string) *Busca {
	return &Busca{APIKey: apiKey, Cliente: http.DefaultClient, URL: "https://api.tavily.com/search"}
}

func (b *Busca) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "pesquisar_internet",
		Descricao: "Busca informações atualizadas na internet.",
		Parametros: map[string]Parametro{
			"query": {Tipo: "string", Descricao: "termo de busca"},
		},
		Obrigatorios: []string{"query"},
	}
}

func (b *Busca) Nivel() Nivel { return NivelSeguro }

func (b *Busca) Executar(ctx context.Context, args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", fmt.Errorf("parâmetro query ausente ou vazio")
	}

	payload := map[string]any{"api_key": b.APIKey, "query": query, "search_depth": "basic"}
	corpo, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, b.URL, bytes.NewReader(corpo))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := b.Cliente.Do(req)
	if err != nil {
		return "", fmt.Errorf("erro ao conectar na busca: %w", err)
	}
	defer resp.Body.Close()

	respostaBruta, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(respostaBruta), nil
}
