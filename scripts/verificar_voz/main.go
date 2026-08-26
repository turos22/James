// Comando verificar_voz: gera uma amostra usando o MESMO caminho do jamesd —
// config.Carregar lê o config.toml e voz.Sintetizador chama o edge-tts.
//
// Existe porque trocar voz_tts é uma mudança de configuração que nenhum teste
// unitário cobre de ponta a ponta: o teste do Sintetizador usa um executável
// falso, e o teste do config só confere que o campo foi lido. Uma voz que não
// existe no edge-tts só falharia em produção, no primeiro turno falado.
package main

import (
	"context"
	"fmt"
	"os"

	"james/internal/config"
	"james/internal/voz"
)

func main() {
	cfg, err := config.Carregar("config.toml")
	if err != nil {
		fmt.Fprintln(os.Stderr, "configuração:", err)
		os.Exit(1)
	}
	fmt.Printf("voz lida do config.toml: %q\n", cfg.VozTTS)

	destino := "audios"
	if len(os.Args) > 2 {
		destino = os.Args[2]
	}
	caminho, err := voz.NovoSintetizador(destino).Gerar(
		context.Background(), os.Args[1], cfg.VozTTS, "verificacao-de-voz")
	if err != nil {
		fmt.Fprintln(os.Stderr, "falhou ao gerar:", err)
		os.Exit(1)
	}

	info, err := os.Stat(caminho)
	if err != nil || info.Size() == 0 {
		fmt.Fprintln(os.Stderr, "gerou, mas o arquivo está vazio ou ausente")
		os.Exit(1)
	}
	fmt.Printf("ok: %s (%d bytes)\n", caminho, info.Size())
}
