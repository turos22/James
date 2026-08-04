package gemini

import (
	"testing"

	"google.golang.org/genai"

	"james/internal/agente"
	"james/internal/ferramentas"
)

func TestParaGenaiContentsMapeiaPapeis(t *testing.T) {
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "oi"},
		{Papel: agente.PapelModelo, Texto: "olá", Chamadas: []agente.ChamadaFerramenta{
			{Nome: "eco", Args: map[string]any{"x": "1"}},
		}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
			{Nome: "eco", Saida: "resultado"},
		}},
	}

	contents := paraGenaiContents(mensagens)
	if len(contents) != 3 {
		t.Fatalf("esperava 3 Content, vieram %d", len(contents))
	}
	if contents[0].Role != "user" {
		t.Errorf("contents[0].Role = %q, queria user", contents[0].Role)
	}
	if contents[1].Role != "model" {
		t.Errorf("contents[1].Role = %q, queria model", contents[1].Role)
	}
	if contents[2].Role != "function" {
		t.Errorf("contents[2].Role = %q, queria function (bug #6 do Anexo A: app.go antigo usava \"tool\")", contents[2].Role)
	}
}

func TestParaGenaiToolMapeiaParametros(t *testing.T) {
	decls := []ferramentas.Declaracao{
		{
			Nome:      "pesquisar_internet",
			Descricao: "busca na internet",
			Parametros: map[string]ferramentas.Parametro{
				"query": {Tipo: "string", Descricao: "termo de busca"},
			},
			Obrigatorios: []string{"query"},
		},
	}

	tool := paraGenaiTool(decls)
	if len(tool.FunctionDeclarations) != 1 {
		t.Fatalf("esperava 1 FunctionDeclaration, vieram %d", len(tool.FunctionDeclarations))
	}
	fd := tool.FunctionDeclarations[0]
	if fd.Name != "pesquisar_internet" {
		t.Errorf("Name = %q, queria pesquisar_internet", fd.Name)
	}
	if fd.Parameters.Properties["query"].Type != genai.TypeString {
		t.Error("propriedade query deveria ser TypeString")
	}
	if len(fd.Parameters.Required) != 1 || fd.Parameters.Required[0] != "query" {
		t.Errorf("Required = %v, queria [query]", fd.Parameters.Required)
	}
}

func TestParaRespostaExtraiTextoEChamadas(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Parts: []*genai.Part{
						{Text: "texto final"},
						{FunctionCall: &genai.FunctionCall{Name: "eco", Args: map[string]any{"x": "1"}}},
					},
				},
			},
		},
	}

	r := paraResposta(resp)
	if r.Texto != "texto final" {
		t.Errorf("Texto = %q, queria texto final", r.Texto)
	}
	if len(r.Chamadas) != 1 || r.Chamadas[0].Nome != "eco" {
		t.Errorf("Chamadas = %+v, queria 1 chamada pra eco", r.Chamadas)
	}
}

func TestParaRespostaCandidatoVazioDevolveRespostaZero(t *testing.T) {
	resp := &genai.GenerateContentResponse{}
	r := paraResposta(resp)
	if r.Texto != "" || len(r.Chamadas) != 0 {
		t.Errorf("esperava Resposta zero, veio %+v", r)
	}
}
