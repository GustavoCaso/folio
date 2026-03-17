import logging
from pathlib import Path
from unittest.mock import patch

import pytest

from pdf_to_md.cli import main
from pdf_to_md.converter import UnsupportedFormatError


def test_cli_custom_output(tmp_path: Path, caplog: pytest.LogCaptureFixture) -> None:
    input_pdf = tmp_path / "book.pdf"
    custom_out = tmp_path / "custom.md"

    with patch("pdf_to_md.cli.convert", return_value=custom_out):
        with patch("sys.argv", ["pdf-to-md", str(input_pdf), "-o", str(custom_out)]):
            with caplog.at_level(logging.INFO, logger="pdf_to_md"):
                main()

    assert any(str(custom_out) in r.message for r in caplog.records)


def test_cli_unsupported_format_exits_1(tmp_path: Path) -> None:
    input_file = tmp_path / "book.epub"

    with patch("pdf_to_md.cli.convert", side_effect=UnsupportedFormatError("no handler")):
        with patch("sys.argv", ["pdf-to-md", str(input_file)]):
            with pytest.raises(SystemExit) as exc:
                main()
    assert exc.value.code == 1


def test_cli_file_not_found_exits_1(tmp_path: Path) -> None:
    input_file = tmp_path / "missing.pdf"

    with patch("pdf_to_md.cli.convert", side_effect=FileNotFoundError()):
        with patch("sys.argv", ["pdf-to-md", str(input_file)]):
            with pytest.raises(SystemExit) as exc:
                main()
    assert exc.value.code == 1


def test_cli_logs_converted_at_info(tmp_path: Path, caplog: pytest.LogCaptureFixture) -> None:
    input_pdf = tmp_path / "book.pdf"
    output_md = tmp_path / "book.md"

    with patch("pdf_to_md.cli.convert", return_value=output_md):
        with patch("sys.argv", ["pdf-to-md", str(input_pdf)]):
            with caplog.at_level(logging.INFO, logger="pdf_to_md"):
                main()

    assert any("Converted" in r.message and str(output_md) in r.message for r in caplog.records)


def test_cli_log_level_warning_suppresses_info(tmp_path: Path, caplog: pytest.LogCaptureFixture) -> None:
    input_pdf = tmp_path / "book.pdf"
    output_md = tmp_path / "book.md"

    with patch("pdf_to_md.cli.convert", return_value=output_md):
        with patch("sys.argv", ["pdf-to-md", str(input_pdf), "--log-level", "WARNING"]):
            with caplog.at_level(logging.DEBUG, logger="pdf_to_md"):
                main()

    info_records = [r for r in caplog.records if r.levelno == logging.INFO]
    assert len(info_records) == 0


def test_cli_logs_warning_on_unsupported_format(tmp_path: Path, caplog: pytest.LogCaptureFixture) -> None:
    input_file = tmp_path / "book.epub"

    with patch("pdf_to_md.cli.convert", side_effect=UnsupportedFormatError("no handler")):
        with patch("sys.argv", ["pdf-to-md", str(input_file)]):
            with caplog.at_level(logging.WARNING, logger="pdf_to_md"):
                with pytest.raises(SystemExit):
                    main()

    assert any(r.levelno == logging.WARNING for r in caplog.records)


def test_cli_logs_warning_on_file_not_found(tmp_path: Path, caplog: pytest.LogCaptureFixture) -> None:
    input_file = tmp_path / "missing.pdf"

    with patch("pdf_to_md.cli.convert", side_effect=FileNotFoundError()):
        with patch("sys.argv", ["pdf-to-md", str(input_file)]):
            with caplog.at_level(logging.WARNING, logger="pdf_to_md"):
                with pytest.raises(SystemExit):
                    main()

    assert any(r.levelno == logging.WARNING for r in caplog.records)
