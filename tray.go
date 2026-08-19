package main

import (
	_ "embed"
	goruntime "runtime"

	"github.com/energye/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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
	go systray.Run(func() {
		systray.SetIcon(trayIconBytes)
		systray.SetTitle("WaiLauncher")
		systray.SetTooltip("WaiLauncher")

		itemShow := systray.AddMenuItem("Показать лаунчер", "Открыть окно WaiLauncher")
		itemStop := systray.AddMenuItem("Остановить игру", "Завершить запущенную игру")
		systray.AddSeparator()
		itemQuit := systray.AddMenuItem("Выход", "Закрыть WaiLauncher")

		itemStop.Disable()
		a.trayStopItem = itemStop

		itemShow.Click(func() {
			wruntime.WindowShow(a.ctx)
			wruntime.WindowUnminimise(a.ctx)
		})
		itemStop.Click(func() {
			a.StopGame()
		})
		itemQuit.Click(func() {
			wruntime.Quit(a.ctx)
		})
	}, func() {})
}

// updateTrayPlaying toggles the "Stop game" tray item based on game state.
func (a *App) updateTrayPlaying(playing bool) {
	item, _ := a.trayStopItem.(interface {
		Enable()
		Disable()
	})
	if item == nil {
		return
	}
	if playing {
		item.Enable()
	} else {
		item.Disable()
	}
}
