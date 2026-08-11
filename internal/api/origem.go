package api

import (
	"log/slog"
	"net/http"
	"strings"
)

// origemPermitida decide se um handshake de WebSocket vindo de navegador pode
// subir. É a checagem que `CheckOrigin: return true` desligava.
//
// POR QUE ISTO IMPORTA. WebSocket não passa por CORS: uma página em
// qualquer site abre `new WebSocket("ws://127.0.0.1:8080/ws")` e o navegador
// completa o handshake sem pedir licença a ninguém. Do lado do jamesd essa
// conexão chega do loopback, e ResolverPerfil (internal/permissao) devolve
// Privilegiado pra loopback — ou seja, a página passava a falar com o James
// como se fosse o Arthur, e o James executa PowerShell. O gate de aprovação
// era a única coisa entre um site qualquer e a máquina.
//
// O que salva aqui é uma propriedade do navegador, não uma esperteza nossa:
// o header Origin é preenchido pelo próprio navegador e a página NÃO tem como
// forjá-lo. Comparar Origin com a origem do próprio servidor fecha o vetor.
//
// O QUE ISTO NÃO RESOLVE, e é honesto dizer: processo local rodando na mesma
// conta do usuário. Ele omite o header, forja o que quiser, e lê qualquer
// token que a gente inventasse pra guardar em disco — a conta do SO é a
// fronteira, e ela já foi atravessada nesse cenário. Um token de sessão
// somado a esta checagem seria teatro: não acrescenta nada contra o
// navegador (Origin já basta) nem contra o processo local (que lê o token).
func origemPermitida(r *http.Request) bool {
	origem := r.Header.Get("Origin")
	if origem == "" {
		// Cliente que não é navegador: o james-device (websockets do Python),
		// curl, os testes. Recusar aqui derrubaria a ponte de tela sem
		// proteger nada — ver o parágrafo acima.
		return true
	}

	esperada := "http://" + r.Host
	if r.TLS != nil {
		esperada = "https://" + r.Host
	}

	// Esquema e host são case-insensitive por especificação de URL; a
	// comparação byte a byte reprovaria "HTTP://" de um cliente legítimo.
	// Não há normalização de porta implícita aqui de propósito: os dois
	// atendimentos do jamesd usam porta explícita (8080 e 8443), então
	// "mesma origem" é sempre string com porta dos dois lados.
	if strings.EqualFold(origem, esperada) {
		return true
	}

	slog.Warn("handshake de WebSocket recusado por origem estranha",
		"origem", origem, "esperada", esperada, "remoto", r.RemoteAddr)
	return false
}
