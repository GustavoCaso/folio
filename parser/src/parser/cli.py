import argparse
import logging
import os
import sys
import time
from pathlib import Path

from parser.converter import convert
from parser.logging_config import configure

logger = logging.getLogger(__name__)


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="parser", description="Convert documents to Markdown")
    subparsers = p.add_subparsers(dest="command")

    convert_cmd = subparsers.add_parser("convert", help="Convert a document directly (no server)")
    convert_cmd.add_argument("input", help="Path to input document")
    convert_cmd.add_argument("--output", "-o", help="Output path (default: same name with .md)", default=None)
    convert_cmd.set_defaults(func=_convert)

    return p


def _convert(args: argparse.Namespace) -> None:
    input_path = Path(args.input)
    output_path = Path(args.output) if args.output else input_path.with_suffix(".md")
    logger.info(
        "cli convert start",
        extra={"input_path": str(input_path), "output_path": str(output_path)},
    )
    started = time.monotonic()
    markdown = convert(input_path)
    output_path.write_text(markdown, encoding="utf-8")
    logger.info(
        "cli convert done",
        extra={
            "input_path": str(input_path),
            "output_path": str(output_path),
            "md_bytes": len(markdown.encode("utf-8")),
            "dur_ms": int((time.monotonic() - started) * 1000),
        },
    )
    print(f"Written: {output_path}")


def main() -> None:
    configure(os.environ.get("LOG_LEVEL"))
    p = build_parser()
    args = p.parse_args()
    if not hasattr(args, "func"):
        p.print_help()
        sys.exit(1)
    args.func(args)
