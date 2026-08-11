package parser

import "strings"

// textRect is a single text-run extracted from a PDF page, carrying just
// the fields the classifier needs. Mirrors the shape returned by pdfium's
// GetPageTextStructured, decoupled from the pdfium types so the classifier
// stays pure and testable without a pdfium instance.
type textRect struct {
	Text     string
	FontName string
	// Top and Bottom are the rect's vertical position in points (pdfium's
	// CharPosition.Top/Bottom), used to detect paragraph breaks within a
	// contiguous PROSE run. Zero-valued for rects built outside a real
	// pdfium page (e.g. hand-written unit tests not exercising paragraph
	// splitting), which is safe since a zero/zero gap never exceeds the
	// split threshold.
	Top    float64
	Bottom float64
}

type blockKind int

const (
	blockProse blockKind = iota
	blockCode
	blockH1
	blockH2
	blockH3
)

// block is a run of contiguous same-classification textRects merged into
// one renderable unit.
type block struct {
	Kind blockKind
	Text string
}

// monoFontNeedles are case-insensitive substrings identifying monospace/code
// fonts, validated against a real book in scripts/pdf-extract-test/FINDINGS.md.
var monoFontNeedles = []string{"mono", "courier", "consolas", "menlo", "sourcecodepro"}

// headingFontTiers maps known heading font names (validated against a real
// book, see FINDINGS.md) to heading block kinds. Font-size clustering is not
// used: pdfium's WASM mode returns FontInformation.Size == 1.0 for every
// rect (unresolved unit/scale bug), so heading tier is name-based only.
var headingFontTiers = map[string]blockKind{
	"FuturaStd-CondensedBold": blockH1,
	"DogmaOT-Bold":            blockH2,
	"FuturaStd-Bold":          blockH3,
}

func isMonoFont(name string) bool {
	lower := strings.ToLower(name)
	for _, needle := range monoFontNeedles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

// classifyRect returns the block kind for a single rect's font name, or
// (kind, false) if the font name gives no signal (empty/unknown), in which
// case the caller should inherit the previous rect's classification.
func classifyRect(fontName string) (blockKind, bool) {
	if fontName == "" {
		return blockProse, false
	}
	if kind, ok := headingFontTiers[fontName]; ok {
		return kind, true
	}
	if isMonoFont(fontName) {
		return blockCode, true
	}
	return blockProse, true
}

// paragraphGapMultiplier is how many times the previous line's own height
// (Top - Bottom) the gap to the next rect must exceed to count as a
// paragraph break rather than an ordinary line wrap. Line-to-line gaps
// within a wrapped paragraph are typically close to one line height;
// section/paragraph breaks leave noticeably more whitespace.
const paragraphGapMultiplier = 1.5

// isParagraphBreak reports whether the vertical gap between prev and next
// looks like a paragraph/section break rather than an ordinary wrapped
// line. It compares the gap (prev.Bottom - next.Top, in PDF points where Y
// increases upward, so a "gap" is prev.Bottom minus next.Top) against
// paragraphGapMultiplier times prev's own line height (prev.Top -
// prev.Bottom). A zero-valued line height (rects with no position data, as
// in older unit tests) never triggers a split.
func isParagraphBreak(prev, next textRect) bool {
	lineHeight := prev.Top - prev.Bottom
	if lineHeight <= 0 {
		return false
	}
	gap := prev.Bottom - next.Top
	return gap > paragraphGapMultiplier*lineHeight
}

// classifyBlocks groups a page's text rects into merged blocks. Rects with
// no font-name signal inherit the previous rect's classification (falling
// back to PROSE if there is no previous rect). A run of CODE rects only
// stays CODE if it spans >=2 lines; a single-line CODE run is demoted to
// PROSE so inline code mentions in running prose (e.g. "the `let` keyword")
// don't become their own one-line code block — see FINDINGS.md gap #2. A
// PROSE run splits into a new block whenever isParagraphBreak fires, so a
// page of visually distinct paragraphs doesn't collapse into one <p>.
func classifyBlocks(rects []textRect) []block {
	if len(rects) == 0 {
		return nil
	}

	type run struct {
		kind                 blockKind
		lines                []string
		wasDemoted           bool // true if this run started life as a demoted single-line CODE run
		paragraphBreakBefore bool
	}
	var runs []run
	prevKind := blockProse
	var prevRect *textRect

	for i := range rects {
		r := &rects[i]
		kind, known := classifyRect(r.FontName)
		if !known {
			kind = prevKind
		}
		prevKind = kind

		splitParagraph := kind == blockProse && prevRect != nil && isParagraphBreak(*prevRect, *r)
		prevRect = r

		if len(runs) > 0 && runs[len(runs)-1].kind == kind && !splitParagraph {
			runs[len(runs)-1].lines = append(runs[len(runs)-1].lines, r.Text)
			continue
		}
		runs = append(runs, run{kind: kind, lines: []string{r.Text}, paragraphBreakBefore: splitParagraph})
	}

	// Demote single-line CODE runs to PROSE, then re-merge adjacent PROSE
	// runs that are now the same kind (e.g. PROSE, demoted-CODE, PROSE) —
	// but never re-merge across an explicit paragraph-break split, or a
	// large vertical gap between two real paragraphs would collapse back
	// into one giant block.
	for i := range runs {
		if runs[i].kind == blockCode && len(runs[i].lines) < 2 {
			runs[i].kind = blockProse
			runs[i].wasDemoted = true
		}
	}
	merged := runs[:0:0]
	for _, rn := range runs {
		if len(merged) > 0 && merged[len(merged)-1].kind == rn.kind &&
			!rn.paragraphBreakBefore && (merged[len(merged)-1].wasDemoted || rn.wasDemoted) {
			merged[len(merged)-1].lines = append(merged[len(merged)-1].lines, rn.lines...)
			merged[len(merged)-1].wasDemoted = merged[len(merged)-1].wasDemoted || rn.wasDemoted
			continue
		}
		merged = append(merged, rn)
	}

	blocks := make([]block, 0, len(merged))
	for _, rn := range merged {
		if rn.kind == blockCode {
			blocks = append(blocks, block{Kind: rn.kind, Text: strings.Join(rn.lines, "\n")})
			continue
		}
		// Prose rects carry their own inter-word spacing (pdfium emits
		// trailing/leading spaces on rect boundaries); joining with "" avoids
		// double-spacing, joining with " " would too when the rect is
		// already unspaced (e.g. multi-line body text wrapped mid-sentence).
		blocks = append(blocks, block{Kind: rn.kind, Text: joinProseLines(rn.lines)})
	}
	return blocks
}

// joinProseLines concatenates prose lines, inserting a space between two
// lines only when neither side already ends/starts with whitespace.
func joinProseLines(lines []string) string {
	var buf strings.Builder
	for i, l := range lines {
		if i > 0 {
			prev := buf.String()
			needsSpace := len(prev) > 0 && len(l) > 0 &&
				!strings.HasSuffix(prev, " ") && !strings.HasPrefix(l, " ") &&
				!strings.ContainsAny(l[:1], ".,;:!?)]")
			if needsSpace {
				buf.WriteByte(' ')
			}
		}
		buf.WriteString(l)
	}
	return buf.String()
}
