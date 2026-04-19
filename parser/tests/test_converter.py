from unittest.mock import patch

import pytest

from parser.converter import UnsupportedFormatError, convert


def test_convert_pdf_calls_handler(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")

    with patch("parser.formats.pdf.convert_pdf", return_value="# Hello") as mock_handler:
        result = convert(pdf)
        mock_handler.assert_called_once_with(pdf)
        assert result == "# Hello"


def test_convert_unsupported_format(tmp_path):
    f = tmp_path / "doc.xyz"
    f.write_bytes(b"data")
    with pytest.raises(UnsupportedFormatError):
        convert(f)
