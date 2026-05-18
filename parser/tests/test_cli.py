from pathlib import Path
from unittest.mock import MagicMock, patch

from docling_core.types.doc.base import ImageRefMode
from docling_core.types.doc.document import DoclingDocument

from parser.cli import build_parser


def _make_mock_doc(output_content: str = "") -> MagicMock:
    mock_doc = MagicMock(spec=DoclingDocument)

    def _fake_save(filename: object, image_mode: object) -> None:
        Path(str(filename)).write_text(output_content, encoding="utf-8")

    mock_doc.save_as_markdown.side_effect = _fake_save
    return mock_doc


def test_convert_command_calls_converter(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")
    out = tmp_path / "doc.md"

    p = build_parser()
    args = p.parse_args(["convert", str(pdf), "--output", str(out)])

    mock_doc = _make_mock_doc()
    with (
        patch("parser.cli.convert", return_value=mock_doc) as mock_convert,
        patch("parser.cli.extract_images", return_value=iter([])),
    ):
        args.func(args)
        mock_convert.assert_called_once_with(pdf)


def test_convert_command_default_output(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")

    p = build_parser()
    args = p.parse_args(["convert", str(pdf)])

    mock_doc = _make_mock_doc()
    with (
        patch("parser.cli.convert", return_value=mock_doc) as mock_convert,
        patch("parser.cli.extract_images", return_value=iter([])),
    ):
        args.func(args)
        mock_convert.assert_called_once_with(pdf)


def test_convert_command_html_calls_converter(tmp_path):
    html = tmp_path / "doc.html"
    html.write_text("<html><body>hi</body></html>")
    out = tmp_path / "doc.md"

    p = build_parser()
    args = p.parse_args(["convert", str(html), "--output", str(out)])

    mock_doc = _make_mock_doc("# Hello")
    with patch("parser.cli.convert", return_value=mock_doc) as mock_convert:
        args.func(args)
        mock_convert.assert_called_once_with(html)


def test_convert_command_html_default_output(tmp_path):
    html = tmp_path / "page.html"
    html.write_text("<html><body>hi</body></html>")

    p = build_parser()
    args = p.parse_args(["convert", str(html)])

    mock_doc = _make_mock_doc("# Hello")
    with patch("parser.cli.convert", return_value=mock_doc):
        args.func(args)

    expected = tmp_path / "page.md"
    assert expected.exists()


def test_convert_always_uses_save_as_markdown(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")
    out = tmp_path / "doc.md"

    p = build_parser()
    args = p.parse_args(["convert", str(pdf), "--output", str(out)])

    mock_doc = _make_mock_doc()
    with (
        patch("parser.cli.convert", return_value=mock_doc),
        patch("parser.formats.pdf.pdf.post_process_code_blocks", False),
        patch("parser.cli.extract_images", return_value=iter([])),
    ):
        args.func(args)

    mock_doc.save_as_markdown.assert_called_once_with(
        filename=out, image_mode=ImageRefMode.PLACEHOLDER
    )
    mock_doc.export_to_markdown.assert_not_called()


def test_convert_html_always_uses_save_as_markdown(tmp_path):
    html = tmp_path / "doc.html"
    html.write_text("<html><body>hi</body></html>")
    out = tmp_path / "doc.md"

    p = build_parser()
    args = p.parse_args(["convert", str(html), "--output", str(out)])

    mock_doc = _make_mock_doc()
    with (
        patch("parser.cli.convert", return_value=mock_doc),
        patch("parser.formats.html.html.post_process_code_blocks", False),
    ):
        args.func(args)

    mock_doc.save_as_markdown.assert_called_once_with(
        filename=out, image_mode=ImageRefMode.REFERENCED
    )
    mock_doc.export_to_markdown.assert_not_called()


def test_cli_uses_placeholder_mode_and_extracts_images(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")
    out = tmp_path / "doc.md"

    p = build_parser()
    args = p.parse_args(["convert", str(pdf), "--output", str(out)])

    mock_doc = _make_mock_doc("# Hi\n\n<!-- image -->\n")

    fake_images = [("image_000000_aabb.png", b"\x89PNG")]

    with (
        patch("parser.cli.convert", return_value=mock_doc),
        patch("parser.formats.pdf.pdf.post_process_code_blocks", False),
        patch("parser.cli.extract_images", return_value=iter(fake_images)),
    ):
        args.func(args)

    content = out.read_text(encoding="utf-8")
    assert "<!-- image -->" not in content
    assert "![Image](doc_artifacts/image_000000_aabb.png)" in content

    mock_doc.save_as_markdown.assert_called_once_with(
        filename=out, image_mode=ImageRefMode.PLACEHOLDER
    )


def test_convert_post_process_still_runs(tmp_path):
    pdf = tmp_path / "doc.pdf"
    pdf.write_bytes(b"%PDF")
    out = tmp_path / "doc.md"
    out.write_text("```\nprint('hello')\n```", encoding="utf-8")

    p = build_parser()
    args = p.parse_args(["convert", str(pdf), "--output", str(out)])

    mock_doc = _make_mock_doc()

    def fake_save_as_markdown(filename: object, image_mode: object) -> None:
        pass  # file already written above, simulating save_as_markdown writing it

    mock_doc.save_as_markdown.side_effect = fake_save_as_markdown

    with (
        patch("parser.cli.convert", return_value=mock_doc),
        patch("parser.formats.pdf.pdf.post_process_code_blocks", True),
        patch("parser.postprocess.enrich_code_blocks", return_value="enriched") as mock_enrich,
        patch("parser.cli.extract_images", return_value=iter([])),
    ):
        args.func(args)

    mock_enrich.assert_called_once()
    assert out.read_text(encoding="utf-8") == "enriched"
