package renderer

import (
	"bytes"
	"fmt"
	"strings"

	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/yuin/goldmark"
	highlighting "github.com/yuin/goldmark-highlighting/v2"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"go.abhg.dev/goldmark/anchor"
	"go.abhg.dev/goldmark/toc"
)

var anchorExtender = anchor.Extender{}

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

// codeBlockIDs walks the AST and returns the data-block-id values for all
// FencedCodeBlock and CodeBlock nodes in document order.
func codeBlockIDs(doc ast.Node) []string {
	var ids []string
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Kind() == ast.KindFencedCodeBlock || n.Kind() == ast.KindCodeBlock {
			if v, ok := n.AttributeString("data-block-id"); ok {
				ids = append(ids, string(v.([]byte)))
			}
		}
		return ast.WalkContinue, nil
	})
	return ids
}

// newMarkdownRendered constructs the goldmark rendered with a WrapperRenderer that
// injects data-block-id onto each code block wrapper in document order.
func newMarkdownRendered(codeIDs []string) goldmark.Markdown {
	idx := 0
	// wrapper wraps each code block in a <div data-block-id="..."> so
	// highlight anchoring can locate it regardless of whether Chroma or the
	// plain fallback renders the inner <pre>.
	wrapper := highlighting.WrapperRenderer(func(w util.BufWriter, ctx highlighting.CodeBlockContext, entering bool) {
		if entering {
			id := ""
			if idx < len(codeIDs) {
				id = codeIDs[idx]
				idx++
			}
			if ctx.Highlighted() {
				fmt.Fprintf(w, `<div data-block-id="%s">`, id) //nolint:errcheck
			} else {
				lang, hasLang := ctx.Language()
				if hasLang && lang != nil {
					fmt.Fprintf(w, `<div data-block-id="%s"><pre><code class="language-%s">`, id, lang) //nolint:errcheck
				} else {
					fmt.Fprintf(w, `<div data-block-id="%s"><pre><code>`, id) //nolint:errcheck
				}
			}
		} else {
			if ctx.Highlighted() {
				w.WriteString("</div>") //nolint:errcheck
			} else {
				w.WriteString("</code></pre></div>\n") //nolint:errcheck
			}
		}
	})

	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			highlighting.NewHighlighting(
				highlighting.WithStyle("github"),
				highlighting.WithGuessLanguage(true),
				highlighting.WithFormatOptions(
					chromahtml.WithLineNumbers(false),
					chromahtml.WithClasses(false),
				),
				highlighting.WithWrapperRenderer(wrapper),
			),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
			renderer.WithNodeRenderers(
				util.Prioritized(&anchor.Renderer{
					Position: anchorExtender.Position,
					Unsafe:   anchorExtender.Unsafe,
				}, 100),
			),
		),
	)
}

// sharedParser stateless goldmark parser that runs the
// blockIDTransformer and anchor.Transformer.
// Used to build the AST before rendering the markdown.
var sharedParser = goldmark.New(
	goldmark.WithExtensions(
		extension.GFM,
	),
	goldmark.WithParserOptions(
		parser.WithAutoHeadingID(),
		parser.WithASTTransformers(
			util.Prioritized(
				&anchor.Transformer{
					Texter:     anchorExtender.Texter,
					Position:   anchorExtender.Position,
					Attributer: anchorExtender.Attributer,
				}, 100),
			util.Prioritized(&blockIDTransformer{}, 999),
		),
	),
).Parser()

// Render converts Markdown to HTML with data-block-id attributes on block elements.
// A collapsible table of contents is prepended when headings are present.
func Render(src []byte) (string, error) {
	doc := sharedParser.Parse(text.NewReader(src))

	tree, err := toc.Inspect(doc, src, toc.MaxDepth(3))
	if err != nil {
		return "", err
	}

	md := newMarkdownRendered(codeBlockIDs(doc))

	var buf bytes.Buffer

	if list := toc.RenderList(tree); list != nil {
		_, bufError := buf.WriteString(`<details id="toc-wrapper"><summary id="toc-toggle">Contents</summary>`)
		if bufError != nil {
			return "", bufError
		}
		if err := md.Renderer().Render(&buf, src, list); err != nil {
			return "", err
		}
		_, bufError = buf.WriteString(`</details>`)
		if bufError != nil {
			return "", bufError
		}
	}

	if err := md.Renderer().Render(&buf, src, doc); err != nil {
		return "", err
	}

	return buf.String(), nil
}
