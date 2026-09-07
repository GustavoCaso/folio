package parser

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/enums"
	"github.com/klippa-app/go-pdfium/references"
	"github.com/klippa-app/go-pdfium/requests"
)

type pageImage struct {
	pngBytes []byte
}

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

// pageEmbeddedImages extracts every FPDF_PAGEOBJ_IMAGE object on the page as
// a standalone PNG, per-object (not a full-page render).
func pageEmbeddedImages(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, pageRef requests.Page) ([]pageImage, error) {
	countResp, err := instance.FPDFPage_CountObjects(&requests.FPDFPage_CountObjects{Page: pageRef})
	if err != nil {
		return nil, err
	}

	var images []pageImage
	for i := 0; i < countResp.Count; i++ {
		objResp, err := instance.FPDFPage_GetObject(&requests.FPDFPage_GetObject{Page: pageRef, Index: i})
		if err != nil {
			continue
		}
		typeResp, err := instance.FPDFPageObj_GetType(&requests.FPDFPageObj_GetType{PageObject: objResp.PageObject})
		if err != nil || typeResp.Type != enums.FPDF_PAGEOBJ_IMAGE {
			continue
		}

		png, ok := renderImageObjectPNG(instance, doc, pageRef, objResp.PageObject)
		if ok {
			images = append(images, pageImage{pngBytes: png})
		}
	}
	return images, nil
}

func renderImageObjectPNG(instance pdfium.Pdfium, doc references.FPDF_DOCUMENT, pageRef requests.Page, obj references.FPDF_PAGEOBJECT) ([]byte, bool) {
	bmpResp, err := instance.FPDFImageObj_GetRenderedBitmap(&requests.FPDFImageObj_GetRenderedBitmap{
		Document: doc, Page: pageRef, ImageObject: obj,
	})
	if err != nil {
		return nil, false
	}
	widthResp, err := instance.FPDFBitmap_GetWidth(&requests.FPDFBitmap_GetWidth{Bitmap: bmpResp.Bitmap})
	if err != nil {
		return nil, false
	}
	heightResp, err := instance.FPDFBitmap_GetHeight(&requests.FPDFBitmap_GetHeight{Bitmap: bmpResp.Bitmap})
	if err != nil {
		return nil, false
	}
	strideResp, err := instance.FPDFBitmap_GetStride(&requests.FPDFBitmap_GetStride{Bitmap: bmpResp.Bitmap})
	if err != nil {
		return nil, false
	}
	bufResp, err := instance.FPDFBitmap_GetBuffer(&requests.FPDFBitmap_GetBuffer{Bitmap: bmpResp.Bitmap})
	if err != nil {
		return nil, false
	}

	img := bgraBufferToImage(bufResp.Buffer, widthResp.Width, heightResp.Height, strideResp.Stride)
	return encodePNG(img)
}
