package parser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/GustavoCaso/folio/ui/internal/hub"
	"github.com/GustavoCaso/folio/ui/internal/logging"
	epubRender "github.com/GustavoCaso/folio/ui/internal/renderer/epub"
	"github.com/raitucarp/epub"
	"golang.org/x/net/html"
)

// maxFallbackTitleLen is the maximum rune length for a fallback chapter
// title derived from heading text, before truncation with an ellipsis.
const maxFallbackTitleLen = 120

// headingTags are the element names considered chapter headings when
// deriving a fallback title from a chapter's rendered content.
var headingTags = map[string]bool{
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true,
}

// Store is the subset of repository.Store this package needs — matches
// the shape converter.Runner depends on for its own Store parameter.
type Store interface {
	MarkJobDone(ctx context.Context, id, outputPath, title, author string, cover []byte) error
	MarkJobFailed(ctx context.Context, id, errMsg string) error
}

// epubParser parses epub bytes in-process and writes rendered chapters +
// toc to disk, marking the job done or failed via Store.
type epubParser struct {
	store   Store
	hub     *hub.Hub
	dataDir string
	logger  *slog.Logger
}

// NewEPUB constructs the epub Parser. h may be nil, in which case status
// events are not published (Store is still updated).
func NewEPUB(store Store, h *hub.Hub, dataDir string) Parser {
	return &epubParser{store: store, hub: h, dataDir: dataDir, logger: slog.Default()}
}

type tocEntry struct {
	Title      string     `json:"title"`
	ChapterIdx int        `json:"chapter_idx"`
	Anchor     string     `json:"anchor"`
	Items      []tocEntry `json:"items"`
}

// Convert parses data as epub bytes and writes one chapter-N.html per
// spine item plus toc.json under dataDir/jobID, then marks the job done or
// failed. requestID, filename, and h are accepted for interface uniformity
// but unused — epubParser uses its own stored hub.
func (p *epubParser) Convert(ctx context.Context, jobID, requestID, filename string, data []byte, h *hub.Hub) error {
	log := p.logger.With("job_id", jobID)
	start := time.Now()
	log.Info("epub conversion start", "bytes", len(data))

	reader, err := epub.NewReader(data)
	if err != nil {
		return p.fail(log, jobID, fmt.Sprintf("parse epub: %v", err))
	}

	outDir := filepath.Join(p.dataDir, jobID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return p.fail(log, jobID, fmt.Sprintf("create output dir: %v", err))
	}

	spineIdxByHref := spineIndexByHref(&reader)

	spine := reader.Spine()
	// docByIdx and orderedHrefs let us render chapters and compute
	// fallback titles after the TOC tree has been built, so we know which
	// spine indices got zero TOC coverage at any depth.
	docByIdx := make(map[int]*html.Node, len(spine))
	for i, item := range spine {
		doc := reader.ReadContentHTMLById(item.ID)
		if doc == nil {
			continue
		}
		out, err := epubRender.Render(doc, i, resolveImage(&reader))
		if err != nil {
			return p.fail(log, jobID, fmt.Sprintf("render chapter %d: %v", i, err))
		}
		if err := os.WriteFile(filepath.Join(outDir, fmt.Sprintf("chapter-%d.html", i)), []byte(out), 0o644); err != nil {
			return p.fail(log, jobID, fmt.Sprintf("write chapter %d: %v", i, err))
		}
		docByIdx[i] = doc
	}

	tocEntries := buildTOCTree(&reader, spineIdxByHref)
	tocEntries = appendFallbackEntries(tocEntries, spine, docByIdx)

	tocBytes, err := json.Marshal(tocEntries)
	if err != nil {
		return p.fail(log, jobID, fmt.Sprintf("marshal toc: %v", err))
	}
	if err := os.WriteFile(filepath.Join(outDir, "toc.json"), tocBytes, 0o644); err != nil {
		return p.fail(log, jobID, fmt.Sprintf("write toc: %v", err))
	}

	title := reader.Title()
	author := reader.Author()
	cover := coverBytes(&reader)

	if err := p.store.MarkJobDone(ctx, jobID, outDir, title, author, cover); err != nil {
		log.Error("mark job done failed", logging.Err(err))
		return err
	}

	log.Info("epub conversion done",
		"output_path", outDir,
		"chapters", len(docByIdx),
		"toc_entries", len(tocEntries),
		"dur_ms", time.Since(start).Milliseconds(),
	)
	if p.hub != nil {
		p.hub.Publish(jobID, hub.StatusEvent{
			Status: "DONE",
			Title:  title,
			Author: author,
		})
	}
	return nil
}

// ConvertFromURL is not supported for epub jobs — epub sources are always
// uploaded, never fetched from a URL.
func (p *epubParser) ConvertFromURL(ctx context.Context, jobID, requestID, sourceURL string, h *hub.Hub) error {
	return errors.New("epub: import from URL is not supported")
}

func (p *epubParser) fail(log *slog.Logger, jobID, msg string) error {
	log.Error("epub conversion failed", "error", msg)
	if err := p.store.MarkJobFailed(context.Background(), jobID, msg); err != nil {
		log.Error("mark job failed errored", logging.Err(err))
	}
	if p.hub != nil {
		p.hub.Publish(jobID, hub.StatusEvent{Status: "FAILED", Error: msg})
	}
	return errors.New(msg)
}

// spineIndexByHref builds a normalized-href -> spine-index lookup, so TOC
// tree nodes (at any depth) can resolve their own chapter_idx independently
// from their own Href, without needing a flat title map that risks
// collisions between a chapter-level TOC entry and a subsection entry
// pointing at the same file (see buildTOCTree).
func spineIndexByHref(reader *epub.Reader) map[string]int {
	idx := make(map[string]int)
	for i, item := range reader.Spine() {
		idx[normalizeHref(item.Href)] = i
	}
	return idx
}

