package ferramentas

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
)

// Arquivos é a ferramenta de OLHAR o disco (spec 2026-10-14 arquivos, D1 a D9):
// listar, ler, buscar e revelar. Para mexer, alterar_arquivos.
//
// NivelPerigoso de propósito: o desktop (Privilegiado) roda livre e o celular
// (Restrito) pede aprovação a cada leitura. Valores secretos chegam
// mascarados (mascara.go); revelar mostra o valor de verdade, sempre com
// aprovação, e o registra no SegredosLidos para a memória não guardá-lo.
type Arquivos struct {
	// Segredos recebe os valores que o revelar mostrou. Pode ser nil.
	Segredos *SegredosLidos
}

func NovoArquivos(segredos *SegredosLidos) *Arquivos { return &Arquivos{Segredos: segredos} }

func (a *Arquivos) Declaracao() Declaracao {
	return Declaracao{
		Nome: "arquivos",
		Descricao: "Olha pastas e arquivos do computador do Arthur, sem pedir aprovação. Use para listar uma pasta, " +
			"ler um arquivo de texto ou procurar por nome ou por conteúdo; não use cat, ls, find nem grep pelo " +
			"terminal para ler. Para criar, editar, mover, copiar ou apagar, use alterar_arquivos. " +
			"Valores secretos (senhas, chaves, tokens) aparecem como •••• de propósito: o nome da chave e a estrutura " +
			"continuam visíveis, e isso basta para saber se algo está preenchido. Só use 'revelar' (que pede aprovação " +
			"e manda o valor para a Anthropic) quando o Arthur pedir o valor em si. " +
			"O caminho é completo (/home/... ou começando por ~).",
		Parametros: map[string]Parametro{
			"acao":              {Tipo: "string", Descricao: "'listar' (pasta), 'ler' (arquivo de texto), 'buscar' (por nome ou conteúdo) ou 'revelar' (ler sem a máscara, pede aprovação)"},
			"caminho":           {Tipo: "string", Descricao: "pasta ou arquivo, absoluto ou começando por ~. Em 'buscar', a pasta onde procurar"},
			"nome":              {Tipo: "string", Descricao: "para 'buscar': trecho do nome ou glob (ex: '*.go')"},
			"conteudo":          {Tipo: "string", Descricao: "para 'buscar': texto a procurar dentro dos arquivos"},
			"incluir_ocultos":   {Tipo: "boolean", Descricao: "para 'listar' e 'buscar': incluir arquivos e pastas que começam com ponto"},
			"a_partir_da_linha": {Tipo: "integer", Descricao: "para 'ler': primeira linha a mostrar (a partir de 1)"},
			"linhas":            {Tipo: "integer", Descricao: "para 'ler': quantas linhas mostrar (padrão 200, máximo 2000)"},
		},
		Obrigatorios: []string{"acao"},
	}
}

func (a *Arquivos) Nivel() Nivel { return NivelPerigoso }

// ConfirmacaoNecessaria implementa ExigeConfirmacao: só o revelar pede, e só
// quando há o que revelar. Pedido inválido (caminho relativo, arquivo que não
// existe) devolve o erro direto, sem cartão: não há nada a aprovar.
func (a *Arquivos) ConfirmacaoNecessaria(args map[string]any) bool {
	if acao, _ := args["acao"].(string); acao != "revelar" {
		return false
	}
	caminho, err := caminhoDoPedido(args)
	if err != nil {
		return false
	}
	info, err := os.Stat(caminho)
	return err == nil && !info.IsDir()
}

// DescricaoDaConfirmacao implementa DescreveConfirmacao.
func (a *Arquivos) DescricaoDaConfirmacao(args map[string]any) string {
	if acao, _ := args["acao"].(string); acao != "revelar" {
		return ""
	}
	caminho, err := caminhoDoPedido(args)
	if err != nil {
		return ""
	}
	return "mostrar os valores secretos de " + caminho + " ao modelo (vão para a Anthropic e ficam no histórico desta conversa)"
}

func caminhoDoPedido(args map[string]any) (string, error) {
	bruto, _ := args["caminho"].(string)
	return ResolverCaminho(bruto)
}

// inteiroOpcional lê um inteiro a partir de 1 de um argumento que veio do JSON
// (float64). Ausente vale 0; qualquer outra coisa é erro.
func inteiroOpcional(args map[string]any, nome string) (int, error) {
	bruto, existe := args[nome]
	if !existe || bruto == nil {
		return 0, nil
	}
	numero, ok := bruto.(float64)
	if !ok || numero != math.Trunc(numero) || numero < 1 {
		return 0, fmt.Errorf("%s precisa ser um número inteiro a partir de 1", nome)
	}
	return int(numero), nil
}

func booleanoOpcional(args map[string]any, nome string) (bool, error) {
	bruto, existe := args[nome]
	if !existe || bruto == nil {
		return false, nil
	}
	valor, ok := bruto.(bool)
	if !ok {
		return false, fmt.Errorf("%s precisa ser verdadeiro ou falso", nome)
	}
	return valor, nil
}

func (a *Arquivos) Executar(_ context.Context, args map[string]any) (string, error) {
	acao, _ := args["acao"].(string)
	switch acao {
	case "listar", "ler", "buscar", "revelar":
	case "":
		return "", errors.New("acao ausente: use listar, ler, buscar ou revelar")
	default:
		return "", fmt.Errorf("acao %q desconhecida: use listar, ler, buscar ou revelar", acao)
	}
	caminho, err := caminhoDoPedido(args)
	if err != nil {
		return "", err
	}
	ocultos, err := booleanoOpcional(args, "incluir_ocultos")
	if err != nil {
		return "", err
	}
	de, err := inteiroOpcional(args, "a_partir_da_linha")
	if err != nil {
		return "", err
	}
	linhas, err := inteiroOpcional(args, "linhas")
	if err != nil {
		return "", err
	}

	switch acao {
	case "listar":
		return listarPasta(caminho, ocultos)
	case "buscar":
		nome, _ := args["nome"].(string)
		conteudo, _ := args["conteudo"].(string)
		return buscarArquivos(caminho, opcoesBusca{Nome: nome, Conteudo: conteudo, IncluirOcultos: ocultos})
	case "ler":
		r, err := lerArquivo(caminho, de, linhas, false)
		return r.Texto, err
	default: // revelar
		r, err := lerArquivo(caminho, de, linhas, true)
		if err != nil {
			return "", err
		}
		a.Segredos.Registrar(r.Ocultos) // *SegredosLidos nil não guarda nada
		return r.Texto, nil
	}
}
