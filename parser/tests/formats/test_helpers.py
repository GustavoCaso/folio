from unittest.mock import patch

import pytest

from parser.formats.helpers import bool_env


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
