import os
from pathlib import Path

_HTML_SUFFIXES = frozenset({".html", ".htm"})


def bool_env(var: str, default: bool) -> bool:
    val = os.environ.get(var)
    if val is None:
        return default
    return val.lower() in ("1", "true", "yes")


def get_format_settings(suffix: str) -> bool:
    """Return post_process_code_blocks for the given file extension."""
    if suffix in _HTML_SUFFIXES:
        from parser.formats.html.html import post_process_code_blocks

        return post_process_code_blocks
    from parser.formats.pdf.pdf import post_process_code_blocks

    return post_process_code_blocks


def extract_metadata(path: Path, suffix: str, generate_cover: bool) -> tuple[str, str, bytes]:
    """Extract (title, author, cover) from a document based on its file extension."""
    if suffix in _HTML_SUFFIXES:
        from parser.formats.html.metadata import extract_metadata as _html_meta

        return _html_meta(path, generate_cover=generate_cover)
    if suffix == ".pdf":
        from parser.formats.pdf.metadata import extract_metadata as _pdf_meta

        return _pdf_meta(path, generate_cover=generate_cover)
    return "", "", b""
