from unittest.mock import MagicMock, patch

import pypdfium2 as pdfium
from docling_core.types.doc.base import BoundingBox, CoordOrigin
from PIL import Image

from parser.formats.pdf.images import _bbox_to_crop, extract_images, rewrite_image_placeholders


def test_bbox_to_crop_converts_bottomleft_to_margins():
    # Page 100x200 pt. bbox l=10, b=50, r=90, t=150 (BOTTOMLEFT)
    # crop = (left_margin, bottom_margin, right_margin, top_margin)
    # = (l, b, W-r, H-t) = (10, 50, 10, 50)
    bbox = BoundingBox(l=10, t=150, r=90, b=50, coord_origin=CoordOrigin.BOTTOMLEFT)
    crop = _bbox_to_crop(bbox, page_width=100, page_height=200)
    assert crop == (10.0, 50.0, 10.0, 50.0)


def test_extract_images_yields_filename_and_bytes(tmp_path):
    pdf_doc = pdfium.PdfDocument.new()
    pdf_doc.new_page(width=200, height=200)
    pdf_path = tmp_path / "test.pdf"
    pdf_doc.save(str(pdf_path))
    pdf_doc.close()

    mock_prov = MagicMock()
    mock_prov.page_no = 1
    mock_prov.bbox = BoundingBox(l=0, t=200, r=200, b=0, coord_origin=CoordOrigin.BOTTOMLEFT)

    mock_pic = MagicMock()
    mock_pic.prov = [mock_prov]

    mock_page_size = MagicMock()
    mock_page_size.width = 200.0
    mock_page_size.height = 200.0

    mock_doc = MagicMock()
    mock_doc.pictures = [mock_pic]
    mock_doc.pages = {1: MagicMock(size=mock_page_size)}

    results = list(extract_images(mock_doc, pdf_path, scale=1.0))
    assert len(results) == 1
    filename, data = results[0]
    assert filename.startswith("image_000000_")
    assert filename.endswith(".png")
    assert len(data) > 0
    assert data[:4] == b"\x89PNG"


def test_extract_images_filename_includes_index_and_hash(tmp_path):
    pdf_doc = pdfium.PdfDocument.new()
    pdf_doc.new_page(width=100, height=100)
    pdf_path = tmp_path / "test.pdf"
    pdf_doc.save(str(pdf_path))
    pdf_doc.close()

    mock_prov = MagicMock()
    mock_prov.page_no = 1
    mock_prov.bbox = BoundingBox(l=0, t=100, r=100, b=0, coord_origin=CoordOrigin.BOTTOMLEFT)

    mock_pic = MagicMock()
    mock_pic.prov = [mock_prov]

    mock_page_size = MagicMock()
    mock_page_size.width = 100.0
    mock_page_size.height = 100.0

    mock_doc = MagicMock()
    mock_doc.pictures = [mock_pic]
    mock_doc.pages = {1: MagicMock(size=mock_page_size)}

    results = list(extract_images(mock_doc, pdf_path, scale=1.0))
    filename, _ = results[0]
    parts = filename.replace(".png", "").split("_")
    assert parts[0] == "image"
    assert parts[1].isdigit() and len(parts[1]) == 6
    assert len(parts[2]) == 8  # 8-char hash


def test_extract_images_skips_picture_with_no_prov(tmp_path):
    pdf_doc = pdfium.PdfDocument.new()
    pdf_doc.new_page(width=100, height=100)
    pdf_path = tmp_path / "test.pdf"
    pdf_doc.save(str(pdf_path))
    pdf_doc.close()

    mock_pic = MagicMock()
    mock_pic.prov = []

    mock_doc = MagicMock()
    mock_doc.pictures = [mock_pic]

    results = list(extract_images(mock_doc, pdf_path, scale=1.0))
    assert results == []


def test_bbox_to_crop_clamps_negative_margins():
    # bbox extends 5 pts outside left and top of page
    bbox = BoundingBox(l=-5, t=205, r=90, b=50, coord_origin=CoordOrigin.BOTTOMLEFT)
    crop = _bbox_to_crop(bbox, page_width=100, page_height=200)
    assert crop[0] == 0.0  # left clamped
    assert crop[3] == 0.0  # top clamped
    assert crop[1] == 50.0  # bottom unchanged
    assert crop[2] == 10.0  # right unchanged


def test_bbox_to_crop_handles_topleft_origin():
    # TOPLEFT: l=10, t=50 (from top), r=90, b=150 (from top) on 200-tall page
    # After converting to BOTTOMLEFT: b = 200-150=50, t = 200-50=150
    # crop = (10, 50, 10, 50)
    bbox = BoundingBox(l=10, t=50, r=90, b=150, coord_origin=CoordOrigin.TOPLEFT)
    crop = _bbox_to_crop(bbox, page_width=100, page_height=200)
    assert crop == (10.0, 50.0, 10.0, 50.0)


def test_extract_images_skips_zero_size_bitmap(tmp_path):
    pdf_doc = pdfium.PdfDocument.new()
    pdf_doc.new_page(width=100, height=100)
    pdf_path = tmp_path / "test.pdf"
    pdf_doc.save(str(pdf_path))
    pdf_doc.close()

    mock_prov = MagicMock()
    mock_prov.page_no = 1
    mock_prov.bbox = BoundingBox(l=0, t=100, r=100, b=0, coord_origin=CoordOrigin.BOTTOMLEFT)

    mock_pic = MagicMock()
    mock_pic.prov = [mock_prov]

    mock_page_size = MagicMock()
    mock_page_size.width = 100.0
    mock_page_size.height = 100.0

    mock_doc = MagicMock()
    mock_doc.pictures = [mock_pic]
    mock_doc.pages = {1: MagicMock(size=mock_page_size)}

    zero_img = Image.new("RGB", (0, 0))

    with patch("pypdfium2.PdfPage.render") as mock_render:
        mock_bitmap = MagicMock()
        mock_bitmap.to_pil.return_value = zero_img
        mock_render.return_value = mock_bitmap
        results = list(extract_images(mock_doc, pdf_path, scale=1.0))

    assert results == []


def test_extract_images_empty_when_no_pictures(tmp_path):
    pdf_doc = pdfium.PdfDocument.new()
    pdf_doc.new_page(width=100, height=100)
    pdf_path = tmp_path / "test.pdf"
    pdf_doc.save(str(pdf_path))
    pdf_doc.close()

    mock_doc = MagicMock()
    mock_doc.pictures = []

    results = list(extract_images(mock_doc, pdf_path, scale=1.0))
    assert results == []


def test_rewrite_replaces_placeholders_in_order():
    md = "# Title\n\n<!-- image -->\n\nSome text\n\n<!-- image -->\n"
    images = [("image_000000_aabbccdd.png", b""), ("image_000001_eeff0011.png", b"")]
    result = rewrite_image_placeholders(md, images)
    assert "![Image](image_000000_aabbccdd.png)" in result
    assert "![Image](image_000001_eeff0011.png)" in result
    assert "<!-- image -->" not in result


def test_rewrite_no_placeholders_returns_unchanged():
    md = "# Title\n\nNo images here.\n"
    result = rewrite_image_placeholders(md, [])
    assert result == md


def test_rewrite_fewer_images_than_placeholders_leaves_remainder():
    md = "<!-- image -->\n<!-- image -->\n<!-- image -->\n"
    images = [("image_000000_aabb.png", b"")]
    result = rewrite_image_placeholders(md, images)
    assert "![Image](image_000000_aabb.png)" in result
    assert result.count("<!-- image -->") == 2
