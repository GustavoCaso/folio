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
)

type blockIDTransformer struct{}

func (t *blockIDTransformer) Transform(node *ast.Document, _ text.Reader, _ parser.Context) {
	counter := 0
	_ = ast.Walk(node, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n.Kind() {
		case ast.KindHeading, ast.KindParagraph, ast.KindFencedCodeBlock, ast.KindCodeBlock:
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
		extension.Table,
		highlighting.NewHighlighting(
			highlighting.WithStyle("github"),
			highlighting.WithFormatOptions(
				chromahtml.WithLineNumbers(false),
				chromahtml.WithClasses(false),
			),
		),
	),
	goldmark.WithParserOptions(
		parser.WithASTTransformers(
			util.Prioritized(&blockIDTransformer{}, 999),
		),
	),
	goldmark.WithRendererOptions(
		html.WithUnsafe(),
	),
)

var blockIDRegex = regexp.MustCompile(`<(h[1-6]|p|pre)([^>]*)>`)
var blockIDCounter = 0

// Render converts Markdown to HTML with data-block-id attributes on block elements.
func Render(src []byte) (string, error) {
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return "", err
	}
	result := buf.String()

	blockIDCounter = 0
	result = blockIDRegex.ReplaceAllStringFunc(result, func(match string) string {
		blockIDCounter++
		// Extract tag name and any attributes
		closeIdx := strings.Index(match, ">")
		tagPart := match[1:closeIdx]
		parts := strings.Fields(tagPart)
		tag := parts[0]
		attrs := ""
		if len(parts) > 1 {
			attrs = " " + strings.Join(parts[1:], " ")
		}
		return fmt.Sprintf(`<%s%s data-block-id="%s-%d">`, tag, attrs, tag, blockIDCounter)
	})

	return result, nil
}
