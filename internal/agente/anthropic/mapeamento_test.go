package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"testing"

	sdk "github.com/anthropics/anthropic-sdk-go"

	"james/internal/agente"
	"james/internal/ferramentas"
)

// jsonDe serializa o que o SDK mandaria pela rede e devolve genérico: é o que
// importa de verdade (o corpo da requisição), não a forma dos structs.
func jsonDe(t *testing.T, v any) map[string]any {
	t.Helper()
	bruto, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(bruto, &m); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return m
}

func mensagensDe(t *testing.T, req agente.Requisicao) []any {
	t.Helper()
	corpo := jsonDe(t, paraParams(req, "claude-haiku-5-5"))
	msgs, _ := corpo["messages"].([]any)
	return msgs
}

func blocosDe(t *testing.T, msg any) []map[string]any {
	t.Helper()
	m := msg.(map[string]any)
	var blocos []map[string]any
	for _, b := range m["content"].([]any) {
		blocos = append(blocos, b.(map[string]any))
	}
	return blocos
}

func TestSistemaViraUmBlocoComCacheEfemero(t *testing.T) {
	corpo := jsonDe(t, paraParams(agente.Requisicao{Sistema: "Você é o James."}, "claude-haiku-5-5"))

	sistema, ok := corpo["system"].([]any)
	if !ok || len(sistema) != 1 {
		t.Fatalf("system = %v, queria um bloco só", corpo["system"])
	}
	bloco := sistema[0].(map[string]any)
	if bloco["type"] != "text" || bloco["text"] != "Você é o James." {
		t.Errorf("bloco do system = %v", bloco)
	}
	cache, _ := bloco["cache_control"].(map[string]any)
	if cache["type"] != "ephemeral" {
		t.Errorf("cache_control = %v, queria ephemeral", bloco["cache_control"])
	}
}

func TestSistemaVazioNaoMandaBlocoVazio(t *testing.T) {
	corpo := jsonDe(t, paraParams(agente.Requisicao{}, "claude-haiku-5-5"))
	if _, existe := corpo["system"]; existe {
		t.Errorf("system presente com sistema vazio: %v", corpo["system"])
	}
}

func TestDeclaracaoViraFerramentaComSchema(t *testing.T) {
	req := agente.Requisicao{Declaracoes: []ferramentas.Declaracao{{
		Nome:      "executar_comando_sistema",
		Descricao: "Executa comandos.",
		Parametros: map[string]ferramentas.Parametro{
			"comando": {Tipo: "string", Descricao: "o comando"},
			"limite":  {Tipo: "integer", Descricao: "quantos"},
		},
		Obrigatorios: []string{"comando"},
	}}}

	corpo := jsonDe(t, paraParams(req, "claude-haiku-5-5"))

	ferramentasJSON := corpo["tools"].([]any)
	if len(ferramentasJSON) != 1 {
		t.Fatalf("tools = %v", corpo["tools"])
	}
	f := ferramentasJSON[0].(map[string]any)
	if f["name"] != "executar_comando_sistema" || f["description"] != "Executa comandos." {
		t.Errorf("nome/descrição = %v / %v", f["name"], f["description"])
	}
	esquema := f["input_schema"].(map[string]any)
	if esquema["type"] != "object" {
		t.Errorf("input_schema.type = %v", esquema["type"])
	}
	props := esquema["properties"].(map[string]any)
	comando := props["comando"].(map[string]any)
	if comando["type"] != "string" || comando["description"] != "o comando" {
		t.Errorf("propriedade comando = %v", comando)
	}
	if props["limite"].(map[string]any)["type"] != "integer" {
		t.Errorf("propriedade limite = %v", props["limite"])
	}
	if !reflect.DeepEqual(esquema["required"], []any{"comando"}) {
		t.Errorf("required = %v", esquema["required"])
	}
}

