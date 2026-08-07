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

type Requisicao struct {
	Sistema     string
	Mensagens   []Mensagem
	Declaracoes []ferramentas.Declaracao
}

type Resposta struct {
	Texto    string
	Chamadas []ChamadaFerramenta
}

// Provider é a fronteira que torna o loop testável com respostas
// roteirizadas, sem gastar token e sem abrir janela.
type Provider interface {
	Gerar(ctx context.Context, req Requisicao) (Resposta, error)
	GerarStream(ctx context.Context, req Requisicao, emitir func(string)) (Resposta, error)
}
