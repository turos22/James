package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

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
	return c.ws.WriteMessage(websocket.TextMessage, envelope)
}

func (s *Servidor) wsDispositivo(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("erro no upgrade WS do device", "erro", err)
		return
	}
	defer conn.Close()

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
			var dados struct {
				Saida string `json:"saida"`
				Erro  string `json:"erro"`
			}
			if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
				continue
			}
			s.DispositivoHub.ResolverResultado(envelope.ID, dados.Saida, dados.Erro)

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
