package main

import (
	"context"
	"log"

	"mairu/internal/desktop"
	"mairu/ui"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func main() {
	app := desktop.NewApp()

	err := wails.Run(&options.App{
		Title:     "Mairu",
		Width:     1200,
		Height:    800,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: ui.Assets,
		},
		Menu:       app.BuildMenu(),
		OnStartup:  app.Startup,
		OnShutdown: app.Shutdown,
		OnBeforeClose: func(ctx context.Context) bool {
			wailsRuntime.WindowHide(ctx)
			return true // prevent actual close — tray "Quit" calls Quit()
		},
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
