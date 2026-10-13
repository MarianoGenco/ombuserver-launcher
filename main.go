// Launcher de OmbuServer: ventana con Wails (Go + WebView2) sobre el núcleo ombu/.
//
// Reemplaza a la interfaz Qt del launcher de Slash (therecipe/qt, abandonado en 2020): se mantiene
// el diseño (ventana sin marco 1024x600, novedades a la derecha, JUGAR abajo) y la lógica de ombu/.
//
//	wails build -platform windows/amd64     ->  build/bin/OmbuServer Launcher.exe
package main

import (
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
const Version = "1.0.2"

func main() {
	limpiarActualizacion()
	app := NuevaApp()
	graficos := nuevosGraficos() // figuras del chat: las baja del servidor (graficos.go)
	go graficos.precargar()
	err := wails.Run(&options.App{
		Title:            "OmbuServer",
		Width:            1024,
		Height:           600,
		DisableResize:    true,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 8, G: 9, B: 16, A: 255},
		AssetServer:      &assetserver.Options{Assets: assets, Handler: graficos},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
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
