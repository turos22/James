package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func TestOrigemPermitida(t *testing.T) {
	casos := []struct {
		nome   string
		origem string
		host   string
		comTLS bool
		quer   bool
	}{
		{
			// O device Python, o curl e qualquer cliente que não seja
			// navegador não mandam Origin. Recusar aqui derrubaria a ponte de
			// tela — e não protegeria nada: processo local da mesma conta
			// forja qualquer header de qualquer jeito.
			nome: "sem origem nenhuma", origem: "", host: "127.0.0.1:8080", quer: true,
		},
		{
			// O shell Wails navega pra http://127.0.0.1:8080/, então a SPA do
			// desktop tem exatamente esta origem.
			nome: "mesma origem em loopback", origem: "http://127.0.0.1:8080",
			host: "127.0.0.1:8080", quer: true,
		},
		{
			nome: "mesma origem na VPN com TLS", origem: "https://desktop-dc358rs.tail1234.ts.net:8443",
			host: "desktop-dc358rs.tail1234.ts.net:8443", comTLS: true, quer: true,
		},
		{
			nome:   "esquema e host em maiúscula continuam a mesma origem",
			origem: "HTTP://127.0.0.1:8080", host: "127.0.0.1:8080", quer: true,
		},
		{
			// O ataque que importa: uma página que o Arthur abriu no
			// navegador manda new WebSocket("ws://127.0.0.1:8080/ws"). O
			// navegador é obrigado a mandar o Origin verdadeiro — a página não
			// tem como forjar — e é isso que torna a checagem eficaz aqui.
			nome: "site qualquer falando com o loopback", origem: "https://site-qualquer.example",
			host: "127.0.0.1:8080", quer: false,
		},
		{
			nome: "mesmo host em porta diferente", origem: "http://127.0.0.1:9999",
			host: "127.0.0.1:8080", quer: false,
		},
		{
			nome: "esquema errado pro atendimento TLS", origem: "http://desktop.tail1234.ts.net:8443",
			host: "desktop.tail1234.ts.net:8443", comTLS: true, quer: false,
		},
		{
			// iframe com sandbox e página de file:// mandam literalmente
			// "null". Não é ausência de origem, é uma origem opaca.
			nome: "origem opaca", origem: "null", host: "127.0.0.1:8080", quer: false,
		},
		{
			nome: "origem que só começa igual", origem: "http://127.0.0.1:8080.example.com",
			host: "127.0.0.1:8080", quer: false,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/ws", nil)
			r.Host = caso.host
			if caso.origem != "" {
				r.Header.Set("Origin", caso.origem)
			}
			if caso.comTLS {
				r.TLS = &tls.ConnectionState{}
			}

			if got := origemPermitida(r); got != caso.quer {
				t.Errorf("origemPermitida(origem=%q, host=%q, tls=%v) = %v, queria %v",
					caso.origem, caso.host, caso.comTLS, got, caso.quer)
			}
		})
	}
}

// TestHandshakeDeOrigemEstranhaEhRecusadoDeVerdade prova a FIAÇÃO, não a
// função: origemPermitida podia estar perfeita e não estar ligada no
// upgrader. Só um handshake real contra o servidor real mostra isso.
//
// Encena o ataque: uma página em outro site abrindo WebSocket pro loopback do
// jamesd. O navegador preenche o Origin sozinho e a página não tem como
// mentir — é essa propriedade que a checagem explora.
func TestHandshakeDeOrigemEstranhaEhRecusadoDeVerdade(t *testing.T) {
	servidor := servidorWSDeTeste(t)
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws"

	cabecalho := http.Header{}
	cabecalho.Set("Origin", "https://site-qualquer.example")

	conn, resp, err := websocket.DefaultDialer.Dial(urlWS, cabecalho)
	if err == nil {
		conn.Close()
		t.Fatal("o handshake subiu com origem de outro site — o CheckOrigin não está ligado no upgrader")
	}
	if resp == nil {
		t.Fatalf("erro sem resposta HTTP: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, queria %d", resp.StatusCode, http.StatusForbidden)
	}
}

// TestHandshakeDaPropriaOrigemSobe é a contraparte: apertar demais quebraria o
// shell do desktop, que navega pra http://127.0.0.1:8080/ e portanto abre o WS
// com essa origem exata.
func TestHandshakeDaPropriaOrigemSobe(t *testing.T) {
	servidor := servidorWSDeTeste(t)
	defer servidor.Close()

	cabecalho := http.Header{}
	cabecalho.Set("Origin", servidor.URL) // http://127.0.0.1:<porta>

	conn, _, err := websocket.DefaultDialer.Dial(
		"ws"+strings.TrimPrefix(servidor.URL, "http")+"/ws", cabecalho)
	if err != nil {
		t.Fatalf("o cliente da própria origem foi recusado: %v", err)
	}
	conn.Close()
}
