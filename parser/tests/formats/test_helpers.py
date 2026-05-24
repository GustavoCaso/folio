from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

from parser.formats.helpers import bool_env, extract_metadata


class TestBoolEnv:
    def test_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            assert bool_env("MISSING_VAR", False) is False

    def test_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            assert bool_env("MISSING_VAR", True) is True

    @pytest.mark.parametrize("val", ["1", "true", "True", "TRUE", "yes", "Yes", "YES"])
    def test_truthy_values(self, val):
        with patch.dict("os.environ", {"SOME_VAR": val}, clear=True):
            assert bool_env("SOME_VAR", False) is True

    @pytest.mark.parametrize("val", ["0", "false", "no", "off", ""])
    def test_falsy_values(self, val):
        with patch.dict("os.environ", {"SOME_VAR": val}, clear=True):
            assert bool_env("SOME_VAR", True) is False


class TestExtractMetadataDispatch:
    def _mock_urlopen(self, body: bytes) -> MagicMock:
        mock_resp = MagicMock()
        mock_resp.read.return_value = body
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)
        return mock_resp

    def test_path_html_dispatches_to_file(self, tmp_path: Path):
        p = tmp_path / "page.html"
        p.write_text("<html><head><title>File Title</title></head></html>", encoding="utf-8")
        title, _, _ = extract_metadata(p, ".html", generate_cover=False)
        assert title == "File Title"

    def test_path_pdf_dispatches_to_pdf(self, tmp_path: Path):
        p = tmp_path / "doc.pdf"
        p.write_bytes(b"")
        # PDF extractor returns empty for invalid file — just verify no crash and correct dispatch
        title, author, cover = extract_metadata(p, ".pdf", generate_cover=False)
        assert isinstance(title, str)

    def test_path_unknown_suffix_returns_empty(self, tmp_path: Path):
        p = tmp_path / "doc.xyz"
        p.write_bytes(b"")
        assert extract_metadata(p, ".xyz", generate_cover=False) == ("", "", b"")

    def test_url_html_fetches_page(self):
        html = b"<html><head><title>URL Title</title></head></html>"
        with patch("urllib.request.urlopen", return_value=self._mock_urlopen(html)):
            title, _, _ = extract_metadata(
                "https://example.com/page", ".html", generate_cover=False
            )
        assert title == "URL Title"

    def test_url_no_extension_treated_as_html(self):
        html = b"<html><head><title>Blog Post</title></head></html>"
        with patch("urllib.request.urlopen", return_value=self._mock_urlopen(html)):
            title, _, _ = extract_metadata(
                "https://example.com/blog/post", "", generate_cover=False
            )
        assert title == "Blog Post"

    def test_url_pdf_returns_empty(self):
        # PDF URLs cannot be re-fetched for metadata
        result = extract_metadata("https://example.com/doc.pdf", ".pdf", generate_cover=False)
        assert result == ("", "", b"")
