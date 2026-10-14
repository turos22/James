package api

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"

	"james/internal/agente"
	"james/internal/memoria"
	"james/internal/permissao"
)

// Achado do aceite da autonomia de arquivos (2026-10-14): numa conversa que
// passa do orçamento da janela, persistirNovasMensagens recebia o tamanho do
// histórico COMPLETO como ponto de partida, mas o loop devolve as mensagens a
// partir da JANELA (mais curta). Resultado: `todas[24:]` num slice de 9 e um
// panic (a conexão caía e o app apagava a mensagem), ou, quando o contador não
// passava do fim, mensagens novas puladas sem erro nenhum. Bug de 2026-08-04
// que as ferramentas de arquivo, com saídas grandes, tornaram fácil de bater.
func TestTurnoDepoisQueAJanelaEncurtaPersisteExatamenteOQueEhNovo(t *testing.T) {
	_, s := servidorComDispositivoDeTeste(t)
	s.Memoria = memoria.Janela{OrcamentoTokens: 40} // cada mensagem abaixo estoura sozinha
	s.Loop.Provider = &providerFixo{texto: "resposta do James"}
	ctx := context.Background()

	var sessaoID uuid.UUID
	var historico []agente.Mensagem
	for i := 1; i <= 6; i++ {
		pergunta := fmt.Sprintf("pergunta %d: %s", i, strings.Repeat("texto longo para estourar o orçamento da janela ", 12))
		if _, _, err := s.processarTurno(ctx, agente.ModoTexto, permissao.Privilegiado, &sessaoID, &historico, pergunta, nil, nil, nil); err != nil {
			t.Fatalf("turno %d: %v", i, err)
		}
	}

	linhas, err := s.Mensagens.ListarPorSessao(ctx, sessaoID, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(linhas) != 12 {
		var papeis []string
		for _, l := range linhas {
			papeis = append(papeis, l.Papel)
		}
		t.Fatalf("%d mensagens no banco, queria 12 (6 perguntas e 6 respostas): %v", len(linhas), papeis)
	}
	for i, l := range linhas {
		querPapel := "usuario"
		if i%2 == 1 {
			querPapel = "modelo"
		}
		if l.Papel != querPapel {
			t.Errorf("mensagem %d tem papel %q, queria %q", i, l.Papel, querPapel)
		}
		if i%2 == 0 && !strings.HasPrefix(l.Conteudo, fmt.Sprintf("pergunta %d:", i/2+1)) {
			t.Errorf("mensagem %d = %.30q..., queria a pergunta %d", i, l.Conteudo, i/2+1)
		}
	}
}
