package main

import (
	"embed"
	"os"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	filePath, themeName, bench := parseArgs()
	app := NewApp(filePath, themeName, bench)

	start := time.Now()
	if bench {
		println("[bench] main-start")
	}

	err := wails.Run(&options.App{
		Title:  "mdlight",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 0x18, G: 0x20, B: 0x2e, A: 0xff},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}

	if bench {
		println("[bench] main-exit duration:", time.Since(start).Round(time.Millisecond).String())
	}
}

// parseArgs parses the command-line arguments MDLight cares about:
//
//	mdlight [path] [--theme name|--theme=name] [--bench]
//
// No third-party flag library — the surface is small enough that a manual
// loop is clearer and adds no dependencies. Unknown flags are silently ignored
// so future flags don't cause hard failures on older installs.
func parseArgs() (filePath, themeName string, bench bool) {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--theme" && i+1 < len(args):
			themeName = args[i+1]
			i++
		case strings.HasPrefix(arg, "--theme="):
			themeName = strings.TrimPrefix(arg, "--theme=")
		case arg == "--bench":
			bench = true
		case !strings.HasPrefix(arg, "--"):
			if filePath == "" {
				filePath = arg
			}
		}
	}
	return
}
