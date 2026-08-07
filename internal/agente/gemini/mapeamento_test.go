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
	if contents[2].Role != "user" {
		t.Errorf("contents[2].Role = %q, queria user — a API Gemini não tem role \"function\"; "+
			"NewContentFromFunctionResponse do SDK usa RoleUser por padrão (corrige o bug #6 do Anexo A, "+
			"onde o app.go antigo usava \"tool\" sem confirmar se era aceito)", contents[2].Role)
	}
}

func TestParaGenaiContentsLevaImagemDeResultadoDeFerramenta(t *testing.T) {
	imagem := []byte{0x89, 'P', 'N', 'G', 1, 2, 3}
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
			{Nome: "ver_tela", Saida: "e1 Button \"Salvar\" (820,455)", Imagem: imagem},
		}},
	}

	contents := paraGenaiContents(mensagens)
	if len(contents) != 1 {
		t.Fatalf("esperava 1 Content, vieram %d", len(contents))
	}

	var achouResposta, achouImagem bool
	for _, part := range contents[0].Parts {
		if part.FunctionResponse != nil && part.FunctionResponse.Name == "ver_tela" {
			achouResposta = true
		}
		if part.InlineData != nil {
			achouImagem = true
			if part.InlineData.MIMEType != "image/png" {
				t.Errorf("MIMEType = %q, queria image/png", part.InlineData.MIMEType)
			}
			if string(part.InlineData.Data) != string(imagem) {
				t.Error("os bytes da imagem chegaram alterados")
			}
		}
	}
	if !achouResposta {
		t.Error("o FunctionResponse sumiu — a imagem não pode substituir o texto, tem que somar")
	}
	if !achouImagem {
		t.Error("a imagem foi descartada em silêncio, que é exatamente o bug desta task")
	}
}

func TestParaGenaiContentsNaoInventaImagemQuandoNaoHa(t *testing.T) {
	mensagens := []agente.Mensagem{
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
			{Nome: "eco", Saida: "resultado"},
		}},
	}

	for _, part := range paraGenaiContents(mensagens)[0].Parts {
		if part.InlineData != nil {
			t.Error("resultado sem imagem gerou InlineData")
		}
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
