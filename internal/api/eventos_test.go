package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

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
