package ferramentas

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"os/exec"
	"strings"
	"unicode/utf16"
)

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
