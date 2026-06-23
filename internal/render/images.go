package render

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

func RewriteImages(htmlStr, baseDir string) string {
	if !strings.Contains(htmlStr, "<img") {
		return htmlStr
	}

	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "img" {
			rewriteImgNode(n, baseDir)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return htmlStr
	}
	return buf.String()
}

func rewriteImgNode(n *html.Node, baseDir string) {
	src := attrVal(n, "src")
	if src == "" {
		return
	}

	switch {
	case strings.HasPrefix(src, "data:"):
		return

	case strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://"):
		alt := attrVal(n, "alt")
		placeholder := remotePlaceholderNode(src, alt)
		n.Parent.InsertBefore(placeholder, n)
		n.Parent.RemoveChild(n)
		return

	case strings.HasPrefix(src, "/"):
		return

	default:
		absPath := filepath.Join(baseDir, filepath.FromSlash(src))
		data, err := os.ReadFile(absPath)
		if err != nil {
			alt := attrVal(n, "alt")
			placeholder := missingPlaceholderNode(src, alt)
			n.Parent.InsertBefore(placeholder, n)
			n.Parent.RemoveChild(n)
			return
		}

		mimeType := http.DetectContentType(data)
		mimeType = refineMIME(mimeType, absPath)

		encoded := base64.StdEncoding.EncodeToString(data)
		setAttr(n, "src", "data:"+mimeType+";base64,"+encoded)
	}
}

func remotePlaceholderNode(url, alt string) *html.Node {
	label := alt
	if label == "" {
		label = "remote image"
	}
	span := &html.Node{
		Type: html.ElementNode,
		Data: "span",
		Attr: []html.Attribute{
			{Key: "class", Val: "remote-image-placeholder"},
			{Key: "data-src", Val: url},
			{Key: "title", Val: url},
			{Key: "role", Val: "button"},
			{Key: "tabindex", Val: "0"},
		},
	}
	text := &html.Node{
		Type: html.TextNode,
		Data: "[image] " + label + " (click to load)",
	}
	span.AppendChild(text)
	return span
}

func missingPlaceholderNode(src, alt string) *html.Node {
	label := alt
	if label == "" {
		label = src
	}
	span := &html.Node{
		Type: html.ElementNode,
		Data: "span",
		Attr: []html.Attribute{
			{Key: "class", Val: "missing-image-placeholder"},
			{Key: "title", Val: "Image not found: " + src},
		},
	}
	text := &html.Node{
		Type: html.TextNode,
		Data: "[missing] " + label,
	}
	span.AppendChild(text)
	return span
}

func attrVal(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func setAttr(n *html.Node, name, value string) {
	for i, a := range n.Attr {
		if a.Key == name {
			n.Attr[i].Val = value
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: name, Val: value})
}

func refineMIME(detected, path string) string {
	if detected != "application/octet-stream" && detected != "text/plain; charset=utf-8" {
		return detected
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".svg":
		return "image/svg+xml"
	case ".avif":
		return "image/avif"
	case ".webp":
		return "image/webp"
	}
	return detected
}
