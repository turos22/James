package gemini

import (
	"google.golang.org/genai"

	"james/internal/agente"
	"james/internal/ferramentas"
)

func paraGenaiContents(mensagens []agente.Mensagem) []*genai.Content {
	var contents []*genai.Content
	for _, m := range mensagens {
		switch m.Papel {
		case agente.PapelUsuario:
			parts := []*genai.Part{{Text: m.Texto}}
			if len(m.Imagem) > 0 {
				parts = append(parts, &genai.Part{InlineData: &genai.Blob{MIMEType: "image/png", Data: m.Imagem}})
			}
			contents = append(contents, &genai.Content{Role: "user", Parts: parts})
		case agente.PapelModelo:
			var parts []*genai.Part
			if m.Texto != "" {
				parts = append(parts, &genai.Part{Text: m.Texto})
			}
			for _, c := range m.Chamadas {
				parts = append(parts, &genai.Part{FunctionCall: &genai.FunctionCall{Name: c.Nome, Args: c.Args}})
			}
			contents = append(contents, &genai.Content{Role: "model", Parts: parts})
		case agente.PapelFerramenta:
			var parts []*genai.Part
			for _, r := range m.Resultados {
				saida := r.Saida
				if r.Erro != "" {
					saida = "erro: " + r.Erro
				}
				parts = append(parts, &genai.Part{FunctionResponse: &genai.FunctionResponse{
					Name:     r.Nome,
					Response: map[string]any{"output": saida},
				}})
			}
			// "function" é o role correto pra resposta de tool na API Gemini —
			// corrige o bug #6 do Anexo A, onde o app.go antigo usava "tool"
			// sem confirmar se era aceito.
			contents = append(contents, &genai.Content{Role: "function", Parts: parts})
		}
	}
	return contents
}

func paraGenaiTool(decls []ferramentas.Declaracao) *genai.Tool {
	var fds []*genai.FunctionDeclaration
	for _, d := range decls {
		props := map[string]*genai.Schema{}
		for nome, p := range d.Parametros {
			props[nome] = &genai.Schema{Type: tipoGenai(p.Tipo), Description: p.Descricao}
		}
		fds = append(fds, &genai.FunctionDeclaration{
			Name:        d.Nome,
			Description: d.Descricao,
			Parameters:  &genai.Schema{Type: genai.TypeObject, Properties: props, Required: d.Obrigatorios},
		})
	}
	return &genai.Tool{FunctionDeclarations: fds}
}

func tipoGenai(tipo string) genai.Type {
	switch tipo {
	case "integer":
		return genai.TypeInteger
	case "number":
		return genai.TypeNumber
	case "boolean":
		return genai.TypeBoolean
	default:
		return genai.TypeString
	}
}

func paraResposta(resp *genai.GenerateContentResponse) agente.Resposta {
	var r agente.Resposta
	if len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return r
	}
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Text != "" {
			r.Texto = part.Text
		}
		if part.FunctionCall != nil {
			r.Chamadas = append(r.Chamadas, agente.ChamadaFerramenta{
				ID:   part.FunctionCall.Name,
				Nome: part.FunctionCall.Name,
				Args: part.FunctionCall.Args,
			})
		}
	}
	return r
}
