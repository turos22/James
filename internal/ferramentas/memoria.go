package ferramentas

import (
	"context"
	"fmt"
	"strings"
)

// vaultMemoria é o subconjunto de memoria.Vault que esta ferramenta precisa.
// Definido localmente, em vez de importar o tipo concreto, porque
// internal/memoria já importa internal/agente (pra Janela.Montar calcular
// custo sobre agente.Mensagem), e internal/agente importa internal/ferramentas
// (pro contrato Ferramenta) — importar memoria.Vault aqui fecharia o ciclo
// ferramentas → memoria → agente → ferramentas. *memoria.Vault satisfaz esta
// interface por tipagem estrutural, sem o pacote ferramentas nunca importar
// memoria: quem monta o Servidor (Task 11, cmd/jamesd/main.go) já importa os
// dois e passa um *memoria.Vault aqui.
type vaultMemoria interface {
	Acrescentar(assunto, texto string) error
	Assuntos() ([]string, error)
	Slug(assunto string) string
}

// SalvarNaMemoria é o único caminho de escrita na memória durável do James.
//
// NivelSeguro: escrever no próprio vault não executa nada, não controla a
// máquina e não sai do diretório de memória (o Vault concreto garante isso
// via ResolverCaminho). Não passa pelo gate de confirmação da Fase 3 — pedir
// aprovação pra cada fato guardado transformaria a memória em fricção e ela
// deixaria de ser usada.
type SalvarNaMemoria struct {
	Vault vaultMemoria
}

func NovoSalvarNaMemoria(v vaultMemoria) *SalvarNaMemoria {
	return &SalvarNaMemoria{Vault: v}
}

func (s *SalvarNaMemoria) Declaracao() Declaracao {
	descricao := "Guarda um fato durável sobre o Arthur na sua memória, para você lembrar " +
		"em conversas futuras — ela sobrevive ao fim desta conversa. Use para preferência, " +
		"decisão, fato pessoal ou estado de projeto. Não use para trivialidade, e nunca " +
		"copie saída bruta de outra ferramenta: escreva a conclusão com suas palavras, curto."

	// A lista de assuntos existentes é o que impede o modelo de inventar
	// arquivo novo a cada chamada em vez de acrescentar num que já existe.
	if assuntos, err := s.Vault.Assuntos(); err == nil && len(assuntos) > 0 {
		descricao += " Assuntos que já existem, prefira um deles: " + strings.Join(assuntos, ", ") + "."
	}

	return Declaracao{
		Nome:      "salvar_na_memoria",
		Descricao: descricao,
		Parametros: map[string]Parametro{
			"assunto": {Tipo: "string", Descricao: "tema curto da nota, minúsculas e sem acento (ex: preferencias-de-codigo)"},
			"texto":   {Tipo: "string", Descricao: "o fato a guardar, em uma frase"},
		},
		Obrigatorios: []string{"assunto", "texto"},
	}
}

func (s *SalvarNaMemoria) Nivel() Nivel { return NivelSeguro }

func (s *SalvarNaMemoria) Executar(_ context.Context, args map[string]any) (string, error) {
	assunto, _ := args["assunto"].(string)
	texto, _ := args["texto"].(string)
	if strings.TrimSpace(assunto) == "" || strings.TrimSpace(texto) == "" {
		return "", fmt.Errorf("parâmetros assunto e texto são obrigatórios")
	}

	if err := s.Vault.Acrescentar(assunto, texto); err != nil {
		return "", err
	}
	return fmt.Sprintf("Guardado na memória, no assunto %q.", s.Vault.Slug(assunto)), nil
}
