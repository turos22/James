package ferramentas

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Escrita de arquivos (spec 2026-10-14 arquivos, D10 e D16): criar e editar.
// Sempre atômica: temporário na mesma pasta e rename, modo preservado.
//
// Estas funções não pedem aprovação nem conferem o piso: isso é da ferramenta
// alterar_arquivos. Aqui mora só o que acontece no disco, e cada operação tem
// uma versão "validar" que diz o que faria SEM escrever, para o cartão de
// aprovação e para a reconferência na hora de executar.

const tetoDeEscrita = 1024 * 1024

// renomear é trocável nos testes para provar a atomicidade.
var renomear = os.Rename

// alvoReal resolve link simbólico: editar ou sobrescrever um link mexe no
// destino e mantém o link. Link quebrado é erro.
func alvoReal(caminho string) (string, error) {
	info, err := os.Lstat(caminho)
	if err != nil {
		return caminho, nil // não existe: o chamador decide
	}
	if info.Mode()&fs.ModeSymlink == 0 {
		return caminho, nil
	}
	real, err := filepath.EvalSymlinks(caminho)
	if err != nil {
		return "", fmt.Errorf("%s é um link quebrado", caminho)
	}
	return real, nil
}

func escreverAtomico(destino string, dados []byte, modo fs.FileMode) error {
	temporario, err := os.CreateTemp(filepath.Dir(destino), ".james-*")
	if err != nil {
		return erroDeEscrita(destino, err)
	}
	nome := temporario.Name()
	pronto := false
	defer func() {
		if !pronto {
			temporario.Close()
			os.Remove(nome)
		}
	}()
	if _, err := temporario.Write(dados); err != nil {
		return erroDeEscrita(destino, err)
	}
	if err := temporario.Chmod(modo); err != nil {
		return erroDeEscrita(destino, err)
	}
	if err := temporario.Sync(); err != nil {
		return erroDeEscrita(destino, err)
	}
	if err := temporario.Close(); err != nil {
		return erroDeEscrita(destino, err)
	}
	if err := renomear(nome, destino); err != nil {
		return erroDeEscrita(destino, err)
	}
	pronto = true
	return nil
}

func erroDeEscrita(caminho string, err error) error {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("sem permissão para escrever em %s", caminho)
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("não existe: %s", filepath.Dir(caminho))
	default:
		return fmt.Errorf("não consegui escrever %s: %v", caminho, err)
	}
}

// --- criar ---------------------------------------------------------------------

// validarCriar confere o pedido de criar sem escrever nada. Devolve quantos
// bytes seriam gravados e se isso sobrescreve um arquivo que já existe.
func validarCriar(caminho, conteudo string, sobrescrever, criarPastas bool) (tamanho int, sobrescreve bool, err error) {
	if len(conteudo) > tetoDeEscrita {
		return 0, false, fmt.Errorf("conteúdo acima de 1 MB (%d bytes); divida em arquivos menores", len(conteudo))
	}
	alvo, err := alvoReal(caminho)
	if err != nil {
		return 0, false, err
	}
	info, err := os.Stat(alvo)
	switch {
	case err == nil && info.IsDir():
		return 0, false, fmt.Errorf("%s é uma pasta", caminho)
	case err == nil && !sobrescrever:
		return 0, false, fmt.Errorf("%s já existe; use sobrescrever: true se for isso mesmo", caminho)
	case err == nil:
		return len(conteudo), true, nil
	case !errors.Is(err, fs.ErrNotExist):
		return 0, false, erroDeEscrita(caminho, err)
	}

	pai := filepath.Dir(alvo)
	infoPai, err := os.Stat(pai)
	switch {
	case err == nil && !infoPai.IsDir():
		return 0, false, fmt.Errorf("%s não é uma pasta", pai)
	case errors.Is(err, fs.ErrNotExist) && !criarPastas:
		return 0, false, fmt.Errorf("a pasta %s não existe; use criar_pastas: true para criá-la", pai)
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return 0, false, erroDeEscrita(caminho, err)
	}
	return len(conteudo), false, nil
}

