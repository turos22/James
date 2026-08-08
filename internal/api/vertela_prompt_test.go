package api

import (
	"os"
	"strings"
	"testing"
)

// O item de visão do system prompt instruía o modelo a analisar um print
// que nada produzia — o encanamento existia, mas nenhum caminho o
// alimentava. Agora que ver_tela existe, o prompt tem que nomear a
// ferramenta, senão continua sendo instrução sem referente.
func TestSystemPromptNomeiaVerTela(t *testing.T) {
	conteudo, err := os.ReadFile("../../prompts/sistema.md")
	if err != nil {
		t.Fatalf("erro ao ler o system prompt: %v", err)
	}

	texto := string(conteudo)
	if !strings.Contains(texto, "ver_tela") {
		t.Error("o system prompt não menciona ver_tela")
	}
	if !strings.Contains(texto, "manipular_interface, chame ver_tela") {
		t.Error("o system prompt não liga ver_tela a manipular_interface — a sequência é o que evita o palpite de coordenada")
	}
}
