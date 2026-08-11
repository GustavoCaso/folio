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
