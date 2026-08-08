// Package epubrender injects highlight-anchoring block IDs into parsed EPUB
// chapter HTML and serializes it back to a string, mirroring the
// data-block-id convention used by the goldmark-based renderer for
// PDF/Markdown documents (see internal/renderer).
package epubrender

import (
	"bytes"
	"encoding/base64"
	"fmt"

	"golang.org/x/net/html"
)

// blockTags are the element names treated as block-level for highlight
// anchoring purposes, mirroring the goldmark renderer's set (headings,
// paragraphs, code blocks, lists, list items).
var blockTags = map[string]bool{
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"p": true, "pre": true, "ul": true, "ol": true, "li": true,
}

// ImageResolver returns the bytes and MIME type for an image referenced by
// href within the chapter, or ok=false if not found.
type ImageResolver func(href string) (data []byte, mimeType string, ok bool)

// Render walks a parsed chapter document, injects data-block-id attributes
// on block-level elements (numbered ch{chapterIdx}-{tag}-{n}), rewrites
// <img src> to base64 data URIs via resolveImage, and returns the
// serialized HTML.
func Render(doc *html.Node, chapterIdx int, resolveImage ImageResolver) (string, error) {
	counter := 0
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if blockTags[n.Data] {
				counter++
				n.Attr = append(n.Attr, html.Attribute{
					Key: "data-block-id",
					Val: fmt.Sprintf("ch%d-%s-%d", chapterIdx, n.Data, counter),
				})
			}
			if n.Data == "img" && resolveImage != nil {
				rewriteImageSrc(n, resolveImage)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)

	var buf bytes.Buffer
	if err := html.Render(&buf, doc); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func rewriteImageSrc(n *html.Node, resolveImage ImageResolver) {
	for i, attr := range n.Attr {
		if attr.Key != "src" {
			continue
		}
		data, mimeType, ok := resolveImage(attr.Val)
		if !ok {
			return
		}
		n.Attr[i].Val = fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))
		return
	}
}
