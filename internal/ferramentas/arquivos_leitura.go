package ferramentas

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

// Leitura de arquivos (spec 2026-10-14 arquivos, seção 4): listar, ler, buscar.
// Funções puras sobre o disco; a ferramenta `arquivos` (arquivos.go) só
// valida os argumentos e despacha.

const maxEntradasListadas = 200

// erroDeLeitura devolve uma mensagem clara sem o texto cru do SO, que pode
// trazer outro caminho.
func erroDeLeitura(caminho string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("não existe: %s", caminho)
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("sem permissão para ler: %s", caminho)
	default:
		return fmt.Errorf("não consegui ler %s", caminho)
	}
}

func tamanhoLegivel(bytes int64) string {
	const unidade = 1024
	if bytes < unidade {
		return fmt.Sprintf("%d B", bytes)
	}
	valor, sufixo := float64(bytes), ""
	for _, s := range []string{"KB", "MB", "GB", "TB"} {
		valor /= unidade
		sufixo = s
		if valor < unidade {
			break
		}
	}
	return fmt.Sprintf("%.1f %s", valor, sufixo)
}

type entradaListada struct {
	grupo int // 0 pastas, 1 o resto
	nome  string
	linha string
}

// listarPasta lista o conteúdo de uma pasta: pastas primeiro, depois o resto,
// cada grupo por nome sem diferenciar maiúscula. Link simbólico aparece como
// "nome -> destino" e NÃO é seguido.
func listarPasta(caminho string, incluirOcultos bool) (string, error) {
	info, err := os.Stat(caminho)
	if err != nil {
		return "", erroDeLeitura(caminho, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s é um arquivo, use ler", caminho)
	}
	itens, err := os.ReadDir(caminho)
	if err != nil {
		return "", erroDeLeitura(caminho, err)
	}

	var entradas []entradaListada
	for _, item := range itens {
		nome := item.Name()
		if !incluirOcultos && strings.HasPrefix(nome, ".") {
			continue
		}
		fi, err := item.Info()
		if err != nil {
			continue // sumiu entre o ReadDir e o Info
		}
		data := fi.ModTime().Format("2006-01-02 15:04")
		switch {
		case fi.Mode()&fs.ModeSymlink != 0:
			destino, _ := os.Readlink(caminho + string(os.PathSeparator) + nome)
			entradas = append(entradas, entradaListada{1, nome,
				fmt.Sprintf("link %10s  %s  %s -> %s", "-", data, nome, destino)})
		case fi.IsDir():
			entradas = append(entradas, entradaListada{0, nome,
				fmt.Sprintf("pasta %9s  %s  %s/", "-", data, nome)})
		default:
			entradas = append(entradas, entradaListada{1, nome,
				fmt.Sprintf("arquivo %7s  %s  %s", tamanhoLegivel(fi.Size()), data, nome)})
		}
	}
	if len(entradas) == 0 {
		return caminho + ": (pasta vazia)", nil
	}

	sort.SliceStable(entradas, func(a, b int) bool {
		if entradas[a].grupo != entradas[b].grupo {
			return entradas[a].grupo < entradas[b].grupo
		}
		return strings.ToLower(entradas[a].nome) < strings.ToLower(entradas[b].nome)
	})

	var saida strings.Builder
	fmt.Fprintf(&saida, "%s: %d entradas\n", caminho, len(entradas))
	for i, e := range entradas {
		if i == maxEntradasListadas {
			fmt.Fprintf(&saida, "… e mais %d entradas (use buscar para achar um nome)\n", len(entradas)-maxEntradasListadas)
			break
		}
		saida.WriteString(e.linha + "\n")
	}
	return strings.TrimRight(saida.String(), "\n"), nil
}

// --- ler -----------------------------------------------------------------------

const (
	linhasPadrao     = 200
	linhasMaximo     = 2000
	bytesPorChamada  = 32 * 1024
	bytesPorLinha    = 8 * 1024
	tetoDeLeitura    = 8 * 1024 * 1024
	bytesParaBinario = 8 * 1024
)

type resultadoLeitura struct {
	Texto   string
	Ocultos []string // valores que a máscara escondeu (o revelar os registra)
}

func cortarNoRune(s string, max int) string {
	if len(s) <= max {
		return s
	}
	corte := max
	for corte > 0 && !utf8.RuneStart(s[corte]) {
		corte--
	}
	return s[:corte]
}

// lerArquivo lê um arquivo de texto em páginas de linhas numeradas.
//
// A MÁSCARA (mascara.go) é aplicada ao arquivo todo ANTES de cortar em linhas:
// um bloco de chave privada que começa numa página não pode vazar o miolo na
// seguinte. Efeito colateral aceito: um bloco PEM vira uma linha só, então a
// numeração acompanha o texto mostrado, não o arquivo cru.
//
// revelar devolve o texto sem máscara (e os valores que seriam escondidos);
// só a ação revelar da ferramenta, que sempre pede aprovação, passa true.
func lerArquivo(caminho string, aPartirDaLinha, linhas int, revelar bool) (resultadoLeitura, error) {
	info, err := os.Stat(caminho)
	if err != nil {
		return resultadoLeitura{}, erroDeLeitura(caminho, err)
	}
	if info.IsDir() {
		return resultadoLeitura{}, fmt.Errorf("%s é uma pasta, use listar", caminho)
	}
	arquivo, err := os.Open(caminho)
	if err != nil {
		return resultadoLeitura{}, erroDeLeitura(caminho, err)
	}
	defer arquivo.Close()
	bruto, err := io.ReadAll(io.LimitReader(arquivo, tetoDeLeitura))
	if err != nil {
		return resultadoLeitura{}, erroDeLeitura(caminho, err)
	}
	if len(bruto) == 0 {
		return resultadoLeitura{Texto: caminho + ": (arquivo vazio)"}, nil
	}
	if bytes.IndexByte(bruto[:min(len(bruto), bytesParaBinario)], 0) >= 0 {
		return resultadoLeitura{Texto: fmt.Sprintf("%s: arquivo binário, não leio (%d bytes)", caminho, info.Size())}, nil
	}

	texto := strings.ToValidUTF8(string(bruto), "�")
	mascarado, ocultos := Mascarar(texto, CaminhoSensivel(caminho))
	if !revelar {
		texto = mascarado
	}

	todas := strings.Split(texto, "\n")
	if todas[len(todas)-1] == "" {
		todas = todas[:len(todas)-1]
	}
	total := len(todas)

	inicio := aPartirDaLinha
	if inicio < 1 {
		inicio = 1
	}
	if inicio > total {
		return resultadoLeitura{}, fmt.Errorf("%s só tem %d linhas", caminho, total)
	}
	limite := linhas
	if limite <= 0 {
		limite = linhasPadrao
	}
	if limite > linhasMaximo {
		limite = linhasMaximo
	}

	var saida strings.Builder
	fmt.Fprintf(&saida, "%s: %d linhas\n", caminho, total)
	usados, mostradas := 0, 0
	for i := inicio - 1; i < total && mostradas < limite; i++ {
		linha := strings.TrimSuffix(todas[i], "\r")
		if len(linha) > bytesPorLinha {
			linha = cortarNoRune(linha, bytesPorLinha) + fmt.Sprintf(" … [linha cortada: %d bytes]", len(todas[i]))
		}
		formatada := fmt.Sprintf("%6d  %s\n", i+1, linha)
		if mostradas > 0 && usados+len(formatada) > bytesPorChamada {
			break
		}
		saida.WriteString(formatada)
		usados += len(formatada)
		mostradas++
	}
	if proxima := inicio + mostradas; proxima <= total {
		fmt.Fprintf(&saida, "[continua na linha %d — use a_partir_da_linha: %d]\n", proxima, proxima)
	}
	if info.Size() > tetoDeLeitura {
		fmt.Fprintf(&saida, "[arquivo com %d bytes: li só os primeiros 8 MB]\n", info.Size())
	}
	return resultadoLeitura{Texto: strings.TrimRight(saida.String(), "\n"), Ocultos: ocultos}, nil
}
