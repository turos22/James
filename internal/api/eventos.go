package api

import (
	"errors"
	"log/slog"

	"github.com/google/uuid"

	"james/internal/agente"
)

// emitirFunc manda um evento tipado pra quem quer que esteja do outro lado —
// um cliente WS específico (conexaoCliente.Enviar) ou todo Privilegiado
// conectado (ClientesHub.BroadcastParaPrivilegiados). tratarMensagem
// (ws.go) e tratarEntradaDeVoz (dispositivo.go) montam os MESMOS eventos
// pra um turno — ferramenta_iniciada, ferramenta_concluida, resposta_final,
// audio_pronto e erro — e só divergem em qual emitirFunc passam pra
// hooksDeFerramenta e emitirDesfechoDoTurno abaixo. O broadcast nunca
// precisa saber que existe envio direto, e vice-versa: a única coisa
// injetada é "pra quem", nunca "o quê".
type emitirFunc func(tipo string, dados any)

// payloadFerramentaIniciada é o formato de "ferramenta_iniciada". Struct
// nomeada em vez de map[string]string: um typo na chave (ex.: "nomee")
// virava sucesso silencioso em tempo de compilação com map literal — aqui
// vira erro de compilação, e os testes decodificam contra este mesmo tipo.
type payloadFerramentaIniciada struct {
	Nome string `json:"nome"`
}

// payloadFerramentaConcluida é o formato de "ferramenta_concluida". Saida
// foi acrescentada pela Task 18 — é o que alimenta a aba Tela com o texto
// que ver_tela devolve (superfície + lista de elementos). Vem vazia para
// ferramenta que não produz saída (ex.: erro antes de executar) — o
// contrato não é "sempre preenchida", é "sempre presente". Imagem nunca
// entra aqui: é binário pesado e está fora do alcance desta fase (ver
// agente.ResultadoFerramenta.Imagem).
type payloadFerramentaConcluida struct {
	Nome  string `json:"nome"`
	Erro  string `json:"erro"`
	Saida string `json:"saida"`
}

// payloadRespostaFinal é o formato de "resposta_final".
type payloadRespostaFinal struct {
	Texto    string `json:"texto"`
	SessaoID string `json:"sessao_id"`
}

// payloadAudioPronto é o formato de "audio_pronto".
type payloadAudioPronto struct {
	ID string `json:"id"`
}

// payloadFalaFrase é o formato de "fala_frase": uma frase pronta pra falar
// (já sem Markdown) que o jamesd manda ao device. Indice dá a ordem; o device
// toca na ordem do índice e não na de chegada.
type payloadFalaFrase struct {
	Texto  string `json:"texto"`
	Indice int    `json:"indice"`
}

// payloadFalaErro é o formato de "fala_erro": o turno falhou e o device fala
// esta mensagem (a mesma, segura, que o app mostra na tela).
type payloadFalaErro struct {
	Mensagem string `json:"mensagem"`
}

// payloadErro é o formato de "erro" — compartilhado pelos três handlers que
// emitem esse evento (tratarMensagem, tratarEntradaDeVoz e tratarRetomar).
type payloadErro struct {
	Mensagem string `json:"mensagem"`
}

// payloadTranscricao é o formato de "transcricao": o texto que o device
// entendeu numa entrada falada. Só o caminho de voz emite. O caminho de
// texto não precisa — o cliente que digitou já desenhou o próprio balão
// (envio_local) antes mesmo de a mensagem sair.
type payloadTranscricao struct {
	Texto string `json:"texto"`
}

// mensagemDoHistorico é uma mensagem da conversa retomada, no formato que a
// interface exibe. Só papel e texto: o front monta o balão a partir disso.
type mensagemDoHistorico struct {
	Papel string `json:"papel"`
	Texto string `json:"texto"`
}

// payloadSessaoRetomada é o formato de "sessao_retomada". Antes carregava só
// a CONTAGEM de mensagens (`mensagens int`) — achado da verificação ao vivo
// da Fase 6: o Gemini lembrava do histórico (reconstruído certo pro
// contexto), mas a tela ficava em branco, porque nada além da contagem
// chegava ao front. Agora Mensagens carrega o conteúdo, montado por
// mensagensExibiveis a partir do MESMO histórico que acabou de ser
// reconstruído para o modelo — mesma fonte, mesmo instante, sem segunda
// consulta ao Postgres (que além do custo extra teria a janela errada: ver
// comentário de limiteMensagensSessao em ws.go).
type payloadSessaoRetomada struct {
	SessaoID  string                `json:"sessao_id"`
	Mensagens []mensagemDoHistorico `json:"mensagens"`
}

