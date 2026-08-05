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
