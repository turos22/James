package ferramentas

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestComandoSistemaExecutaViaExecutorInjetado(t *testing.T) {
	var comandoRecebido string
	c := &ComandoSistema{
		Executor: func(ctx context.Context, comando string) (ResultadoComando, error) {
			comandoRecebido = comando
			return ResultadoComando{Stdout: "saída do comando"}, nil
		},
		MaxBytesSaida: 1000,
	}

	saida, err := c.Executar(context.Background(), map[string]any{"comando": "Get-Process"})
	if err != nil {
		t.Fatalf("Executar devolveu erro: %v", err)
	}
	if !strings.Contains(saida, "saída do comando") {
		t.Errorf("saida = %q, deveria conter a stdout do comando", saida)
	}
	if comandoRecebido != "Get-Process" {
		t.Errorf("comando repassado = %q, queria Get-Process", comandoRecebido)
	}
}

func TestComandoSistemaRejeitaArgumentoAusente(t *testing.T) {
	c := &ComandoSistema{Executor: func(context.Context, string) (ResultadoComando, error) {
		return ResultadoComando{}, nil
	}}
	if _, err := c.Executar(context.Background(), map[string]any{}); err == nil {
		t.Fatal("esperava erro por parâmetro comando ausente, veio nil")
	}
}

func TestComandoSistemaPropagaErroDoExecutor(t *testing.T) {
	c := &ComandoSistema{Executor: func(context.Context, string) (ResultadoComando, error) {
		return ResultadoComando{}, errors.New("falhou")
	}}
	if _, err := c.Executar(context.Background(), map[string]any{"comando": "x"}); err == nil {
		t.Fatal("esperava erro propagado do Executor, veio nil")
	}
}

func TestComandoSistemaExitCodeDiferenteDeZeroViraErro(t *testing.T) {
	c := &ComandoSistema{
		Executor: func(context.Context, string) (ResultadoComando, error) {
			return ResultadoComando{Stderr: "não encontrado", ExitCode: 1}, nil
		},
		MaxBytesSaida: 1000,
	}
	_, err := c.Executar(context.Background(), map[string]any{"comando": "Get-Foo"})
	if err == nil {
		t.Fatal("exit code 1 deveria virar erro, veio nil")
	}
	if !strings.Contains(err.Error(), "não encontrado") {
		t.Errorf("erro deveria carregar o stderr, veio %q", err.Error())
	}
}

func TestComandoSistemaNivelPerigoso(t *testing.T) {
	c := NovoComandoSistema()
	if c.Nivel() != NivelPerigoso {
		t.Error("executar_comando_sistema deve ser NivelPerigoso")
	}
}

func TestComandoSistemaConfirmacaoNecessaria(t *testing.T) {
	casos := []struct {
		nome    string
		args    map[string]any
		querido bool
	}{
		{"leitura não exige", map[string]any{"comando": "Get-Process"}, false},
		{"escrita exige", map[string]any{"comando": "Remove-Item x"}, true},
		{"argumento ausente exige", map[string]any{}, true},
		{"argumento vazio exige", map[string]any{"comando": ""}, true},
		{"argumento de tipo errado exige", map[string]any{"comando": 42}, true},
	}

	c := NovoComandoSistema()
	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			if got := c.ConfirmacaoNecessaria(caso.args); got != caso.querido {
				t.Errorf("ConfirmacaoNecessaria(%v) = %v, queria %v", caso.args, got, caso.querido)
			}
		})
	}
}

// TestComandoSistemaImplementaExigeConfirmacao trava a asserção de tipo que o
// loop faz: sem ela, o gate silenciosamente não pega a ferramenta.
func TestComandoSistemaImplementaExigeConfirmacao(t *testing.T) {
	var _ ExigeConfirmacao = NovoComandoSistema()
}

func TestCodificarComandoProduzUTF16LEBase64(t *testing.T) {
	// "A" em UTF-16LE é 0x41 0x00; em base64, "QQA=".
	if got := codificarComando("A"); got != "QQA=" {
		t.Errorf("codificarComando(\"A\") = %q, queria QQA=", got)
	}
}

// TestExecutarPowerShellDeVerdadeComAcentoEAspas roda o PowerShell real: é o
// único jeito de provar que o comando atravessa as duas camadas de parsing de
// shell sem se quebrar. Não usa mock de propósito.
func TestExecutarPowerShellDeVerdadeComAcentoEAspas(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o PowerShell real")
	}
	res, err := executarPowerShell(context.Background(), `Write-Output "ação 'com' aspas"`)
	if err != nil {
		t.Fatalf("executarPowerShell devolveu erro: %v", err)
	}
	if !strings.Contains(res.Stdout, "ação 'com' aspas") {
		t.Errorf("stdout = %q, queria o texto com acento e aspas intactos", res.Stdout)
	}
	if res.ExitCode != 0 {
		t.Errorf("exit code = %d, queria 0", res.ExitCode)
	}
}

