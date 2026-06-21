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
		// Match the default-dark theme background so there's no flash of
		// white before the frontend applies the theme stylesheet.
		BackgroundColour: &options.RGBA{R: 0x18, G: 0x20, B: 0x2e, A: 0xff},
		// EnableFileDrop activates Wails' native file drag-and-drop handling,
		// giving the JS OnFileDrop callback absolute paths to dropped files.
		//
		// DisableWebViewDrop prevents WebKitGTK from handling the drop itself:
		// without this, dropping a file (e.g. an image or PDF) causes the
		// webview to navigate away and replace the entire app UI with a native
		// browser file view. Both flags are required together on Linux.
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown, // M5: clean up the file watcher on exit
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
		os.Exit(1)
	}
}

// parseArgs parses the command-line arguments MDLight cares about:
//
//	mdlight [path] [--theme name|--theme=name]
//
// No third-party flag library — the surface is small enough that a manual
// loop is clearer and adds no dependencies. Unknown flags are silently ignored
// so future flags don't cause hard failures on older installs.
func parseArgs() (filePath, themeName string) {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--theme" && i+1 < len(args):
			themeName = args[i+1]
			i++ // consume the value token
		case strings.HasPrefix(arg, "--theme="):
			themeName = strings.TrimPrefix(arg, "--theme=")
		case !strings.HasPrefix(arg, "--"):
			// First non-flag argument is the file path.
			if filePath == "" {
				filePath = arg
			}
		}
	}
	return
}
