// Comando shell: invólucro Wails da janela do James — frameless (os
// controles de janela são os três círculos da SPA, ver
// web/src/shell/controlesJanela.tsx), opaca e redimensionável, com o
// WebView2 apontando pro jamesd local. Não tem lógica de aplicação
// própria: toda UI e todo estado vivem no jamesd + web/.
//
// Era um widget 100x100 transparente e always-on-top, porque a interface
// era uma bolinha flutuante que crescia ao abrir o chat. A Fase 7A trocou
// isso por um aplicativo com barra lateral, e o modelo de widget perdeu a
// função.
package main

import (
	"context"
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
		Width:            1100,
		Height:           760,
		MinWidth:         900,
		MinHeight:        600,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 12, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets},
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
