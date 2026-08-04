package main

import (
	"context"
	"embed"

	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

var assets embed.FS

func main() {
	// Ausência de .env não é erro: em produção as variáveis vêm do ambiente.
	_ = godotenv.Load()

	app := NewApp()

	err := wails.Run(&options.App{
		Title:       "Athena",
		Width:       100,
		Height:      100,
		Frameless:   true,
		AlwaysOnTop: true,
		AssetServer: &assetserver.Options{Assets: assets},
		Windows: &windows.Options{
			WebviewIsTransparent:              true,
			WindowIsTranslucent:               true,
			BackdropType:                      windows.None,
			DisableFramelessWindowDecorations: true,
			Theme:                             windows.Dark,
		},

		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			go app.IniciarReceptorPython()
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
