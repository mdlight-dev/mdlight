package main

import (
	"embed"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	filePath, themeName := parseArgs()
	app := NewApp(filePath, themeName)

	err := wails.Run(&options.App{
		Title:  "mdlight",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// Match the default-dark theme background so there's no flash
		// of a different colour before the frontend paints.
		BackgroundColour: &options.RGBA{R: 0x18, G: 0x20, B: 0x2e, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}
}

// parseArgs parses the mdlight command line.
//
// Supported forms:
//
//	mdlight                          → no file, open picker
//	mdlight file.md                  → open file.md with default theme
//	mdlight file.md --theme nord     → open file.md with Nord theme
//	mdlight --theme nord file.md     → same, flag before positional
//
// No third-party flag library is used — the surface is small enough that
// a manual loop is clearer and has no dependencies.
func parseArgs() (filePath, themeName string) {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--theme" && i+1 < len(args):
			themeName = args[i+1]
			i++ // consume the value
		case strings.HasPrefix(args[i], "--theme="):
			themeName = strings.TrimPrefix(args[i], "--theme=")
		case !strings.HasPrefix(args[i], "--"):
			// First non-flag argument is the file path.
			if filePath == "" {
				filePath = args[i]
			}
		}
	}
	return
}
