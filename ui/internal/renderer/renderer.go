package renderer

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/anchor"
	"go.abhg.dev/goldmark/toc"
)

type blockIDTransformer struct{}

func (t *blockIDTransformer) Transform(node *ast.Document, _ text.Reader, _ parser.Context) {
	counter := 0
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.Kind() {
		case ast.KindHeading, ast.KindParagraph, ast.KindFencedCodeBlock, ast.KindCodeBlock, ast.KindList, ast.KindListItem:
			counter++
			kind := strings.ToLower(strings.TrimPrefix(n.Kind().String(), ""))
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "%s-%d", kind, counter)
			n.SetAttributeString("data-block-id", buf.Bytes())
		}
		return ast.WalkContinue, nil
	})
}

var md = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
			highlighting.WithFormatOptions(
				chromahtml.WithLineNumbers(false),
				chromahtml.WithClasses(false),
			),
		),
		&anchor.Extender{},
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
		parser.WithASTTransformers(
			util.Prioritized(&blockIDTransformer{}, 999),
		),
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

var preRegex = regexp.MustCompile(`<(pre)([^>]*)>`)
var preCounter = 0

// Render converts Markdown to HTML with data-block-id attributes on block elements.
// A collapsible table of contents is prepended when headings are present.
func Render(src []byte) (string, error) {
	reader := text.NewReader(src)
	doc := md.Parser().Parse(reader)

	tree, err := toc.Inspect(doc, src, toc.MaxDepth(3))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer

	if list := toc.RenderList(tree); list != nil {
		buf.WriteString(`<details id="toc-wrapper"><summary id="toc-toggle">Contents</summary>`)
		if err := md.Renderer().Render(&buf, src, list); err != nil {
			return "", err
		}
		buf.WriteString(`</details>`)
	}

	if err := md.Renderer().Render(&buf, src, doc); err != nil {
		return "", err
	}

	result := buf.String()

	preCounter = 0
	result = preRegex.ReplaceAllStringFunc(result, func(match string) string {
		preCounter++
		closeIdx := strings.Index(match, ">")
		tagPart := match[1:closeIdx]
		parts := strings.Fields(tagPart)
		tag := parts[0]
		attrs := ""
		if len(parts) > 1 {
			attrs = " " + strings.Join(parts[1:], " ")
		}
		return fmt.Sprintf(`<%s%s data-block-id="%s-%d">`, tag, attrs, tag, preCounter)
	})

	return result, nil
}
