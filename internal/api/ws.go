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
	// Já foi `return true`, com a justificativa de que a rede fechada pela
	// VPN/loopback (spec I1/I2) tornava a origem irrelevante. A justificativa
	// estava errada: WebSocket não passa por CORS, então "rede fechada" não
	// impede uma página aberta no navegador DESTA máquina de abrir uma conexão
	// pro loopback — e loopback é Privilegiado. Ver origem.go.
	CheckOrigin: origemPermitida,
}

// limiteMensagensSessao é o teto de linhas trazidas do Postgres por sessão,
// tanto pro REST listar a conversa (ListarPorSessao, do começo) quanto pra
// retomada via WS reconstruir o histórico (ListarRecentesPorSessao, do
// fim) — mesmo número, semânticas opostas.
const limiteMensagensSessao = 200

// tamanhoMaximoMensagemCliente é o teto de uma mensagem vinda de um cliente
// WS (o shell desktop ou o celular pela VPN).
const tamanhoMaximoMensagemCliente = 1 << 20 // 1 MB

func (s *Servidor) ws(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("erro no upgrade WS", "erro", err)
		return
	}
	defer conn.Close()
	// Sem isto o gorilla aceita mensagem de qualquer tamanho, e esta rota
	// também atende o celular pela VPN. O cliente só manda texto de conversa e
	// aprovar/negar — 1 MB é folga de ordens de grandeza sobre o caso real.
	// O WS do device tem teto próprio, bem maior, porque por lá passa PNG.
	conn.SetReadLimit(tamanhoMaximoMensagemCliente)

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
			case "nova_sessao":
				mensagens <- envelope
			}
		}
	}()

	for envelope := range mensagens {
		switch envelope.Tipo {
		case "retomar":
			s.tratarRetomar(r.Context(), cliente, envelope, &sessaoID, &historico)
		case "nova_sessao":
			s.tratarNovaSessao(cliente, envelope, &sessaoID, &historico)
		default:
			s.tratarMensagem(r.Context(), cliente, envelope, &sessaoID, &historico)
		}
	}
}

func (s *Servidor) tratarMensagem(ctx context.Context, cliente *conexaoCliente, envelope Envelope, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	var dados struct {
		Texto string `json:"texto"`
	}
	emitir := func(tipo string, dados any) { cliente.Enviar(tipo, envelope.ID, dados) }

	if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
		emitir("erro", payloadErro{Mensagem: "corpo inválido"})
		return
	}

	aoIniciar, aoConcluir := hooksDeFerramenta(emitir)
	resp, audioID, err := s.processarTurno(ctx, agente.ModoTexto, cliente.Perfil(), sessaoID, historico, dados.Texto, aoIniciar, aoConcluir, nil)
	if err != nil {
		emitir("erro", payloadErro{Mensagem: mensagemDeErroDoTurno(err, "texto")})
		return
	}

	// sessaoID só é lido AQUI, depois de processarTurno: numa conversa nova
	// o ponteiro ainda aponta pra uuid.Nil antes da chamada (a sessão nasce
	// dentro dela). Ler antes mandaria sessao_id vazio pro cliente.
	emitirDesfechoDoTurno(emitir, resp, *sessaoID, audioID)
}

// tratarRetomar roda no laço sequencial, junto com "mensagem", porque mexe
// em sessaoID e historico — o mesmo motivo que mantém "mensagem" fora da
// goroutine de leitura.
func (s *Servidor) tratarRetomar(ctx context.Context, cliente *conexaoCliente, envelope Envelope, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	var dados struct {
		SessaoID string `json:"sessao_id"`
	}
	if err := json.Unmarshal(envelope.Dados, &dados); err != nil {
		cliente.Enviar("erro", envelope.ID, payloadErro{Mensagem: "corpo inválido"})
		return
	}

	sessao, err := uuid.Parse(dados.SessaoID)
	if err != nil {
		cliente.Enviar("erro", envelope.ID, payloadErro{Mensagem: "id de sessão inválido"})
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
		cliente.Enviar("erro", envelope.ID, payloadErro{Mensagem: mensagemCliente})
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
		cliente.Enviar("erro", envelope.ID, payloadErro{Mensagem: "não consegui carregar a conversa"})
		return
	}

	if err := aplicarRetomada(sessao, linhas, sessaoID, historico); err != nil {
		// Mesmo critério dos branches acima: log completo local, mensagem
		// genérica pro cliente. Este era o único caminho de erro de
		// tratarRetomar sem slog — se a coluna `chamadas` corromper em
		// produção, sem isso a única evidência seria o que o usuário
		// copiasse da tela.
		slog.Error("erro ao reconstruir histórico para retomada", "erro", err, "sessao", sessao)
		cliente.Enviar("erro", envelope.ID, payloadErro{Mensagem: "não consegui reconstruir a conversa"})
		return
	}

	cliente.Enviar("sessao_retomada", envelope.ID, payloadSessaoRetomada{
		SessaoID:  sessao.String(),
		Mensagens: mensagensExibiveis(*historico),
	})
}

