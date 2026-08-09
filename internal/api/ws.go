package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"

	"james/internal/agente"
	"james/internal/armazenamento/postgres"
	"james/internal/ferramentas"
	"james/internal/permissao"
)

var upgrader = websocket.Upgrader{
	// A rede já é fechada pela VPN/loopback (spec I1/I2) — Origin de
	// browser não é a fronteira de segurança aqui.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// limiteMensagensSessao é o teto de linhas trazidas do Postgres por sessão,
// tanto pro REST listar a conversa (ListarPorSessao, do começo) quanto pra
// retomada via WS reconstruir o histórico (ListarRecentesPorSessao, do
// fim) — mesmo número, semânticas opostas.
const limiteMensagensSessao = 200

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
			case "retomar":
				mensagens <- envelope
			}
		}
	}()

	for envelope := range mensagens {
		switch envelope.Tipo {
		case "retomar":
			s.tratarRetomar(r.Context(), cliente, envelope, &sessaoID, &historico)
		default:
			s.tratarMensagem(r.Context(), cliente, envelope, &sessaoID, &historico)
		}
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

	// sessaoID só é lido AQUI, depois de processarTurno: numa conversa nova
	// o ponteiro ainda aponta pra uuid.Nil antes da chamada (a sessão nasce
	// dentro dela). Ler antes mandaria sessao_id vazio pro cliente.
	cliente.Enviar("resposta_final", envelope.ID, map[string]string{
		"texto":     resp.Texto,
		"sessao_id": sessaoID.String(),
	})
	if audioID != "" {
		cliente.Enviar("audio_pronto", envelope.ID, map[string]string{"id": audioID})
	}
}

// tratarRetomar roda no laço sequencial, junto com "mensagem", porque mexe
// em sessaoID e historico — o mesmo motivo que mantém "mensagem" fora da
// goroutine de leitura.
func (s *Servidor) tratarRetomar(ctx context.Context, cliente *conexaoCliente, envelope Envelope, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	var dados struct {
		SessaoID string `json:"sessao_id"`
	}
	if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": "corpo inválido"})
		return
	}

	sessao, err := uuid.Parse(dados.SessaoID)
	if err != nil {
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": "id de sessão inválido"})
		return
	}

	if _, err := s.Sessoes.BuscarPorID(ctx, sessao); err != nil {
		// Sessão inexistente precisa ser recusa explícita. Aceitar e
		// devolver "sessao_retomada" com zero mensagem deixa a conexão com
		// sessaoID adotado que não existe em `sessoes` — o turno seguinte
		// roda normal, mas persistirNovasMensagens estoura a foreign key e
		// só loga, e a conversa passa a não gravar nada em silêncio.
		mensagemCliente := "não consegui verificar essa conversa"
		if errors.Is(err, pgx.ErrNoRows) {
			// Rotina: cliente mandou um id que não existe (apagada, digitada
			// errado). Não é falha do servidor.
			mensagemCliente = "essa conversa não existe mais"
			slog.Warn("retomada pedida para sessão inexistente", "erro", err, "sessao", sessao)
		} else {
			// Qualquer outro erro é falha real de banco — merece Error, não
			// Warn, pra não se perder no ruído de tentativas de retomar
			// sessão apagada.
			slog.Error("erro ao verificar sessão para retomada", "erro", err, "sessao", sessao)
		}
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": mensagemCliente})
		return
	}

	// ListarRecentesPorSessao, não ListarPorSessao: a retomada precisa da
	// CAUDA da conversa (continuar de onde parou), não do começo — que é o
	// que ListarPorSessao devolve quando a sessão estoura o limite (usado
	// pelo REST, que exibe a conversa desde o início). Usar o método errado
	// aqui faria memoria.Janela.Montar montar a cauda de um pedaço antigo,
	// e o turno seguinte rodaria com contexto desatualizado sem erro nenhum.
	linhas, err := s.Mensagens.ListarRecentesPorSessao(ctx, sessao, limiteMensagensSessao)
	if err != nil {
		// Erro de banco fica no log local: pode carregar host, porta ou
		// detalhe de conexão, e esta mensagem é renderizada numa tela que
		// o celular também abre pela VPN.
		slog.Error("erro ao carregar mensagens para retomada", "erro", err, "sessao", sessao)
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": "não consegui carregar a conversa"})
		return
	}

	if err := aplicarRetomada(sessao, linhas, sessaoID, historico); err != nil {
		// Mesmo critério dos branches acima: log completo local, mensagem
		// genérica pro cliente. Este era o único caminho de erro de
		// tratarRetomar sem slog — se a coluna `chamadas` corromper em
		// produção, sem isso a única evidência seria o que o usuário
		// copiasse da tela.
		slog.Error("erro ao reconstruir histórico para retomada", "erro", err, "sessao", sessao)
		cliente.Enviar("erro", envelope.ID, map[string]string{"mensagem": "não consegui reconstruir a conversa"})
		return
	}

	cliente.Enviar("sessao_retomada", envelope.ID, map[string]any{
		"sessao_id": sessao.String(),
		"mensagens": len(linhas),
	})
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

	// A memória durável entra pelo system prompt, não pela janela: nota não é
	// conversa e não pode competir com recência por orçamento. Ler a cada
	// turno (e não no boot) é o que faz a nota recém-escrita valer já no turno
	// seguinte.
	sistemaPrompt := s.SistemaPrompt
	if s.Notas != nil {
		if bloco, err := s.Notas.Renderizar(); err != nil {
			// Memória é melhor-esforço na leitura, igual ao áudio: vault
			// quebrado não pode derrubar a conversa.
			slog.Warn("erro ao ler memória durável, seguindo sem ela", "erro", err)
		} else if bloco != "" {
			sistemaPrompt = sistemaPrompt + "\n\n" + bloco
		}
	}

	loop := *s.Loop
	loop.AoIniciarFerramenta = aoIniciar
	loop.AoConcluirFerramenta = aoConcluir

	// Ferramentas que mantêm estado por conversa (perguntar_ao_claude_code)
	// leem o id da sessão daqui — é o único canal que atravessa api → agente
	// → ferramentas.
	ctx = ferramentas.ComSessao(ctx, sessaoID.String())

	resp, mensagens, err := loop.Executar(ctx, sistemaPrompt, janela, entrada, perfil, s.criarSolicitarAprovacao(*sessaoID))
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
		argumentosJSON, _ := json.Marshal(chamada.Args)
		aprovacao, err := s.Aprovacoes.Criar(ctx, sessaoID, chamada.Nome, argumentosJSON, "cliente-restrito")
		if err != nil {
			return false, err
		}

		espera := s.AprovacoesPendentes.aguardar(aprovacao.ID.String())

		// Map nil em Go vira `null` no JSON, não `{}`. O gate de aprovação
		// roda antes de qualquer validação de argumento, então uma chamada
		// sem parâmetros chegaria ao front como argumentos: null — e o card
		// de aprovação itera esse valor. "Sem argumentos" é objeto vazio.
		argumentos := chamada.Args
		if argumentos == nil {
			argumentos = map[string]any{}
		}

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
			"argumentos": argumentos,
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

