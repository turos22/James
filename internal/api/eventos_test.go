package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"james/internal/agente"
	"james/internal/dispositivo"
)

// dadosDoEvento devolve os `dados` crus do primeiro envelope de `tipo` numa
// lista de eventos capturados — nil se o tipo não apareceu.
func dadosDoEvento(eventos []Envelope, tipo string) json.RawMessage {
	for _, e := range eventos {
		if e.Tipo == tipo {
			return e.Dados
		}
	}
	return nil
}

// TestFormatoDeEventosIdenticoEntreTextoEVoz prova, por teste e não por
// leitura, que a extração da Parte A (Task 18) não deixou tratarMensagem
// (texto) e tratarEntradaDeVoz (voz) divergirem na forma dos eventos que
// compartilham: ferramenta_iniciada e ferramenta_concluida. É o mesmo tipo
// de regressão que motivou a extração — o evento "token" sobreviveu num
// caminho e sumiu do outro até alguém rodar o grep (Task 5) — só que agora
// pego por teste, não por leitura de código.
//
// Compara os BYTES CRUS do payload, não só que os dois decodificam pro
// mesmo tipo Go: se um caminho ganhasse um campo a mais (ou usasse uma tag
// json diferente) sem o outro, decodificar os dois contra o mesmo struct
// esconderia a divergência — campo extra é ignorado por encoding/json, e um
// zero-value seria indistinguível de "campo nunca existiu". Comparação de
// string pega isso.
func TestFormatoDeEventosIdenticoEntreTextoEVoz(t *testing.T) {
	toolTexto := &ferramentaFalsaComSaida{nome: "ver_tela", saida: "superficie: DISPLAY1"}
	sTexto := servidorComFerramentaFalsa(t, toolTexto, "ver_tela")
	servidorTexto := httptest.NewServer(sTexto.Rotas())
	defer servidorTexto.Close()
	eventosTexto := capturarEventosDeUmTurnoNoServidor(t, servidorTexto, "olha a tela")

	toolVoz := &ferramentaFalsaComSaida{nome: "ver_tela", saida: "superficie: DISPLAY1"}
	sVoz := servidorComFerramentaFalsa(t, toolVoz, "ver_tela")
	sVoz.DispositivoHub = dispositivo.NovoHub()
	servidorVoz := httptest.NewServer(sVoz.Rotas())
	defer servidorVoz.Close()
	eventosVoz := capturarEventosDeEntradaDeVozNoServidor(t, servidorVoz, "olha a tela")

	for _, tipo := range []string{"ferramenta_iniciada", "ferramenta_concluida"} {
		dTexto := dadosDoEvento(eventosTexto, tipo)
		dVoz := dadosDoEvento(eventosVoz, tipo)
		if dTexto == nil || dVoz == nil {
			t.Fatalf("evento %q ausente em algum caminho: texto=%s voz=%s", tipo, dTexto, dVoz)
		}
		if string(dTexto) != string(dVoz) {
			t.Errorf("payload de %q divergiu entre texto e voz:\ntexto: %s\nvoz:   %s", tipo, dTexto, dVoz)
		}
	}
}

// TestMensagensExibiveisFiltraFerramentaEModeloSemTexto prova o achado da
// verificação ao vivo da Fase 6: retomar sessão reconstrói o histórico certo
// pro modelo, mas o evento sessao_retomada só mandava a CONTAGEM de
// mensagens, não o conteúdo — a tela ficava em branco mesmo com o Gemini
// lembrando de tudo. mensagensExibiveis é o filtro entre "o que o modelo usa
// de contexto" (historico inteiro, todos os papéis) e "o que a tela pode
// desenhar como balão" (usuário e modelo com texto). Papel ferramenta fica
// de fora porque já tem superfície própria (aba Atividade, visualizador); e
// modelo sem texto fica de fora porque essas linhas só carregam chamada de
// ferramenta (Chamadas preenchido, Texto vazio) — sem filtro virariam balão
// em branco. O teste também prova que a ordem cronológica sobrevive ao
// filtro.
func TestMensagensExibiveisFiltraFerramentaEModeloSemTexto(t *testing.T) {
	historico := []agente.Mensagem{
		{Papel: agente.PapelUsuario, Texto: "o que tem na tela?"},
		{Papel: agente.PapelModelo, Chamadas: []agente.ChamadaFerramenta{{ID: "c1", Nome: "ver_tela"}}},
		{Papel: agente.PapelFerramenta, Resultados: []agente.ResultadoFerramenta{{ID: "c1", Nome: "ver_tela", Saida: "superficie: DISPLAY1"}}},
		{Papel: agente.PapelModelo, Texto: "tem a calculadora aberta"},
	}

	mensagens := mensagensExibiveis(historico)

	esperado := []mensagemDoHistorico{
		{Papel: "usuario", Texto: "o que tem na tela?"},
		{Papel: "modelo", Texto: "tem a calculadora aberta"},
	}
	if len(mensagens) != len(esperado) {
		t.Fatalf("esperava %d mensagens exibíveis, veio %d: %+v", len(esperado), len(mensagens), mensagens)
	}
	for i := range esperado {
		if mensagens[i] != esperado[i] {
			t.Errorf("mensagem %d veio errada: %+v, queria %+v", i, mensagens[i], esperado[i])
		}
	}
}
