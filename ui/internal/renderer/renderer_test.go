package renderer_test

import (
	"strings"
	"testing"

	"github.com/GustavoCaso/folio/ui/internal/renderer"
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
	// FencedCodeBlock generates <pre><code>, but data-block-id is set on pre
	if !strings.Contains(html, `data-block-id="pre-`) {
		t.Errorf("expected data-block-id on code block, got: %s", html)
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
