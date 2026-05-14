from __future__ import annotations

from unittest.mock import MagicMock, patch

from parser.formats.pdf.metadata import extract_metadata


def _mock_pdf(title: str = "", author: str = "") -> MagicMock:
    mock_doc = MagicMock()
    mock_doc.get_metadata_value.side_effect = lambda key: {
        "Title": title,
        "Author": author,
    }.get(key, "")
    mock_doc.close = MagicMock()
    return mock_doc


def _mock_pdf_with_cover(title: str = "", author: str = "") -> MagicMock:
    def save_side_effect(buf, format):  # noqa: A002
        buf.write(b"\x89PNG\r\n\x1a\n")

    mock_doc = _mock_pdf(title, author)
    mock_page = MagicMock()
    mock_bitmap = MagicMock()
    mock_pil = MagicMock()
    mock_pil.save.side_effect = save_side_effect
    mock_bitmap.to_pil.return_value = mock_pil
    mock_page.render.return_value = mock_bitmap
    mock_doc.__getitem__ = lambda self, i: mock_page
    return mock_doc


def test_extract_metadata_returns_title_and_author(tmp_path):
    with patch(
        "parser.formats.pdf.metadata.pypdfium2.PdfDocument",
        return_value=_mock_pdf("My Book", "Jane Doe"),
    ):
        title, author, cover = extract_metadata(tmp_path / "fake.pdf", generate_cover=False)
    assert title == "My Book"
    assert author == "Jane Doe"
    assert cover == b""


def test_extract_metadata_empty_when_missing(tmp_path):
    with patch("parser.formats.pdf.metadata.pypdfium2.PdfDocument", return_value=_mock_pdf()):
        title, author, cover = extract_metadata(tmp_path / "fake.pdf", generate_cover=False)
    assert title == ""
    assert author == ""
    assert cover == b""


def test_extract_metadata_returns_empty_on_error(tmp_path):
    with patch(
        "parser.formats.pdf.metadata.pypdfium2.PdfDocument", side_effect=Exception("bad pdf")
    ):
        title, author, cover = extract_metadata(tmp_path / "fake.pdf", generate_cover=False)
    assert title == ""
    assert author == ""
    assert cover == b""


def test_extract_metadata_returns_cover_when_requested(tmp_path):
    with patch(
        "parser.formats.pdf.metadata.pypdfium2.PdfDocument",
        return_value=_mock_pdf_with_cover("My Book", "Jane Doe"),
    ):
        title, author, cover = extract_metadata(tmp_path / "fake.pdf", generate_cover=True)
    assert title == "My Book"
    assert author == "Jane Doe"
    assert len(cover) > 0


def test_extract_metadata_no_cover_when_not_requested(tmp_path):
    with patch(
        "parser.formats.pdf.metadata.pypdfium2.PdfDocument",
        return_value=_mock_pdf_with_cover("My Book", "Jane Doe"),
    ):
        _, _, cover = extract_metadata(tmp_path / "fake.pdf", generate_cover=False)
    assert cover == b""