// TestExecutarPowerShellDeVerdadeRespeitaOTimeout prova que o processo é
// morto de verdade, não só abandonado.
func TestExecutarPowerShellDeVerdadeRespeitaOTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o PowerShell real")
	}
	c := &ComandoSistema{Executor: executarPowerShell, Timeout: 2 * time.Second, MaxBytesSaida: 1000}

	inicio := time.Now()
	_, err := c.Executar(context.Background(), map[string]any{"comando": "Start-Sleep -Seconds 30"})
	decorrido := time.Since(inicio)

	if err == nil {
		t.Fatal("esperava erro de timeout, veio nil")
	}
	if !strings.Contains(err.Error(), "tempo limite") {
		t.Errorf("erro deveria dizer que foi timeout, veio %q", err.Error())
	}
	if decorrido > 10*time.Second {
		t.Errorf("demorou %s — o processo não foi morto no timeout", decorrido)
	}
}

func TestExecutarPowerShellDeVerdadeCapturaExitCode(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o PowerShell real")
	}
	res, err := executarPowerShell(context.Background(), "exit 3")
	if err != nil {
		t.Fatalf("executarPowerShell devolveu erro: %v", err)
	}
	if res.ExitCode != 3 {
		t.Errorf("exit code = %d, queria 3", res.ExitCode)
	}
}

// TestExecutarPowerShellDeVerdadeNaoDeixaBOM é cinto e suspensório: mesmo
// com UTF8Encoding::new($false) (que não deveria gravar BOM), confirma que
// nenhum U+FEFF sobrevive até o Go. BOM vazando pro modelo é sujeira que não
// aparece na tela e reaparece como bug estranho lá na frente.
func TestExecutarPowerShellDeVerdadeNaoDeixaBOM(t *testing.T) {
	if testing.Short() {
		t.Skip("usa o PowerShell real")
	}
	res, err := executarPowerShell(context.Background(), `Write-Output "ok"`)
	if err != nil {
		t.Fatalf("executarPowerShell devolveu erro: %v", err)
	}
	bom := string(rune(0xFEFF))
	if strings.HasPrefix(res.Stdout, bom) {
		t.Errorf("stdout começa com BOM: %q", res.Stdout)
	}
}

// TestPrefixoDeEncodingNaoVazaParaOClassificador prova, e não apenas assume,
// que ConfirmacaoNecessaria roda sobre o comando cru dos args — nunca sobre a
// linha de comando que executarPowerShell de fato monta pro sistema.
//
// A premissa mudou junto com a política (ver classificador.go). Antes, o que
// reprovava a versão montada era a newline do prefixo UTF-8, contra a regra
// ehUmaLinhaSo do classificador de leitura. Hoje a newline é irrelevante, mas
// existe uma premissa mais forte no lugar: a linha real passa por
// -EncodedCommand, e -EncodedCommand é marca de ofuscação — se ela vazasse
// pro classificador, TODO comando passaria a exigir confirmação, inclusive
// Get-Process.
//
// loop.go despacharSemHook chama ConfirmacaoNecessaria(chamada.Args) e depois
// Executar(ctx, chamada.Args) com o mesmo mapa, sem mutação entre as duas —
// este teste é a contraparte em nível de unidade dessa leitura de código.
func TestPrefixoDeEncodingNaoVazaParaOClassificador(t *testing.T) {
	comandoCru := "Get-Process"

	// Confere a premissa com o que executarPowerShell realmente manda pro
	// sistema, não com uma string inventada à mão.
	comandoComoSeriaEnviado := "powershell -NoProfile -NonInteractive -EncodedCommand " +
		codificarComando(prefixoUTF8+comandoCru)
	if !ComandoPrecisaConfirmacao(comandoComoSeriaEnviado) {
		t.Fatal("premissa quebrou: a linha montada deveria exigir confirmação por causa do -EncodedCommand")
	}

	// Caminho real: ConfirmacaoNecessaria recebe os args do jeito que
	// chegam do modelo, sem qualquer prefixo de encoding.
	c := NovoComandoSistema()
	if c.ConfirmacaoNecessaria(map[string]any{"comando": comandoCru}) {
		t.Fatal("ConfirmacaoNecessaria exigiu confirmação pra Get-Process — o prefixo de encoding vazou pro classificador")
	}
}
