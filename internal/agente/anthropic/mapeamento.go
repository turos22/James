// Package anthropic implementa agente.Provider sobre a API da Anthropic
// (Claude). Este arquivo é só o mapeamento entre as mensagens do James e os
// tipos do SDK, sem rede, para ser testável por inspeção do JSON.
package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	sdk "github.com/anthropics/anthropic-sdk-go"

	"james/internal/agente"
	"james/internal/ferramentas"
)

// maxTokens é folgado de propósito. O Haiku 5.5 pensa por padrão e o
// pensamento sai do mesmo orçamento: com 300, uma resposta com o sistema.md
// real saiu cortada pela metade (stop_reason=max_tokens), medido no spike.
const maxTokens = 4096

// paraParams monta a requisição. Modo voz desliga o pensamento (de 1,5–2 s
// para ~0,7 s até a primeira frase); modo texto pensa, com esforço baixo.
func paraParams(req agente.Requisicao, modelo string) sdk.MessageNewParams {
	p := sdk.MessageNewParams{
		Model:     sdk.Model(modelo),
		MaxTokens: maxTokens,
		Messages:  paraMensagens(req.Mensagens),
		Tools:     paraFerramentas(req.Declaracoes),
	}

	// Um único ponto de cache, no system: a ordem de renderização é tools,
	// system, messages, então ele cobre as ferramentas também. O system muda
	// quando a memória ou o resumo mudam; aí o cache erra uma vez e volta a
	// acertar, e isso só custa dinheiro, nunca correção.
	if req.Sistema != "" {
		p.System = []sdk.TextBlockParam{{
			Text:         req.Sistema,
			CacheControl: sdk.NewCacheControlEphemeralParam(),
		}}
	}

	if req.Modo == agente.ModoVoz {
		p.Thinking = sdk.ThinkingConfigParamUnion{OfDisabled: &sdk.ThinkingConfigDisabledParam{}}
	} else {
		p.Thinking = sdk.ThinkingConfigParamUnion{OfAdaptive: &sdk.ThinkingConfigAdaptiveParam{}}
		p.OutputConfig = sdk.OutputConfigParam{Effort: sdk.OutputConfigEffortLow}
	}
	return p
}

func paraFerramentas(decls []ferramentas.Declaracao) []sdk.ToolUnionParam {
	var tools []sdk.ToolUnionParam
	for _, d := range decls {
		props := map[string]any{}
		for nome, p := range d.Parametros {
			props[nome] = map[string]any{"type": tipoJSON(p.Tipo), "description": p.Descricao}
		}
		t := sdk.ToolParam{
			Name:        d.Nome,
			Description: sdk.String(d.Descricao),
			InputSchema: sdk.ToolInputSchemaParam{Properties: props, Required: d.Obrigatorios},
		}
		tools = append(tools, sdk.ToolUnionParam{OfTool: &t})
	}
	return tools
}

func tipoJSON(tipo string) string {
	switch tipo {
	case "integer", "number", "boolean":
		return tipo
	default:
		return "string"
	}
}

// desambiguador entrega ids de tool_use únicos na requisição inteira. O
// histórico guardado na era do Gemini usa o NOME da ferramenta como id, então
// a segunda chamada de executar_comando_sistema numa sessão repetiria o id, e
// a API recusa (400). A primeira ocorrência de um id fica como está; as
// seguintes ganham _2, _3...
type desambiguador struct{ vistos map[string]int }

func (d *desambiguador) unico(base string) string {
	if d.vistos == nil {
		d.vistos = map[string]int{}
	}
	d.vistos[base]++
	if n := d.vistos[base]; n > 1 {
		return fmt.Sprintf("%s_%d", base, n)
	}
	return base
}

