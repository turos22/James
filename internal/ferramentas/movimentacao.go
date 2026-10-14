package ferramentas

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Mover, copiar e apagar (spec 2026-10-14 arquivos, D10, D12, D13).
//
// Como em escrita.go, aqui só mora o que acontece no disco. Cada operação tem
// uma versão "validar", sem efeito, que a ferramenta chama antes do cartão de
// aprovação e de novo na hora de executar.

// Teto da cópia de pasta, medido ANTES de começar (falhar no meio deixaria
// meia pasta). Variáveis para os testes não precisarem de 5000 arquivos.
var (
	maxArquivosNaCopia = 5000
	maxBytesNaCopia    = int64(500 * 1024 * 1024)
)

// renomearParaMover é trocável nos testes para simular outro sistema de arquivos.
var renomearParaMover = os.Rename

func pastaProtegidaErro(acao, caminho string) error {
	return fmt.Errorf("%s é uma pasta protegida (raiz, home ou pasta de sistema): não %s", caminho, acao)
}

func dentroDe(caminho, pasta string) bool {
	return strings.HasPrefix(filepath.Clean(caminho)+string(filepath.Separator), filepath.Clean(pasta)+string(filepath.Separator))
}

func conferirDestino(origem, destino string, infoOrigem fs.FileInfo) error {
	if _, err := os.Lstat(destino); err == nil {
		return fmt.Errorf("o destino já existe: %s", destino)
	}
	pai := filepath.Dir(destino)
	if info, err := os.Stat(pai); err != nil || !info.IsDir() {
		return fmt.Errorf("a pasta do destino não existe: %s", pai)
	}
	if infoOrigem.IsDir() && dentroDe(destino, origem) {
		return errors.New("não dá para pôr uma pasta dentro de si mesma")
	}
	return nil
}

func existeOuErro(caminho string) (fs.FileInfo, error) {
	info, err := os.Lstat(caminho)
	if err != nil {
		return nil, erroDeLeitura(caminho, err)
	}
	return info, nil
}

// --- mover ---------------------------------------------------------------------

func validarMover(origem, destino string) error {
	info, err := existeOuErro(origem)
	if err != nil {
		return err
	}
	if CaminhoProtegido(origem) {
		return pastaProtegidaErro("movo", origem)
	}
	return conferirDestino(origem, destino, info)
}

func moverArquivo(origem, destino string) (string, error) {
	if err := validarMover(origem, destino); err != nil {
		return "", err
	}
	err := renomearParaMover(origem, destino)
	if err == nil {
		return fmt.Sprintf("movi %s → %s", origem, destino), nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		return "", fmt.Errorf("não consegui mover %s: %v", origem, err)
	}

	// Outro sistema de arquivos: copia, CONFERE, e só então remove a origem. Se
	// qualquer passo falhar, a origem fica onde estava e a cópia parcial sai.
	if err := copiarSemValidar(origem, destino); err != nil {
		os.RemoveAll(destino)
		return "", err
	}
	if !copiaConfere(origem, destino) {
		os.RemoveAll(destino)
		return "", errors.New("a cópia não bateu com o original; não apaguei a origem")
	}
	if err := os.RemoveAll(origem); err != nil {
		return "", fmt.Errorf("copiei para %s, mas não consegui remover a origem: %v", destino, err)
	}
	return fmt.Sprintf("movi %s → %s", origem, destino), nil
}

func copiaConfere(origem, destino string) bool {
	nA, bA, errA := medirArvore(origem)
	nB, bB, errB := medirArvore(destino)
	return errA == nil && errB == nil && nA == nB && bA == bB
}

// --- copiar --------------------------------------------------------------------

// medirArvore conta itens que não são pasta (arquivos e links) e os bytes dos
// arquivos comuns. Não segue links.
func medirArvore(raiz string) (arquivos int, bytes int64, err error) {
	err = filepath.WalkDir(raiz, func(caminho string, d fs.DirEntry, errAndar error) error {
		if errAndar != nil {
			return errAndar
		}
		if d.IsDir() {
			return nil
		}
		arquivos++
		if info, err := d.Info(); err == nil && info.Mode().IsRegular() {
			bytes += info.Size()
		}
		return nil
	})
	return arquivos, bytes, err
}

