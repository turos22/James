package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"james/internal/config"
	"james/internal/ferramentas"
	"james/internal/memoria"
)

// TestConstruirComandoSistemaAplicaConfig é o teste que a Task 8 deixou sem
// rede de segurança: prova que Timeout e MaxBytesSaida chegam com o valor do
// config, não zero, sem precisar subir o jamesd inteiro (executar() abre
// Postgres de verdade e bloqueia num canal que só retorna com o servidor
// caindo — não dá pra chamar de teste).
func TestConstruirComandoSistemaAplicaConfig(t *testing.T) {
	cfg := &config.Config{
		TerminalTimeoutSegundos: 42,
		TerminalMaxBytesSaida:   999,
	}

	c := construirComandoSistema(cfg)

	if c.Timeout != 42*time.Second {
		t.Errorf("Timeout = %s, queria %s", c.Timeout, 42*time.Second)
	}
	if c.MaxBytesSaida != 999 {
		t.Errorf("MaxBytesSaida = %d, queria 999", c.MaxBytesSaida)
	}
}

// TestConstruirBuscaAplicaConfig prova que Cliente.Timeout e MaxResultados
// chegam com o valor do config, não zero — mesma falha silenciosa que
// motivou a extração em TestConstruirComandoSistemaAplicaConfig: a ferramenta
// funciona do mesmo jeito, só que sem timeout e sem limite, e compilar não
// acusa nada.
func TestConstruirBuscaAplicaConfig(t *testing.T) {
	cfg := &config.Config{
		TavilyAPIKey:         "chave-de-teste",
		BuscaTimeoutSegundos: 15,
		BuscaMaxResultados:   3,
	}

	b := construirBusca(cfg)

	if b == nil {
		t.Fatal("construirBusca devolveu nil")
	}
	if b.Cliente.Timeout != 15*time.Second {
		t.Errorf("Cliente.Timeout = %s, queria %s", b.Cliente.Timeout, 15*time.Second)
	}
	if b.MaxResultados != 3 {
		t.Errorf("MaxResultados = %d, queria 3", b.MaxResultados)
	}
}

// TestConstruirClaudeCodeAplicaConfig prova que Diretorio e Timeout chegam
// com o valor do config — mesma falha silenciosa das duas ferramentas
// anteriores: sem isso, a verificação ao vivo da Task 17 falhou com
// "diretório de trabalho do Claude Code não configurado" porque
// construirClaudeCode ainda era só o ponto de costura, sem aplicar nada.
func TestConstruirClaudeCodeAplicaConfig(t *testing.T) {
	cfg := &config.Config{
		ClaudeCodeDiretorio:       "C:\\projetos\\alvo",
		ClaudeCodeTimeoutSegundos: 120,
	}

	c := construirClaudeCode(cfg)

	if c == nil {
		t.Fatal("construirClaudeCode devolveu nil")
	}
	if c.Diretorio != "C:\\projetos\\alvo" {
		t.Errorf("Diretorio = %q, queria C:\\\\projetos\\\\alvo", c.Diretorio)
	}
	if c.Timeout != 120*time.Second {
		t.Errorf("Timeout = %s, queria %s", c.Timeout, 120*time.Second)
	}
}

func TestRegistrarTerminalNaoRegistraQuandoNaoSuportado(t *testing.T) {
	registro := ferramentas.NovoRegistro()
	if err := registrarTerminal(registro, &config.Config{}, false); err != nil {
		t.Fatalf("registrarTerminal devolveu erro: %v", err)
	}
	if _, ok := registro.Buscar("executar_comando_sistema"); ok {
		t.Error("terminal registrado numa plataforma sem suporte")
	}
}

func TestRegistrarTerminalRegistraQuandoSuportado(t *testing.T) {
	registro := ferramentas.NovoRegistro()
	if err := registrarTerminal(registro, &config.Config{}, true); err != nil {
		t.Fatalf("registrarTerminal devolveu erro: %v", err)
	}
	if _, ok := registro.Buscar("executar_comando_sistema"); !ok {
		t.Error("terminal não registrado numa plataforma com suporte")
	}
}

func TestConstruirProvedorEscolhePelaConfig(t *testing.T) {
	casos := []struct {
		nome    string
		cfg     config.Config
		tipo    string
		querErr bool
	}{
		{"anthropic", config.Config{Provedor: "anthropic", ClaudeAPIKey: "sk-ant-teste", Modelo: "claude-haiku-5-5"}, "*anthropic.Provider", false},
		{"gemini", config.Config{Provedor: "gemini", GeminiAPIKey: "chave", Modelo: "gemini-2.5-flash"}, "*gemini.Provider", false},
		{"desconhecido", config.Config{Provedor: "openai"}, "", true},
	}
	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			p, err := construirProvedor(context.Background(), &c.cfg)
			if c.querErr {
				if err == nil {
					t.Fatal("esperava erro, veio nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("construirProvedor devolveu erro: %v", err)
			}
			if got := fmt.Sprintf("%T", p); got != c.tipo {
				t.Errorf("tipo = %s, queria %s", got, c.tipo)
			}
		})
	}
}