func TestUsuarioModeloEFerramentaViramUserAssistantUser(t *testing.T) {
	req := agente.Requisicao{Mensagens: []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "lista os arquivos"},
		{Papel: agente.PapelModelo, Texto: "Vou ver.", Chamadas: []agente.ChamadaFerramenta{
			{ID: "toolu_a", Nome: "ls", Args: map[string]any{"dir": "."}},
			{ID: "toolu_b", Nome: "df", Args: map[string]any{}},
		}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
			{ID: "toolu_a", Nome: "ls", Saida: "a.txt"},
			{ID: "toolu_b", Nome: "df", Saida: "50%"},
		}},
	}}

	msgs := mensagensDe(t, req)
	if len(msgs) != 3 {
		t.Fatalf("%d mensagens, queria 3", len(msgs))
	}
	papeis := []string{"user", "assistant", "user"}
	for i, m := range msgs {
		if m.(map[string]any)["role"] != papeis[i] {
			t.Errorf("mensagem %d tem papel %v, queria %s", i, m.(map[string]any)["role"], papeis[i])
		}
	}

	assistente := blocosDe(t, msgs[1])
	if len(assistente) != 3 || assistente[0]["type"] != "text" || assistente[1]["type"] != "tool_use" || assistente[2]["type"] != "tool_use" {
		t.Fatalf("blocos do assistente = %v", assistente)
	}
	if assistente[1]["id"] != "toolu_a" || assistente[1]["name"] != "ls" {
		t.Errorf("primeiro tool_use = %v", assistente[1])
	}
	if !reflect.DeepEqual(assistente[1]["input"], map[string]any{"dir": "."}) {
		t.Errorf("input = %v", assistente[1]["input"])
	}

	resultados := blocosDe(t, msgs[2])
	if len(resultados) != 2 || resultados[0]["type"] != "tool_result" {
		t.Fatalf("blocos do resultado = %v", resultados)
	}
	if resultados[0]["tool_use_id"] != "toolu_a" || resultados[1]["tool_use_id"] != "toolu_b" {
		t.Errorf("pareamento quebrado: %v / %v", resultados[0]["tool_use_id"], resultados[1]["tool_use_id"])
	}
}

// O histórico antigo (era do Gemini) guarda o NOME da ferramenta como ID. A
// Anthropic exige id único e tool_use pareado com tool_result: sem
// desambiguar, a segunda chamada de executar_comando_sistema numa conversa
// daria 400 pra sempre naquela sessão.
func TestIdsLegadosRepetidosViramUnicosEPareados(t *testing.T) {
	const legado = "executar_comando_sistema"
	req := agente.Requisicao{Mensagens: []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "um"},
		{Papel: agente.PapelModelo, Chamadas: []agente.ChamadaFerramenta{
			{ID: legado, Nome: legado, Args: map[string]any{"comando": "a"}},
			{ID: legado, Nome: legado, Args: map[string]any{"comando": "b"}},
		}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
			{ID: legado, Nome: legado, Saida: "A"},
			{ID: legado, Nome: legado, Saida: "B"},
		}},
		{Papel: agente.PapelUsuario, Texto: "dois"},
		{Papel: agente.PapelModelo, Chamadas: []agente.ChamadaFerramenta{
			{ID: legado, Nome: legado, Args: map[string]any{"comando": "c"}},
		}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{
			{ID: legado, Nome: legado, Saida: "C"},
		}},
	}}

	msgs := mensagensDe(t, req)

	var usos, resultados []string
	for _, m := range msgs {
		for _, b := range blocosDe(t, m) {
			switch b["type"] {
			case "tool_use":
				usos = append(usos, b["id"].(string))
			case "tool_result":
				resultados = append(resultados, b["tool_use_id"].(string))
			}
		}
	}
	esperado := []string{legado, legado + "_2", legado + "_3"}
	if !reflect.DeepEqual(usos, esperado) {
		t.Errorf("ids dos tool_use = %v, queria %v", usos, esperado)
	}
	if !reflect.DeepEqual(resultados, esperado) {
		t.Errorf("ids dos tool_result = %v, queria %v (pareados por posição)", resultados, esperado)
	}
}

