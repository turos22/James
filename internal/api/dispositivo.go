package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/permissao"
)

// conexaoDispositivo implementa dispositivo.Conexao sobre o WS real —
// é o que internal/dispositivo.Hub usa pra mandar acao_tela.
type conexaoDispositivo struct {
	ws *websocket.Conn
	mu sync.Mutex
}

func (c *conexaoDispositivo) Enviar(envelope []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// Sem deadline, um socket travado prende WriteMessage indefinidamente —
	// e como dispositivo.Hub.EnviarAcao chama Enviar antes do próprio select
	// de timeout/cancelamento (hub.go), esse travamento também escaparia do
	// timeout do turno que está esperando a ação. Mesmo bug que
	// conexaoCliente.Enviar já teve corrigido na Task 17 (clientes.go).
	_ = c.ws.SetWriteDeadline(time.Now().Add(tempoLimiteEscrita))
	return c.ws.WriteMessage(websocket.TextMessage, envelope)
}

// tamanhoMaximoMensagemDevice é folgado de propósito: um PNG de 1920x1080 em
// base64 fica na casa de centenas de KB, e o teto precisa caber vários com
// sobra. O ponto não é apertar — é deixar de aceitar qualquer tamanho, que é
// o comportamento padrão do gorilla quando SetReadLimit nunca é chamado.
const tamanhoMaximoMensagemDevice = 32 << 20 // 32 MB

// tempoSemNoticiaDoDevice é quanto a leitura espera por QUALQUER frame do
// device antes de declarar a conexão morta. Três heartbeats do device (15s
// cada, ver conexao.py) com folga: o ponto é derrubar socket zumbi, não
// disputar com uma oscilação de rede.
const tempoSemNoticiaDoDevice = 50 * time.Second

// decodificarResultadoAcao lê o payload de resultado_acao. A imagem chega em
// base64 porque o envelope é JSON. Base64 quebrada não vira erro do turno:
// o texto do resultado ainda é útil sozinho, e derrubar tudo por causa de um
// PNG mal formado seria pior que seguir sem ele. Já o JSON de topo quebrado é
// outra história: se "saida" e "erro" não decodificam, nada no payload é
// confiável, e devolver sucesso vazio em silêncio esconderia uma falha real
// do device — por isso vira erro explícito, não string vazia.
func decodificarResultadoAcao(bruto []byte) (saida, erro string, imagem []byte) {
	var dados struct {
		Saida     string `json:"saida"`
		Erro      string `json:"erro"`
		ImagemB64 string `json:"imagem_b64"`
	}
	if err := json.Unmarshal(bruto, &dados); err != nil {
		slog.Warn("resultado_acao com payload ilegível do device, tratando como erro", "erro", err)
		return "", "resultado_acao ilegível do device", nil
	}
	if dados.ImagemB64 != "" {
		decodificada, err := base64.StdEncoding.DecodeString(dados.ImagemB64)
		if err != nil {
			slog.Warn("imagem_b64 inválida no resultado_acao, seguindo sem imagem", "erro", err)
		} else {
			imagem = decodificada
		}
	}
	return dados.Saida, dados.Erro, imagem
}

