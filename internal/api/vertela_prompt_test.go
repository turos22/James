package api

import (
	"os"
	"strconv"
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

// Spec 2026-10-14 arquivos, seção 6. O prompt é a única coisa que diz ao modelo
// que criar, editar e apagar arquivo pede aprovação e que arquivo se olha pela
// ferramenta (com a máscara), não por cat.
func lerPrompt(t *testing.T) string {
	t.Helper()
	conteudo, err := os.ReadFile("../../prompts/sistema.md")
	if err != nil {
		t.Fatalf("erro ao ler o system prompt: %v", err)
	}
	return strings.ReplaceAll(string(conteudo), "\r\n", "\n")
}

func TestSystemPromptNomeiaAsFerramentasDeArquivo(t *testing.T) {
	texto := lerPrompt(t)

	for _, esperado := range []string{"arquivos", "alterar_arquivos", "revelar", "••••"} {
		if !strings.Contains(texto, esperado) {
			t.Errorf("o system prompt não menciona %q", esperado)
		}
	}
	if !strings.Contains(texto, "nunca cat") && !strings.Contains(texto, "não use cat") {
		t.Error("o system prompt não manda ler arquivo pela ferramenta em vez de cat/ls/find")
	}
}

func TestSystemPromptNaoMandaMaisCriarEEscreverSemPerguntar(t *testing.T) {
	texto := lerPrompt(t)

	if strings.Contains(texto, "criar, escrever, abrir") {
		t.Error("a regra 1 ainda manda criar e escrever sem perguntar: contradiz a aprovação dos arquivos")
	}
	if !strings.Contains(texto, "aprovação") {
		t.Error("o system prompt não diz que mexer em arquivo pede aprovação")
	}
}

func TestSystemPromptProibeGuardarConteudoDeArquivoNaMemoria(t *testing.T) {
	texto := lerPrompt(t)
	i := strings.Index(texto, "UMA FRASE, SUAS PALAVRAS")
	if i < 0 {
		t.Fatal("a regra de memória sumiu")
	}
	regra := texto[i:]
	if fim := strings.Index(regra, "\n9. "); fim > 0 {
		regra = regra[:fim]
	}
	if !strings.Contains(regra, "arquivos") {
		t.Error("a regra de memória não inclui a ferramenta arquivos na lista do que não vai para a memória")
	}
}

func TestSystemPromptTemAsRegrasNumeradasEmSequencia(t *testing.T) {
	texto := lerPrompt(t)

	for n := 1; n <= 10; n++ {
		if !strings.Contains(texto, "\n"+strconv.Itoa(n)+". ") {
			t.Errorf("falta a regra %d (renumeração errada?)", n)
		}
	}
	if strings.Contains(texto, "\n11. ") {
		t.Error("há uma regra 11 inesperada")
	}
}
