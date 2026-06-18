package render_test

import (
	"strings"
	"testing"

	"mdlight/internal/render"
)

// ---------------------------------------------------------------------------
// Front matter
// ---------------------------------------------------------------------------

func TestFrontMatter_PopulatesFields(t *testing.T) {
	src := []byte(`---
title: My Document
date: 2026-06-18
tags:
  - go
  - markdown
---

# Hello

Body text here.
`)
	p := render.Render(src)

	if p.FrontMatter.Title != "My Document" {
		t.Errorf("Title: got %q, want %q", p.FrontMatter.Title, "My Document")
	}
	if p.FrontMatter.Date != "2026-06-18" {
		t.Errorf("Date: got %q, want %q", p.FrontMatter.Date, "2026-06-18")
	}
	if len(p.FrontMatter.Tags) != 2 || p.FrontMatter.Tags[0] != "go" {
		t.Errorf("Tags: got %v, want [go markdown]", p.FrontMatter.Tags)
	}
}

func TestFrontMatter_MalformedDoesNotBlockRender(t *testing.T) {
	// A malformed YAML block must be stripped from the body AND the document
	// must still render. This is a hard contract from §2 of the LDD.
	src := []byte(`---
title: [unclosed bracket
tags: {bad: yaml: here
---

# Still renders

This paragraph should appear in the output.
`)
	p := render.Render(src)

	// Front matter is zero-valued, not an error.
	if p.FrontMatter.Title != "" {
		t.Errorf("expected zero-valued FrontMatter.Title, got %q", p.FrontMatter.Title)
	}

	// Body must still render — malformed YAML must not dump raw text or block output.
	if !strings.Contains(p.HTML, "Still renders") {
		t.Errorf("body did not render: HTML = %q", p.HTML)
	}
	if strings.Contains(p.HTML, "unclosed bracket") {
		t.Errorf("raw YAML leaked into rendered HTML: %q", p.HTML)
	}
}

func TestFrontMatter_MissingFencePassesBodyThrough(t *testing.T) {
	src := []byte(`# No front matter

Just a plain document.
`)
	p := render.Render(src)

	if p.FrontMatter.Title != "" {
		t.Errorf("expected empty FrontMatter for file without front matter, got %+v", p.FrontMatter)
	}
	if !strings.Contains(p.HTML, "No front matter") {
		t.Errorf("body did not render: %q", p.HTML)
	}
}

func TestFrontMatter_OpeningFenceWithoutClosingTreatedAsBody(t *testing.T) {
	// An opening --- with no closing fence means the whole file is body.
	src := []byte(`---
title: dangling

# Heading

Paragraph.
`)
	p := render.Render(src)
	// The whole file was treated as body — heading should render.
	if !strings.Contains(p.HTML, "Heading") {
		t.Errorf("expected file to render as body when closing fence is absent, got: %q", p.HTML)
	}
}

func TestFrontMatter_DashesFollowedByTextNotAFence(t *testing.T) {
	// "----" or "---title:" must not be treated as an opening fence.
	src := []byte(`----
title: not front matter

# Heading
`)
	p := render.Render(src)
	if p.FrontMatter.Title != "" {
		t.Errorf("'----' should not be parsed as a front matter fence, got title=%q", p.FrontMatter.Title)
	}
	if !strings.Contains(p.HTML, "Heading") {
		t.Errorf("body did not render: %q", p.HTML)
	}
}

func TestFrontMatter_ClosingFenceWithTrailingTextNotAFence(t *testing.T) {
	// "---not-a-fence" on the closing line must not terminate front matter early
	// and must not strip content from the body.
	src := []byte(`---
title: Real Title
---

# Body heading

Some content after a real closing fence.
`)
	p := render.Render(src)
	if p.FrontMatter.Title != "Real Title" {
		t.Errorf("Title: got %q, want %q", p.FrontMatter.Title, "Real Title")
	}
	if !strings.Contains(p.HTML, "Body heading") {
		t.Errorf("body did not render correctly: %q", p.HTML)
	}
}

// ---------------------------------------------------------------------------
// Security: raw HTML escaping
// ---------------------------------------------------------------------------

