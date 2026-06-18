// Package render converts a raw Markdown byte slice into a DocumentPayload
// ready to send to the Svelte frontend. It is the only place in the codebase
// that knows how Markdown becomes HTML.
//
// Pipeline (in order):
//  1. Strip and parse YAML front matter before goldmark sees the body.
//  2. Scan the body for mermaid fenced blocks and math delimiters.
//  3. Feed the remaining body to goldmark (CommonMark + GFM + chroma).
//  4. Count words and estimate reading time from the stripped body.
package render

import (
	"bytes"
	"regexp"
	"strings"
	"unicode"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"gopkg.in/yaml.v2"
)

// wordsPerMinute is the assumed adult reading pace for reading-time estimation.
const wordsPerMinute = 200

// FrontMatter holds the structured fields extracted from a YAML front-matter
// block. All fields are optional; a missing or malformed block leaves this
// zero-valued rather than producing an error.
type FrontMatter struct {
	Title string   `yaml:"title"`
	Date  string   `yaml:"date"`
	Tags  []string `yaml:"tags"`
}

// DocumentPayload is the complete result of rendering a Markdown file.
// It is the type returned by OpenFile and sent across the Wails boundary.
type DocumentPayload struct {
	HTML         string
	FrontMatter  FrontMatter
	WordCount    int
	ReadingMins  int
	NeedsMermaid bool
	NeedsMath    bool
}

// mermaidFenceRe matches a fenced code block whose language tag is "mermaid".
// It only needs to detect presence, not parse the contents.
var mermaidFenceRe = regexp.MustCompile("(?m)^```\\s*mermaid\\s*$")

// mathDelimRe matches the four common math delimiter forms:
//
//	$$...$$  (display)   \[...\]  (display)
//	$...$    (inline)    \(...\)  (inline)
//
// Detecting any of these is sufficient to set NeedsMath — we do not need to
// validate the math content itself here.
//
// The inline pattern is \$[^\s\d$] rather than \$[^$]: valid inline math
// always opens with a non-whitespace, non-digit character (e.g. $x^2$,
// $\alpha$), whereas currency amounts like "$5" or "$50" start with a digit.
// \$[^\s$] was previously used but still matched "$5" since digits are neither
// whitespace nor "$". Excluding \d fixes that false positive.
var mathDelimRe = regexp.MustCompile(`\$\$|\$[^\s\d$]|\\\[|\\\(`)

// goldmarkInstance is the configured goldmark converter. It is built once and
// reused across calls — goldmark is safe for concurrent use after construction.
var goldmarkInstance = goldmark.New(
	goldmark.WithExtensions(
		// GFM: tables, strikethrough, task lists, autolinks.
		extension.GFM,
		// Syntax highlighting for fenced code blocks via chroma.
		// WithClasses(true) emits CSS class names, not inline styles, so
		// each theme can define its own code-color palette.
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"), // fallback; themes override via CSS
			highlighting.WithFormatOptions(
				chromahtml.WithClasses(true),
				chromahtml.WithLineNumbers(false),
			),
		),
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(), // gives each heading an id="" for TOC anchors (v1.0)
	),
	goldmark.WithRendererOptions(
	// html.WithUnsafe() is intentionally NOT set.
	// Raw HTML in a Markdown source file is escaped, never rendered.
	// See §6 of the LDD.
	//
	// html.WithXHTML() is also not set — the default is standard HTML5
	// output (e.g. <br>, not <br />), which is correct for a webview target.
	),
)

