package epub

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

	out, err := Render(doc, 3, nil, nil)
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

	out, err := Render(doc, 0, resolver, nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	if !strings.Contains(out, `src="data:image/png;base64,`) {
		t.Errorf("output missing base64 data URI:\n%s", out)
	}
}

func renderString(t *testing.T, src string, resolveImage ImageResolver, resolveLink LinkResolver) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("html.Parse: %v", err)
	}
	out, err := Render(doc, 0, resolveImage, resolveLink)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return out
}

func TestRender_OutputsBodyChildrenOnly(t *testing.T) {
	src := `<?xml version="1.0"?><html xmlns="http://www.w3.org/1999/xhtml"><head>` +
		`<title>Chapter</title><meta charset="utf-8"/><link rel="stylesheet" href="../style.css"/>` +
		`</head><body class="calibre"><p>Text</p></body></html>`

	out := renderString(t, src, nil, nil)

	if want := `<p data-block-id="ch0-p-1">Text</p>`; out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestRender_StripsPresentationalAndUnsafeAttrs(t *testing.T) {
	src := `<html><body>` +
		`<section id="s1" class="pcalibre1" epub:type="chapter" style="color:red" data-uri="x.xhtml#s1">` +
		`<p class="pcalibre8" onclick="alert(1)" data-block-id="forged">Hi <i class="pcalibre17">there</i></p>` +
		`<!-- comment --><style>p{}</style><script>alert(1)</script>` +
		`</section></body></html>`

	out := renderString(t, src, nil, nil)

	want := `<section id="s1"><p data-block-id="ch0-p-1">Hi <i>there</i></p></section>`
	if out != want {
		t.Errorf("got  %q\nwant %q", out, want)
	}
}

func TestRender_RewritesSVGImageHref(t *testing.T) {
	src := `<html><body><svg xmlns:xlink="http://www.w3.org/1999/xlink"><image xlink:href="cover.jpeg"/></svg></body></html>`
	resolver := func(href string) ([]byte, string, bool) {
		return []byte("jpg"), "image/jpeg", href == "cover.jpeg"
	}

	out := renderString(t, src, resolver, nil)

	if !strings.Contains(out, `xlink:href="data:image/jpeg;base64,`) {
		t.Errorf("SVG image not rewritten:\n%s", out)
	}
}

func TestRender_RewritesOrDropsLinks(t *testing.T) {
	src := `<html><body><p><a href="keep">a</a><a href="change">b</a><a href="drop">c</a><a id="no-href">d</a></p></body></html>`
	resolver := func(href string) (string, bool) {
		switch href {
		case "keep":
			return href, true
		case "change":
			return "?chapter=2#x", true
		}
		return "", false
	}

	out := renderString(t, src, nil, resolver)

	want := `<p data-block-id="ch0-p-1"><a href="keep">a</a><a href="?chapter=2#x">b</a><a>c</a><a id="no-href">d</a></p>`
	if out != want {
		t.Errorf("got  %q\nwant %q", out, want)
	}
}
