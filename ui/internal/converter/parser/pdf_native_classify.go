package parser

// textRect is a single text-run extracted from a PDF page, carrying just
// the fields the AI segmentation pass needs. Mirrors the shape returned by
// pdfium's GetPageTextStructured, decoupled from the pdfium types so this
// package stays testable without a pdfium instance.
type textRect struct {
	Text     string
	FontName string
	// Top and Bottom are the rect's vertical position in points (pdfium's
	// CharPosition.Top/Bottom), given to the model as a paragraph/section
	// break signal.
	Top    float64
	Bottom float64
	// Left and Right are the rect's horizontal position in points
	// (pdfium's CharPosition.Left/Right), used only to distinguish a
	// duplicate glyph run pdfium occasionally re-reports at the same
	// position (see dedupeRepeatedGlyphRuns) from two genuinely separate
	// occurrences of the same text advancing rightward across the line.
	Left  float64
	Right float64
}

type blockKind int

const (
	blockProse blockKind = iota
	blockCode
	blockH1
	blockH2
	blockH3
)

// block is one logical unit ready for rendering, as segmented and
// classified by aiCleaner.Clean.
type block struct {
	Kind blockKind
	Text string
}