func paraMensagens(mensagens []agente.Mensagem) []sdk.MessageParam {
	var out []sdk.MessageParam
	var ids desambiguador
	// Ids do último tool_use do modelo, em ordem: os resultados que vêm a
	// seguir são pareados por POSIÇÃO, porque os ids legados são todos iguais
	// e não dizem qual resultado é de qual chamada.
	var ultimosIDs []string

	for _, m := range mensagens {
		switch m.Papel {
		case agente.PapelUsuario:
			var blocos []sdk.ContentBlockParamUnion
			if m.Texto != "" {
				blocos = append(blocos, sdk.NewTextBlock(m.Texto))
			}
			if len(m.Imagem) > 0 {
				blocos = append(blocos, sdk.NewImageBlockBase64("image/png", base64.StdEncoding.EncodeToString(m.Imagem)))
			}
			if len(blocos) > 0 {
				out = append(out, sdk.NewUserMessage(blocos...))
			}

		case agente.PapelModelo:
			var blocos []sdk.ContentBlockParamUnion
			if m.Texto != "" {
				blocos = append(blocos, sdk.NewTextBlock(m.Texto))
			}
			ultimosIDs = ultimosIDs[:0]
			for _, c := range m.Chamadas {
				base := c.ID
				if base == "" {
					base = c.Nome
				}
				id := ids.unico(base)
				ultimosIDs = append(ultimosIDs, id)
				args := c.Args
				if args == nil {
					args = map[string]any{}
				}
				blocos = append(blocos, sdk.NewToolUseBlock(id, args, c.Nome))
			}
			// A API rejeita mensagem do assistente sem conteúdo.
			if len(blocos) > 0 {
				out = append(out, sdk.NewAssistantMessage(blocos...))
			}

		case agente.PapelFerramenta:
			var blocos []sdk.ContentBlockParamUnion
			for i, r := range m.Resultados {
				if i >= len(ultimosIDs) {
					// Resultado sem a chamada correspondente: mandar daria
					// 400 por tool_result órfão. Não acontece com o loop
					// atual; fica registrado aqui por segurança.
					continue
				}
				blocos = append(blocos, paraToolResult(ultimosIDs[i], r))
			}
			if len(blocos) > 0 {
				out = append(out, sdk.NewUserMessage(blocos...))
			}
		}
	}
	return out
}

func paraToolResult(id string, r agente.ResultadoFerramenta) sdk.ContentBlockParamUnion {
	texto := r.Saida
	if r.Erro != "" {
		texto = r.Erro
	}
	conteudo := []sdk.ToolResultBlockParamContentUnion{{OfText: &sdk.TextBlockParam{Text: texto}}}
	if len(r.Imagem) > 0 {
		conteudo = append(conteudo, sdk.ToolResultBlockParamContentUnion{OfImage: &sdk.ImageBlockParam{
			Source: sdk.ImageBlockParamSourceUnion{OfBase64: &sdk.Base64ImageSourceParam{
				Data:      base64.StdEncoding.EncodeToString(r.Imagem),
				MediaType: "image/png",
			}},
		}})
	}
	return sdk.ContentBlockParamUnion{OfToolResult: &sdk.ToolResultBlockParam{
		ToolUseID: id,
		Content:   conteudo,
		IsError:   sdk.Bool(r.Erro != ""),
	}}
}

// paraResposta junta o texto, as chamadas de ferramenta e o uso. Blocos de
// pensamento não entram: são do modelo, não do usuário, e não vão pra fala.
func paraResposta(msg sdk.Message) agente.Resposta {
	var r agente.Resposta
	for _, bloco := range msg.Content {
		switch b := bloco.AsAny().(type) {
		case sdk.TextBlock:
			r.Texto += b.Text
		case sdk.ToolUseBlock:
			args := map[string]any{}
			if len(b.Input) > 0 {
				_ = json.Unmarshal(b.Input, &args)
			}
			if args == nil {
				args = map[string]any{}
			}
			r.Chamadas = append(r.Chamadas, agente.ChamadaFerramenta{ID: b.ID, Nome: b.Name, Args: args})
		}
	}
	r.Uso = agente.Uso{
		// Entrada soma o que foi escrito no cache; Saida já inclui o pensamento.
		Entrada:      int(msg.Usage.InputTokens + msg.Usage.CacheCreationInputTokens),
		Saida:        int(msg.Usage.OutputTokens),
		CacheLeitura: int(msg.Usage.CacheReadInputTokens),
		Pensamento:   int(msg.Usage.OutputTokensDetails.ThinkingTokens),
	}
	return r
}