func criarArquivo(caminho, conteudo string, sobrescrever, criarPastas bool) (string, error) {
	n, sobrescreve, err := validarCriar(caminho, conteudo, sobrescrever, criarPastas)
	if err != nil {
		return "", err
	}
	alvo, err := alvoReal(caminho)
	if err != nil {
		return "", err
	}
	modo := fs.FileMode(0o644)
	if sobrescreve {
		info, err := os.Stat(alvo)
		if err != nil {
			return "", erroDeEscrita(caminho, err)
		}
		modo = info.Mode().Perm()
	} else if criarPastas {
		if err := os.MkdirAll(filepath.Dir(alvo), 0o755); err != nil {
			return "", erroDeEscrita(caminho, err)
		}
	}
	if err := escreverAtomico(alvo, []byte(conteudo), modo); err != nil {
		return "", err
	}
	verbo := "criei"
	if sobrescreve {
		verbo = "sobrescrevi"
	}
	return fmt.Sprintf("%s %s (%d bytes)", verbo, caminho, n), nil
}

// --- editar --------------------------------------------------------------------

type edicao struct {
	alvo   string
	modo   fs.FileMode
	novo   string
	trocas int
}

// prepararEdicao lê o arquivo e calcula o texto novo, sem escrever. É a
// validação do editar: a ferramenta a chama antes do cartão e de novo na hora
// de executar, porque o arquivo pode ter mudado no meio.
func prepararEdicao(caminho, antigo, novo string, todas bool) (edicao, error) {
	if antigo == "" {
		return edicao{}, errors.New("trecho_antigo vazio: diga qual trecho trocar")
	}
	alvo, err := alvoReal(caminho)
	if err != nil {
		return edicao{}, err
	}
	info, err := os.Stat(alvo)
	if err != nil {
		return edicao{}, erroDeLeitura(caminho, err)
	}
	if info.IsDir() {
		return edicao{}, fmt.Errorf("%s é uma pasta", caminho)
	}
	if info.Size() > tetoDeEscrita {
		return edicao{}, fmt.Errorf("%s tem mais de 1 MB; não edito arquivo desse tamanho", caminho)
	}
	bruto, err := os.ReadFile(alvo)
	if err != nil {
		return edicao{}, erroDeLeitura(caminho, err)
	}
	if bytes.IndexByte(bruto[:min(len(bruto), bytesParaBinario)], 0) >= 0 {
		return edicao{}, fmt.Errorf("%s é um arquivo binário; não edito", caminho)
	}
	texto := string(bruto)

	n := strings.Count(texto, antigo)
	if n == 0 && strings.Contains(texto, "\r\n") && strings.Contains(antigo, "\n") && !strings.Contains(antigo, "\r\n") {
		// Arquivo CRLF e trecho escrito com \n: acha com \r\n e mantém o CRLF.
		antigo = strings.ReplaceAll(antigo, "\n", "\r\n")
		if !strings.Contains(novo, "\r\n") {
			novo = strings.ReplaceAll(novo, "\n", "\r\n")
		}
		n = strings.Count(texto, antigo)
	}
	switch {
	case n == 0:
		return edicao{}, fmt.Errorf("o trecho não aparece no arquivo (0 ocorrências)")
	case n > 1 && !todas:
		return edicao{}, fmt.Errorf("o trecho aparece %d vezes; seja mais específico ou use todas: true", n)
	}
	limite := 1
	if todas {
		limite = n
	}
	return edicao{alvo: alvo, modo: info.Mode().Perm(), novo: strings.Replace(texto, antigo, novo, limite), trocas: limite}, nil
}

// validarEditar diz quantos trechos seriam trocados, sem escrever.
func validarEditar(caminho, antigo, novo string, todas bool) (int, error) {
	e, err := prepararEdicao(caminho, antigo, novo, todas)
	return e.trocas, err
}

func editarArquivo(caminho, antigo, novo string, todas bool) (string, error) {
	e, err := prepararEdicao(caminho, antigo, novo, todas)
	if err != nil {
		return "", err
	}
	if len(e.novo) > tetoDeEscrita {
		return "", errors.New("o arquivo editado passaria de 1 MB")
	}
	if err := escreverAtomico(e.alvo, []byte(e.novo), e.modo); err != nil {
		return "", err
	}
	palavra := "trecho"
	if e.trocas != 1 {
		palavra = "trechos"
	}
	return fmt.Sprintf("editei %s: %d %s trocado(s)", caminho, e.trocas, palavra), nil
}
