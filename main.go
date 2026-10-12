package main

import (
	"embed"
	"os"

	"WaiLauncher/internal/launcher"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Some endpoint-protection products (e.g. КИБЕРЩИТ/Kaspersky-based AV) inject
	// DLLs into the WebView2 renderer, which breaks Chromium's GPU compositing and
	// leaves the window painting only the background colour (black screen). Force
	// software compositing unless the operator already supplied their own args.
	if _, ok := os.LookupEnv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS"); !ok {
		os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS", "--disable-gpu --disable-gpu-compositing")
	}

	root, err := launcher.DefaultRoot()
	if err != nil {
		root = "data"
	}
	_ = launcher.InitLogger(root)
	launcher.LogInfo("Starting WaiLauncher version %s", launcherVersion)

	backendApp := NewApp()

	app := application.New(application.Options{
		Name:        "WaiLauncher",
		Description: "Modern Minecraft Launcher",
		Services: []application.Service{
			application.NewService(backendApp),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Windows: application.WindowsOptions{
			WndClass: "WaiLauncherWebviewWindow",
		},
	})

	backendApp.initApp(app)

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:            "WaiLauncher",
		Width:            1280,
		Height:           800,
		MinWidth:         1024,
		MinHeight:        640,
		Frameless:        true,
		EnableFileDrop:   true,
		BackgroundColour: application.NewRGB(7, 9, 13),
		URL:              "/",
		Windows: application.WindowsWindow{
			Theme: application.Dark,
		},
	})

	backendApp.setWindow(win)

	err = app.Run()
	if err != nil {
		launcher.LogError("Wails runtime error: %v", err)
	}
}
