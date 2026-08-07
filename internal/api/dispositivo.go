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

// decodificarResultadoAcao lê o payload de resultado_acao. A imagem chega em
// base64 porque o envelope é JSON. Base64 quebrada não vira erro do turno:
// o texto do resultado ainda é útil sozinho, e derrubar tudo por causa de um
// PNG mal formado seria pior que seguir sem ele.
func decodificarResultadoAcao(bruto []byte) (saida, erro string, imagem []byte) {
	var dados struct {
		Saida     string `json:"saida"`
		Erro      string `json:"erro"`
		ImagemB64 string `json:"imagem_b64"`
	}
	if err := json.Unmarshal(bruto, &dados); err != nil {
		return "", "", nil
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
	defer s.DispositivoHub.Desregistrar()

	// Conversa por voz do device é sempre tratada como perfil Privilegiado:
	// é a mesma máquina física, e as respostas vão pros clientes
	// Privilegiado conectados (não existe "cliente" WS pra voz em si).
	var sessaoID uuid.UUID
	var historico []agente.Mensagem

	for {
		var envelope Envelope
		if err := conn.ReadJSON(&envelope); err != nil {
			return
		}

		switch envelope.Tipo {
		case "resultado_acao":
			saida, erroDoDevice, imagem := decodificarResultadoAcao(envelope.Dados)
			s.DispositivoHub.ResolverResultado(envelope.ID, saida, erroDoDevice, imagem)

		case "entrada_voz":
			var dados struct {
				Texto string `json:"texto"`
			}
			if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
				continue
			}
			s.tratarEntradaDeVoz(r.Context(), envelope.ID, dados.Texto, &sessaoID, &historico)

		case "registro", "heartbeat":
			// Fase 1 só precisa aceitar sem derrubar a conexão — telemetria
			// de status_dispositivo fica como polimento futuro (não bloqueia
			// nenhum critério de aceitação).
		}
	}
}

func (s *Servidor) tratarEntradaDeVoz(ctx context.Context, id, texto string, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	resp, audioID, err := s.processarTurno(ctx, permissao.Privilegiado, sessaoID, historico, texto,
		func(c agente.ChamadaFerramenta) {
			s.Clientes.BroadcastParaPrivilegiados("ferramenta_iniciada", id, map[string]string{"nome": c.Nome})
		},
		func(rf agente.ResultadoFerramenta) {
			s.Clientes.BroadcastParaPrivilegiados("ferramenta_concluida", id, map[string]string{"nome": rf.Nome, "erro": rf.Erro})
		},
	)
	if err != nil {
		s.Clientes.BroadcastParaPrivilegiados("erro", id, map[string]string{"mensagem": err.Error()})
		return
	}
	s.Clientes.BroadcastParaPrivilegiados("token", id, map[string]string{"texto": resp.Texto})
	s.Clientes.BroadcastParaPrivilegiados("resposta_final", id, map[string]string{"texto": resp.Texto})
	if audioID != "" {
		s.Clientes.BroadcastParaPrivilegiados("audio_pronto", id, map[string]string{"id": audioID})
	}
}
