package main

import (
	"embed"
	"time"

	"OpenSmurfManager/internal/telemetry"
	"OpenSmurfManager/internal/updater"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Time-zero for app-start latency. Captured as early as possible so
	// the reported number reflects real cold-boot cost (Go init + Wails
	// runtime + WebView2 handshake).
	startTime := time.Now()

	// Telemetry is best-effort — if it can't initialise (unwritable config
	// dir, etc.) we log to stderr and keep running.
	if err := telemetry.Init("OpenSmurfManager", updater.Version); err != nil {
		println("telemetry init failed:", err.Error())
	}
	defer telemetry.Close()

	// Create an instance of the app structure
	app := NewApp()
	app.startTime = startTime

	// Create application with options
	// Start with login size (vertical), will resize after unlock
	err := wails.Run(&options.App{
		Title:     "SmurfManager",
		// One size across login + main so unlocking doesn't resize the window.
		Width:     520,
		Height:    760,
		MinWidth:  520,
		MinHeight: 760,
		MaxWidth:  700,
		MaxHeight: 900,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 15, A: 1}, // Match --color-background
		OnStartup:        app.startup,
		// Frameless: native title bar is removed; the app paints its own
		// title bar (see WindowFrame.tsx) so chrome blends with the UI.
		// Windows 11 still renders a rounded outline + DWM drop shadow
		// automatically for frameless windows.
		Frameless: true,
		Windows: &windows.Options{
			DisableFramelessWindowDecorations: false,
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableWindowIcon:                 false,
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
