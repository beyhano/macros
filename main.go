package main

import (
	"embed"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

//go:embed version.json
var versionJSON []byte

// updaterWindowCSS restyles the built-in update window to match the app's
// dark theme and improve the release-notes typography.
const updaterWindowCSS = `
:root {
  --bg:        #0b0b10;
  --surface:   #15151c;
  --surface-2: #1e1e28;
  --fg:        #e7e9ee;
  --fg-dim:    #a2a8b4;
  --fg-faint:  #6d7280;
  --border:    #2a2b36;
  --accent:    #3b82f6;
  --accent-fg: #ffffff;
  --accent-dim:#3b82f633;
  --radius:    12px;
  --radius-sm: 8px;
  --pad:       22px;
}
.u__title  { font-size: 15px; }
.u__subtitle { font-size: 12px; }
.u__notes  { font-size: 12.5px; line-height: 1.5; color: var(--fg-dim); max-height: 240px; overflow-y: auto; }
.u__btn    { border-radius: 8px; font-weight: 600; }
`

func init() {
	// Register a custom event whose associated data type is string.
	// This is not required, but the binding generator will pick up registered events
	// and provide a strongly typed JS/TS API for them.
	application.RegisterEvent[string]("time")
}

// MatchAsset selects the release asset matching the running platform by exact
// filename. Repo releases ship unrenamed basenames: "macros" (linux/darwin
// raw) and "macros.exe" (windows). Returns -1 when no asset matches.
func MatchAsset(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	expect := "macros"
	if req.Platform == "windows" || req.Platform == "Windows" {
		expect = "macros.exe"
	}
	for i, a := range assets {
		if strings.EqualFold(a.Name, expect) {
			return i
		}
	}
	return -1
}

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	app := application.New(application.Options{
		Name:        "macros",
		Description: "A demo of using raw HTML & CSS",
		Services: []application.Service{
			application.NewService(&GreetService{}),
			application.NewService(NewMacrosService()),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	var vf struct {
		Current struct {
			Version string `json:"version"`
		} `json:"current"`
	}
	if err := json.Unmarshal(versionJSON, &vf); err != nil {
		log.Printf("updater: version.json parse failed: %v", err)
	}
	curVersion := strings.TrimPrefix(vf.Current.Version, "v")

	ghProv, err := github.New(github.Config{
		Repository:    "beyhano/macros",
		ChecksumAsset: "SHA256SUMS",
		AssetMatcher:  MatchAsset,
	})
	if err != nil {
		log.Printf("updater: github provider: %v", err)
	} else if curVersion != "" {
		if err := app.Updater.Init(updater.Config{
			CurrentVersion: curVersion,
			Providers:      []updater.Provider{ghProv},
			Window: &updater.BuiltinWindow{
				CSS: updaterWindowCSS,
				Options: updater.WindowOptions{
					Title:  "Macros Güncelleme",
					Width:  520,
					Height: 480,
				},
			},
		}); err != nil {
			log.Printf("updater: init: %v", err)
		}
	} else {
		log.Println("updater: no version, skipping")
	}

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "Macro Düzenleyici",
		// Window sized to the golden ratio (1000 / 618 ≈ 1.618).
		Width:  1000,
		Height: 618,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(6, 7, 15),
		URL:              "/",
	})

	// Create a goroutine that emits an event containing the current time every second.
	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			app.Event.Emit("time", now)
			time.Sleep(time.Second)
		}
	}()

	// Run the application. This blocks until the application has been exited.
	err = app.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
