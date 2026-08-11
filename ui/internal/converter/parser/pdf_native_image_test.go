package parser

import (
	"image/color"
	"testing"
)

func TestBGRABufferToImage_SinglePixel(t *testing.T) {
	// pdfium byte order is B,G,R,A.
	buf := []byte{10, 20, 30, 255}
	img := bgraBufferToImage(buf, 1, 1, 4)

	got := img.At(0, 0)
	r, g, b, a := got.RGBA()
	want := color.RGBA{R: 30, G: 20, B: 10, A: 255}
	if uint8(r>>8) != want.R || uint8(g>>8) != want.G || uint8(b>>8) != want.B || uint8(a>>8) != want.A {
		t.Errorf("expected %+v, got r=%d g=%d b=%d a=%d", want, r>>8, g>>8, b>>8, a>>8)
	}
}

func TestBGRABufferToImage_RespectsStridePadding(t *testing.T) {
	// 2x1 image but stride implies 1 extra padding byte per row beyond the
	// 2*4=8 pixel bytes, which must not shift row 2's pixel data.
	buf := []byte{
		255, 0, 0, 255, 0, 255, 0, 255, 0, 0, // row 0: 2 pixels + 2 pad bytes
	}
	img := bgraBufferToImage(buf, 2, 1, 10)

	r0, g0, b0, _ := img.At(0, 0).RGBA()
	if uint8(r0>>8) != 0 || uint8(g0>>8) != 0 || uint8(b0>>8) != 255 {
		t.Errorf("pixel (0,0): expected B=255 mapped to R=0,G=0,B=255, got r=%d g=%d b=%d", r0>>8, g0>>8, b0>>8)
	}
	r1, g1, b1, _ := img.At(1, 0).RGBA()
	if uint8(r1>>8) != 0 || uint8(g1>>8) != 255 || uint8(b1>>8) != 0 {
		t.Errorf("pixel (1,0): expected G=255 mapped to R=0,G=255,B=0, got r=%d g=%d b=%d", r1>>8, g1>>8, b1>>8)
	}
}

func TestBGRABufferToImage_Dimensions(t *testing.T) {
	buf := make([]byte, 3*2*4)
	img := bgraBufferToImage(buf, 3, 2, 3*4)
	bounds := img.Bounds()
	if bounds.Dx() != 3 || bounds.Dy() != 2 {
		t.Errorf("expected 3x2 image, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}