func TestResultadoComErroViraToolResultComIsError(t *testing.T) {
	req := agente.Requisicao{Mensagens: []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "x"},
		{Papel: agente.PapelModelo, Chamadas: []agente.ChamadaFerramenta{{ID: "t1", Nome: "ls"}}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{{ID: "t1", Nome: "ls", Erro: "permissão negada"}}},
	}}

	r := blocosDe(t, mensagensDe(t, req)[2])[0]

	if r["is_error"] != true {
		t.Errorf("is_error = %v, queria true", r["is_error"])
	}
	conteudo := r["content"].([]any)[0].(map[string]any)
	if conteudo["type"] != "text" || conteudo["text"] != "permissão negada" {
		t.Errorf("conteúdo = %v", conteudo)
	}
}

func TestResultadoComImagemTrazTextoEImagemBase64(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 1, 2, 3}
	req := agente.Requisicao{Mensagens: []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "x"},
		{Papel: agente.PapelModelo, Chamadas: []agente.ChamadaFerramenta{{ID: "t1", Nome: "ver_tela"}}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{{ID: "t1", Nome: "ver_tela", Saida: "1 elemento", Imagem: png}}},
	}}

	r := blocosDe(t, mensagensDe(t, req)[2])[0]

	conteudo := r["content"].([]any)
	if len(conteudo) != 2 {
		t.Fatalf("conteúdo = %v, queria texto + imagem", conteudo)
	}
	if conteudo[0].(map[string]any)["text"] != "1 elemento" {
		t.Errorf("texto = %v", conteudo[0])
	}
	imagem := conteudo[1].(map[string]any)
	fonte := imagem["source"].(map[string]any)
	if imagem["type"] != "image" || fonte["media_type"] != "image/png" || fonte["data"] != base64.StdEncoding.EncodeToString(png) {
		t.Errorf("imagem = %v", imagem)
	}
}

func TestModeloSemTextoNaoGeraBlocoDeTextoVazio(t *testing.T) {
	req := agente.Requisicao{Mensagens: []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "x"},
		{Papel: agente.PapelModelo, Texto: "", Chamadas: []agente.ChamadaFerramenta{{ID: "t1", Nome: "ls"}}},
	}}

	for _, b := range blocosDe(t, mensagensDe(t, req)[1]) {
		if b["type"] == "text" {
			t.Errorf("bloco de texto presente: %v (a API rejeita texto vazio)", b)
		}
	}
}

func TestMensagemDoModeloTotalmenteVaziaEOmitida(t *testing.T) {
	req := agente.Requisicao{Mensagens: []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "x"},
		{Papel: agente.PapelModelo},
		{Papel: agente.PapelUsuario, Texto: "y"},
	}}

	for _, m := range mensagensDe(t, req) {
		if m.(map[string]any)["role"] == "assistant" {
			t.Errorf("assistant sem conteúdo foi mandado: %v (a API rejeita)", m)
		}
	}
}

func TestUsuarioComImagemTrazTextoEImagem(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G'}
	req := agente.Requisicao{Mensagens: []agente.Mensagem{{Papel: agente.PapelUsuario, Texto: "olha", Imagem: png}}}

	blocos := blocosDe(t, mensagensDe(t, req)[0])
	if len(blocos) != 2 || blocos[0]["type"] != "text" || blocos[1]["type"] != "image" {
		t.Errorf("blocos = %v", blocos)
	}
}

