package ferramentas

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sort"
	"strings"
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
