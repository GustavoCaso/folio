from unittest.mock import MagicMock, patch

from docling_core.types.doc.base import ImageRefMode
from docling_core.types.doc.document import DoclingDocument

import parser.formats.html.html as html_mod


class TestBoolEnv:
    def test_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            assert html_mod._bool_env("MISSING_VAR", False) is False

    def test_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            assert html_mod._bool_env("MISSING_VAR", True) is True

    def test_truthy_values(self):
        for val in ("1", "true", "True", "TRUE", "yes"):
            with patch.dict("os.environ", {"SOME_VAR": val}, clear=True):
                assert html_mod._bool_env("SOME_VAR", False) is True

    def test_falsy_values(self):
        for val in ("0", "false", "no", "off", ""):
            with patch.dict("os.environ", {"SOME_VAR": val}, clear=True):
                assert html_mod._bool_env("SOME_VAR", True) is False


class TestImageMode:
    def test_default_is_placeholder(self):
        with patch.dict("os.environ", {}, clear=True):
            assert html_mod._image_mode() == ImageRefMode.PLACEHOLDER

    def test_embedded(self):
        with patch.dict("os.environ", {"HTML_IMAGE_MODE": "embedded"}, clear=True):
            assert html_mod._image_mode() == ImageRefMode.EMBEDDED

    def test_referenced(self):
        with patch.dict("os.environ", {"HTML_IMAGE_MODE": "referenced"}, clear=True):
            assert html_mod._image_mode() == ImageRefMode.REFERENCED

    def test_unknown_falls_back_to_placeholder(self):
        with patch.dict("os.environ", {"HTML_IMAGE_MODE": "bogus"}, clear=True):
            assert html_mod._image_mode() == ImageRefMode.PLACEHOLDER

    def test_case_insensitive(self):
        with patch.dict("os.environ", {"HTML_IMAGE_MODE": "EMBEDDED"}, clear=True):
            assert html_mod._image_mode() == ImageRefMode.EMBEDDED


class TestBackendOptions:
    def test_fetch_images_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = html_mod._backend_options()
            assert opts.fetch_images is False

    def test_fetch_images_enabled(self):
        with patch.dict("os.environ", {"HTML_FETCH_IMAGES": "true"}, clear=True):
            opts = html_mod._backend_options()
            assert opts.fetch_images is True


class TestConvertHtml:
    def test_returns_docling_document(self, tmp_path):
        html = tmp_path / "doc.html"
        html.write_text("<html><body>Hello</body></html>")

        mock_document = MagicMock(spec=DoclingDocument)
        mock_result = MagicMock()
        mock_result.document = mock_document

        with patch.object(html_mod._converter, "convert", return_value=mock_result):
            result = html_mod.convert_html(html)

        assert result is mock_document

    def test_calls_converter_with_path_string(self, tmp_path):
        html = tmp_path / "doc.html"
        html.write_text("<html><body>Hello</body></html>")

        mock_result = MagicMock()

        with patch.object(html_mod._converter, "convert", return_value=mock_result) as mock_convert:
            html_mod.convert_html(html)

        mock_convert.assert_called_once_with(str(html))
