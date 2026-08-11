package parser

import (
	"encoding/base64"
	"fmt"
	"html"
	"strings"
)

// blockTags maps each blockKind to its rendered HTML tag, matching the
// data-block-id convention used by renderer/epub (ch{N}-{tag}-{M}).
var blockTags = map[blockKind]string{
	blockProse: "p",
	blockCode:  "pre",
	blockH1:    "h1",
	blockH2:    "h2",
	blockH3:    "h3",
}

// renderChapterHTML renders a chapter's classified blocks as HTML, injecting
// data-block-id="ch{chapterIdx}-{tag}-{n}" on each block in the same scheme
// renderer/epub uses, so highlight anchoring and the reader path are shared
// between the epub and native-PDF pipelines.
func renderChapterHTML(chapterIdx int, blocks []block) string {
	var buf strings.Builder
	for i, b := range blocks {
		tag := blockTags[b.Kind]
		n := i + 1
		if b.Kind == blockCode {
			fmt.Fprintf(&buf, `<pre data-block-id="ch%d-pre-%d"><code>%s</code></pre>`,
				chapterIdx, n, html.EscapeString(b.Text))
			continue
		}
		fmt.Fprintf(&buf, `<%s data-block-id="ch%d-%s-%d">%s</%s>`,
			tag, chapterIdx, tag, n, html.EscapeString(b.Text), tag)
	}
	return buf.String()
}

// renderImageTag renders an <img> tag embedding data as a base64 data URI,
// matching renderer/epub's inline-image handling.
func renderImageTag(data []byte, mimeType string) string {
	return fmt.Sprintf(`<img src="data:%s;base64,%s">`, mimeType, base64.StdEncoding.EncodeToString(data))
}