// mensagensExibiveis filtra o histórico reconstruído (que alimenta o
// CONTEXTO do modelo, com todos os papéis) para o que a interface consegue
// desenhar como balão de conversa. Ficam de fora:
//   - papel ferramenta: é saída de ferramenta, não fala da conversa — já tem
//     superfície própria na aba Atividade e no visualizador (ver_tela).
//     Mandar aqui encheria o balão de conversa com listas de dezenas de
//     elementos.
//   - qualquer mensagem com Texto vazio, o que na prática captura o modelo
//     quando a linha só carrega chamada de ferramenta (Chamadas preenchido,
//     Texto vazio) — sem esse filtro viraria balão em branco na tela.
//
// Preserva a ordem cronológica de `historico`, que já vem nessa ordem de
// reconstruirHistorico.
func mensagensExibiveis(historico []agente.Mensagem) []mensagemDoHistorico {
	mensagens := make([]mensagemDoHistorico, 0, len(historico))
	for _, m := range historico {
		if m.Texto == "" {
			continue
		}
		switch m.Papel {
		case agente.PapelUsuario, agente.PapelModelo:
			mensagens = append(mensagens, mensagemDoHistorico{Papel: string(m.Papel), Texto: m.Texto})
		}
	}
	return mensagens
}

// mensagemDeErroDoTurno loga o erro real e devolve a frase que pode ir pra
// tela. Existe porque tratarMensagem e tratarEntradaDeVoz mandavam
// err.Error() cru: derrubando o Postgres, o cliente recebia usuário, banco,
// host e porta na cara (reproduzido ao vivo na auditoria da Fase 6). É a
// mesma regra que tratarRetomar já seguia e que o comentário dele explica —
// esta tela é a que o celular abre pela VPN.
//
// ErroTetoIteracoes é a exceção deliberada: não é falha de infraestrutura, é
// desfecho normal do loop (R5 pede mensagem honesta), e não carrega nada
// sobre a máquina.
func mensagemDeErroDoTurno(err error, origem string) string {
	slog.Error("turno falhou", "erro", err, "origem", origem)
	if errors.Is(err, agente.ErroTetoIteracoes) {
		return "não consegui concluir esse pedido — precisei de passos demais"
	}
	return "não consegui completar esse turno"
}

// hooksDeFerramenta monta os callbacks AoIniciarFerramenta/AoConcluirFerramenta
// que processarTurno espera, emitindo ferramenta_iniciada e
// ferramenta_concluida pelo emitir passado. É a forma compartilhada entre
// tratarMensagem e tratarEntradaDeVoz — os dois precisam do mesmo payload
// exato, porque o front (web/src/lib/eventos.ts) tem um tipo só pra cada
// evento. Essa duplicação já produziu regressão real: o evento "token" foi
// removido de um caminho e sobreviveu no outro até alguém rodar o grep
// (Task 5).
func hooksDeFerramenta(emitir emitirFunc) (aoIniciar func(agente.ChamadaFerramenta), aoConcluir func(agente.ResultadoFerramenta)) {
	aoIniciar = func(c agente.ChamadaFerramenta) {
		emitir("ferramenta_iniciada", payloadFerramentaIniciada{Nome: c.Nome})
	}
	aoConcluir = func(rf agente.ResultadoFerramenta) {
		emitir("ferramenta_concluida", payloadFerramentaConcluida{Nome: rf.Nome, Erro: rf.Erro, Saida: rf.Saida})
	}
	return aoIniciar, aoConcluir
}

// emitirDesfechoDoTurno manda resposta_final (e audio_pronto, se um áudio
// foi gerado) pelo emitir passado — mesmo par usado por hooksDeFerramenta,
// e mesmo motivo: tratarMensagem e tratarEntradaDeVoz precisam terminar o
// turno com payload idêntico.
func emitirDesfechoDoTurno(emitir emitirFunc, resp agente.Resposta, sessaoID uuid.UUID, audioID string) {
	emitir("resposta_final", payloadRespostaFinal{Texto: resp.Texto, SessaoID: sessaoID.String()})
	if audioID != "" {
		emitir("audio_pronto", payloadAudioPronto{ID: audioID})
	}
}