func TestConstruirSintetizadorEscolhePeloMotor(t *testing.T) {
	piper, err := construirSintetizador(&config.Config{MotorVoz: "piper", TTSURL: "http://127.0.0.1:8180"}, "audios")
	if err != nil {
		t.Fatalf("piper devolveu erro: %v", err)
	}
	if piper.Extensao != ".wav" {
		t.Errorf("piper: Extensao = %q, queria .wav", piper.Extensao)
	}

	edge, err := construirSintetizador(&config.Config{MotorVoz: "edge"}, "audios")
	if err != nil {
		t.Fatalf("edge devolveu erro: %v", err)
	}
	if edge.Extensao != "" {
		t.Errorf("edge: Extensao = %q, queria vazia (mp3 por padrão)", edge.Extensao)
	}

	if _, err := construirSintetizador(&config.Config{MotorVoz: "festival"}, "audios"); err == nil {
		t.Error("motor desconhecido deveria devolver erro")
	}
}

func TestRegistrarArquivosNaoRegistraQuandoNaoSuportado(t *testing.T) {
	registro := ferramentas.NovoRegistro()

	if err := registrarArquivos(registro, ferramentas.NovoSegredosLidos(), false); err != nil {
		t.Fatalf("registrarArquivos devolveu erro: %v", err)
	}

	for _, nome := range []string{"arquivos", "alterar_arquivos"} {
		if _, ok := registro.Buscar(nome); ok {
			t.Errorf("%s registrada numa plataforma sem suporte", nome)
		}
	}
}

func TestRegistrarArquivosRegistraAsDuasQuandoSuportado(t *testing.T) {
	registro := ferramentas.NovoRegistro()

	if err := registrarArquivos(registro, ferramentas.NovoSegredosLidos(), true); err != nil {
		t.Fatalf("registrarArquivos devolveu erro: %v", err)
	}

	for _, nome := range []string{"arquivos", "alterar_arquivos"} {
		if _, ok := registro.Buscar(nome); !ok {
			t.Errorf("%s não registrada", nome)
		}
	}
}

// O ponto da spec: o revelar e o salvar_na_memoria falam do MESMO SegredosLidos
// no registro de verdade, não só nos testes de cada ferramenta.
func TestRevelarEMemoriaNoRegistroRealCompartilhamOSegredo(t *testing.T) {
	pasta := t.TempDir()
	env := filepath.Join(pasta, ".env")
	if err := os.WriteFile(env, []byte("CHAVE=abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	segredos := ferramentas.NovoSegredosLidos()
	registro := ferramentas.NovoRegistro()
	if err := registrarArquivos(registro, segredos, true); err != nil {
		t.Fatal(err)
	}
	if err := registrarMemoria(registro, memoria.NovoVault(filepath.Join(pasta, "Memoria"), 2000), segredos); err != nil {
		t.Fatal(err)
	}
	arquivos, _ := registro.Buscar("arquivos")
	salvar, _ := registro.Buscar("salvar_na_memoria")

	if _, err := arquivos.Executar(context.Background(), map[string]any{"acao": "revelar", "caminho": env}); err != nil {
		t.Fatalf("revelar: %v", err)
	}
	_, err := salvar.Executar(context.Background(), map[string]any{"assunto": "projeto", "texto": "a chave é abcdef123456"})

	if err == nil || !strings.Contains(err.Error(), "valor secreto") {
		t.Fatalf("a memória aceitou o valor revelado: %v", err)
	}
}

// Sem as ferramentas de arquivo (Windows), a regra 6 do prompt mandaria o
// modelo chamar uma ferramenta que não existe e proibiria cat/ls/find.
func TestPromptAvisaQueNaoHaFerramentaDeArquivoQuandoNaoSuportado(t *testing.T) {
	base := "REGRA 6. ARQUIVOS: use a ferramenta arquivos"

	semSuporte := ajustarPromptParaPlataforma(base, false)
	if !strings.HasPrefix(semSuporte, base) {
		t.Error("o prompt original foi alterado em vez de acrescido")
	}
	for _, trecho := range []string{"NÃO existem", "arquivos", "alterar_arquivos", "ignore a regra 6"} {
		if !strings.Contains(semSuporte, trecho) {
			t.Errorf("o aviso não diz %q:\n%s", trecho, semSuporte)
		}
	}

	if comSuporte := ajustarPromptParaPlataforma(base, true); comSuporte != base {
		t.Errorf("com suporte o prompt deveria ficar igual, veio %q", comSuporte)
	}
}