func TestPensamentoEEsforcoPorModo(t *testing.T) {
	voz := jsonDe(t, paraParams(agente.Requisicao{Modo: agente.ModoVoz}, "claude-haiku-5-5"))
	if !reflect.DeepEqual(voz["thinking"], map[string]any{"type": "disabled"}) {
		t.Errorf("voz: thinking = %v, queria disabled", voz["thinking"])
	}

	texto := jsonDe(t, paraParams(agente.Requisicao{Modo: agente.ModoTexto}, "claude-haiku-5-5"))
	if t1, _ := texto["thinking"].(map[string]any); t1["type"] != "adaptive" {
		t.Errorf("texto: thinking = %v, queria adaptive", texto["thinking"])
	}
	saida, _ := texto["output_config"].(map[string]any)
	if saida["effort"] != "low" {
		t.Errorf("texto: output_config = %v, queria effort low", texto["output_config"])
	}
	if _, existe := voz["output_config"]; existe {
		t.Errorf("voz não deveria mandar output_config: %v", voz["output_config"])
	}
}

// 300 já cortou resposta no spike: com o sistema.md real o pensamento comeu o
// limite e a resposta saiu pela metade (stop_reason=max_tokens).
func TestMaxTokensEFolgadoEModeloVemDoParametro(t *testing.T) {
	corpo := jsonDe(t, paraParams(agente.Requisicao{}, "claude-haiku-5-5"))
	if corpo["max_tokens"] != float64(4096) {
		t.Errorf("max_tokens = %v, queria 4096", corpo["max_tokens"])
	}
	if corpo["model"] != "claude-haiku-5-5" {
		t.Errorf("model = %v", corpo["model"])
	}
}

func mensagemDeJSON(t *testing.T, bruto string) sdk.Message {
	t.Helper()
	var m sdk.Message
	if err := json.Unmarshal([]byte(bruto), &m); err != nil {
		t.Fatalf("não consegui montar a Message: %v", err)
	}
	return m
}

func TestRespostaJuntaTextoChamadasEUso(t *testing.T) {
	msg := mensagemDeJSON(t, `{
		"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-5-5",
		"content":[
			{"type":"thinking","thinking":"","signature":"x"},
			{"type":"text","text":"Vou listar."},
			{"type":"tool_use","id":"toolu_1","name":"executar_comando_sistema","input":{"comando":"ls"}}
		],
		"stop_reason":"tool_use","stop_sequence":null,
		"usage":{"input_tokens":10,"output_tokens":300,"cache_read_input_tokens":1500,
		         "cache_creation_input_tokens":20,"output_tokens_details":{"thinking_tokens":200}}
	}`)

	r := paraResposta(msg)

	if r.Texto != "Vou listar." {
		t.Errorf("Texto = %q (o bloco de pensamento não pode entrar)", r.Texto)
	}
	if len(r.Chamadas) != 1 {
		t.Fatalf("Chamadas = %v", r.Chamadas)
	}
	c := r.Chamadas[0]
	if c.ID != "toolu_1" || c.Nome != "executar_comando_sistema" || c.Args["comando"] != "ls" {
		t.Errorf("chamada = %+v", c)
	}
	// Entrada soma o que foi escrito no cache; Saida já inclui o pensamento.
	esperado := agente.Uso{Entrada: 30, Saida: 300, CacheLeitura: 1500, Pensamento: 200}
	if r.Uso != esperado {
		t.Errorf("Uso = %+v, queria %+v", r.Uso, esperado)
	}
}

func TestChamadaSemArgumentosViraMapaVazioNaoNil(t *testing.T) {
	msg := mensagemDeJSON(t, `{
		"id":"m","type":"message","role":"assistant","model":"x",
		"content":[{"type":"tool_use","id":"t","name":"ver_tela","input":{}}],
		"stop_reason":"tool_use","stop_sequence":null,
		"usage":{"input_tokens":1,"output_tokens":1}
	}`)

	r := paraResposta(msg)

	if r.Chamadas[0].Args == nil {
		t.Error("Args = nil; ferramentas leem do mapa e nil quebra a asserção de tipo mais adiante")
	}
}
