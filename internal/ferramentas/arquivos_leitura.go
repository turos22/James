package ferramentas

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
	if !info.Mode().IsRegular() {
		// Abrir um pipe com nome bloqueia até alguém escrever nele.
		return resultadoLeitura{}, fmt.Errorf("%s não é um arquivo comum (pipe, dispositivo ou socket); não leio", caminho)
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

// --- buscar --------------------------------------------------------------------

const (
	maxResultadosDaBusca = 100
	profundidadeDaBusca  = 8
	tamanhoMaxParaBuscar = 2 * 1024 * 1024
	prazoDaBusca         = 5 * time.Second
	larguraDoTrecho      = 160
)

// Pastas que quase nunca são o que o Arthur quer varrer. Só são puladas
// quando o caminho pedido não está dentro de uma delas.
var pastasDeCache = map[string]bool{
	".git": true, "node_modules": true, ".venv": true, "__pycache__": true, ".cache": true, "vendor": true,
}

type opcoesBusca struct {
	Nome           string
	Conteudo       string
	IncluirOcultos bool

	agora func() time.Time // relógio injetável nos testes
}

func casaNome(nome, pedido string) bool {
	nome, pedido = strings.ToLower(nome), strings.ToLower(pedido)
	if strings.ContainsAny(pedido, "*?[") {
		ok, err := filepath.Match(pedido, nome)
		return err == nil && ok
	}
	return strings.Contains(nome, pedido)
}

func raizDentroDeCache(raiz string) bool {
	for _, parte := range strings.Split(filepath.ToSlash(raiz), "/") {
		if pastasDeCache[parte] {
			return true
		}
	}
	return false
}

// linhasQueCasam procura o texto no arquivo JÁ MASCARADO. Buscar no texto
// cru e mascarar só a linha devolvida viraria um oráculo: o modelo
// perguntaria "o .env contém tal valor?" e a resposta (achou ou não achou)
// confirmaria o palpite. No texto mascarado, o valor de um segredo não é
// encontrável por busca nenhuma.
func linhasQueCasam(caminho string, tamanho int64, conteudo string) []string {
	if tamanho > tamanhoMaxParaBuscar {
		return nil
	}
	bruto, err := os.ReadFile(caminho)
	if err != nil || bytes.IndexByte(bruto[:min(len(bruto), bytesParaBinario)], 0) >= 0 {
		return nil
	}
	texto, _ := Mascarar(strings.ToValidUTF8(string(bruto), "�"), CaminhoSensivel(caminho))
	procurado := strings.ToLower(conteudo)
	var achadas []string
	for i, linha := range strings.Split(texto, "\n") {
		if !strings.Contains(strings.ToLower(linha), procurado) {
			continue
		}
		linha = strings.TrimSpace(linha)
		if utf8.RuneCountInString(linha) > larguraDoTrecho {
			linha = string([]rune(linha)[:larguraDoTrecho]) + "…"
		}
		achadas = append(achadas, fmt.Sprintf("%s:%d: %s", caminho, i+1, linha))
	}
	return achadas
}

// buscarArquivos procura por nome (trecho ou glob), por conteúdo, ou pelos dois
// (o nome filtra os arquivos e o conteúdo é procurado dentro deles).
func buscarArquivos(raiz string, o opcoesBusca) (string, error) {
	if strings.TrimSpace(o.Nome) == "" && strings.TrimSpace(o.Conteudo) == "" {
		return "", errors.New("informe nome ou conteudo para buscar")
	}
	info, err := os.Stat(raiz)
	if err != nil {
		return "", erroDeLeitura(raiz, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s é um arquivo, use ler", raiz)
	}
	agora := o.agora
	if agora == nil {
		agora = time.Now
	}
	inicio := agora()
	pularCache := !raizDentroDeCache(raiz)

	var resultados []string
	cortadaPorTempo, cortadaPorQuantidade := false, false

	err = filepath.WalkDir(raiz, func(caminho string, d fs.DirEntry, err error) error {
		if err != nil {
			if d != nil && d.IsDir() && caminho != raiz {
				return filepath.SkipDir // pasta sem permissão no meio: pula
			}
			if caminho == raiz {
				return err
			}
			return nil
		}
		if agora().Sub(inicio) > prazoDaBusca {
			cortadaPorTempo = true
			return filepath.SkipAll
		}
		if caminho == raiz {
			return nil
		}
		nome := d.Name()
		rel, _ := filepath.Rel(raiz, caminho)
		if strings.Count(filepath.ToSlash(rel), "/")+1 > profundidadeDaBusca {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !o.IncluirOcultos && strings.HasPrefix(nome, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil // não segue links: evita ciclo e fuga para fora da pasta pedida
		}
		if d.IsDir() {
			if pularCache && pastasDeCache[nome] {
				return filepath.SkipDir
			}
			if o.Conteudo == "" && o.Nome != "" && casaNome(nome, o.Nome) {
				resultados = append(resultados, "pasta    "+caminho+"/")
			}
			return nil
		}
		if o.Nome != "" && !casaNome(nome, o.Nome) {
			return nil
		}
		if o.Conteudo != "" && !d.Type().IsRegular() {
			return nil // ler o conteúdo de um pipe trava a busca
		}
		if o.Conteudo == "" {
			resultados = append(resultados, "arquivo  "+caminho)
		} else if fi, err := d.Info(); err == nil {
			resultados = append(resultados, linhasQueCasam(caminho, fi.Size(), o.Conteudo)...)
		}
		if len(resultados) >= maxResultadosDaBusca {
			cortadaPorQuantidade = true
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", erroDeLeitura(raiz, err)
	}

	if len(resultados) == 0 {
		return "nenhum resultado em " + raiz, nil
	}
	if len(resultados) > maxResultadosDaBusca {
		resultados = resultados[:maxResultadosDaBusca]
	}
	saida := strings.Join(resultados, "\n")
	if cortadaPorQuantidade {
		saida += fmt.Sprintf("\n[busca cortada em %d resultados: refine o pedido]", maxResultadosDaBusca)
	}
	if cortadaPorTempo {
		saida += "\n[busca cortada: passou de 5 s; refine o pedido ou procure numa pasta menor]"
	}
	return saida, nil
}
