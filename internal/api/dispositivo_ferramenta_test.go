package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"james/internal/dispositivo"
	"james/internal/ferramentas"
)

// TestEntradaDeVozConsegueUsarFerramentaDeTela trava o deadlock do WS do
// device: wsDispositivo chama tratarEntradaDeVoz DENTRO do próprio laço de
// leitura da conexão. Enquanto o turno falado corre — e ele pode levar
// minutos — ninguém lê o resultado_acao que o device manda de volta, então
// toda ver_tela/manipular_interface pedida POR VOZ espera o timeout do Hub e
// falha, com o device vivo e tendo respondido na hora.
//
// É o mesmo bug que ws() já teve consertado pro caminho de texto (ver
// TestWSMesmaConexaoConsegueAprovarSuaPropriaAcao): laço de leitura que
// processa de forma síncrona não consegue ler o frame seguinte da mesma
// conexão. A correção lá foi separar leitura de processamento; aqui nunca foi
// feita.
func TestEntradaDeVozConsegueUsarFerramentaDeTela(t *testing.T) {
	hub := dispositivo.NovoHub()
	// Encurta a espera: o ponto do teste é passar ou falhar rápido, não medir
	// o teto real de produção (10s global, 30s pra ver_tela).
	hub.Timeout = 2 * time.Second

	s := servidorComFerramentaFalsa(t, ferramentas.NovoVerTela(hub), "ver_tela")
	s.DispositivoHub = hub
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	urlBase := "ws" + strings.TrimPrefix(servidor.URL, "http")

	connCliente, _, err := websocket.DefaultDialer.Dial(urlBase+"/ws", nil)
	if err != nil {
		t.Fatalf("erro ao conectar cliente: %v", err)
	}
	defer connCliente.Close()

	connDevice, _, err := websocket.DefaultDialer.Dial(urlBase+"/ws/dispositivo", nil)
	if err != nil {
		t.Fatalf("erro ao conectar device: %v", err)
	}
	defer connDevice.Close()

	if err := connDevice.WriteJSON(Envelope{
		Tipo: "entrada_voz", ID: "v1",
		Dados: codificarDados(map[string]string{"texto": "olha a tela"}),
	}); err != nil {
		t.Fatalf("erro ao enviar entrada_voz: %v", err)
	}

	// Device de mentira: responde acao_tela imediatamente. Só ele escreve na
	// conexão a partir daqui (gorilla não aceita escritor concorrente), por
	// isso a goroutine sobe DEPOIS do entrada_voz acima.
	go func() {
		for {
			var recebido Envelope
			if err := connDevice.ReadJSON(&recebido); err != nil {
				return
			}
			if recebido.Tipo != "acao_tela" {
				continue
			}
			_ = connDevice.WriteJSON(Envelope{
				Tipo: "resultado_acao", ID: recebido.ID,
				Dados: codificarDados(map[string]string{
					"saida": "superficie: DISPLAY1 (primaria) 1920x1080 escala 100%",
				}),
			})
		}
	}()

	connCliente.SetReadDeadline(time.Now().Add(15 * time.Second))
	for i := 0; i < 10; i++ {
		var recebido Envelope
		if err := connCliente.ReadJSON(&recebido); err != nil {
			t.Fatalf("erro ao ler no cliente: %v", err)
		}
		if recebido.Tipo != "ferramenta_concluida" {
			continue
		}
		var payload payloadFerramentaConcluida
		if err := json.Unmarshal(recebido.Dados, &payload); err != nil {
			t.Fatalf("erro ao decodificar ferramenta_concluida: %v", err)
		}
		if payload.Erro != "" {
			t.Fatalf("ver_tela por voz falhou com %q — o device respondeu na hora; quem não leu a resposta foi o jamesd", payload.Erro)
		}
		if payload.Saida == "" {
			t.Fatal("ferramenta_concluida veio sem saida — ver_tela por voz não entregou a leitura da tela")
		}
		return
	}
	t.Fatal("ferramenta_concluida nunca chegou ao cliente privilegiado")
}

// TestReconexaoDoDeviceNaoDesregistraAConexaoNova trava o segundo modo de
// falha da ponte: wsDispositivo faz `defer s.DispositivoHub.Desregistrar()`
// incondicional, e Desregistrar zera a conexão atual seja ela qual for.
//
// O device reconecta sozinho a cada 2s (conexao.py). Se a conexão velha
// morrer DEPOIS de a nova já ter se registrado — o caso normal quando o TCP
// fica meio-aberto (sleep/wake, queda de Wi-Fi, VPN oscilando), porque o lado
// Go não tem read deadline nem ping/pong e só percebe a morte muito depois —
// o defer da velha apaga o registro da nova. A partir daí o Hub responde
// "máquina física indisponível" pra sempre, com um device vivo e conectado do
// outro lado, e nada reconecta: do ponto de vista do Python, a conexão está
// perfeita.
func TestReconexaoDoDeviceNaoDesregistraAConexaoNova(t *testing.T) {
	hub := dispositivo.NovoHub()
	hub.Timeout = 2 * time.Second

	s := servidorComFerramentaFalsa(t, ferramentas.NovoVerTela(hub), "ver_tela")
	s.DispositivoHub = hub
	servidor := httptest.NewServer(s.Rotas())
	defer servidor.Close()

	urlWS := "ws" + strings.TrimPrefix(servidor.URL, "http") + "/ws/dispositivo"

	velha, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar o device velho: %v", err)
	}

	nova, _, err := websocket.DefaultDialer.Dial(urlWS, nil)
	if err != nil {
		t.Fatalf("erro ao conectar o device novo: %v", err)
	}
	defer nova.Close()

	// A nova precisa ter chegado ao Registrar antes de a velha cair — é
	// exatamente a ordem que o bug exige, e a que acontece na prática quando o
	// Python já reconectou e só depois o socket velho é declarado morto.
	esperarRegistro(t, hub)

	velha.Close()
	// Dá tempo do handler da conexão velha notar o EOF e rodar o defer.
	time.Sleep(300 * time.Millisecond)

	pronto := make(chan struct{})
	go func() {
		defer close(pronto)
		for {
			var recebido Envelope
			if err := nova.ReadJSON(&recebido); err != nil {
				return
			}
			if recebido.Tipo != "acao_tela" {
				continue
			}
			_ = nova.WriteJSON(Envelope{
				Tipo: "resultado_acao", ID: recebido.ID,
				Dados: codificarDados(map[string]string{"saida": "superficie: DISPLAY1 1920x1080 escala 100%"}),
			})
		}
	}()

	if _, err := hub.EnviarAcao(t.Context(), dispositivo.AcaoTela{Acao: "ver_tela"}); err != nil {
		t.Fatalf("EnviarAcao falhou com %v — o device novo está conectado e vivo; quem o desregistrou foi o defer da conexão velha", err)
	}
}

// esperarRegistro espera o Hub ter alguma conexão registrada, sondando com
// uma ação que falha rápido. Sem isso o teste dependeria de um sleep chutado
// pro handshake do WS chegar ao Registrar.
func esperarRegistro(t *testing.T, hub *dispositivo.Hub) {
	t.Helper()
	prazo := time.Now().Add(2 * time.Second)
	for time.Now().Before(prazo) {
		ctx, cancelar := context.WithTimeout(t.Context(), 20*time.Millisecond)
		_, err := hub.EnviarAcao(ctx, dispositivo.AcaoTela{Acao: "sonda"})
		cancelar()
		if !errors.Is(err, dispositivo.ErroDispositivoAusente) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("o Hub nunca registrou nenhuma conexão de device")
}
