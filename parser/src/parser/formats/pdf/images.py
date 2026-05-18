import hashlib
import io
from collections.abc import Iterator
from pathlib import Path

import pypdfium2 as pdfium
from docling_core.types.doc.base import BoundingBox, CoordOrigin
from docling_core.types.doc.document import DoclingDocument

_DEFAULT_SCALE = 0.5 * 150 / 72  # 75 DPI
PLACEHOLDER = "<!-- image -->"


def _bbox_to_crop(
    bbox: BoundingBox, page_width: float, page_height: float
) -> tuple[float, float, float, float]:
    """Convert a BOTTOMLEFT bbox to pypdfium2 crop margins (left, bottom, right, top)."""
    bl = (
        bbox
        if bbox.coord_origin == CoordOrigin.BOTTOMLEFT
        else bbox.to_bottom_left_origin(page_height)
    )
    return (
        max(0.0, bl.l),
        max(0.0, bl.b),
        max(0.0, page_width - bl.r),
        max(0.0, page_height - bl.t),
    )


def extract_images(
    doc: DoclingDocument,
    pdf_path: Path,
    scale: float = _DEFAULT_SCALE,
) -> Iterator[tuple[str, bytes]]:
    """Render each picture bbox from the PDF using pypdfium2.

    Yields (filename, png_bytes) pairs in document order, matching the order
    of <!-- image --> placeholders produced by save_as_markdown(PLACEHOLDER).
    Renders one crop at a time — no full page images held in memory.
    """
    pictures = list(doc.pictures)
    if not pictures:
        return

    pdf = pdfium.PdfDocument(str(pdf_path))
    try:
        for idx, pic in enumerate(pictures):
            if not pic.prov:
                continue
            prov = pic.prov[0]
            page_no = prov.page_no  # 1-based
            page_size = doc.pages[page_no].size

            crop = _bbox_to_crop(prov.bbox, page_size.width, page_size.height)

            pdf_page = pdf[page_no - 1]  # pypdfium2 is 0-based
            try:
                bitmap = pdf_page.render(scale=scale, crop=crop)
                pil_image = bitmap.to_pil()
                if pil_image.size == (0, 0):
                    continue
                buf = io.BytesIO()
                pil_image.save(buf, format="PNG")
                data = buf.getvalue()
            finally:
                pdf_page.close()

            content_hash = hashlib.sha256(data).hexdigest()[:8]
            filename = f"image_{idx:06d}_{content_hash}.png"
            yield filename, data
    finally:
        pdf.close()


def rewrite_image_placeholders(
    markdown: str,
    images: list[tuple[str, bytes]],
) -> str:
    """Replace <!-- image --> placeholders with ![Image](filename) in document order."""
    for filename, _ in images:
        if PLACEHOLDER not in markdown:
            break
        markdown = markdown.replace(PLACEHOLDER, f"![Image]({filename})", 1)
    return markdown
