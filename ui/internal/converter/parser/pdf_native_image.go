package parser

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// bgraBufferToImage converts pdfium's native BGRA bitmap buffer into a
// standard library image.Image (pdfium renders in BGRA byte order, not
// RGBA). Ported from the validated experiment in
// scripts/pdf-extract-test/pdfium_test/main.go.
func bgraBufferToImage(buf []byte, width, height, stride int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		rowStart := y * stride
		for x := range width {
			i := rowStart + x*4
			if i+3 >= len(buf) {
				break
			}
			b, g, r, a := buf[i], buf[i+1], buf[i+2], buf[i+3]
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	return img
}

// encodePNG encodes img as PNG bytes.
func encodePNG(img image.Image) ([]byte, bool) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, false
	}
	return buf.Bytes(), true
}
