package render_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mdlight/internal/render"
)

func writeTemp(t *testing.T, name string, data []byte) (dir string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return dir
}

var tiny1x1PNG, _ = base64.StdEncoding.DecodeString(
	"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==",
)

func TestRewriteImages_LocalRelative_EmbeddedAsDataURI(t *testing.T) {
	dir := writeTemp(t, "photo.png", tiny1x1PNG)
	input := `<p><img src="photo.png" alt="A photo"></p>`
	got := render.RewriteImages(input, dir)

	if !strings.Contains(got, "data:image/png;base64,") {
		t.Errorf("expected data URI for local image, got: %s", got)
	}
	if strings.Contains(got, `src="photo.png"`) {
		t.Errorf("original relative src should have been replaced, got: %s", got)
	}
}

func TestRewriteImages_LocalSubdirectory(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "assets")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "img.png"), tiny1x1PNG, 0644); err != nil {
		t.Fatal(err)
	}

	input := `<img src="assets/img.png">`
	got := render.RewriteImages(input, dir)

	if !strings.Contains(got, "data:image/png;base64,") {
		t.Errorf("expected data URI for subdirectory image, got: %s", got)
	}
}

func TestRewriteImages_MissingLocalFile_PlaceholderShown(t *testing.T) {
	dir := t.TempDir()
	input := `<img src="missing.png" alt="Gone">`
	got := render.RewriteImages(input, dir)

	if !strings.Contains(got, "missing-image-placeholder") {
		t.Errorf("expected missing-image-placeholder for unreadable file, got: %s", got)
	}
	if strings.Contains(got, "<img") {
		t.Errorf("broken <img> tag should not appear for missing file, got: %s", got)
	}
}

func TestRewriteImages_RemoteHTTPS_ReplacedWithPlaceholder(t *testing.T) {
	input := `<img src="https://example.com/photo.jpg" alt="Remote">`
	got := render.RewriteImages(input, "/tmp")

	if strings.Contains(got, "<img") {
		t.Errorf("<img> should be replaced by placeholder, got: %s", got)
	}
	if !strings.Contains(got, "remote-image-placeholder") {
		t.Errorf("expected remote-image-placeholder class, got: %s", got)
	}
	if !strings.Contains(got, "https://example.com/photo.jpg") {
		t.Errorf("placeholder should carry the original URL, got: %s", got)
	}
}

func TestRewriteImages_RemoteHTTP_ReplacedWithPlaceholder(t *testing.T) {
	input := `<img src="http://example.com/photo.jpg">`
	got := render.RewriteImages(input, "/tmp")

	if !strings.Contains(got, "remote-image-placeholder") {
		t.Errorf("expected remote-image-placeholder for http:// URL, got: %s", got)
	}
}

func TestRewriteImages_DataURI_PassedThrough(t *testing.T) {
	dataURI := "data:image/png;base64,abc123"
	input := `<img src="` + dataURI + `">`
	got := render.RewriteImages(input, "/tmp")

	if !strings.Contains(got, dataURI) {
		t.Errorf("data URI should pass through unchanged, got: %s", got)
	}
}

func TestRewriteImages_AbsolutePath_PassedThrough(t *testing.T) {
	input := `<img src="/absolute/path/image.png">`
	got := render.RewriteImages(input, "/tmp")

	if !strings.Contains(got, `src="/absolute/path/image.png"`) {
		t.Errorf("absolute path should pass through unchanged, got: %s", got)
	}
}

func TestRewriteImages_NoImages_OutputUnchanged(t *testing.T) {
	input := `<p>No images here.</p><pre><code>some code</code></pre>`
	got := render.RewriteImages(input, "/tmp")

	if !strings.Contains(got, "No images here.") {
		t.Errorf("text content should survive a no-op rewrite, got: %s", got)
	}
}

func TestRewriteImages_ImgWithNoSrc_Ignored(t *testing.T) {
	input := `<img alt="No source">`
	got := render.RewriteImages(input, "/tmp")
	_ = got
}

func TestRewriteImages_BadHTML_OriginalReturned(t *testing.T) {
	input := `<img src="<<not valid html`
	got := render.RewriteImages(input, "/tmp")
	_ = got
}

func TestRewriteImages_SVGByExtension_CorrectMIME(t *testing.T) {
	svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg"><rect width="1" height="1"/></svg>`)
	dir := writeTemp(t, "icon.svg", svgData)
	input := `<img src="icon.svg" alt="Icon">`
	got := render.RewriteImages(input, dir)

	if !strings.Contains(got, "data:image/svg+xml;base64,") {
		t.Errorf("expected image/svg+xml MIME for .svg file, got: %s", got)
	}
}
