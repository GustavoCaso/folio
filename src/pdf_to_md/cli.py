import argparse
import logging
from pathlib import Path

from pdf_to_md.converter import UnsupportedFormatError, convert

logger = logging.getLogger("pdf_to_md")


def main() -> None:
    parser = argparse.ArgumentParser(description="Convert PDF (and other formats) to Markdown")
    parser.add_argument("input", type=Path, help="Input file path")
    parser.add_argument("-o", "--output", type=Path, default=None, help="Output Markdown file path")
    parser.add_argument(
        "--log-level",
        default="INFO",
        choices=["DEBUG", "INFO", "WARNING", "ERROR"],
        help="Set the logging level (default: INFO)",
    )
    args = parser.parse_args()

    log_level = getattr(logging, args.log_level)
    logging.basicConfig(
        level=log_level,
        format="%(levelname)s: %(message)s",
    )
    logger.setLevel(log_level)

    try:
        output = convert(args.input, args.output)
        logger.info("Converted: %s", output)
    except UnsupportedFormatError as e:
        logger.warning("Error: %s", e)
        raise SystemExit(1)
    except FileNotFoundError:
        logger.warning("Error: File not found: %s", args.input)
        raise SystemExit(1)