// reconstruirHistorico faz o caminho de volta de persistirNovasMensagens:
// converte linhas da tabela `mensagens` em []agente.Mensagem para retomar
// uma sessão anterior.
//
// A coluna `chamadas` é polimórfica de propósito — persistirNovasMensagens
// grava ali OU m.Chamadas OU m.Resultados, o que estiver preenchido. Na
// volta, a desambiguação é pelo papel, nunca por tentativa de decodificar
// nos dois formatos: ChamadaFerramenta e ResultadoFerramenta compartilham
// ID e Nome, então uma tentativa de decodificar teria sucesso nos dois e
// escolheria o errado em silêncio.
//
// Imagem nunca é reconstruída — o Postgres nunca a persistiu (ver
// ResultadoFerramenta.Imagem); retomar uma sessão com ver_tela recupera o
// texto e a saída, não o frame.
func reconstruirHistorico(linhas []postgres.Mensagem) ([]agente.Mensagem, error) {
	historico := make([]agente.Mensagem, 0, len(linhas))

	for _, linha := range linhas {
		mensagem := agente.Mensagem{
			Papel: agente.Papel(linha.Papel),
			Texto: linha.Conteudo,
		}

		if len(linha.Chamadas) > 0 {
			switch mensagem.Papel {
			case agente.PapelFerramenta:
				if err := json.Unmarshal(linha.Chamadas, &mensagem.Resultados); err != nil {
					return nil, fmt.Errorf("mensagem id=%d (papel %q): resultados ilegíveis: %w", linha.ID, linha.Papel, err)
				}
			case agente.PapelModelo:
				if err := json.Unmarshal(linha.Chamadas, &mensagem.Chamadas); err != nil {
					return nil, fmt.Errorf("mensagem id=%d (papel %q): chamadas ilegíveis: %w", linha.ID, linha.Papel, err)
				}
			default:
				return nil, fmt.Errorf("mensagem id=%d: papel %q inesperado com a coluna chamadas preenchida", linha.ID, linha.Papel)
			}
		}

		historico = append(historico, mensagem)
	}

	return historico, nil
}

// aplicarRetomada adota uma sessão existente na conexão. Só escreve nos
// ponteiros depois de a reconstrução ter dado certo: falhar no meio deixaria
// a conexão com sessão adotada e histórico parcial, que é pior que não
// retomar — o modelo receberia um contexto truncado sem ninguém saber.
func aplicarRetomada(sessao uuid.UUID, linhas []postgres.Mensagem, sessaoID *uuid.UUID, historico *[]agente.Mensagem) error {
	reconstruido, err := reconstruirHistorico(linhas)
	if err != nil {
		return err
	}
	*sessaoID = sessao
	*historico = reconstruido
	return nil
}
