package ferramentas

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
	"unicode/utf16"
)

// ComandoSistema executa PowerShell. Não há allowlist de comando — a
// contenção é confirmação humana pro que destrói, abre a máquina ou executa
// código baixado (ver classificador.go), mais timeout e limite de saída.
type ComandoSistema struct {
	// Executor recebe o ctx pra que o timeout mate o processo de verdade, e
	// não só abandone a espera.
	Executor func(ctx context.Context, comando string) (ResultadoComando, error)

	// Timeout zero desliga o limite de tempo. MaxBytesSaida zero desliga a
	// truncagem. Ambos vêm do config.toml (Task 8).
	Timeout       time.Duration
	MaxBytesSaida int
}

func NovoComandoSistema() *ComandoSistema {
	return &ComandoSistema{Executor: executarPowerShell}
}

// codificarComando devolve o comando em UTF-16LE codificado em base64, que é
// o formato aceito por -EncodedCommand. Isso elimina a classe inteira de
// bugs de aspas, acento e caractere especial atravessando duas camadas de
// parsing de shell — o comando chega ao PowerShell exatamente como saiu daqui.
func codificarComando(comando string) string {
	unidades := utf16.Encode([]rune(comando))
	bytesLE := make([]byte, 0, len(unidades)*2)
	for _, u := range unidades {
		bytesLE = append(bytesLE, byte(u), byte(u>>8))
	}
	return base64.StdEncoding.EncodeToString(bytesLE)
}

// prefixoUTF8 força a saída do PowerShell pra UTF-8 sem BOM antes do comando
// do usuário. Sem console real (stdout indo pra pipe, não pra um terminal de
// verdade), o powershell.exe emite no codepage OEM da máquina — 850 nesta,
// mas varia por locale — e texto acentuado volta corrompido; o James
// responde em português, então isso é o caso comum, não a borda.
// UTF8Encoding::new($false) e não [System.Text.Encoding]::UTF8 porque o
// segundo grava um BOM (U+FEFF) no início do stream.
const prefixoUTF8 = "[Console]::OutputEncoding = [System.Text.UTF8Encoding]::new($false)\n"

func executarPowerShell(ctx context.Context, comando string) (ResultadoComando, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive",
		"-EncodedCommand", codificarComando(prefixoUTF8+comando))

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	// Cinto e suspensório: mesmo com new($false), tira um BOM que porventura
	// sobreviva. BOM vazando pro modelo é sujeira que não aparece na tela e
	// reaparece como bug estranho lá na frente.
	bom := string(rune(0xFEFF))
	res := ResultadoComando{
		Stdout: strings.TrimPrefix(stdout.String(), bom),
		Stderr: strings.TrimPrefix(stderr.String(), bom),
	}

	// Exit code diferente de zero não é erro de infraestrutura: é resultado
	// do comando, e quem decide o que fazer com ele é o Executar.
	var saiuComErro *exec.ExitError
	if errors.As(err, &saiuComErro) {
		res.ExitCode = saiuComErro.ExitCode()
		return res, nil
	}
	return res, err
}

func (c *ComandoSistema) Declaracao() Declaracao {
	return Declaracao{
		Nome:      "executar_comando_sistema",
		Descricao: "Executa comandos PowerShell. Se não souber o comando pra uma tarefa, use pesquisar_internet primeiro.",
		Parametros: map[string]Parametro{
			"comando": {Tipo: "string", Descricao: "comando PowerShell a executar"},
		},
		Obrigatorios: []string{"comando"},
	}
}

func (c *ComandoSistema) Nivel() Nivel { return NivelPerigoso }

// ConfirmacaoNecessaria implementa ExigeConfirmacao: comando destrutivo pede
// confirmação humana mesmo do perfil Privilegiado. Comando comum — inclusive
// de escrita — passa direto. Argumento ausente ou de tipo errado devolve true
// (fail closed); a validação de argumento em si continua em Executar.
func (c *ComandoSistema) ConfirmacaoNecessaria(args map[string]any) bool {
	comando, ok := args["comando"].(string)
	if !ok {
		return true
	}
	return ComandoPrecisaConfirmacao(comando)
}

func (c *ComandoSistema) Executar(ctx context.Context, args map[string]any) (string, error) {
	comando, ok := args["comando"].(string)
	if !ok || comando == "" {
		return "", fmt.Errorf("parâmetro comando ausente ou vazio")
	}

	ctxComando := ctx
	if c.Timeout > 0 {
		var cancelar context.CancelFunc
		ctxComando, cancelar = context.WithTimeout(ctx, c.Timeout)
		defer cancelar()
	}

	res, err := c.Executor(ctxComando, comando)

	// Timeout é reportado COMO timeout, distinguível de erro de execução: o
	// modelo precisa saber que o comando não terminou, e não que ele falhou.
	if errors.Is(ctxComando.Err(), context.DeadlineExceeded) {
		return "", fmt.Errorf("comando excedeu o tempo limite de %s e foi encerrado", c.Timeout)
	}
	if err != nil {
		return "", fmt.Errorf("erro ao executar: %w", err)
	}
	if res.ExitCode != 0 {
		return "", fmt.Errorf("comando terminou com exit code %d: %s",
			res.ExitCode, truncar(res.Stderr, c.MaxBytesSaida))
	}
	return formatarResultadoComando(res, c.MaxBytesSaida), nil
}
