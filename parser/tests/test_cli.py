from unittest.mock import MagicMock, patch

from docling_core.types.doc.document import DoclingDocument

from parser.cli import build_parser


def _make_mock_doc(markdown: str = "# Hello") -> MagicMock:
    mock_doc = MagicMock(spec=DoclingDocument)
    mock_doc.export_to_markdown.return_value = markdown
    return mock_doc


def test_convert_command_calls_converter(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")
    out = tmp_path / "doc.md"

    p = build_parser()
    args = p.parse_args(["convert", str(pdf), "--output", str(out)])

    mock_doc = _make_mock_doc()
    with patch("parser.cli.convert", return_value=mock_doc) as mock_convert:
        args.func(args)
        mock_convert.assert_called_once_with(pdf)


def test_convert_command_default_output(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")

    p = build_parser()
    args = p.parse_args(["convert", str(pdf)])

    mock_doc = _make_mock_doc()
    with patch("parser.cli.convert", return_value=mock_doc) as mock_convert:
        args.func(args)
        mock_convert.assert_called_once_with(pdf)
