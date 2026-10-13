// Package agente executa o loop central do James: monta contexto, chama o
// Provider, despacha ferramentas respeitando o perfil de permissão, e
// repete até ter uma resposta final ou estourar o teto de iterações.
package agente

import (
	"context"

	"james/internal/ferramentas"
)

type Papel string

const (
	PapelUsuario    Papel = "usuario"
	PapelModelo     Papel = "modelo"
	PapelFerramenta Papel = "ferramenta"
)

type ChamadaFerramenta struct {
	ID   string
	Nome string
	Args map[string]any
}

type ResultadoFerramenta struct {
	ID    string
	Nome  string
	Saida string
	Erro  string
	// Imagem é preenchida só por ferramenta que implementar
	// FerramentaComImagem (Fase 5, Task 5) — hoje nenhuma o faz. json:"-"
	// porque, igual Mensagem.Imagem, não deve ser persistida no Postgres:
	// o banco é log/auditoria, não deveria carregar binário pesado a cada
	// turno com ver_tela.
	Imagem []byte `json:"-"`
}

type Mensagem struct {
	Papel      Papel
	Texto      string
	Imagem     []byte
	Chamadas   []ChamadaFerramenta
	Resultados []ResultadoFerramenta
}

// Modo diz de onde veio o turno, e com isso quanto vale esperar pelo modelo.
// O provedor Anthropic usa pra decidir se o modelo pensa antes de responder:
// na voz cada décimo de segundo é ouvido, no texto a qualidade pesa mais. O
// Gemini ignora este campo.
//
// O valor zero é ModoTexto de propósito: quem esquece de preencher ganha o
// comportamento mais lento e mais correto, nunca a resposta apressada.
type Modo int

const (
	ModoTexto Modo = iota
	ModoVoz
)

type Requisicao struct {
	Sistema     string
	Mensagens   []Mensagem
	Declaracoes []ferramentas.Declaracao
	Modo        Modo
}

// Uso é o consumo de tokens de uma chamada ao modelo. Existe pra o custo ser
// visível por turno: o teto do Arthur é de R$ 20 a 30 por mês.
type Uso struct {
	Entrada      int
	Saida        int
	CacheLeitura int
	Pensamento   int
}

// Somar devolve a soma dos dois usos, sem alterar nenhum. O loop chama o
// modelo várias vezes por turno (uma por rodada de ferramenta) e o turno
// precisa do total.
func (u Uso) Somar(outro Uso) Uso {
	return Uso{
		Entrada:      u.Entrada + outro.Entrada,
		Saida:        u.Saida + outro.Saida,
		CacheLeitura: u.CacheLeitura + outro.CacheLeitura,
		Pensamento:   u.Pensamento + outro.Pensamento,
	}
}

type Resposta struct {
	Texto    string
	Chamadas []ChamadaFerramenta
	Uso      Uso
}

// Provider é a fronteira que torna o loop testável com respostas
// roteirizadas, sem gastar token e sem abrir janela.
type Provider interface {
	Gerar(ctx context.Context, req Requisicao) (Resposta, error)
	GerarStream(ctx context.Context, req Requisicao, emitir func(string)) (Resposta, error)
}
