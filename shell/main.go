// Comando shell: invólucro Wails mínimo — janela frameless, transparente,
// always-on-top, WebView2 apontando pro jamesd local. Não tem lógica de
// aplicação própria: toda UI e todo estado vivem no jamesd + web/ (spec
// 4.9/4.10). Nenhum binding wailsjs/go/main/App sobrevive — eles morreram
// junto com o app.go antigo.
package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
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
	})
	if err != nil {
		println("erro ao subir o shell:", err.Error())
	}
}
