from unittest.mock import patch

import parser.formats.converter as converter_mod


class TestBackendOptions:
    def test_fetch_images_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.fetch_images is False

    def test_fetch_images_enabled(self):
        with patch.dict("os.environ", {"HTML_FETCH_IMAGES": "true"}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.fetch_images is True

    def test_render_page_default_false(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.render_page is False

    def test_render_page_enabled(self):
        with patch.dict("os.environ", {"HTML_RENDER_PAGE": "true"}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.render_page is True

    def test_render_dpi_default_96(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.render_dpi == 96

    def test_render_dpi_set_from_env(self):
        with patch.dict("os.environ", {"HTML_RENDER_DPI": "72"}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.render_dpi == 72

    def test_render_device_scale_default_1(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.render_device_scale == 1.0

    def test_render_device_scale_set_from_env(self):
        with patch.dict("os.environ", {"HTML_RENDER_DEVICE_SCALE": "0.5"}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.render_device_scale == 0.5

    def test_add_title_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.add_title is True

    def test_add_title_disabled(self):
        with patch.dict("os.environ", {"HTML_ADD_TITLE": "false"}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.add_title is False

    def test_infer_furniture_default_true(self):
        with patch.dict("os.environ", {}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.infer_furniture is True

    def test_infer_furniture_disabled(self):
        with patch.dict("os.environ", {"HTML_INFER_FURNITURE": "false"}, clear=True):
            opts = converter_mod.html_backend_options()
            assert opts.infer_furniture is False
