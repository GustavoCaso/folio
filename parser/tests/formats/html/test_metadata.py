from __future__ import annotations

import base64
from typing import TYPE_CHECKING
from unittest.mock import MagicMock, patch

if TYPE_CHECKING:
    from pathlib import Path

from parser.formats.html.metadata import extract_metadata, extract_metadata_from_url


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

    def test_og_title_preferred_over_title_tag(self, tmp_path):
        p = _write_html(
            tmp_path,
            (
                "<html><head>"
                "<title>My Page | Site Name</title>"
                '<meta property="og:title" content="My Page">'
                "</head></html>"
            ),
        )
        title, _, _ = extract_metadata(p, generate_cover=False)
        assert title == "My Page"

    def test_title_tag_used_when_no_og_title(self, tmp_path):
        p = _write_html(tmp_path, "<html><head><title>Fallback Title</title></head></html>")
        title, _, _ = extract_metadata(p, generate_cover=False)
        assert title == "Fallback Title"

    def test_og_title_via_name_attribute(self, tmp_path):
        p = _write_html(
            tmp_path,
            '<html><head><meta name="og:title" content="OG Name Title"></head></html>',
        )
        title, _, _ = extract_metadata(p, generate_cover=False)
        assert title == "OG Name Title"

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

    def test_cover_fetched_from_https_og_image(self, tmp_path):
        fake_image = b"\x89PNG\r\n\x1a\nFAKE"
        p = _write_html(
            tmp_path,
            '<html><head><meta property="og:image" content="https://example.com/img.png"></head></html>',
        )
        mock_resp = MagicMock()
        mock_resp.read.return_value = fake_image
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)
        with patch("urllib.request.urlopen", return_value=mock_resp):
            _, _, cover = extract_metadata(p, generate_cover=True)
        assert cover == fake_image

    def test_cover_empty_when_https_fetch_fails(self, tmp_path):
        p = _write_html(
            tmp_path,
            '<html><head><meta property="og:image" content="https://example.com/img.png"></head></html>',
        )
        with patch("urllib.request.urlopen", side_effect=OSError("connection refused")):
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

    def test_cover_empty_for_unknown_url_scheme(self, tmp_path):
        p = _write_html(
            tmp_path,
            '<html><head><meta property="og:image" content="ftp://example.com/img.png"></head></html>',
        )
        _, _, cover = extract_metadata(p, generate_cover=True)
        assert cover == b""


class TestExtractMetadataFromUrl:
    _HTML = (
        "<html><head>"
        "<title>Full Title | Site</title>"
        '<meta property="og:title" content="Clean Title">'
        '<meta name="author" content="Ada Lovelace">'
        '<meta property="og:image" content="https://example.com/cover.png">'
        "</head></html>"
    )

    def _mock_urlopen(self, body: bytes):
        mock_resp = MagicMock()
        mock_resp.read.return_value = body
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)
        return mock_resp

    def test_extracts_og_title_and_author(self):
        with patch("urllib.request.urlopen", return_value=self._mock_urlopen(self._HTML.encode())):
            title, author, _ = extract_metadata_from_url(
                "https://example.com/page", generate_cover=False
            )
        assert title == "Clean Title"
        assert author == "Ada Lovelace"

    def test_fetches_og_image_cover(self):
        fake_cover = b"\x89PNG\r\n\x1a\nFAKE"
        img_resp = self._mock_urlopen(fake_cover)
        html_resp = self._mock_urlopen(self._HTML.encode())
        with patch("urllib.request.urlopen", side_effect=[html_resp, img_resp]):
            _, _, cover = extract_metadata_from_url("https://example.com/page", generate_cover=True)
        assert cover == fake_cover

    def test_returns_empty_on_fetch_failure(self):
        with patch("urllib.request.urlopen", side_effect=OSError("timeout")):
            title, author, cover = extract_metadata_from_url(
                "https://example.com/page", generate_cover=False
            )
        assert title == ""
        assert author == ""
        assert cover == b""
