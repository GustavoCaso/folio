package epubrender

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestRender_InjectsBlockIDs(t *testing.T) {
	src := `<html><body><h1>Title</h1><p>First para.</p><p>Second para.</p></body></html>`
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}

	out, err := Render(doc, 3, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, want := range []string{
		`data-block-id="ch3-h1-1"`,
		`data-block-id="ch3-p-2"`,
		`data-block-id="ch3-p-3"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\ngot: %s", want, out)
		}
	}
}

func TestRender_RewritesImageSrcToDataURI(t *testing.T) {
	src := `<html><body><p>See <img src="images/fig1.png"/></p></body></html>`
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}

	resolver := func(href string) ([]byte, string, bool) {
		if href == "images/fig1.png" {
			return []byte("fake-png-bytes"), "image/png", true
		}
		return nil, "", false
	}

	out, err := Render(doc, 0, resolver)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(out, `src="data:image/png;base64,`) {
		t.Errorf("output missing base64 data URI:\n%s", out)
	}
}
