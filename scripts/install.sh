#!/bin/sh
set -eu

REPO="mdlight-dev/mdlight"
BIN_DIR="${MDLIGHT_BIN_DIR:-$HOME/.local/bin}"
APP_DIR="${HOME}/.local/share/applications"
ICON_DIR="${HOME}/.local/share/icons/hicolor/256x256/apps"
PIXMAPS_DIR="${HOME}/.local/share/pixmaps"

detect_arch() {
    os=$(uname -s | tr '[:upper:]' '[:lower:]')
    arch=$(uname -m)
    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) echo "Unsupported architecture: $arch"; exit 1 ;;
    esac
    case "$os" in
        linux) os="linux" ;;
        darwin) os="darwin" ;;
        mingw*|cygwin*) os="windows" ;;
        *) echo "Unsupported OS: $os"; exit 1 ;;
    esac
    echo "${os}_${arch}"
}

get_latest_version() {
    url="https://api.github.com/repos/${REPO}/releases/latest"
    version=$(curl -sL "$url" | grep '"tag_name"' | head -1 | cut -d'"' -f4)
    if [ -z "$version" ]; then
        echo "Failed to detect latest version" >&2
        exit 1
    fi
    echo "$version"
}

main() {
    plat=$(detect_arch)
    os=$(echo "$plat" | cut -d_ -f1)
    arch=$(echo "$plat" | cut -d_ -f2)

    version="${1:-$(get_latest_version)}"
    echo "Installing MDLight $version ($os/$arch)..."

    ext=""
    [ "$os" = "windows" ] && ext=".exe"

    url="https://github.com/${REPO}/releases/download/${version}/mdlight_${version}_${os}_${arch}${ext}"

    mkdir -p "$BIN_DIR" "$APP_DIR" "$ICON_DIR" "$PIXMAPS_DIR"

    echo "Downloading binary..."
    curl -sSL "$url" -o "${BIN_DIR}/mdlight${ext}"
    chmod +x "${BIN_DIR}/mdlight${ext}"
    echo "Binary installed to ${BIN_DIR}/mdlight${ext}"

    # Remove stale binary from common system paths
    if [ -f /usr/local/bin/mdlight ] && [ "$(dirname "$BIN_DIR")" != "/usr/local/bin" ]; then
        echo "Warning: old mdlight found at /usr/local/bin/. Run 'sudo rm /usr/local/bin/mdlight' to avoid conflicts."
    fi

    # Install .desktop file
    desktop_url="https://raw.githubusercontent.com/${REPO}/${version}/build/linux/mdlight.desktop"
    if curl -sSL "$desktop_url" -o "${APP_DIR}/mdlight.desktop"; then
        chmod +x "${APP_DIR}/mdlight.desktop"
        sed -i "s|^Exec=mdlight |Exec=${BIN_DIR}/mdlight |" "${APP_DIR}/mdlight.desktop"
        echo "Desktop entry installed to ${APP_DIR}/mdlight.desktop"
    else
        echo "Warning: could not download .desktop file (no network or version tag not on main branch)"
        cat > "${APP_DIR}/mdlight.desktop" << EOF
[Desktop Entry]
Type=Application
Name=MDLight
GenericName=Markdown Reader
Comment=A beautiful, lightweight Markdown reader
Exec=${BIN_DIR}/mdlight %f
Icon=mdlight
Terminal=false
Categories=Utility;TextEditor;
MimeType=text/markdown;text/x-markdown;
Keywords=markdown;reader;editor;md;
StartupNotify=true
StartupWMClass=mdlight
EOF
        chmod +x "${APP_DIR}/mdlight.desktop"
    fi

    # Download icon
    icon_url="https://raw.githubusercontent.com/${REPO}/${version}/build/appicon.png"
    if curl -sSL "$icon_url" -o "${ICON_DIR}/mdlight.png"; then
        cp "${ICON_DIR}/mdlight.png" "${PIXMAPS_DIR}/mdlight.png"
        echo "Icon installed"
    fi

    # Update icon theme cache
    HICOLOR_DIR="${HOME}/.local/share/icons/hicolor"
    if [ -d "$HICOLOR_DIR" ] && [ ! -f "$HICOLOR_DIR/index.theme" ]; then
        cat > "$HICOLOR_DIR/index.theme" << EOF
[Icon Theme]
Name=Hicolor
Comment=Fallback icon theme
Hidden=true
Directories=256x256/apps

[256x256/apps]
Size=256
Context=Applications
Type=Threshold
EOF
    fi
    if command -v gtk-update-icon-cache >/dev/null 2>&1; then
        gtk-update-icon-cache "$HICOLOR_DIR" 2>/dev/null || true
    fi

    # Update desktop database
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database "$APP_DIR" 2>/dev/null || true
    fi

    # PATH check
    case ":$PATH:" in
        *:"${BIN_DIR}":*)
            ;;
        *)
            shell_rc="${HOME}/.bashrc"
            [ -n "${ZSH_VERSION:-}" ] && shell_rc="${HOME}/.zshrc"
            [ -f "${HOME}/.config/fish/config.fish" ] && shell_rc="${HOME}/.config/fish/config.fish"

            case "$shell_rc" in
                *.fish)
                    echo "set -U fish_user_paths ${BIN_DIR} \$fish_user_paths" >> "$shell_rc"
                    ;;
                *)
                    echo "export PATH=\"${BIN_DIR}:\$PATH\"" >> "$shell_rc"
                    ;;
            esac
            echo "Added ${BIN_DIR} to PATH in ${shell_rc}"
            echo "Restart your shell or run: export PATH=\"${BIN_DIR}:\$PATH\""
            ;;
    esac

    echo ""
    echo "MDLight installed. Run: mdlight file.md"
}

main "$@"