// stripFrontMatter splits src into (frontMatterBlock, body).
// If no front-matter fence is found, frontMatterBlock is nil and body is src
// unchanged.
//
// The contract: a malformed YAML block is still stripped from body — the caller
// will see a nil frontMatterBlock, not raw YAML text in the rendered output.
func stripFrontMatter(src []byte) (frontMatterYAML []byte, body []byte) {
	// The opening fence must be exactly "---" followed immediately by a newline
	// (or CRLF). "----" or "---title:" are not valid fences.
	var rest []byte
	switch {
	case bytes.HasPrefix(src, []byte("---\n")):
		rest = src[4:]
	case bytes.HasPrefix(src, []byte("---\r\n")):
		rest = src[5:]
	default:
		return nil, src
	}

	// Search for a closing fence: a newline followed by exactly "---" and then
	// another newline, CRLF, or end of input. "---not-a-fence" must not match.
	//
	// We scan manually rather than using bytes.Index so we can enforce the
	// "nothing after ---" constraint on the closing line.
	lines := bytes.Split(rest, []byte("\n"))
	var fmLines [][]byte
	closingIdx := -1
	for i, line := range lines {
		// Normalise CRLF so "---\r" is treated the same as "---".
		trimmed := bytes.TrimSuffix(line, []byte("\r"))
		if bytes.Equal(trimmed, []byte("---")) {
			closingIdx = i
			break
		}
		fmLines = append(fmLines, line)
	}

	if closingIdx == -1 {
		// No valid closing fence — treat the whole file as body.
		return nil, src
	}

	frontMatterYAML = bytes.Join(fmLines, []byte("\n"))
	bodyLines := lines[closingIdx+1:]
	body = bytes.Join(bodyLines, []byte("\n"))
	// Trim a single leading newline from the body so the document doesn't
	// start with a blank line where the closing fence was.
	body = bytes.TrimPrefix(body, []byte("\n"))
	return frontMatterYAML, body
}

// parseFrontMatter attempts to unmarshal yamlBytes into a FrontMatter struct.
// If unmarshalling fails for any reason, it returns a zero-valued FrontMatter
// and swallows the error — a parse failure must never propagate out of OpenFile.
func parseFrontMatter(yamlBytes []byte) FrontMatter {
	if len(yamlBytes) == 0 {
		return FrontMatter{}
	}
	var fm FrontMatter
	if err := yaml.Unmarshal(yamlBytes, &fm); err != nil {
		// Swallowed intentionally. See §2 of the LDD.
		return FrontMatter{}
	}
	return fm
}

// detectFeatures scans the Markdown body for syntax that requires optional
// heavy libraries, setting the corresponding flags in the payload. This scan
// happens before rendering so the frontend can gate library loading without
// parsing HTML.
func detectFeatures(body []byte, payload *DocumentPayload) {
	payload.NeedsMermaid = mermaidFenceRe.Match(body)
	payload.NeedsMath = mathDelimRe.Match(body)
}

// countWords counts whitespace-separated tokens that contain at least one
// letter or digit, ignoring punctuation-only tokens like "---", "!!!", or
// "***". This is intentionally simple: the goal is an accurate enough
// reading-time estimate, not a publishable word count.
func countWords(body []byte) int {
	count := 0
	for _, token := range strings.Fields(string(body)) {
		for _, r := range token {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				count++
				break
			}
		}
	}
	return count
}

// readingMins converts a word count to an estimated reading time in minutes,
// rounding up so a 50-word snippet shows "1 min" rather than "0 min".
func readingMins(wordCount int) int {
	mins := wordCount / wordsPerMinute
	if wordCount%wordsPerMinute > 0 {
		mins++
	}
	return mins
}

// Render is the single entry point for the pipeline. It accepts the raw
// contents of a Markdown file and returns a fully populated DocumentPayload.
// It never returns an error: rendering failures degrade gracefully (the
// document may render partially, but the application will not crash).
func Render(src []byte) DocumentPayload {
	var payload DocumentPayload

	// Step 1: front matter.
	fmYAML, body := stripFrontMatter(src)
	payload.FrontMatter = parseFrontMatter(fmYAML)

	// Step 2: feature detection on the body before goldmark touches it.
	detectFeatures(body, &payload)

	// Step 3: word count from the raw body (before HTML tags are inserted).
	payload.WordCount = countWords(body)
	payload.ReadingMins = readingMins(payload.WordCount)

	// Step 4: render Markdown → HTML.
	var buf strings.Builder
	if err := goldmarkInstance.Convert(body, &buf); err != nil {
		// goldmark very rarely returns errors, but if it does we emit whatever
		// partial output it produced rather than surfacing the error upstream.
		payload.HTML = buf.String()
		return payload
	}
	payload.HTML = buf.String()

	return payload
}
