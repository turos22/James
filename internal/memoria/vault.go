package memoria

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// Vault é a memória durável do James: arquivos markdown, um por assunto,
// que entram inteiros no system prompt a cada turno.
//
// Não confundir com Janela: aquela corta a conversa recente por orçamento de
// token, esta guarda fato durável sobre o Arthur. Os dois orçamentos são
// separados de propósito — nota nenhuma deve sumir porque a conversa ficou
// grande.
type Vault struct {
	Raiz            string
	OrcamentoTokens int

	mu sync.Mutex
}

func NovoVault(raiz string, orcamentoTokens int) *Vault {
	return &Vault{Raiz: raiz, OrcamentoTokens: orcamentoTokens}
}

// Acrescentar adiciona uma linha ao arquivo do assunto.
//
// O_APPEND em vez de ler-modificar-gravar porque o WebSocket e o device de
// voz escrevem pelo mesmo processarTurno, e na nuvem os dois aplicativos
// fazem o mesmo: append não perde escrita concorrente, ler-modificar-gravar
// perde.
func (v *Vault) Acrescentar(assunto, texto string) error {
	// Achatar espaço em branco mantém uma nota por linha. Nota multilinha
	// quebraria o formato de lista e, no corte por orçamento, deixaria meia
	// nota no bloco.
	texto = strings.Join(strings.Fields(texto), " ")
	if texto == "" {
		return fmt.Errorf("texto da nota vazio")
	}

	caminho, err := ResolverCaminho(v.Raiz, assunto)
	if err != nil {
		return err
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if err := os.MkdirAll(v.Raiz, 0o755); err != nil {
		return fmt.Errorf("erro ao criar diretório de memória: %w", err)
	}

	arq, err := os.OpenFile(caminho, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("erro ao abrir nota: %w", err)
	}
	defer arq.Close()

	if _, err := fmt.Fprintf(arq, "- %s\n", texto); err != nil {
		return fmt.Errorf("erro ao gravar nota: %w", err)
	}
	return nil
}

// Assuntos lista os assuntos existentes, em ordem alfabética. Consumido pela
// declaração de salvar_na_memoria: sem essa lista, o modelo inventa assunto
// novo a cada chamada em vez de acrescentar num existente.
//
// Vault ausente não é erro: é o estado do primeiro boot.
func (v *Vault) Assuntos() ([]string, error) {
	entradas, err := os.ReadDir(v.Raiz)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("erro ao ler diretório de memória: %w", err)
	}

	var assuntos []string
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		assuntos = append(assuntos, strings.TrimSuffix(e.Name(), ".md"))
	}
	sort.Strings(assuntos)
	return assuntos, nil
}
