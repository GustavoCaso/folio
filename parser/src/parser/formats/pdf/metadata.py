from __future__ import annotations

import io
import logging
from typing import TYPE_CHECKING

import pypdfium2

if TYPE_CHECKING:
    from pathlib import Path

logger = logging.getLogger(__name__)


def extract_metadata(doc: object, pdf_path: Path) -> tuple[str, str]:
    """Return (title, author) from PDF metadata. Empty strings if absent."""
    try:
        pdf = pypdfium2.PdfDocument(str(pdf_path))
        try:
            title = pdf.get_metadata_value("Title")
            author = pdf.get_metadata_value("Author")
            return title, author
        finally:
            pdf.close()
    except Exception:
        logger.warning("metadata extraction failed", exc_info=True)
        return "", ""


def render_cover(pdf_path: Path) -> bytes:
    """Render page 1 of a PDF as PNG at 72 DPI. Returns empty bytes on failure."""
    try:
        pdf = pypdfium2.PdfDocument(str(pdf_path))
        try:
            page = pdf[0]
            bitmap = page.render(scale=1)
            pil_image = bitmap.to_pil()
            buf = io.BytesIO()
            pil_image.save(buf, format="PNG")
            return buf.getvalue()
        finally:
            pdf.close()
    except Exception:
        logger.warning("cover render failed", exc_info=True)
        return b""