func TestHTMLEscaping_RawHTMLIsEscaped(t *testing.T) {
	// §6 of the LDD: goldmark runs without html.WithUnsafe(). Raw HTML in
	// the source must be escaped, never rendered as markup.
	src := []byte(`# Safe

<script>alert('xss')</script>

<img src="x" onerror="evil()">

Normal paragraph.
`)
	p := render.Render(src)

	// The script tag must not appear as executable markup.
	if strings.Contains(p.HTML, "<script>") {
		t.Errorf("raw <script> tag was not escaped: %q", p.HTML)
	}
	// The img onerror must not appear unescaped.
	if strings.Contains(p.HTML, `onerror="evil()"`) {
		t.Errorf("raw <img onerror> was not escaped: %q", p.HTML)
	}
	// Normal content must still render.
	if !strings.Contains(p.HTML, "Normal paragraph") {
		t.Errorf("normal content missing from output: %q", p.HTML)
	}
}

// ---------------------------------------------------------------------------
// GFM extensions
// ---------------------------------------------------------------------------

func TestGFM_TablesRender(t *testing.T) {
	src := []byte(`| Name | Value |
|------|-------|
| foo  | 42    |
`)
	p := render.Render(src)
	if !strings.Contains(p.HTML, "<table>") && !strings.Contains(p.HTML, "<table ") {
		t.Errorf("GFM table not rendered: %q", p.HTML)
	}
}

func TestGFM_StrikethroughRenders(t *testing.T) {
	src := []byte(`~~struck~~`)
	p := render.Render(src)
	if !strings.Contains(p.HTML, "<del>") {
		t.Errorf("GFM strikethrough not rendered: %q", p.HTML)
	}
}

func TestGFM_TaskListsRender(t *testing.T) {
	src := []byte(`- [x] done
- [ ] todo
`)
	p := render.Render(src)
	if !strings.Contains(p.HTML, `type="checkbox"`) {
		t.Errorf("GFM task list not rendered: %q", p.HTML)
	}
}

func TestGFM_AutolinksRender(t *testing.T) {
	src := []byte(`Visit https://example.com for more.`)
	p := render.Render(src)
	if !strings.Contains(p.HTML, `href="https://example.com"`) {
		t.Errorf("GFM autolink not rendered: %q", p.HTML)
	}
}

// ---------------------------------------------------------------------------
// Fenced code blocks + syntax highlighting
// ---------------------------------------------------------------------------

func TestSyntaxHighlighting_FencedBlockHasChromaClasses(t *testing.T) {
	// Chroma must emit CSS class names, not inline styles, so themes can
	// define their own code-color palettes. §2 of the LDD.
	src := []byte("```go\npackage main\n\nfunc main() {}\n```\n")
	p := render.Render(src)

	// Chroma wraps highlighted output in a <code class="...chroma..."> or
	// similar; we check that class= is present and style= is absent on the
	// token spans.
	if !strings.Contains(p.HTML, "chroma") {
		t.Errorf("expected chroma CSS classes in output, got: %q", p.HTML)
	}
	// Inline styles would start with style="color:
	if strings.Contains(p.HTML, `style="color:`) {
		t.Errorf("chroma emitted inline styles; expected CSS classes only")
	}
}

func TestSyntaxHighlighting_UnknownLanguageRendersAsPlainCode(t *testing.T) {
	src := []byte("```unknownlang\nsome code here\n```\n")
	p := render.Render(src)
	if !strings.Contains(p.HTML, "some code here") {
		t.Errorf("unknown language fenced block not rendered: %q", p.HTML)
	}
}

// ---------------------------------------------------------------------------
// Feature detection: NeedsMermaid / NeedsMath
// ---------------------------------------------------------------------------

func TestFeatureDetection_MermaidBlock(t *testing.T) {
	src := []byte("# Doc\n\n```mermaid\ngraph TD;\n  A-->B;\n```\n")
	p := render.Render(src)
	if !p.NeedsMermaid {
		t.Error("NeedsMermaid should be true when a mermaid fenced block is present")
	}
	if p.NeedsMath {
		t.Error("NeedsMath should be false when no math delimiters are present")
	}
}

func TestFeatureDetection_MathDisplayDelimiters(t *testing.T) {
	src := []byte("# Doc\n\n$$E = mc^2$$\n")
	p := render.Render(src)
	if !p.NeedsMath {
		t.Error("NeedsMath should be true for $$ display math")
	}
	if p.NeedsMermaid {
		t.Error("NeedsMermaid should be false when no mermaid block is present")
	}
}

