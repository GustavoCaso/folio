import argparse
import logging
import os
import sys
import time
from pathlib import Path

from docling_core.types.doc.base import ImageRefMode

from parser.formats.converter import convert
from parser.formats.helpers import extract_metadata, get_format_settings
from parser.formats.pdf.images import PLACEHOLDER, extract_images
from parser.logging_config import configure

logger = logging.getLogger(__name__)


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="parser", description="Convert documents to Markdown")
    subparsers = p.add_subparsers(dest="command")

    convert_cmd = subparsers.add_parser("convert", help="Convert a document directly (no server)")
    convert_cmd.add_argument("input", help="Path to input document")
    convert_cmd.add_argument(
        "--output", "-o", help="Output path (default: same name with .md)", default=None
    )
    convert_cmd.set_defaults(func=_convert)

    metadata_cmd = subparsers.add_parser("metadata", help="Extract metadata from a PDF")
    metadata_cmd.add_argument("input", help="Path to PDF")
    metadata_cmd.add_argument(
        "--cover",
        "-c",
        help="Save cover PNG to this path (default: <input>.cover.png)",
        nargs="?",
        const="",
    )
    metadata_cmd.set_defaults(func=_metadata)

    return p


def _convert(args: argparse.Namespace) -> None:
    input_path = Path(args.input)
    output_path = Path(args.output) if args.output else input_path.with_suffix(".md")
    logger.info(
        "cli convert start",
        extra={"input_path": str(input_path), "output_path": str(output_path)},
    )
    started = time.monotonic()
    suffix = input_path.suffix.lower() or ".pdf"
    do_post_process = get_format_settings(suffix)
    doc = convert(input_path)

    if suffix == ".pdf":
        doc.save_as_markdown(filename=output_path, image_mode=ImageRefMode.PLACEHOLDER)
        markdown = output_path.read_text(encoding="utf-8")
        scale_val = float(os.environ.get("PDF_IMAGES_SCALE", "0.5")) * 150 / 72
        artifacts_dir = output_path.parent / (output_path.stem + "_artifacts")
        has_images = False
        for name, data in extract_images(doc, input_path, scale=scale_val):
            if not has_images:
                artifacts_dir.mkdir(exist_ok=True)
                has_images = True
            markdown = markdown.replace(PLACEHOLDER, f"![Image]({artifacts_dir.name}/{name})", 1)
            (artifacts_dir / name).write_bytes(data)
        if do_post_process:
            from parser.postprocess import enrich_code_blocks

            markdown = enrich_code_blocks(markdown)
        output_path.write_text(markdown, encoding="utf-8")
    else:
        doc.save_as_markdown(filename=output_path, image_mode=ImageRefMode.REFERENCED)
        if do_post_process:
            from parser.postprocess import enrich_code_blocks

            output_path.write_text(
                enrich_code_blocks(output_path.read_text(encoding="utf-8")), encoding="utf-8"
            )

    md_bytes = output_path.stat().st_size if output_path.exists() else 0
    logger.info(
        "cli convert done",
        extra={
            "input_path": str(input_path),
            "output_path": str(output_path),
            "md_bytes": md_bytes,
            "dur_ms": int((time.monotonic() - started) * 1000),
        },
    )
    print(f"Written: {output_path}")


def _metadata(args: argparse.Namespace) -> None:
    input_path = Path(args.input)
    generate_cover = args.cover is not None
    suffix = input_path.suffix.lower() or ".pdf"
    title, author, cover = extract_metadata(input_path, suffix, generate_cover=generate_cover)
    print(f"Title:  {title or '(none)'}")
    print(f"Author: {author or '(none)'}")

    if generate_cover:
        cover_path = Path(args.cover) if args.cover else input_path.with_suffix(".cover.png")
        if cover:
            cover_path.write_bytes(cover)
            print(f"Cover:  {cover_path} ({len(cover)} bytes)")
        else:
            print("Cover:  (failed)")


def main() -> None:
    configure(os.environ.get("LOG_LEVEL"))
    p = build_parser()
    args = p.parse_args()
    if not hasattr(args, "func"):
        p.print_help()
        sys.exit(1)
    args.func(args)
