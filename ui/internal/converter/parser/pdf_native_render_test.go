package parser

import "testing"

func TestRenderChapterHTML_ProseBlockGetsPTagAndBlockID(t *testing.T) {
	blocks := []block{{Kind: blockProse, Text: "Hello world."}}
	out := renderChapterHTML(0, blocks)

	want := `<p data-block-id="ch0-p-1">Hello world.</p>`
	if out != want {
		t.Errorf("expected %q, got %q", want, out)
	}
}

func TestRenderChapterHTML_CodeBlockGetsPreCodeTag(t *testing.T) {
	blocks := []block{{Kind: blockCode, Text: "fn main() {}"}}
	out := renderChapterHTML(2, blocks)

	want := `<pre data-block-id="ch2-pre-1"><code>fn main() {}</code></pre>`
	if out != want {
		t.Errorf("expected %q, got %q", want, out)
	}
}

func TestRenderChapterHTML_HeadingTiersMapToTags(t *testing.T) {
	blocks := []block{
		{Kind: blockH1, Text: "Chapter 2"},
		{Kind: blockH2, Text: "Guessing Game"},
		{Kind: blockH3, Text: "Processing a Guess"},
	}
	out := renderChapterHTML(1, blocks)

	want := `<h1 data-block-id="ch1-h1-1">Chapter 2</h1>` +
		`<h2 data-block-id="ch1-h2-2">Guessing Game</h2>` +
		`<h3 data-block-id="ch1-h3-3">Processing a Guess</h3>`
	if out != want {
		t.Errorf("expected %q, got %q", want, out)
	}
}

func TestRenderChapterHTML_EscapesTextContent(t *testing.T) {
	blocks := []block{{Kind: blockProse, Text: "a < b & c > d"}}
	out := renderChapterHTML(0, blocks)

	want := `<p data-block-id="ch0-p-1">a &lt; b &amp; c &gt; d</p>`
	if out != want {
		t.Errorf("expected %q, got %q", want, out)
	}
}

func TestRenderChapterHTML_BlockIDCounterIncrementsAcrossKinds(t *testing.T) {
	blocks := []block{
		{Kind: blockProse, Text: "one"},
		{Kind: blockCode, Text: "two"},
		{Kind: blockProse, Text: "three"},
	}
	out := renderChapterHTML(5, blocks)

	want := `<p data-block-id="ch5-p-1">one</p>` +
		`<pre data-block-id="ch5-pre-2"><code>two</code></pre>` +
		`<p data-block-id="ch5-p-3">three</p>`
	if out != want {
		t.Errorf("expected %q, got %q", want, out)
	}
}

func TestRenderImageTag_EmbedsBase64DataURI(t *testing.T) {
	out := renderImageTag([]byte{0xff, 0x00}, "image/png")
	want := `<img src="data:image/png;base64,/wA=">`
	if out != want {
		t.Errorf("expected %q, got %q", want, out)
	}
}