func TestFeatureDetection_MathInlineDelimiter(t *testing.T) {
	src := []byte("Inline math: $x^2 + y^2 = z^2$.\n")
	p := render.Render(src)
	if !p.NeedsMath {
		t.Error("NeedsMath should be true for inline $ math")
	}
}

func TestFeatureDetection_MathLatexDelimiters(t *testing.T) {
	src := []byte(`Display: \[E = mc^2\] and inline \(a + b\).`)
	p := render.Render(src)
	if !p.NeedsMath {
		t.Error("NeedsMath should be true for \\[ and \\( delimiters")
	}
}

func TestFeatureDetection_NeitherFlagSetForPlainDoc(t *testing.T) {
	src := []byte("# Plain\n\nJust some text and a code block.\n\n```python\nprint('hi')\n```\n")
	p := render.Render(src)
	if p.NeedsMermaid {
		t.Error("NeedsMermaid should be false for a plain document")
	}
	if p.NeedsMath {
		t.Error("NeedsMath should be false for a plain document")
	}
}

func TestFeatureDetection_CurrencyDoesNotTriggerMath(t *testing.T) {
	// "$5", "$50", "$1,000" must not set NeedsMath — digits after $ are currency.
	src := []byte("The price is $5 and the budget is $1000 per month.\n")
	p := render.Render(src)
	if p.NeedsMath {
		t.Error("NeedsMath should be false for currency amounts like $5 or $1000")
	}
}

func TestFeatureDetection_BothFlagsSet(t *testing.T) {
	src := []byte("$$x$$\n\n```mermaid\ngraph LR; A-->B\n```\n")
	p := render.Render(src)
	if !p.NeedsMermaid {
		t.Error("NeedsMermaid should be true")
	}
	if !p.NeedsMath {
		t.Error("NeedsMath should be true")
	}
}

// ---------------------------------------------------------------------------
// Word count and reading time
// ---------------------------------------------------------------------------

func TestWordCount_SimpleBody(t *testing.T) {
	// 10 words
	src := []byte("one two three four five six seven eight nine ten\n")
	p := render.Render(src)
	if p.WordCount != 10 {
		t.Errorf("WordCount: got %d, want 10", p.WordCount)
	}
}

func TestWordCount_PunctuationOnlyTokensIgnored(t *testing.T) {
	// "---", "!!!", "***" contain no letters or digits and must not be counted.
	src := []byte("hello --- world !!! done\n")
	p := render.Render(src)
	if p.WordCount != 3 {
		t.Errorf("WordCount: got %d, want 3 (punctuation-only tokens must not count)", p.WordCount)
	}
}

func TestReadingMins_RoundsUp(t *testing.T) {
	// 1 word → rounds up to 1 min (not 0)
	src := []byte("hello\n")
	p := render.Render(src)
	if p.ReadingMins != 1 {
		t.Errorf("ReadingMins: got %d, want 1 for a single word", p.ReadingMins)
	}
}

func TestReadingMins_LongDoc(t *testing.T) {
	// 400 words → exactly 2 min at 200 wpm
	words := strings.Repeat("word ", 400)
	p := render.Render([]byte(words))
	if p.ReadingMins != 2 {
		t.Errorf("ReadingMins: got %d, want 2 for 400 words", p.ReadingMins)
	}
}

// ---------------------------------------------------------------------------
// Heading IDs (for future TOC anchor support, v1.0)
// ---------------------------------------------------------------------------

func TestHeadingIDs_ArePresent(t *testing.T) {
	src := []byte("# My Heading\n\n## Sub Heading\n")
	p := render.Render(src)
	// parser.WithAutoHeadingID() should emit id= attributes.
	if !strings.Contains(p.HTML, `id="`) {
		t.Errorf("expected heading id= attributes for TOC support, got: %q", p.HTML)
	}
}

// ---------------------------------------------------------------------------
// Empty input
// ---------------------------------------------------------------------------

func TestEmptyInput(t *testing.T) {
	p := render.Render([]byte{})
	// Must not panic; HTML may be empty or whitespace.
	_ = p
}

func TestWhitespaceOnlyInput(t *testing.T) {
	p := render.Render([]byte("   \n\n\t\n"))
	_ = p
}