// tratarNovaSessao esquece a conversa corrente NO SERVIDOR. Roda no laço
// sequencial, junto com "mensagem" e "retomar", porque mexe em sessaoID e
// historico — o mesmo motivo que mantém "mensagem" fora da goroutine de
// leitura.
//
// Zerar só no cliente não bastaria: sessaoID e historico são variáveis
// locais deste handler, então o turno seguinte continuaria gravando na
// sessão velha, sem erro nenhum. É a mesma classe de bug que a auditoria da
// Fase 6 achou no retomar.
//
// Responde com sessao_retomada vazio em vez de inventar um evento novo: o
// cliente já sabe tratar esse evento, e a semântica é a mesma — "esta é a
// conversa em que você está agora". Nenhuma sessão é criada aqui; ela nasce
// no Postgres no primeiro turno, como processarTurno já faz quando sessaoID
// é uuid.Nil.
func (s *Servidor) tratarNovaSessao(cliente *conexaoCliente, envelope Envelope, sessaoID *uuid.UUID, historico *[]agente.Mensagem) {
	*sessaoID = uuid.Nil
	*historico = nil
	cliente.Enviar("sessao_retomada", envelope.ID, payloadSessaoRetomada{
		SessaoID: "",
		// Slice vazio e não nil: map/slice nil viram `null` no JSON, e o
		// tipo TypeScript promete array. Já mordeu neste projeto, no card
		// de aprovação com argumentos nil.
		Mensagens: []mensagemDoHistorico{},
	})
}

// processarTurno é o núcleo compartilhado entre uma mensagem de cliente WS
// (Task 17) e uma entrada de voz vinda do device (Task 18): garante a
// sessão, monta a janela de memória, roda o loop do agente com os hooks de
// visibilidade, persiste as mensagens novas e gera o áudio da resposta
// (Task 15/4.7 da spec — TTS entregue ao cliente). Devolve a resposta e o
// id do áudio gerado (vazio se não gerou); quem chama decide pra quem os
// eventos intermediários e o audio_pronto vão.
func (s *Servidor) processarTurno(ctx context.Context, modo agente.Modo, perfil permissao.Perfil, sessaoID *uuid.UUID, historico *[]agente.Mensagem, texto string, aoIniciar func(agente.ChamadaFerramenta), aoConcluir func(agente.ResultadoFerramenta), aoDelta func(string)) (agente.Resposta, string, error) {
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

	// Resumo rolante (§7 da spec). len(janela) < len(*historico) é o gatilho
	// de transbordo: Montar devolve um sufixo, então encolher significa que
	// descartou. Não é preciso mudar a assinatura de Montar para saber isso.
	//
	// O resumidor lê do Postgres, não das mensagens descartadas:
	// agente.Mensagem não carrega id, e depois de uma retomada o histórico em
	// memória é só a cauda de 200 registros — contar sobre ele resumiria o
	// trecho errado, em silêncio (R4).
	resumoDaSessao := s.atualizarResumo(ctx, *sessaoID, len(janela) < len(*historico), len(janela))

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

	// Depois do bloco do Vault, pelo mesmo motivo que ele não entra na
	// janela: resumo não é conversa e não pode competir com recência por
	// orçamento.
	if resumoDaSessao != "" {
		sistemaPrompt = sistemaPrompt + "\n\nRESUMO DA CONVERSA ATÉ AQUI:\n" + resumoDaSessao
	}

	loop := *s.Loop
	loop.AoIniciarFerramenta = aoIniciar
	loop.AoConcluirFerramenta = aoConcluir
	loop.Modo = modo
	loop.AoDelta = aoDelta

	// Ferramentas que mantêm estado por conversa (perguntar_ao_claude_code)
	// leem o id da sessão daqui — é o único canal que atravessa api → agente
	// → ferramentas.
	ctx = ferramentas.ComSessao(ctx, sessaoID.String())

	inicioDoTurno := time.Now()
	resp, mensagens, err := loop.Executar(ctx, sistemaPrompt, janela, entrada, perfil, s.criarSolicitarAprovacao(*sessaoID))
	if err != nil {
		return agente.Resposta{}, "", err
	}
	// Uma linha por turno com os tokens: o teto do Arthur é de R$ 20 a 30 por
	// mês, e o gasto só é controlável se for visível.
	slog.Info("turno", "modo", nomeDoModo(modo), "entrada", resp.Uso.Entrada, "saida", resp.Uso.Saida,
		"cache", resp.Uso.CacheLeitura, "pensamento", resp.Uso.Pensamento, "dur_ms", time.Since(inicioDoTurno).Milliseconds())

	*historico = mensagens
	s.persistirNovasMensagens(ctx, *sessaoID, mensagens, tamanhoAntes)

	// Na voz quem fala é o device, frase a frase, enquanto o modelo escreve.
	// Gerar áudio aqui também faria o James falar duas vezes (device e app).
	audioID := ""
	if modo != agente.ModoVoz {
		audioID = s.gerarAudioDaResposta(ctx, resp.Texto)
	}
	return resp, audioID, nil
}

