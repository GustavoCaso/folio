from __future__ import annotations

from docling.document_converter import DocumentConverter
from docling_core.types.doc.document import DoclingDocument

_converter: DocumentConverter = DocumentConverter()


def convert_html_url(url: str) -> DoclingDocument:
    """Convert an HTML page or PDF at the given URL using Docling. Returns the DoclingDocument."""
    result = _converter.convert(source=url)
    return result.document
