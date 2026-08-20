package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/endpoint"
)

const appVersion = "0.1.84"
const updaterManifestURL = "https://github.com/MorenoLand/GScript.GSuite/releases/latest/download/latest.json"

//go:embed all:frontend
var assets embed.FS

//go:embed wails-updater.key.pub
var updaterPublicKey []byte

func main() {
	if runtime.GOOS == "linux" {
		_ = os.Setenv("WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS", "1")
	}
	service := NewAppService(appVersion)
	var app *application.App
	var mainWindow *application.WebviewWindow
	app = newApplication(service, &mainWindow)
	service.app = app
	provider, err := endpoint.New(endpoint.Config{URL: updaterManifestURL})
	if err == nil {
		if err := app.Updater.Init(updater.Config{CurrentVersion: appVersion, PublicKey: updaterPublicKey, Providers: []updater.Provider{provider}}); err != nil {
			log.Printf("updater initialization failed: %v", err)
		}
	}
	mainWindow = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:                       "main",
		Title:                      "GSuite",
		Width:                      1280,
		Height:                     800,
		Frameless:                  true,
		URL:                        "/",
		EnableFileDrop:             true,
		DefaultContextMenuDisabled: true,
		Windows:                    application.WindowsWindow{NonClientRegionSupport: true},
	})
	service.window = mainWindow
	mainWindow.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		app.Event.Emit("wails-file-drop", map[string]any{"paths": event.Context().DroppedFiles()})
	})
	mainWindow.Center()
	mainWindow.Show()
	initUpdaterCheck(app, mainWindow)
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func newApplication(service *AppService, mainWindow **application.WebviewWindow) *application.App {
	var app *application.App
	assetsFS, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		log.Fatal(err)
	}
	app = application.New(application.Options{
		Name:             "GSuite",
		Description:      "Open-source Graal editor suite for levels, animations, and maps",
		Services:         []application.Service{application.NewService(service)},
		Assets:           application.AssetOptions{Handler: application.AssetFileServerFS(assetsFS)},
		Flags:            map[string]any{"isGSuiteDesktop": true, "enableFileDrop": true},
		FileAssociations: []string{".nw", ".gmap", ".gani", ".zelda", ".graal"},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "com.morenoland.gsuite",
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				path := firstExistingPathInDir(data.Args, data.WorkingDir)
				if path == "" {
					return
				}
				if *mainWindow != nil {
					(*mainWindow).Show()
					(*mainWindow).Focus()
				}
				app.Event.Emit("open-file", path)
			},
		},
	})
	return app
}

func init() {
	updater.HandleHelperMode()
}

func initUpdaterCheck(app *application.App, window *application.WebviewWindow) {
	if app.Updater == nil {
		return
	}
	go func() {
		release, err := app.Updater.Check(context.Background())
		if err == nil && release != nil {
			window.EmitEvent("update-available", release.Version)
		}
	}()
}
