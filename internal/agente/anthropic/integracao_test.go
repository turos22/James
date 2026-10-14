//go:build integracao

package anthropic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"james/internal/agente"
	"james/internal/dispositivo"
	"james/internal/ferramentas"
	"james/internal/memoria"
)

// PORTÃO DO CA3 (plano da voz em tempo real, Task 5).
//
// O turno de voz roda com o pensamento DESLIGADO, porque é isso que leva o
// Haiku 5.5 de 1,5–2 s para ~0,7 s até a primeira frase. Mas o James vive de
// ferramentas, e a documentação registra, em outro modelo, um modo de falha
// com o pensamento desligado: o modelo escreve a chamada como TEXTO em vez de
// emitir um bloco tool_use. A chamada então nunca roda e nenhum erro aparece.
//
// Este teste chama o modelo de verdade, com o sistema.md real e as
// declarações reais das ferramentas, e exige tool_use em todos os pedidos.
//
// Roda só sob demanda (gasta frações de centavo):
//
//	JAMES_TESTE_ANTHROPIC=1 go test -tags integracao -count=1 -v -run Integracao ./internal/agente/anthropic/

// pedido liga o que o Arthur diria a uma ferramenta que PRECISA ser chamada.
type pedido struct {
	fala      string
	esperadas []string // qualquer uma destas serve
}

var pedidos = []pedido{
	{"Lista os arquivos da pasta atual.", []string{"executar_comando_sistema", "arquivos"}},
	{"Que horas são no sistema operacional?", []string{"executar_comando_sistema"}},
	{"Quanto espaço livre tem no disco?", []string{"executar_comando_sistema"}},
	{"Mostra os cinco processos que mais usam memória.", []string{"executar_comando_sistema"}},
	{"Pesquisa na internet a cotação do dólar hoje.", []string{"pesquisar_internet"}},
	{"Pesquisa a previsão do tempo em Florianópolis.", []string{"pesquisar_internet"}},
	{"Anota na memória que o meu time de futebol é o Flamengo.", []string{"salvar_na_memoria"}},
	{"O que tem na minha tela agora?", []string{"ver_tela"}},
	{"Clica no ponto 100, 100 da tela.", []string{"manipular_interface", "ver_tela"}},
	{"Pergunta ao Claude Code o que este projeto faz.", []string{"perguntar_ao_claude_code"}},
	{"Coloca o foco no Firefox.", []string{"janelas"}},
	{"Troca a música.", []string{"midia"}},
	{"Abre o Dolphin.", []string{"abrir"}},

	// Arquivos (spec 2026-10-14): olhar é por `arquivos`, mexer é por
	// `alterar_arquivos`, e NENHUM destes pode cair no terminal.
	{"Me mostra o que tem na minha pasta de projetos.", []string{"arquivos"}},
	{"Lê o arquivo ~/projetos/config.toml.", []string{"arquivos"}},
	{"Procura onde aparece a palavra jamesd nos meus arquivos em ~/projetos.", []string{"arquivos"}},
	{"O meu .env em ~/projetos/james tem a chave do Claude preenchida?", []string{"arquivos"}},
	{"Cria um arquivo ~/Documentos/teste.txt com a palavra oi.", []string{"alterar_arquivos"}},
	// Olhar o arquivo antes de editar é o comportamento certo: vale `arquivos` ou
	// `alterar_arquivos`; o que não vale é cair no terminal.
	{"Troca a palavra oi por olá no arquivo ~/Documentos/teste.txt.", []string{"arquivos", "alterar_arquivos"}},
	{"Apaga o arquivo ~/Documentos/teste.txt.", []string{"alterar_arquivos"}},
}

// marcasDeImitacao são o que aparece quando o modelo ESCREVE a chamada em vez
// de emiti-la.
var marcasDeImitacao = []string{"executar_comando_sistema(", "alterar_arquivos(", `{"comando"`, "<tool", "<function", "tool_use", "```json"}

