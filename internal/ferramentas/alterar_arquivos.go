package ferramentas

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

// AlterarArquivos é a ferramenta de MEXER no disco (spec 2026-10-14 arquivos,
// D3, D10 e D16): criar, editar, mover, copiar e apagar. Sempre com aprovação
// do Arthur, no desktop também; o cartão mostra o que vai mudar. Apagar manda
// para a lixeira.
type AlterarArquivos struct{}

func NovoAlterarArquivos() *AlterarArquivos { return &AlterarArquivos{} }

func (a *AlterarArquivos) Declaracao() Declaracao {
	return Declaracao{
		Nome: "alterar_arquivos",
		Descricao: "Cria, edita, move, copia ou apaga arquivos do computador do Arthur. TODA chamada pede a aprovação " +
			"dele, e o cartão mostra o que vai mudar. Apagar manda para a lixeira. 'criar' não sobrescreve (a menos " +
			"que sobrescrever seja true) e 'editar' troca um trecho exato que apareça uma vez (ou use todas). " +
			"Para só olhar pastas e arquivos, use a ferramenta arquivos. O caminho é completo (/home/... ou começando por ~).",
		Parametros: map[string]Parametro{
			"acao":          {Tipo: "string", Descricao: "'criar', 'editar', 'mover', 'copiar' ou 'apagar'"},
			"caminho":       {Tipo: "string", Descricao: "para criar, editar e apagar: o arquivo"},
			"conteudo":      {Tipo: "string", Descricao: "para criar: o texto do arquivo (até 1 MB)"},
			"sobrescrever":  {Tipo: "boolean", Descricao: "para criar: permite trocar um arquivo que já existe"},
			"criar_pastas":  {Tipo: "boolean", Descricao: "para criar: cria as pastas que faltarem no caminho"},
			"trecho_antigo": {Tipo: "string", Descricao: "para editar: o trecho exato a trocar (tem de aparecer uma vez, ou use todas)"},
			"trecho_novo":   {Tipo: "string", Descricao: "para editar: o texto que entra no lugar (pode ser vazio para apagar o trecho)"},
			"todas":         {Tipo: "boolean", Descricao: "para editar: troca todas as ocorrências do trecho"},
			"origem":        {Tipo: "string", Descricao: "para mover e copiar: o que será movido ou copiado"},
			"destino":       {Tipo: "string", Descricao: "para mover e copiar: para onde (não pode existir)"},
		},
		Obrigatorios: []string{"acao"},
	}
}

func (a *AlterarArquivos) Nivel() Nivel { return NivelPerigoso }

// planoDeAlteracao é o que um pedido VAI fazer, já validado. Existe para a
// validação, o cartão e a execução saírem da mesma conta: o cartão não pode
// prometer uma coisa e o disco receber outra.
type planoDeAlteracao struct {
	acao      string
	caminho   string // só para o log
	descricao string
	executar  func(ctx context.Context) (string, error)
}

var acoesDeAlteracao = map[string]bool{"criar": true, "editar": true, "mover": true, "copiar": true, "apagar": true}

const dicaDasAcoes = "use criar, editar, mover, copiar ou apagar"

func caminhoDoParametro(args map[string]any, nome string) (string, error) {
	bruto, existe := args[nome]
	texto, ok := bruto.(string)
	if !existe || !ok || strings.TrimSpace(texto) == "" {
		return "", fmt.Errorf("parâmetro %s ausente", nome)
	}
	return ResolverCaminho(texto)
}

// textoPresente exige que o parâmetro exista e seja texto, mas aceita vazio:
// criar um arquivo vazio e apagar um trecho (trecho_novo vazio) são pedidos
// legítimos.
func textoPresente(args map[string]any, nome string) (string, error) {
	bruto, existe := args[nome]
	texto, ok := bruto.(string)
	if !existe || !ok {
		return "", fmt.Errorf("parâmetro %s ausente", nome)
	}
	return texto, nil
}

func previaDoTrecho(texto string) string {
	texto = strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(texto)
	if letras := []rune(texto); len(letras) > 60 {
		return string(letras[:60]) + "…"
	}
	return texto
}

func plural(n int, singular, pluralizado string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, pluralizado)
}

