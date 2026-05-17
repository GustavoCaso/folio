from __future__ import annotations

import base64
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from pathlib import Path

from parser.formats.html.metadata import extract_metadata


def _write_html(tmp_path: Path, content: str) -> Path:
    p = tmp_path / "page.html"
    p.write_text(content, encoding="utf-8")
    return p


class TestExtractMetadata:
    def test_extracts_title_from_title_tag(self, tmp_path):
        p = _write_html(tmp_path, "<html><head><title>My Page</title></head></html>")
        title, author, cover = extract_metadata(p, generate_cover=False)
        assert title == "My Page"
        assert author == ""
        assert cover == b""

    def test_extracts_author_from_meta(self, tmp_path):
        p = _write_html(
            tmp_path,
            '<html><head><meta name="author" content="Jane Doe"></head></html>',
        )
        title, author, cover = extract_metadata(p, generate_cover=False)
        assert author == "Jane Doe"

    def test_extracts_title_and_author_together(self, tmp_path):
        p = _write_html(
            tmp_path,
            (
                "<html><head>"
                "<title>My Book</title>"
                '<meta name="author" content="John Smith">'
                "</head></html>"
            ),
        )
        title, author, _ = extract_metadata(p, generate_cover=False)
        assert title == "My Book"
        assert author == "John Smith"

    def test_returns_empty_when_no_metadata(self, tmp_path):
        p = _write_html(tmp_path, "<html><body>Hello</body></html>")
        title, author, cover = extract_metadata(p, generate_cover=False)
        assert title == ""
        assert author == ""
        assert cover == b""

    def test_returns_empty_on_missing_file(self, tmp_path):
        missing = tmp_path / "missing.html"
        title, author, cover = extract_metadata(missing, generate_cover=False)
        assert title == ""
        assert author == ""
        assert cover == b""

    def test_cover_empty_when_generate_cover_false(self, tmp_path):
        raw = b"\x89PNG\r\n\x1a\nFAKEDATA"
        encoded = base64.b64encode(raw).decode()
        p = _write_html(
            tmp_path,
            f'<html><head><meta property="og:image" content="data:image/png;base64,{encoded}"></head></html>',
        )
        _, _, cover = extract_metadata(p, generate_cover=False)
        assert cover == b""

    def test_cover_decoded_from_og_image_data_uri(self, tmp_path):
        raw = b"\x89PNG\r\n\x1a\nFAKEDATA"
        encoded = base64.b64encode(raw).decode()
        p = _write_html(
            tmp_path,
            f'<html><head><meta property="og:image" content="data:image/png;base64,{encoded}"></head></html>',
        )
        _, _, cover = extract_metadata(p, generate_cover=True)
        assert cover == raw

    def test_cover_empty_for_non_data_uri_og_image(self, tmp_path):
        p = _write_html(
            tmp_path,
            '<html><head><meta property="og:image" content="https://example.com/img.png"></head></html>',
        )
        _, _, cover = extract_metadata(p, generate_cover=True)
        assert cover == b""

    def test_og_image_via_name_attribute(self, tmp_path):
        raw = b"\x89PNG\r\n\x1a\nFAKEDATA"
        encoded = base64.b64encode(raw).decode()
        p = _write_html(
            tmp_path,
            f'<html><head><meta name="og:image" content="data:image/png;base64,{encoded}"></head></html>',
        )
        _, _, cover = extract_metadata(p, generate_cover=True)
        assert cover == raw

    def test_cover_empty_for_invalid_base64(self, tmp_path):
        p = _write_html(
            tmp_path,
            '<html><head><meta property="og:image" content="data:image/png;base64,!!!invalid!!!"></head></html>',
        )
        _, _, cover = extract_metadata(p, generate_cover=True)
        assert cover == b""
