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


def extract_metadata(
    source: Path | str, suffix: str, generate_cover: bool
) -> tuple[str, str, bytes]:
    """Extract (title, author, cover) from a document.

    source is a Path for file-backed documents or a URL string for remote ones.
    For URL sources, HTML metadata is fetched directly from the URL; PDF URLs
    cannot be re-parsed so metadata is skipped.
    """
    if isinstance(source, str):
        # URL import: PDF URLs cannot be re-fetched for metadata (Docling owns the download).
        # Everything else (HTML, no extension, unknown) is fetched as HTML.
        if suffix == ".pdf":
            return "", "", b""
        from parser.formats.html.metadata import extract_metadata_from_url

        return extract_metadata_from_url(source, generate_cover=generate_cover)

    if suffix in _HTML_SUFFIXES:
        from parser.formats.html.metadata import extract_metadata as _html_meta

        return _html_meta(source, generate_cover=generate_cover)
    if suffix == ".pdf":
        from parser.formats.pdf.metadata import extract_metadata as _pdf_meta

        return _pdf_meta(source, generate_cover=generate_cover)
    return "", "", b""