func (a *AlterarArquivos) planejar(args map[string]any) (planoDeAlteracao, error) {
	acao, _ := args["acao"].(string)
	if !acoesDeAlteracao[acao] {
		return planoDeAlteracao{}, fmt.Errorf("acao %q desconhecida: %s", acao, dicaDasAcoes)
	}

	switch acao {
	case "criar":
		caminho, err := caminhoDoParametro(args, "caminho")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		conteudo, err := textoPresente(args, "conteudo")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		sobrescrever, err := booleanoOpcional(args, "sobrescrever")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		criarPastas, err := booleanoOpcional(args, "criar_pastas")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		tamanho, sobrescreve, err := validarCriar(caminho, conteudo, sobrescrever, criarPastas)
		if err != nil {
			return planoDeAlteracao{}, err
		}
		verbo := "criar"
		if sobrescreve {
			verbo = "sobrescrever"
		}
		return planoDeAlteracao{acao, caminho, fmt.Sprintf("%s %s (%d bytes)", verbo, caminho, tamanho),
			func(context.Context) (string, error) {
				return criarArquivo(caminho, conteudo, sobrescrever, criarPastas)
			}}, nil

	case "editar":
		caminho, err := caminhoDoParametro(args, "caminho")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		antigo, err := textoPresente(args, "trecho_antigo")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		novo, err := textoPresente(args, "trecho_novo")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		todas, err := booleanoOpcional(args, "todas")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		trocas, err := validarEditar(caminho, antigo, novo, todas)
		if err != nil {
			return planoDeAlteracao{}, err
		}
		descricao := fmt.Sprintf("editar %s: trocar %s", caminho, plural(trocas, "trecho", "trechos"))
		if !CaminhoSensivel(caminho) { // o cartão de um .env não mostra o que entra nem o que sai
			descricao += fmt.Sprintf(` (de "%s" por "%s")`, previaDoTrecho(antigo), previaDoTrecho(novo))
		}
		return planoDeAlteracao{acao, caminho, descricao,
			func(context.Context) (string, error) { return editarArquivo(caminho, antigo, novo, todas) }}, nil

	case "mover", "copiar":
		origem, err := caminhoDoParametro(args, "origem")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		destino, err := caminhoDoParametro(args, "destino")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		if acao == "mover" {
			if err := validarMover(origem, destino); err != nil {
				return planoDeAlteracao{}, err
			}
			return planoDeAlteracao{acao, origem, "mover " + origem + " → " + destino,
				func(context.Context) (string, error) { return moverArquivo(origem, destino) }}, nil
		}
		if err := validarCopiar(origem, destino); err != nil {
			return planoDeAlteracao{}, err
		}
		return planoDeAlteracao{acao, origem, "copiar " + origem + " → " + destino,
			func(context.Context) (string, error) { return copiarArquivo(origem, destino) }}, nil

	default: // apagar
		caminho, err := caminhoDoParametro(args, "caminho")
		if err != nil {
			return planoDeAlteracao{}, err
		}
		if err := validarApagar(caminho); err != nil {
			return planoDeAlteracao{}, err
		}
		return planoDeAlteracao{acao, caminho, "apagar " + caminho + " (vai para a lixeira)",
			func(ctx context.Context) (string, error) { return apagarArquivo(ctx, caminho) }}, nil
	}
}

// ConfirmacaoNecessaria implementa ExigeConfirmacao: toda alteração válida
// pede aprovação. Ação desconhecida também (falha para o lado seguro). Pedido
// INVÁLIDO de uma ação conhecida (trecho que não existe, destino que já
// existe, caminho relativo, pasta protegida) não pede: não há nada a aprovar, e
// o Executar devolve o erro direto.
func (a *AlterarArquivos) ConfirmacaoNecessaria(args map[string]any) bool {
	if acao, _ := args["acao"].(string); !acoesDeAlteracao[acao] {
		return true
	}
	_, err := a.planejar(args)
	return err == nil
}

// DescricaoDaConfirmacao implementa DescreveConfirmacao: o texto do cartão.
func (a *AlterarArquivos) DescricaoDaConfirmacao(args map[string]any) string {
	plano, err := a.planejar(args)
	if err != nil {
		return ""
	}
	return plano.descricao
}

// camposDeConteudo são os argumentos que carregam o texto gravado no arquivo.
var camposDeConteudo = []string{"conteudo", "trecho_antigo", "trecho_novo"}

// ArgumentosParaRegistro implementa ArgumentosRegistraveis: em arquivo sensível
// (.env, .aws/credentials, chave), o texto gravado vai com os valores
// escondidos, como na leitura. A estrutura continua legível no cartão, então
// quem aprova ainda vê QUE chaves serão escritas.
func (a *AlterarArquivos) ArgumentosParaRegistro(args map[string]any) map[string]any {
	caminho, _ := args["caminho"].(string)
	if !CaminhoSensivel(caminho) {
		return args
	}
	copia := make(map[string]any, len(args))
	for k, v := range args {
		copia[k] = v
	}
	for _, campo := range camposDeConteudo {
		if texto, ok := args[campo].(string); ok {
			copia[campo], _ = Mascarar(texto, true)
		}
	}
	return copia
}

// Executar RECONFERE o pedido inteiro antes de escrever: entre o cartão e o
// clique de aprovar o arquivo pode ter mudado, e a aprovação vale para o que
// foi mostrado, não para o que existir na hora.
func (a *AlterarArquivos) Executar(ctx context.Context, args map[string]any) (string, error) {
	plano, err := a.planejar(args)
	if err != nil {
		return "", err
	}
	saida, err := plano.executar(ctx)
	if err != nil {
		slog.Warn("alteração de arquivo falhou", "acao", plano.acao, "caminho", plano.caminho)
		return "", errors.New(strings.TrimSpace(err.Error()))
	}
	// Ação e caminho, nunca o conteúdo nem os trechos (D13).
	slog.Info("alteração de arquivo", "acao", plano.acao, "caminho", plano.caminho)
	return saida, nil
}
