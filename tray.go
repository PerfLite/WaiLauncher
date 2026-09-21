package main

import (
	_ "embed"
	goruntime "runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed build/windows/icon.ico
var trayIconBytes []byte

// startTray runs the system tray icon (Windows). While a game is running the
// tray offers quick actions without opening the launcher window, so the
// CloseOnLaunch option never leaves the user without a way to stop the game.
func (a *App) startTray() {
	if goruntime.GOOS != "windows" {
		return
	}
	app := a.app
	if app == nil {
		app = application.Get()
	}
	if app == nil {
		return
	}

	tray := app.SystemTray.New()
	tray.SetIcon(trayIconBytes)
	tray.SetTooltip("WaiLauncher")

	menu := application.NewMenu()
	itemShow := menu.Add("Показать лаунчер")
	itemShow.OnClick(func(ctx *application.Context) {
		if a.win != nil {
			a.win.Show()
			a.win.UnMinimise()
			a.win.Focus()
		}
	})

	itemStop := menu.Add("Остановить игру")
	itemStop.SetEnabled(false)
	itemStop.OnClick(func(ctx *application.Context) {
		a.StopGame()
	})
	a.trayStopItem = itemStop

	menu.AddSeparator()
	itemAbout := menu.Add("О программе")
	itemAbout.OnClick(func(ctx *application.Context) {
		if a.win != nil {
			a.win.Show()
			a.win.UnMinimise()
			a.win.Focus()
			a.emit("open-about-modal")
		}
	})
	itemQuit := menu.Add("Выход")
	itemQuit.OnClick(func(ctx *application.Context) {
		app.Quit()
	})

	tray.SetMenu(menu)
	tray.OnClick(func() {
		if a.win != nil {
			a.win.Show()
			a.win.UnMinimise()
			a.win.Focus()
		}
	})
	tray.Run()
}

// updateTrayPlaying toggles the "Stop game" tray item based on game state.
func (a *App) updateTrayPlaying(playing bool) {
	if a.trayStopItem != nil {
		a.trayStopItem.SetEnabled(playing)
	}
}