func validarCopiar(origem, destino string) error {
	info, err := existeOuErro(origem)
	if err != nil {
		return err
	}
	if err := conferirDestino(origem, destino, info); err != nil {
		return err
	}
	if info.IsDir() {
		n, bytes, err := medirArvore(origem)
		if err != nil {
			return erroDeLeitura(origem, err)
		}
		if n > maxArquivosNaCopia {
			return fmt.Errorf("a pasta tem %d arquivos, acima do limite de cópia de %d", n, maxArquivosNaCopia)
		}
		if bytes > maxBytesNaCopia {
			return fmt.Errorf("a pasta tem %s, acima do limite de cópia de %s", tamanhoLegivel(bytes), tamanhoLegivel(maxBytesNaCopia))
		}
	}
	return nil
}

func copiarArquivo(origem, destino string) (string, error) {
	if err := validarCopiar(origem, destino); err != nil {
		return "", err
	}
	if err := copiarSemValidar(origem, destino); err != nil {
		return "", err
	}
	if info, _ := os.Lstat(origem); info != nil && info.IsDir() {
		n, _, _ := medirArvore(destino)
		return fmt.Sprintf("copiei %s → %s (%d arquivos)", origem, destino, n), nil
	}
	return fmt.Sprintf("copiei %s → %s", origem, destino), nil
}

func copiarSemValidar(origem, destino string) error {
	info, err := os.Lstat(origem)
	if err != nil {
		return erroDeLeitura(origem, err)
	}
	switch {
	case info.Mode()&fs.ModeSymlink != 0:
		return copiarLink(origem, destino)
	case info.IsDir():
		return copiarPasta(origem, destino, info.Mode().Perm())
	default:
		return copiarArquivoComum(origem, destino, info.Mode().Perm())
	}
}

func copiarLink(origem, destino string) error {
	alvo, err := os.Readlink(origem)
	if err != nil {
		return erroDeLeitura(origem, err)
	}
	if err := os.Symlink(alvo, destino); err != nil {
		return erroDeEscrita(destino, err)
	}
	return nil
}

// copiarArquivoComum copia em fluxo (a cópia pode ter centenas de MB) para um
// temporário e só então troca de nome: queda no meio não deixa arquivo pela
// metade no destino.
func copiarArquivoComum(origem, destino string, modo fs.FileMode) error {
	entrada, err := os.Open(origem)
	if err != nil {
		return erroDeLeitura(origem, err)
	}
	defer entrada.Close()
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
	if _, err := io.Copy(temporario, entrada); err != nil {
		return erroDeEscrita(destino, err)
	}
	if err := temporario.Chmod(modo); err != nil {
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

func copiarPasta(origem, destino string, modo fs.FileMode) error {
	return filepath.WalkDir(origem, func(caminho string, d fs.DirEntry, errAndar error) error {
		if errAndar != nil {
			return erroDeLeitura(caminho, errAndar)
		}
		rel, _ := filepath.Rel(origem, caminho)
		alvo := filepath.Join(destino, rel)
		info, err := d.Info()
		if err != nil {
			return erroDeLeitura(caminho, err)
		}
		switch {
		case d.IsDir():
			// A pasta nova precisa ser gravável por quem copia, mesmo que a
			// original não seja.
			if err := os.MkdirAll(alvo, info.Mode().Perm()|0o700); err != nil {
				return erroDeEscrita(alvo, err)
			}
			return nil
		case info.Mode()&fs.ModeSymlink != 0:
			return copiarLink(caminho, alvo)
		default:
			return copiarArquivoComum(caminho, alvo, info.Mode().Perm())
		}
	})
}

// --- apagar --------------------------------------------------------------------

func validarApagar(caminho string) error {
	if _, err := existeOuErro(caminho); err != nil {
		return err
	}
	if CaminhoProtegido(caminho) {
		return pastaProtegidaErro("apago", caminho)
	}
	return nil
}

// apagarArquivo manda para a lixeira. Se a lixeira falhar (sistema de arquivos
// sem lixeira, como /tmp), o erro volta e o arquivo CONTINUA onde estava:
// nenhum caminho daqui chama os.Remove.
func apagarArquivo(ctx context.Context, caminho string) (string, error) {
	if err := validarApagar(caminho); err != nil {
		return "", err
	}
	if err := executarLixeira(ctx, caminho); err != nil {
		return "", fmt.Errorf("não consegui mandar %s para a lixeira (%v); o arquivo continua onde estava", caminho, err)
	}
	return "mandei para a lixeira: " + caminho, nil
}
