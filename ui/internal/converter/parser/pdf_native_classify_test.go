package parser

import "testing"

func TestClassifyBlocks_SingleProseRect(t *testing.T) {
	rects := []textRect{
		{Text: "Hello world.", FontName: "NewBaskervilleStd-Roman"},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Kind != blockProse {
		t.Errorf("expected PROSE, got %v", blocks[0].Kind)
	}
	if blocks[0].Text != "Hello world." {
		t.Errorf("expected text %q, got %q", "Hello world.", blocks[0].Text)
	}
}

func TestClassifyBlocks_TwoConsecutiveMonoLinesPromoteToCode(t *testing.T) {
	rects := []textRect{
		{Text: "$ cargo new guessing_game", FontName: "TheSansMonoCd-W5Regular"},
		{Text: "$ cd guessing_game", FontName: "TheSansMonoCd-W5Regular"},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Kind != blockCode {
		t.Errorf("expected CODE, got %v", blocks[0].Kind)
	}
}

func TestClassifyBlocks_SingleMonoLineStaysInlineNotPromoted(t *testing.T) {
	rects := []textRect{
		{Text: "You'll learn about ", FontName: "NewBaskervilleStd-Roman"},
		{Text: "let", FontName: "TheSansMonoCd-W5Regular"},
		{Text: ", methods, and more.", FontName: "NewBaskervilleStd-Roman"},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 merged prose block (inline code span, not promoted), got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Kind != blockProse {
		t.Errorf("expected PROSE (single mono line should not promote to its own CODE block), got %v", blocks[0].Kind)
	}
	if blocks[0].Text != "You'll learn about let, methods, and more." {
		t.Errorf("unexpected merged text: %q", blocks[0].Text)
	}
}

func TestClassifyBlocks_MissingFontInfoInheritsPreviousClassification(t *testing.T) {
	rects := []textRect{
		{Text: ".read_line(&mut guess)", FontName: "TheSansMonoCd-W5Regular"},
		{Text: ".expect(\"Failed to read line\");", FontName: ""},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block (missing-font rect should inherit CODE), got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Kind != blockCode {
		t.Errorf("expected CODE (inherited from previous rect), got %v", blocks[0].Kind)
	}
}

func TestClassifyBlocks_MissingFontInfoAtStartDefaultsToProse(t *testing.T) {
	rects := []textRect{
		{Text: "no previous rect to inherit from", FontName: ""},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 1 || blocks[0].Kind != blockProse {
		t.Fatalf("expected 1 PROSE block, got %+v", blocks)
	}
}

func TestClassifyBlocks_KnownHeadingFontsMapToTiers(t *testing.T) {
	rects := []textRect{
		{Text: "2", FontName: "FuturaStd-CondensedBold"},
		{Text: "Guessing Game", FontName: "DogmaOT-Bold"},
		{Text: "Processing a Guess", FontName: "FuturaStd-Bold"},
		{Text: "Body text follows.", FontName: "NewBaskervilleStd-Roman"},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 4 {
		t.Fatalf("expected 4 blocks, got %d: %+v", len(blocks), blocks)
	}
	want := []blockKind{blockH1, blockH2, blockH3, blockProse}
	for i, k := range want {
		if blocks[i].Kind != k {
			t.Errorf("block %d: expected %v, got %v", i, k, blocks[i].Kind)
		}
	}
}

func TestClassifyBlocks_ContiguousProseRectsMergeWithSpace(t *testing.T) {
	rects := []textRect{
		{Text: "First sentence.", FontName: "NewBaskervilleStd-Roman"},
		{Text: "Second sentence.", FontName: "NewBaskervilleStd-Roman"},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 merged block, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Text != "First sentence. Second sentence." {
		t.Errorf("unexpected merged text: %q", blocks[0].Text)
	}
}

func TestClassifyBlocks_CodeBlockLinesJoinWithNewline(t *testing.T) {
	rects := []textRect{
		{Text: "use std::io;", FontName: "TheSansMonoCd-W5Regular"},
		{Text: "fn main() {", FontName: "TheSansMonoCd-W5Regular"},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 1 || blocks[0].Kind != blockCode {
		t.Fatalf("expected 1 CODE block, got %+v", blocks)
	}
	if blocks[0].Text != "use std::io;\nfn main() {" {
		t.Errorf("unexpected code text: %q", blocks[0].Text)
	}
}

func TestClassifyBlocks_Empty(t *testing.T) {
	blocks := classifyBlocks(nil)
	if len(blocks) != 0 {
		t.Errorf("expected no blocks, got %+v", blocks)
	}
}

func TestClassifyBlocks_WrappedLinesWithinAParagraphStayMerged(t *testing.T) {
	// Normal single-spaced body text: consecutive lines ~12pt apart (typical
	// line height), should stay one PROSE block despite the vertical gap.
	rects := []textRect{
		{Text: "This is the first line of a paragraph ", FontName: "NewBaskervilleStd-Roman", Top: 700, Bottom: 690},
		{Text: "that wraps onto a second line.", FontName: "NewBaskervilleStd-Roman", Top: 688, Bottom: 678},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 merged block (line wrap, not paragraph break), got %d: %+v", len(blocks), blocks)
	}
}

func TestClassifyBlocks_LargeVerticalGapSplitsIntoNewParagraphBlock(t *testing.T) {
	// A TOC-style page: many short lines packed close together, but a big
	// gap between the two "chunks" here simulates two distinct paragraphs
	// (e.g. separated by a section break) that must not be merged into one
	// giant <p>.
	rects := []textRect{
		{Text: "Preface ix", FontName: "NewBaskervilleStd-Roman", Top: 700, Bottom: 692},
		{Text: "1. Introducing Patterns 1", FontName: "NewBaskervilleStd-Roman", Top: 690, Bottom: 682},
		{Text: "2. Data Ingestion Patterns 7", FontName: "NewBaskervilleStd-Roman", Top: 500, Bottom: 492},
		{Text: "3. Error Management 39", FontName: "NewBaskervilleStd-Roman", Top: 498, Bottom: 490},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 PROSE blocks split on the large vertical gap, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0].Text != "Preface ix 1. Introducing Patterns 1" {
		t.Errorf("unexpected first block text: %q", blocks[0].Text)
	}
	if blocks[1].Text != "2. Data Ingestion Patterns 7 3. Error Management 39" {
		t.Errorf("unexpected second block text: %q", blocks[1].Text)
	}
}

func TestClassifyBlocks_ParagraphSplitDoesNotApplyToCodeBlocks(t *testing.T) {
	// Code listings can have larger line gaps than prose (e.g. blank lines
	// between statements) without those being separate "blocks" — a code
	// listing is one block regardless of internal vertical spacing.
	rects := []textRect{
		{Text: "fn main() {", FontName: "TheSansMonoCd-W5Regular", Top: 700, Bottom: 692},
		{Text: "    println!(\"hi\");", FontName: "TheSansMonoCd-W5Regular", Top: 500, Bottom: 492},
	}
	blocks := classifyBlocks(rects)
	if len(blocks) != 1 || blocks[0].Kind != blockCode {
		t.Fatalf("expected 1 CODE block regardless of vertical gap, got %+v", blocks)
	}
}