// buildTOCTree walks the epub's table-of-contents tree (TOC.Items nest
// recursively) ONCE and builds a matching tree of tocEntry values. Each
// node resolves its own ChapterIdx independently from its own Href (via
// spineIdxByHref) rather than inheriting from its parent — in practice a
// subsection's href usually matches the same spine file as its parent, but
// nothing here assumes that.
//
// Anchor is the fragment portion of the node's Href (the substring after
// "#"), or "" if the href has no fragment.
//
// If a node's Href doesn't match any spine item (a broken or external link,
// which can happen in malformed epubs), the node is skipped entirely — it
// is dropped from the tree along with any of its children that also don't
// resolve (children are still walked and may independently resolve to a
// real chapter, in which case they're promoted to replace the dropped
// parent's position... actually: to keep this simple and predictable, a
// node that fails to resolve drops only itself and does NOT walk its
// children, since a broken/external TOC node's subsections are assumed to
// belong to the same broken destination. This is a judgment call — no
// observed real-world epub exercises this path.
func buildTOCTree(reader *epub.Reader, spineIdxByHref map[string]int) []tocEntry {
	toc, err := reader.TableOfContents()
	if err != nil {
		return nil
	}

	var walk func(items []epub.TOC) []tocEntry
	walk = func(items []epub.TOC) []tocEntry {
		var out []tocEntry
		for _, item := range items {
			idx, ok := spineIdxByHref[normalizeHref(item.Href)]
			if !ok {
				// Broken/external link — skip this node (and, per the
				// doc comment above, its children).
				continue
			}
			entry := tocEntry{
				Title:      item.Title,
				ChapterIdx: idx,
				Anchor:     anchorFromHref(item.Href),
				Items:      walk(item.Items),
			}
			out = append(out, entry)
		}
		return out
	}

	return walk(toc.Items)
}

// appendFallbackEntries determines which spine indices got zero TOC
// coverage at any depth of tocEntries (neither as a top-level entry nor as
// a nested subsection), and appends a synthetic top-level tocEntry for each
// such index, in spine order. Title is derived via firstHeadingText, falling
// back to "Chapter {i+1}" if the chapter has no heading. This preserves the
// original behavior of guaranteeing every spine item appears somewhere in
// the TOC, now checked against tree coverage instead of a flat href map.
func appendFallbackEntries(tocEntries []tocEntry, spine []epub.PublicationResource, docByIdx map[int]*html.Node) []tocEntry {
	covered := make(map[int]bool)
	var mark func(entries []tocEntry)
	mark = func(entries []tocEntry) {
		for _, e := range entries {
			covered[e.ChapterIdx] = true
			mark(e.Items)
		}
	}
	mark(tocEntries)

	for i := range spine {
		if covered[i] {
			continue
		}
		doc, ok := docByIdx[i]
		if !ok {
			continue
		}
		title := firstHeadingText(doc)
		if title == "" {
			title = fmt.Sprintf("Chapter %d", i+1)
		}
		tocEntries = append(tocEntries, tocEntry{Title: title, ChapterIdx: i})
	}

	return tocEntries
}

// anchorFromHref returns the fragment portion of href (the substring after
// "#"), or "" if href has no fragment.
func anchorFromHref(href string) string {
	_, anchor, _ := strings.Cut(href, "#")
	return anchor
}

// normalizeHref strips a "#fragment" suffix so TOC hrefs (which often point
// at a specific anchor within a chapter document, e.g. "chapter1.xhtml#s1")
// can be matched against a spine item's plain document href.
func normalizeHref(href string) string {
	base, _, _ := strings.Cut(href, "#")
	return base
}

// firstHeadingText walks doc depth-first in document order and returns the
// normalized, truncated text content of the first h1-h6 element found, or
// "" if the document has no heading. Used as a fallback chapter title when
// no TOC entry matches the spine item's href.
func firstHeadingText(doc *html.Node) string {
	var heading *html.Node
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if heading != nil {
			return
		}
		if n.Type == html.ElementNode && headingTags[n.Data] {
			heading = n
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
			if heading != nil {
				return
			}
		}
	}
	walk(doc)

	if heading == nil {
		return ""
	}

	text := strings.Join(strings.Fields(textContent(heading)), " ")
	return truncateTitle(text, maxFallbackTitleLen)
}

// textContent concatenates the text of all descendant text nodes of n, so
// headings containing nested inline elements (e.g. <em>) are captured in
// full.
func textContent(n *html.Node) string {
	var buf strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return buf.String()
}

// truncateTitle truncates s to at most maxRunes runes on a rune boundary,
// appending an ellipsis if truncation occurred. s is assumed already
// whitespace-normalized.
func truncateTitle(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes]) + "…"
}

// resolveImage returns an epubrender.ImageResolver backed by the epub's
// manifest resources, keyed by href.
func resolveImage(reader *epub.Reader) epubRender.ImageResolver {
	return func(href string) ([]byte, string, bool) {
		res := reader.SelectResourceByHref(href)
		if res == nil {
			return nil, "", false
		}
		return res.Content, res.MIMEType, true
	}
}

// coverBytes returns the epub's cover image bytes, or nil if the
// publication has no discoverable cover. epub.Reader.CoverBytes panics
// internally (nil pointer deref) when no cover is found, so guard with a
// pre-check via Cover().
func coverBytes(reader *epub.Reader) []byte {
	if reader.Cover() == nil {
		return nil
	}
	data, err := reader.CoverBytes()
	if err != nil {
		return nil
	}
	return data
}
