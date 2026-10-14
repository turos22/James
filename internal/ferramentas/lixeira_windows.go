package ferramentas

import (
	"context"
	"errors"
)

// No Windows apagar não existe nesta fatia (spec 2026-10-14 arquivos, D18):
// as ferramentas de arquivo nem são registradas lá. A variável existe para o
// pacote compilar e para o erro, se alguém chamar, ser claro.
var executarLixeira = func(context.Context, string) error {
	return errors.New("indisponível neste sistema (Windows)")
}
