import logging
from collections.abc import Callable
from functools import cache
from pathlib import Path
from types import MappingProxyType

logger = logging.getLogger("pdf_to_md")


class UnsupportedFormatError(Exception):
    pass


@cache
def _get_handlers() -> MappingProxyType[str, Callable[[Path], str]]:
    from pdf_to_md.formats import pdf

    return MappingProxyType({".pdf": pdf.convert})


def convert(input_path: Path, output_path: Path | None = None) -> Path:
    """Convert input file to Markdown. Returns the output path written."""
    handlers = _get_handlers()
    suffix = input_path.suffix.lower()
    handler = handlers.get(suffix)
    if handler is None:
        raise UnsupportedFormatError(
            f"No handler for format '{suffix}'. Supported: {list(handlers)}"
        )

    logger.info("Converting: %s", input_path)
    markdown = handler(input_path)

    if output_path is None:
        output_path = input_path.with_suffix(".md")

    output_path.write_text(markdown, encoding="utf-8")
    logger.info("Written: %s", output_path)
    return output_path