// atualizarResumo devolve o resumo da sessão, atualizando-o antes se a
// janela transbordou. Melhor-esforço em toda linha: resumo quebrado degrada
// para janela truncada — o comportamento anterior a esta fase — e nunca
// derruba o turno (D10).
func (s *Servidor) atualizarResumo(ctx context.Context, sessaoID uuid.UUID, transbordou bool, tamanhoJanela int) string {
	if s.Resumidor == nil || sessaoID == uuid.Nil {
		return ""
	}

	sessao, err := s.Sessoes.BuscarPorID(ctx, sessaoID)
	if err != nil {
		slog.Warn("erro ao ler o resumo da sessão, seguindo sem ele", "erro", err)
		return ""
	}
	if !transbordou {
		return sessao.Resumo
	}

	corte, err := s.Mensagens.IDDeCorte(ctx, sessaoID, tamanhoJanela)
	if err != nil || corte == 0 {
		if err != nil {
			slog.Warn("erro ao calcular o corte da janela, seguindo sem resumir", "erro", err)
		}
		return sessao.Resumo
	}

	var resumidoAte int64
	if sessao.ResumidoAte != nil {
		resumidoAte = *sessao.ResumidoAte
	}

	novas, err := s.Mensagens.ListarNaoResumidas(ctx, sessaoID, corte, resumidoAte)
	if err != nil {
		slog.Warn("erro ao ler as mensagens fora da janela, seguindo sem resumir", "erro", err)
		return sessao.Resumo
	}
	if len(novas) == 0 {
		return sessao.Resumo
	}

	comoMensagens, err := reconstruirHistorico(novas)
	if err != nil {
		slog.Warn("erro ao reconstruir as mensagens a resumir, seguindo sem resumir", "erro", err)
		return sessao.Resumo
	}

	atualizado, err := s.Resumidor.Resumir(ctx, sessao.Resumo, comoMensagens)
	if err != nil {
		slog.Warn("erro ao resumir a conversa, seguindo com a janela truncada", "erro", err)
		return sessao.Resumo
	}

	ultimo := novas[len(novas)-1].ID
	if err := s.Sessoes.AtualizarResumo(ctx, sessaoID, atualizado, ultimo); err != nil {
		slog.Warn("erro ao gravar o resumo, seguindo com ele só neste turno", "erro", err)
	}
	return atualizado
}

func nomeDoModo(m agente.Modo) string {
	if m == agente.ModoVoz {
		return "voz"
	}
	return "texto"
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
