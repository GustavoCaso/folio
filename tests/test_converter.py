import logging
from pathlib import Path
from typing import Generator
from types import MappingProxyType
from unittest.mock import MagicMock, patch

import pytest

from pdf_to_md.converter import UnsupportedFormatError, _get_handlers, convert


def test_convert_auto_names_output(tmp_path: Path) -> None:
    input_pdf = tmp_path / "book.pdf"
    input_pdf.write_bytes(b"%PDF-1.4 fake")

    with patch(
        "pdf_to_md.converter._get_handlers",
        return_value=MappingProxyType({".pdf": lambda p: "# Hello"}),
    ):
        output = convert(input_pdf)

    assert output == tmp_path / "book.md"
    assert (tmp_path / "book.md").read_text() == "# Hello"


def test_convert_custom_output_path(tmp_path: Path) -> None:
    input_pdf = tmp_path / "book.pdf"
    input_pdf.write_bytes(b"%PDF-1.4 fake")
    custom_out = tmp_path / "custom.md"

    with patch(
        "pdf_to_md.converter._get_handlers",
        return_value=MappingProxyType({".pdf": lambda p: "# Hello"}),
    ):
        output = convert(input_pdf, custom_out)

    assert output == custom_out
    assert custom_out.read_text() == "# Hello"


def test_convert_unsupported_format(tmp_path: Path) -> None:
    input_file = tmp_path / "book.epub"
    input_file.write_bytes(b"fake epub")

    with (
        patch(
            "pdf_to_md.converter._get_handlers",
            return_value=MappingProxyType({".pdf": lambda p: "# Hello"}),
        ),
        pytest.raises(UnsupportedFormatError),
    ):
        convert(input_file)


def test_converter_logs_info_on_success(tmp_path: Path, caplog: pytest.LogCaptureFixture) -> None:
    input_pdf = tmp_path / "book.pdf"
    output_md = tmp_path / "book.md"
    input_pdf.write_bytes(b"fake")

    mock_handler = MagicMock(return_value="# Hello")
    with (
        patch(
            "pdf_to_md.converter._get_handlers",
            return_value=MappingProxyType({".pdf": mock_handler}),
        ),
        caplog.at_level(logging.INFO, logger="pdf_to_md"),
    ):
        convert(input_pdf, output_md)

    assert any("book.pdf" in r.message for r in caplog.records if r.levelno == logging.INFO)
    assert any("book.md" in r.message for r in caplog.records if r.levelno == logging.INFO)
