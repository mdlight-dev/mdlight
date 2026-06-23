# MDLight

A beautiful, lightweight Markdown reader — no vaults, no plugins, no accounts. Just open a file and read.

## Quick start

```sh
# Usage
mdlight document.md
mdlight notes.md --theme nord
mdlight                         # opens file picker
```

## Features

- **Full CommonMark + GFM**: tables, strikethrough, task lists, autolinks, fenced code blocks
- **Syntax highlighting**: code blocks styled via chroma with per-theme CSS palettes
- **YAML front matter**: title, date, tags rendered as a metadata card
- **Dark theme**: beautiful default-dark theme, swappable at runtime via `--theme`
- **File watching**: auto-reloads when the file changes on disk
- **Zoom**: Ctrl+= / Ctrl+- / Ctrl+0, status bar indicator
- **Image handling**: local images embedded as data URIs; remote images shown as click-to-load placeholders
- **Atomic saves**: write to temp file, rename over original — no half-written files
- **Drag-and-drop**: drag a Markdown file onto the window to open
- **Word count & reading time**: shown in the status bar

## Installation

### Pre-built binaries

Download from the [releases page](https://github.com/le-blanc/mdlight/releases):

- `mdlight_vX.Y.Z_linux_amd64` — Linux x86_64
- `mdlight_vX.Y.Z_linux_arm64` — Linux ARM64
- `mdlight_vX.Y.Z_darwin_amd64` — macOS Intel
- `mdlight_vX.Y.Z_darwin_arm64` — macOS Apple Silicon
- `mdlight_vX.Y.Z_windows_amd64` — Windows x86_64

Linux also gets `.deb`, `.rpm`, and `.apk` packages.

### From source

Requires Go 1.23+, Node 20+, and platform webview libraries:

```sh
# Linux
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev

# macOS
xcode-select --install

# Windows
# WebView2 is included in Windows 10+
```

```sh
git clone https://github.com/le-blanc/mdlight
cd mdlight
make build
./build/bin/mdlight README.md
```

## Usage

```sh
# Open a file
mdlight file.md

# Open with a specific theme
mdlight file.md --theme nord

# Open the file picker
mdlight
```

### Themes

Built-in themes: `default-dark`

User themes: place `.css` files in `~/.config/mdlight/themes/` and reference by name (without `.css` extension).

## Performance

Measured on Linux x86_64, opening a typical Markdown document:

| Metric | Measured | LDD target |
|--------|----------|------------|
| Resident memory (RSS) | ~210 MB | 70–150 MB |
| Startup (cold) | TBD | <500 ms |

The RSS includes the WebKitGTK webview engine, which is the majority of the footprint. Plain Markdown files incur no extra loading; Mermaid and math libraries are only fetched when the document contains that syntax (v2.0).

## Project structure

```
mdlight/
  main.go              — CLI parsing, wails.Run bootstrap
  app.go               — Wails-bound methods (OpenFile, SaveFile, …)
  internal/
    render/            — goldmark + chroma pipeline, image rewriting
    theme/             — theme discovery and resolution
    watch/             — fsnotify wrapper with debounce
    state/             — recent-files persistence (v1.0)
  frontend/
    src/
      App.svelte       — Svelte application root
      style.css        — structural CSS rules (no hardcoded colors)
      themes/builtin/  — shipped theme files
      assets/fonts/    — Literata + JetBrains Mono
```

## Roadmap

- **v0.1** — Core Markdown reader (current)
- **v1.0** — Table of contents, find, edit mode, 6 built-in themes
- **v2.0** — Mermaid diagrams, KaTeX math, PDF export, focus mode
- **v3.0** — Community theme sharing via GitHub directory

## License

MIT
