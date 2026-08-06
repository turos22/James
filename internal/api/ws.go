package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"james/internal/agente"
	"james/internal/ferramentas"
	"james/internal/permissao"
)

var upgrader = websocket.Upgrader{
	// A rede já é fechada pela VPN/loopback (spec I1/I2) — Origin de
	// browser não é a fronteira de segurança aqui.
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Servidor) ws(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("erro no upgrade WS", "erro", err)
		return
	}
	defer conn.Close()

	cliente := &conexaoCliente{ws: conn, perfil: permissao.ResolverPerfil(r.RemoteAddr)}
	s.Clientes.Registrar(cliente)
	defer s.Clientes.Remover(cliente)

	var sessaoID uuid.UUID
	var historico []agente.Mensagem

	// "mensagem" trata de forma síncrona (loop do agente inteiro, inclusive
	// espera por aprovação humana) — pode levar minutos. "aprovar"/"negar"
	// precisam ser lidos e resolvidos NA HORA, mesmo com uma "mensagem"
	// ainda em voo na mesma conexão: desde que perguntar_ao_claude_code
	// passou a exigir confirmação sempre (D10), o próprio Privilegiado que
	// disparou a chamada pode ser quem precisa aprová-la, pela mesma
	// conexão. Um único goroutine lendo e despachando tudo em série travaria
	// nesse caso — o "aprovar" ficaria atrás do "mensagem" bloqueado na fila
	// de leitura, e nunca seria lido até o timeout de aprovação estourar
	// (confirmado ao vivo: exatos 5 minutos, batendo com o default de 300s).
	//
	// Por isso a leitura vira sua própria goroutine, resolvendo
	// aprovar/negar no ato, e só repassa "mensagem" pro processamento
	// sequencial abaixo — que continua de propósito rodando uma mensagem de
	// cada vez, sem paralelismo, pra não introduzir corrida em historico/
	// sessaoID.
	mensagens := make(chan Envelope)
	go func() {
		defer close(mensagens)
		for {
			var envelope Envelope
			if err := conn.ReadJSON(&envelope); err != nil {
				return
			}
			switch envelope.Tipo {
			case "aprovar":
				s.AprovacoesPendentes.resolver(envelope.ID, true)
			case "negar":
				s.AprovacoesPendentes.resolver(envelope.ID, false)
			case "mensagem":
				mensagens <- envelope
			}
		}
	}()

	for envelope := range mensagens {
		s.tratarMensagem(r.Context(), cliente, envelope, &sessaoID, &historico)
	}
}

func (s *Servidor) tratarMensagem(ctx context.Context, cliente *conexaoCliente, envelope Envelope, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	var dados struct {
		Texto string `json:"texto"`
	}
	if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": "corpo inválido"})
		return
	}

	resp, audioID, err := s.processarTurno(ctx, cliente.Perfil(), sessaoID, historico, dados.Texto,
		func(c agente.ChamadaFerramenta) {
			cliente.Enviar("ferramenta_iniciada", envelope.ID, map[string]string{"nome": c.Nome})
		},
		func(rf agente.ResultadoFerramenta) {
			cliente.Enviar("ferramenta_concluida", envelope.ID, map[string]string{"nome": rf.Nome, "erro": rf.Erro})
		},
	)
	if err != nil {
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": err.Error()})
		return
	}

	cliente.Enviar("token", envelope.ID, map[string]string{"texto": resp.Texto})
	cliente.Enviar("resposta_final", envelope.ID, map[string]string{"texto": resp.Texto})
	if audioID != "" {
		cliente.Enviar("audio_pronto", envelope.ID, map[string]string{"id": audioID})
	}
}

// processarTurno é o núcleo compartilhado entre uma mensagem de cliente WS
// (Task 17) e uma entrada de voz vinda do device (Task 18): garante a
// sessão, monta a janela de memória, roda o loop do agente com os hooks de
// visibilidade, persiste as mensagens novas e gera o áudio da resposta
// (Task 15/4.7 da spec — TTS entregue ao cliente). Devolve a resposta e o
// id do áudio gerado (vazio se não gerou); quem chama decide pra quem os
// eventos intermediários e o audio_pronto vão.
func (s *Servidor) processarTurno(ctx context.Context, perfil permissao.Perfil, sessaoID *uuid.UUID, historico *[]agente.Mensagem, texto string, aoIniciar func(agente.ChamadaFerramenta), aoConcluir func(agente.ResultadoFerramenta)) (agente.Resposta, string, error) {
	if *sessaoID == uuid.Nil {
		sessao, err := s.Sessoes.Criar(ctx, texto)
		if err != nil {
			return agente.Resposta{}, "", err
		}
		*sessaoID = sessao.ID
	}

	tamanhoAntes := len(*historico)
	janela := s.Memoria.Montar(*historico)
	entrada := agente.Mensagem{Papel: agente.PapelUsuario, Texto: texto}

	loop := *s.Loop
	loop.AoIniciarFerramenta = aoIniciar
	loop.AoConcluirFerramenta = aoConcluir

	// Ferramentas que mantêm estado por conversa (perguntar_ao_claude_code)
	// leem o id da sessão daqui — é o único canal que atravessa api → agente
	// → ferramentas.
	ctx = ferramentas.ComSessao(ctx, sessaoID.String())

	resp, mensagens, err := loop.Executar(ctx, s.SistemaPrompt, janela, entrada, perfil, s.criarSolicitarAprovacao(*sessaoID))
	if err != nil {
		return agente.Resposta{}, "", err
	}

	*historico = mensagens
	s.persistirNovasMensagens(ctx, *sessaoID, mensagens, tamanhoAntes)

	audioID := s.gerarAudioDaResposta(ctx, resp.Texto)
	return resp, audioID, nil
}

