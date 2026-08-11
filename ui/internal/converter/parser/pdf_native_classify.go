package parser

import "strings"

// textRect is a single text-run extracted from a PDF page, carrying just
// the fields the classifier needs. Mirrors the shape returned by pdfium's
// GetPageTextStructured, decoupled from the pdfium types so the classifier
// stays pure and testable without a pdfium instance.
type textRect struct {
	Text     string
	FontName string
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

// classifyBlocks groups a page's text rects into merged blocks. Rects with
// no font-name signal inherit the previous rect's classification (falling
// back to PROSE if there is no previous rect). A run of CODE rects only
// stays CODE if it spans >=2 lines; a single-line CODE run is demoted to
// PROSE so inline code mentions in running prose (e.g. "the `let` keyword")
// don't become their own one-line code block — see FINDINGS.md gap #2.
func classifyBlocks(rects []textRect) []block {
	if len(rects) == 0 {
		return nil
	}

	type run struct {
		kind  blockKind
		lines []string
	}
	var runs []run
	prevKind := blockProse

	for _, r := range rects {
		kind, known := classifyRect(r.FontName)
		if !known {
			kind = prevKind
		}
		prevKind = kind

		if len(runs) > 0 && runs[len(runs)-1].kind == kind {
			runs[len(runs)-1].lines = append(runs[len(runs)-1].lines, r.Text)
			continue
		}
		runs = append(runs, run{kind: kind, lines: []string{r.Text}})
	}

	// Demote single-line CODE runs to PROSE, then re-merge adjacent PROSE
	// runs that are now the same kind (e.g. PROSE, demoted-CODE, PROSE).
	for i := range runs {
		if runs[i].kind == blockCode && len(runs[i].lines) < 2 {
			runs[i].kind = blockProse
		}
	}
	merged := runs[:0:0]
	for _, rn := range runs {
		if len(merged) > 0 && merged[len(merged)-1].kind == rn.kind {
			merged[len(merged)-1].lines = append(merged[len(merged)-1].lines, rn.lines...)
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
