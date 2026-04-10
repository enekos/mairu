package desktop

import (
	"github.com/getlantern/systray"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// SetupTray initializes the system tray icon and menu.
// Call this from Startup after services are ready.
func (a *App) SetupTray() {
	go systray.Run(func() {
		systray.SetTitle("Mairu")
		systray.SetTooltip("Mairu Desktop")

		mShow := systray.AddMenuItem("Show Window", "Show the main window")
		systray.AddSeparator()
		mStatus := systray.AddMenuItem("Meilisearch: Starting...", "")
		mStatus.Disable()
		systray.AddSeparator()
		mQuit := systray.AddMenuItem("Quit", "Quit Mairu")

		// Update status when Meilisearch is ready
		go func() {
			<-a.meiliReady // channel set in startup
			mStatus.SetTitle("Meilisearch: Running")
		}()

		for {
			select {
			case <-mShow.ClickedCh:
				wailsRuntime.WindowShow(a.ctx)
			case <-mQuit.ClickedCh:
				wailsRuntime.Quit(a.ctx)
				return
			}
		}
	}, func() {})
}
