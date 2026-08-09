package markdown_test

import (
	"strings"
	"testing"

	renderer "github.com/GustavoCaso/folio/ui/internal/renderer/markdown"
)

func TestRendererInjectsBlockIDs(t *testing.T) {
	md := "# Hello\n\nSome paragraph.\n\nAnother paragraph.\n"
	html, err := renderer.Render([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(html, `data-block-id="`) {
		t.Error("expected data-block-id attributes in output")
	}
}

func TestRendererCodeBlockHasBlockID(t *testing.T) {
	md := "```go\nfmt.Println(\"hello\")\n```\n"
	html, err := renderer.Render([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("HTML: %s", html)
	// FencedCodeBlock is wrapped in a <div data-block-id="fencedcodeblock-N">.
	if !strings.Contains(html, `data-block-id="fencedcodeblock-`) {
		t.Errorf("expected data-block-id on fenced code block wrapper, got: %s", html)
	}
}

func TestRendererTableOfContentsCollapsible(t *testing.T) {
	md := "# Hello\n\n## World\n\nSome paragraph.\n"
	html, err := renderer.Render([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("HTML: %s", html)
	if !strings.Contains(html, `<details id="toc-wrapper">`) {
		t.Errorf("expected <details id=\"toc-wrapper\">, got: %s", html)
	}
	if !strings.Contains(html, `<summary id="toc-toggle">`) {
		t.Errorf("expected <summary id=\"toc-toggle\">, got: %s", html)
	}
}

func TestRendererNoTableOfContentsWhenNoHeadings(t *testing.T) {
	md := "Just a paragraph, no headings.\n"
	html, err := renderer.Render([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(html, `id="toc-wrapper"`) {
		t.Errorf("expected no toc-wrapper when no headings, got: %s", html)
	}
}

func TestRendererAnchorOnHeadings(t *testing.T) {
	md := "# Hello\n\n## World\n\nSome paragraph.\n"
	html, err := renderer.Render([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("HTML: %s", html)
	if !strings.Contains(html, `<a class="anchor" href="#hello">¶</a>`) {
		t.Errorf("expected anchor points rendered, got: %s", html)
	}
}

func TestRendererNoAnchorWithHeading(t *testing.T) {
	md := "Just a paragraph, no headings.\n"
	html, err := renderer.Render([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(html, `<a class="anchor"`) {
		t.Errorf("expected no anchor when no headings, got: %s", html)
	}
}

func TestRendererListAndListElementHasBlockID(t *testing.T) {
	md := "1. First item\n2. Second item\n3. Third item\n4. Fourth item"
	html, err := renderer.Render([]byte(md))
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("HTML: %s", html)
	if !strings.Contains(html, `data-block-id="list-1`) {
		t.Errorf("expected data-block-id on code block, got: %s", html)
	}
}
