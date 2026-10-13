// Launcher de OmbuServer: ventana con Wails (Go + WebView2) sobre el núcleo ombu/.
//
// Reemplaza a la interfaz Qt del launcher de Slash (therecipe/qt, abandonado en 2020): se mantiene
// el diseño (ventana sin marco 1024x600, novedades a la derecha, JUGAR abajo) y la lógica de ombu/.
//
//	wails build -platform windows/amd64     ->  build/bin/OmbuServer Launcher.exe
package main

import (
	"context"
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// Version del launcher (se muestra abajo a la derecha).
const Version = "1.1.3"

func main() {
	limpiarActualizacion()
	app := NuevaApp()
	graficos := nuevosGraficos(app) // figuras del chat: las lee de ombuserver.mpq (graficos.go)
	err := wails.Run(&options.App{
		Title:            "OmbuServer",
		Width:            1024,
		Height:           600,
		DisableResize:    true,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 8, G: 9, B: 16, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets, Handler: graficos},
		OnShutdown:       app.cerrarJuegos, // cierra los Diablo II que abrió el launcher
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			go graficos.precargar()
		},
		Bind: []interface{}{app},
		// Un solo launcher abierto: si lo abren de nuevo, se trae al frente el que ya está.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "ombuserver-launcher-5f3c2a",
			OnSecondInstanceLaunch: app.otraInstancia,
		},
		Windows: &windows.Options{Theme: windows.Dark},
	})
	if err != nil {
		log.Fatal(err)
	}
}