// gerarAudioDaResposta é melhor-esforço: se o Sintetizador não estiver
// configurado (testes, por exemplo) ou a geração falhar, devolve "" e loga
// — nunca derruba o turno por causa do áudio (mesma filosofia de R2).
func (s *Servidor) gerarAudioDaResposta(ctx context.Context, texto string) string {
	if s.Sintetizador == nil || texto == "" {
		return ""
	}
	id := uuid.NewString()
	if _, err := s.Sintetizador.Gerar(ctx, texto, s.VozTTS, id); err != nil {
		slog.Error("erro ao gerar áudio da resposta", "erro", err)
		return ""
	}
	return id
}

// criarSolicitarAprovacao devolve o callback que o Loop chama quando um
// perfil Restrito pede uma ferramenta perigosa: cria a aprovação pendente,
// avisa os clientes Privilegiado, e espera decisão ou timeout (R4).
func (s *Servidor) criarSolicitarAprovacao(sessaoID uuid.UUID) agente.SolicitarAprovacao {
	return func(ctx context.Context, chamada agente.ChamadaFerramenta) (bool, error) {
		argumentos, _ := json.Marshal(chamada.Args)
		aprovacao, err := s.Aprovacoes.Criar(ctx, sessaoID, chamada.Nome, argumentos, "cliente-restrito")
		if err != nil {
			return false, err
		}

		espera := s.AprovacoesPendentes.aguardar(aprovacao.ID.String())
		s.Clientes.BroadcastAprovacaoRequerida(aprovacao.ID.String(), map[string]any{
			// Duplicado de propósito: o ClienteWS do frontend só repassa o
			// campo "dados" do envelope pros manipuladores registrados via
			// .on() (contrato testado em ws.test.ts, não muda) — o id do
			// envelope nunca chega até o handler sozinho. Sem essa
			// duplicação, App.tsx lê dados.id sempre vazio (Bug 1 do spike
			// de 2026-08-06) e o clique em "Aprovar" nunca resolve a
			// aprovação pendente de verdade.
			"id":         aprovacao.ID.String(),
			"ferramenta": chamada.Nome,
			"argumentos": chamada.Args,
		})

		select {
		case aprovado := <-espera:
			estado := "negada"
			if aprovado {
				estado = "aprovada"
			}
			if err := s.Aprovacoes.Decidir(ctx, aprovacao.ID, estado); err != nil {
				slog.Error("erro ao registrar decisão de aprovação", "erro", err)
			}
			return aprovado, nil
		case <-time.After(s.TimeoutAprovacao):
			s.AprovacoesPendentes.remover(aprovacao.ID.String())
			if err := s.Aprovacoes.Decidir(ctx, aprovacao.ID, "expirada"); err != nil {
				slog.Error("erro ao expirar aprovação", "erro", err)
			}
			return false, nil
		case <-ctx.Done():
			// ctx já foi cancelado (cliente desconectou) — precisa de um
			// contexto novo pra ainda conseguir gravar o estado final; sem
			// isso a aprovação ficaria "pendente" pra sempre no banco.
			s.AprovacoesPendentes.remover(aprovacao.ID.String())
			if err := s.Aprovacoes.Decidir(context.Background(), aprovacao.ID, "expirada"); err != nil {
				slog.Error("erro ao expirar aprovação por desconexão", "erro", err)
			}
			return false, ctx.Err()
		}
	}
}

func (s *Servidor) persistirNovasMensagens(ctx context.Context, sessaoID uuid.UUID, todas []agente.Mensagem, apartirDe int) {
	for _, m := range todas[apartirDe:] {
		var chamadasJSON []byte
		switch {
		case len(m.Chamadas) > 0:
			chamadasJSON, _ = json.Marshal(m.Chamadas)
		case len(m.Resultados) > 0:
			chamadasJSON, _ = json.Marshal(m.Resultados)
		}
		if _, err := s.Mensagens.Salvar(ctx, sessaoID, string(m.Papel), m.Texto, chamadasJSON); err != nil {
			slog.Error("erro ao persistir mensagem", "erro", err)
		}
	}
}
