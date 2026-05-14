from unittest.mock import MagicMock, patch

import pytest
from docling_core.types.doc.document import DoclingDocument

from parser.converter import UnsupportedFormatError, convert


def test_convert_pdf_calls_handler(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")

    mock_document = MagicMock(spec=DoclingDocument)
    with patch("parser.formats.pdf.convert_pdf", return_value=mock_document) as mock_handler:
        result = convert(pdf)
        mock_handler.assert_called_once_with(pdf)
        assert result is mock_document


def test_convert_unsupported_format(tmp_path):
    f = tmp_path / "doc.xyz"
    f.write_bytes(b"data")
    with pytest.raises(UnsupportedFormatError):
        convert(f)
