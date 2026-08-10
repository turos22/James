package api

import (
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

// payloadErro é o formato de "erro" — compartilhado pelos três handlers que
// emitem esse evento (tratarMensagem, tratarEntradaDeVoz e tratarRetomar).
type payloadErro struct {
	Mensagem string `json:"mensagem"`
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
