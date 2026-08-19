package main

import (
	"embed"

	"WaiLauncher/internal/launcher"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/logger"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	root, err := launcher.DefaultRoot()
	if err != nil {
		root = "data"
	}
	_ = launcher.InitLogger(root)
	launcher.LogInfo("Starting WaiLauncher version %s", launcherVersion)

	app := NewApp()

	err = wails.Run(&options.App{
		Title:     "WaiLauncher",
		Width:     1280,
		Height:    800,
		MinWidth:  1024,
		MinHeight: 640,
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 7, G: 9, B: 13, A: 255},
		OnStartup:        app.startup,
		Logger:           launcher.GetWailsLogger(),
		LogLevel:         logger.INFO,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		launcher.LogError("Wails runtime error: %v", err)
	}
}
