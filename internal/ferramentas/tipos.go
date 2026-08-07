// Package ferramentas define o contrato de tool que o agente consome e o
// registro onde cada tool concreta se cadastra.
package ferramentas

import "context"

// Nivel é o que o gate de permissão lê pra decidir se uma tool executa sem
// aprovação humana.
type Nivel int

const (
	NivelSeguro   Nivel = iota // leitura, consulta, sem efeito colateral
	NivelPerigoso              // executa, altera estado, controla a máquina
)

// Parametro descreve um argumento de tool pro schema que vai pro Provider.
type Parametro struct {
	Tipo      string // "string", "integer", "number", "boolean"
	Descricao string
}

type Declaracao struct {
	Nome         string
	Descricao    string
	Parametros   map[string]Parametro
	Obrigatorios []string
}

type Ferramenta interface {
	Declaracao() Declaracao
	Nivel() Nivel
	Executar(ctx context.Context, args map[string]any) (string, error)
}

// ExigeConfirmacao é implementada por ferramentas em que uma chamada
// específica precisa de confirmação humana mesmo vinda de um perfil
// Privilegiado — o caso do comando PowerShell de escrita.
//
// O contrato é deliberadamente aditivo: o despacho só usa esta interface pra
// LIGAR a exigência de aprovação, nunca pra desligar a que o Nivel() já
// impõe. É o que garante, na forma do código e não na disciplina de quem
// mexe depois, que o perfil Restrito nunca seja afrouxado.
//
// Ferramenta que não implementa se comporta exatamente como antes.
type ExigeConfirmacao interface {
	ConfirmacaoNecessaria(args map[string]any) bool
}

// FerramentaComImagem é implementada pela ferramenta que vê a tela (Fase 5,
// Task 9) — hoje nenhuma implementa.
//
// Mesmo contrato aditivo de ExigeConfirmacao: ferramenta que não implementa
// se comporta exatamente como antes, e o despacho só usa esta interface pra
// ENRIQUECER o resultado, nunca pra mudar o gate de permissão. A alternativa
// (trocar a assinatura de Ferramenta.Executar) forçaria mudança nas cinco
// ferramentas que não têm imagem, sem ganho nenhum pra elas.
//
// Quem implementa esta interface precisa implementar Ferramenta também: o
// Registro só aceita Ferramenta. O loop nunca chama Executar quando
// ExecutarComImagem existe — mas Executar ainda pode ser chamado por
// código fora do loop (ex.: teste direto da ferramenta), então deve
// delegar pra ExecutarComImagem e simplesmente descartar a imagem, em vez
// de duplicar lógica ou virar stub morto.
type FerramentaComImagem interface {
	ExecutarComImagem(ctx context.Context, args map[string]any) (string, []byte, error)
}
