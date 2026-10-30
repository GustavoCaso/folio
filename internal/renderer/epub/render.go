// Package epub injects highlight-anchoring block IDs into parsed EPUB
// chapter HTML and serializes it back to a string, following the
// data-block-id convention described in ADR 0001.
package epub

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/net/html"
)

// blockTags are the element names treated as block-level for highlight
// anchoring purposes (headings, paragraphs, code blocks, lists, list items).
var blockTags = map[string]bool{
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
	"p": true, "pre": true, "ul": true, "ol": true, "li": true,
}

// droppedTags are elements removed from the output entirely: publisher
// styling and metadata the reader doesn't use, and scripts.
var droppedTags = map[string]bool{
	"link": true, "meta": true, "style": true, "script": true,
}

// ImageResolver returns the bytes and MIME type for an image referenced by
// href within the chapter, or ok=false if not found.
type ImageResolver func(href string) (data []byte, mimeType string, ok bool)

// LinkResolver maps an <a href> value to the href to emit, or ok=false to
// drop the href so a link that can't be followed isn't rendered as broken.
type LinkResolver func(href string) (rewritten string, ok bool)

// Render walks a parsed chapter document and returns the serialized
// children of its <body>. Along the way it:
//   - injects data-block-id attributes on block-level elements (numbered
//     ch{chapterIdx}-{tag}-{n});
//   - rewrites <img src> and SVG <image href> to base64 data URIs via
//     resolveImage;
//   - rewrites or drops <a href> via resolveLink;
//   - removes comments, droppedTags elements, and presentational or unsafe
//     attributes (see keepAttr).
//
// Only attributes and non-text elements are removed, so block numbering
// and character offsets match the source chapter.
func Render(doc *html.Node, chapterIdx int, resolveImage ImageResolver, resolveLink LinkResolver) (string, error) {
	counter := 0
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			n.Attr = filterAttrs(n.Attr)
			if blockTags[n.Data] {
				counter++
				n.Attr = append(n.Attr, html.Attribute{
					Key: "data-block-id",
					Val: fmt.Sprintf("ch%d-%s-%d", chapterIdx, n.Data, counter),
				})
			}
			switch {
			case n.Data == "img" && resolveImage != nil:
				rewriteImageAttr(n, "src", resolveImage)
			case n.Data == "image" && resolveImage != nil:
				rewriteImageAttr(n, "href", resolveImage)
			case n.Data == "a" && resolveLink != nil:
				rewriteLink(n, resolveLink)
			}
		}
		for c := n.FirstChild; c != nil; {
			next := c.NextSibling
			if c.Type == html.CommentNode || (c.Type == html.ElementNode && droppedTags[c.Data]) {
				n.RemoveChild(c)
			} else {
				walk(c)
			}
			c = next
		}
	}
	walk(doc)

	root := findBody(doc)
	if root == nil {
		root = doc
	}
	var buf bytes.Buffer
	for c := root.FirstChild; c != nil; c = c.NextSibling {
		if err := html.Render(&buf, c); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

// keepAttr reports whether a source attribute survives rendering. Dropped:
// class and style (publisher styling), data-* (source tooling, and would
// collide with the reader's own data-block-id), on* event handlers, xmlns
// declarations, and unadjusted prefixed attributes like epub:type. SVG's
// xlink:href is kept: the parser stores it as Namespace "xlink", Key "href".
func keepAttr(a html.Attribute) bool {
	key := strings.ToLower(a.Key)
	switch {
	case key == "class", key == "style":
		return false
	case strings.HasPrefix(key, "data-"), strings.HasPrefix(key, "on"), strings.HasPrefix(key, "xmlns"):
		return false
	case a.Namespace == "" && strings.Contains(key, ":"):
		return false
	}
	return true
}

func filterAttrs(attrs []html.Attribute) []html.Attribute {
	out := attrs[:0]
	for _, a := range attrs {
		if keepAttr(a) {
			out = append(out, a)
		}
	}
	return out
}

func findBody(n *html.Node) *html.Node {
	if n.Type == html.ElementNode && n.Data == "body" {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if body := findBody(c); body != nil {
			return body
		}
	}
	return nil
}

func rewriteImageAttr(n *html.Node, key string, resolveImage ImageResolver) {
	for i, attr := range n.Attr {
		if attr.Key != key {
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

func rewriteLink(n *html.Node, resolveLink LinkResolver) {
	for i, attr := range n.Attr {
		if attr.Key != "href" {
			continue
		}
		if rewritten, ok := resolveLink(attr.Val); ok {
			n.Attr[i].Val = rewritten
		} else {
			n.Attr = append(n.Attr[:i], n.Attr[i+1:]...)
		}
		return
	}
}
