package api

import (
	"encoding/json"
	"log/slog"

	"james/internal/voz"
)

// falador leva a resposta do modelo ao device, frase a frase, enquanto o
// modelo ainda escreve. É o que faz o primeiro som sair na primeira frase em
// vez de esperar a resposta inteira.
//
// Não é seguro pra uso concorrente, e não precisa ser: o loop do agente chama
// delta e os hooks de ferramenta da mesma goroutine, em sequência.
type falador struct {
	conexao   *conexaoDispositivo
	id        string
	quebrador voz.QuebradorDeFrases
	indice    int
}

func novoFalador(conexao *conexaoDispositivo, id string) *falador {
	return &falador{conexao: conexao, id: id}
}

// delta recebe cada pedaço de texto do modelo.
func (f *falador) delta(d string) {
	for _, frase := range f.quebrador.Acrescentar(d) {
		f.frase(frase)
	}
}

// descarregar fala o que ficou no quebrador. É chamado quando uma ferramenta
// começa (o ponto no fim do buffer espera o próximo caractere, que só viria
// depois da ferramenta, e o James ficaria calado justo quando devia dizer o
// que está fazendo) e no fim do turno.
func (f *falador) descarregar() {
	if resto := f.quebrador.Fim(); resto != "" {
		f.frase(resto)
	}
}

func (f *falador) frase(bruta string) {
	limpa := voz.LimparParaFala(bruta)
	if limpa == "" {
		return
	}
	f.enviar("fala_frase", payloadFalaFrase{Texto: limpa, Indice: f.indice})
	f.indice++
}

func (f *falador) fim() { f.enviar("fala_fim", struct{}{}) }

func (f *falador) erro(mensagem string) {
	f.enviar("fala_erro", payloadFalaErro{Mensagem: mensagem})
}

func (f *falador) enviar(tipo string, dados any) {
	bruto, err := json.Marshal(Envelope{Tipo: tipo, ID: f.id, Dados: codificarDados(dados)})
	if err != nil {
		slog.Error("erro ao montar evento de fala", "tipo", tipo, "erro", err)
		return
	}
	if err := f.conexao.Enviar(bruto); err != nil {
		// Device caiu no meio do turno: o turno segue (a resposta ainda vai
		// pro app e pro banco), só não há quem fale.
		slog.Warn("não consegui mandar a fala ao device", "tipo", tipo, "erro", err)
	}
}
