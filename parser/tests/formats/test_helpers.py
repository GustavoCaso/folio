from unittest.mock import patch

import pytest
from docling_core.types.doc.base import ImageRefMode

from parser.formats.helpers import bool_env, image_mode


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


class TestImageMode:
    def test_default_is_placeholder(self):
        with patch.dict("os.environ", {}, clear=True):
            assert image_mode("MY_IMAGE_MODE") == ImageRefMode.PLACEHOLDER

    def test_embedded(self):
        with patch.dict("os.environ", {"MY_IMAGE_MODE": "embedded"}, clear=True):
            assert image_mode("MY_IMAGE_MODE") == ImageRefMode.EMBEDDED

    def test_referenced(self):
        with patch.dict("os.environ", {"MY_IMAGE_MODE": "referenced"}, clear=True):
            assert image_mode("MY_IMAGE_MODE") == ImageRefMode.REFERENCED

    def test_unknown_falls_back_to_placeholder(self):
        with patch.dict("os.environ", {"MY_IMAGE_MODE": "bogus"}, clear=True):
            assert image_mode("MY_IMAGE_MODE") == ImageRefMode.PLACEHOLDER

    def test_case_insensitive(self):
        with patch.dict("os.environ", {"MY_IMAGE_MODE": "EMBEDDED"}, clear=True):
            assert image_mode("MY_IMAGE_MODE") == ImageRefMode.EMBEDDED
