import os
from pathlib import Path

from docling_core.types.doc.base import ImageRefMode

_HTML_SUFFIXES = frozenset({".html", ".htm"})


def bool_env(var: str, default: bool) -> bool:
    val = os.environ.get(var)
    if val is None:
        return default
    return val.lower() in ("1", "true", "yes")


def image_mode(env_var: str) -> ImageRefMode:
    val = os.environ.get(env_var, "placeholder").lower()
    return {"embedded": ImageRefMode.EMBEDDED, "referenced": ImageRefMode.REFERENCED}.get(
        val, ImageRefMode.PLACEHOLDER
    )


def get_format_settings(suffix: str) -> tuple[ImageRefMode, bool]:
    """Return (image_mode, post_process_code_blocks) for the given file extension."""
    if suffix in _HTML_SUFFIXES:
        from parser.formats.html.html import image_mode_value, post_process_code_blocks

        return image_mode_value, post_process_code_blocks
    from parser.formats.pdf.pdf import image_mode_value, post_process_code_blocks

    return image_mode_value, post_process_code_blocks


def extract_metadata(path: Path, suffix: str, generate_cover: bool) -> tuple[str, str, bytes]:
    """Extract (title, author, cover) from a document based on its file extension."""
    if suffix in _HTML_SUFFIXES:
        from parser.formats.html.metadata import extract_metadata as _html_meta

        return _html_meta(path, generate_cover=generate_cover)
    if suffix == ".pdf":
        from parser.formats.pdf.metadata import extract_metadata as _pdf_meta

        return _pdf_meta(path, generate_cover=generate_cover)
    return "", "", b""
