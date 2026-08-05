// Comando shell: invólucro Wails mínimo — janela frameless, transparente,
// always-on-top, WebView2 apontando pro jamesd local. Não tem lógica de
// aplicação própria: toda UI e todo estado vivem no jamesd + web/ (spec
// 4.9/4.10). Nenhum binding wailsjs/go/main/App sobrevive — eles morreram
// junto com o app.go antigo.
package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend
var assets embed.FS

func main() {
	err := wails.Run(&options.App{
		Title:            "James",
		Width:            100,
		Height:           100,
		Frameless:        true,
		AlwaysOnTop:      true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer:      &assetserver.Options{Assets: assets},
		// BackgroundColour sozinho não basta pro WebView2 fazer
		// transparência de verdade por pixel no Windows — sem isso, a
		// janela mostrava um retângulo preto sólido em tudo que não é o
		// painel do chat, em vez de deixar a área de trabalho aparecer.
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
		},
		// A SPA de verdade (servida pelo jamesd, ver controle.go) fala
		// com este servidor de controle pra pedir resize/arraste — não
		// dá pra usar window.runtime do Wails porque essa injeção só
		// acontece em páginas servidas pelo próprio AssetServer do
		// Wails, nunca na página que o jamesd serve.
		OnStartup: func(ctx context.Context) {
			iniciarServidorControle(ctx, chamadasJanelaWails{})
		},
	})
	if err != nil {
		println("erro ao subir o shell:", err.Error())
	}
}