func (s *Servidor) wsDispositivo(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("erro no upgrade WS do device", "erro", err)
		return
	}
	defer conn.Close()
	conn.SetReadLimit(tamanhoMaximoMensagemDevice)

	conexao := &conexaoDispositivo{ws: conn}
	s.DispositivoHub.Registrar(conexao)
	defer s.DispositivoHub.Desregistrar(conexao)

	// Conversa por voz do device é sempre tratada como perfil Privilegiado:
	// é a mesma máquina física, e as respostas vão pros clientes
	// Privilegiado conectados (não existe "cliente" WS pra voz em si).
	var sessaoID uuid.UUID
	var historico []agente.Mensagem

	// Mesmo desenho de ws(), e pelo mesmo motivo — só que aqui o travamento
	// era pior. tratarEntradaDeVoz roda o turno INTEIRO de forma síncrona e
	// pode levar minutos, e o resultado_acao que uma ferramenta de tela espera
	// chega POR ESTA MESMA CONEXÃO. Com um goroutine só lendo e processando em
	// série: o Hub manda acao_tela, o device responde na hora, e ninguém lê a
	// resposta até o turno terminar — o que só acontece depois de a própria
	// ação expirar por timeout. Toda ver_tela e todo manipular_interface
	// pedido POR VOZ falhava assim, com o device saudável.
	// Coberto por TestEntradaDeVozConsegueUsarFerramentaDeTela.
	vozes := make(chan Envelope, 1)
	go func() {
		defer close(vozes)
		for {
			// Renovado a cada leitura bem-sucedida: o device manda heartbeat a
			// cada 15s (conexao.py), então três batidas perdidas significam
			// socket morto. Sem prazo nenhum, um TCP meio-aberto (sleep/wake,
			// Wi-Fi caindo, VPN oscilando) ficava registrado indefinidamente e
			// o Hub despachava ação pra um socket que ninguém lê do outro lado.
			_ = conn.SetReadDeadline(time.Now().Add(tempoSemNoticiaDoDevice))

			var envelope Envelope
			if err := conn.ReadJSON(&envelope); err != nil {
				return
			}

			switch envelope.Tipo {
			case "resultado_acao":
				// Resolvido aqui mesmo, dentro da leitura: ResolverResultado é
				// não-bloqueante por contrato (ver hub.go), então não reintroduz
				// o travamento que este desenho existe pra evitar.
				saida, erroDoDevice, imagem := decodificarResultadoAcao(envelope.Dados)
				s.DispositivoHub.ResolverResultado(envelope.ID, saida, erroDoDevice, imagem)

			case "entrada_voz":
				// Envio não-bloqueante, e é o ponto todo: se a leitura travar
				// esperando o laço abaixo aceitar a fala, o resultado_acao do
				// turno em andamento volta a não ser lido — o mesmo deadlock,
				// um nível acima. Dois turnos falados também não poderiam rodar
				// em paralelo de qualquer forma: compartilham sessaoID e
				// historico.
				select {
				case vozes <- envelope:
				default:
					slog.Warn("turno de voz já em andamento, fala descartada")
					s.Clientes.BroadcastParaPrivilegiados("erro", envelope.ID,
						payloadErro{Mensagem: "ainda estou no pedido anterior — repita quando eu terminar"})
				}

			case "registro", "heartbeat":
				// Fase 1 só precisa aceitar sem derrubar a conexão — telemetria
				// de status_dispositivo fica como polimento futuro (não bloqueia
				// nenhum critério de aceitação).
			}
		}
	}()

	for envelope := range vozes {
		var dados struct {
			Texto string `json:"texto"`
		}
		if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
			continue
		}
		s.tratarEntradaDeVoz(r.Context(), conexao, envelope.ID, dados.Texto, &sessaoID, &historico)
	}
}

func (s *Servidor) tratarEntradaDeVoz(ctx context.Context, conexao *conexaoDispositivo, id, texto string, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	emitir := func(tipo string, dados any) { s.Clientes.BroadcastParaPrivilegiados(tipo, id, dados) }

	// Antes de qualquer processamento: o turno pode levar minutos (ferramenta
	// lenta, espera por aprovação humana) e a tela precisa mostrar no ato o
	// que foi entendido. Sem isso a resposta do James aparecia sozinha, sem
	// balão do que foi falado — o caminho de texto não tem esse problema
	// porque o próprio cliente que digitou desenha o balão localmente.
	emitir("transcricao", payloadTranscricao{Texto: texto})

	aoIniciar, aoConcluir := hooksDeFerramenta(emitir)
	falar := novoFalador(conexao, id)
	// Quando uma ferramenta começa, fala o que o modelo acabou de dizer ("vou
	// olhar a tela") na hora: a próxima frase só existe depois dela.
	aoIniciarFalando := func(c agente.ChamadaFerramenta) {
		falar.descarregar()
		aoIniciar(c)
	}
	resp, _, err := s.processarTurno(ctx, agente.ModoVoz, permissao.Privilegiado, sessaoID, historico, texto, aoIniciarFalando, aoConcluir, falar.delta)
	if err != nil {
		mensagem := mensagemDeErroDoTurno(err, "voz")
		emitir("erro", payloadErro{Mensagem: mensagem})
		falar.erro(mensagem)
		return
	}
	falar.descarregar()
	falar.fim()
	// Mesmo ponto de atenção de tratarMensagem (ws.go): sessaoID só é lido
	// depois de processarTurno, nunca antes.
	emitirDesfechoDoTurno(emitir, resp, *sessaoID, "")
}
