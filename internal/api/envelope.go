package api

import "encoding/json"

// Envelope é o formato comum de toda mensagem WS (spec seção 4.4).
type Envelope struct {
	Tipo  string          `json:"tipo"`
	ID    string          `json:"id"`
	Dados json.RawMessage `json:"dados"`
}

func codificarDados(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}
