from __future__ import annotations

from unittest.mock import MagicMock, patch

from parser.formats.pdf.metadata import extract_metadata, render_cover


def _mock_pdf(title: str = "", author: str = "") -> MagicMock:
    mock_doc = MagicMock()
    mock_doc.get_metadata_value.side_effect = lambda key: {"Title": title, "Author": author}.get(key, "")
    mock_doc.close = MagicMock()
    return mock_doc


def test_extract_metadata_returns_title_and_author(tmp_path):
    with patch("parser.formats.pdf.metadata.pypdfium2.PdfDocument", return_value=_mock_pdf("My Book", "Jane Doe")):
        title, author = extract_metadata(object(), tmp_path / "fake.pdf")
    assert title == "My Book"
    assert author == "Jane Doe"


def test_extract_metadata_empty_when_missing(tmp_path):
    with patch("parser.formats.pdf.metadata.pypdfium2.PdfDocument", return_value=_mock_pdf()):
        title, author = extract_metadata(object(), tmp_path / "fake.pdf")
    assert title == ""
    assert author == ""


def test_extract_metadata_returns_empty_on_error(tmp_path):
    with patch("parser.formats.pdf.metadata.pypdfium2.PdfDocument", side_effect=Exception("bad pdf")):
        title, author = extract_metadata(object(), tmp_path / "fake.pdf")
    assert title == ""
    assert author == ""


def test_render_cover_returns_bytes_on_success(tmp_path):
    def save_side_effect(buf, format):  # noqa: A002
        buf.write(b"\x89PNG\r\n\x1a\n")

    with patch("parser.formats.pdf.metadata.pypdfium2.PdfDocument") as mock_pdf:
        mock_doc = MagicMock()
        mock_page = MagicMock()
        mock_bitmap = MagicMock()
        mock_pil = MagicMock()
        mock_pil.save.side_effect = save_side_effect
        mock_bitmap.to_pil.return_value = mock_pil
        mock_page.render.return_value = mock_bitmap
        mock_doc.__getitem__ = lambda self, i: mock_page
        mock_doc.close = MagicMock()
        mock_pdf.return_value = mock_doc

        result = render_cover(tmp_path / "fake.pdf")

    assert isinstance(result, bytes)
    assert len(result) > 0


def test_render_cover_returns_empty_bytes_on_error(tmp_path):
    with patch("parser.formats.pdf.metadata.pypdfium2.PdfDocument", side_effect=Exception("bad pdf")):
        result = render_cover(tmp_path / "fake.pdf")
    assert result == b""
