from unittest.mock import patch

from parser.cli import build_parser


def test_convert_command_calls_converter(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")
    out = tmp_path / "doc.md"

    p = build_parser()
    args = p.parse_args(["convert", str(pdf), "--output", str(out)])

    with patch("parser.cli.convert", return_value="# Hello") as mock_convert:
        args.func(args)
        mock_convert.assert_called_once_with(pdf)


def test_convert_command_default_output(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")

    p = build_parser()
    args = p.parse_args(["convert", str(pdf)])

    with patch("parser.cli.convert", return_value="# Hello") as mock_convert:
        args.func(args)
        mock_convert.assert_called_once_with(pdf)