func chaveDoEnv(t *testing.T) string {
	t.Helper()
	// godotenv.Read só lê o arquivo: não mexe no ambiente e não imprime nada.
	env, err := godotenv.Read(filepath.Join("..", "..", "..", ".env"))
	if err != nil {
		t.Skipf("sem .env legível: %v", err)
	}
	if env["CLAUDE_API_KEY"] == "" {
		t.Skip("CLAUDE_API_KEY ausente do .env")
	}
	return env["CLAUDE_API_KEY"]
}

func declaracoesReais(t *testing.T) []ferramentas.Declaracao {
	t.Helper()
	hub := dispositivo.NovoHub()
	vault := memoria.NovoVault(t.TempDir(), 2000)
	reg := ferramentas.NovoRegistro()
	for _, f := range []ferramentas.Ferramenta{
		ferramentas.NovoComandoSistema(),
		ferramentas.NovoClaudeCode(),
		ferramentas.NovaBusca("sem-chave", time.Second),
		ferramentas.NovoManipularInterface(hub),
		ferramentas.NovoVerTela(hub),
		ferramentas.NovoSalvarNaMemoria(vault),
		ferramentas.NovoJanelas(hub),
		ferramentas.NovoMidia(hub),
		ferramentas.NovoAbrir(hub),
		ferramentas.NovoArquivos(nil),
		ferramentas.NovoAlterarArquivos(),
	} {
		if err := reg.Registrar(f); err != nil {
			t.Fatalf("registrar: %v", err)
		}
	}
	return reg.Declaracoes()
}

func TestIntegracaoFerramentaComPensamentoDesligado(t *testing.T) {
	if os.Getenv("JAMES_TESTE_ANTHROPIC") != "1" {
		t.Skip("defina JAMES_TESTE_ANTHROPIC=1 para chamar a API de verdade")
	}
	sistema, err := os.ReadFile(filepath.Join("..", "..", "..", "prompts", "sistema.md"))
	if err != nil {
		t.Fatalf("sistema.md: %v", err)
	}
	decls := declaracoesReais(t)

	nomes := map[string]bool{}
	for _, d := range decls {
		nomes[d.Nome] = true
	}
	for _, p := range pedidos {
		for _, esperada := range p.esperadas {
			if !nomes[esperada] {
				t.Fatalf("o teste espera a ferramenta %q, que não existe no registro: %v", esperada, nomes)
			}
		}
	}

	provedor := Novo(chaveDoEnv(t), "claude-haiku-5-5")
	acertos := 0
	for i, p := range pedidos {
		ctx, cancelar := context.WithTimeout(context.Background(), 60*time.Second)
		inicio := time.Now()
		resp, err := provedor.Gerar(ctx, agente.Requisicao{
			Sistema:     string(sistema),
			Mensagens:   []agente.Mensagem{{Papel: agente.PapelUsuario, Texto: p.fala}},
			Declaracoes: decls,
			Modo:        agente.ModoVoz,
		})
		cancelar()
		if err != nil {
			t.Errorf("%2d. %q: erro %v", i+1, p.fala, err)
			continue
		}

		var chamadas []string
		for _, c := range resp.Chamadas {
			chamadas = append(chamadas, c.Nome)
		}
		t.Logf("%2d. %-58q → %v (%.2fs, pensamento=%d tok, texto=%q)",
			i+1, p.fala, chamadas, time.Since(inicio).Seconds(), resp.Uso.Pensamento, resp.Texto)

		ok := false
		for _, c := range resp.Chamadas {
			for _, esperada := range p.esperadas {
				if c.Nome == esperada {
					ok = true
				}
			}
		}
		if !ok {
			t.Errorf("%2d. %q: sem tool_use esperado %v (veio %v)", i+1, p.fala, p.esperadas, chamadas)
		}
		for _, marca := range marcasDeImitacao {
			if strings.Contains(resp.Texto, marca) {
				t.Errorf("%2d. %q: o texto imita uma chamada (%q): %q", i+1, p.fala, marca, resp.Texto)
			}
		}
		if ok {
			acertos++
		}
	}
	t.Logf("RESULTADO DO PORTÃO: %d de %d pedidos viraram tool_use", acertos, len(pedidos))
}
