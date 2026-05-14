from __future__ import annotations

import io
import logging
from typing import TYPE_CHECKING

import pypdfium2

if TYPE_CHECKING:
    from pathlib import Path

logger = logging.getLogger(__name__)


def extract_metadata(pdf_path: Path, generate_cover: bool) -> tuple[str, str, bytes]:
    """Return (title, author, cover) from PDF file. Empty strings if absent."""
    try:
        pdf = pypdfium2.PdfDocument(str(pdf_path))
        try:
            title = pdf.get_metadata_value("Title")
            author = pdf.get_metadata_value("Author") or pdf.get_metadata_value("Creator")
            cover = b""
            if generate_cover:
                cover = render_pdf_page(pdf[0])
            return title, author, cover
        finally:
            pdf.close()
    except Exception:
        logger.warning("metadata extraction failed", exc_info=True)
        return "", "", b""


def render_pdf_page(page: pypdfium2.PdfPage) -> bytes:
    """Render PDF page as PNG at 72 DPI. Returns empty bytes on failure."""
    bitmap = page.render(scale=1)
    pil_image = bitmap.to_pil()
    buf = io.BytesIO()
    pil_image.save(buf, format="PNG")
    return buf.getvalue()
