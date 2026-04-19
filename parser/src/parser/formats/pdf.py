import os
from pathlib import Path

import pypdfium2
from docling.datamodel.base_models import InputFormat
from docling.datamodel.pipeline_options import PdfPipelineOptions
from docling.document_converter import DocumentConverter, PdfFormatOption
from docling_core.types.doc.base import ImageRefMode


def _pipeline_options() -> PdfPipelineOptions:
    kwargs: dict[str, object] = {
        "generate_picture_images": True,
    }
    for env_var, field in (
        ("PDF_LAYOUT_BATCH_SIZE", "layout_batch_size"),
        ("PDF_OCR_BATCH_SIZE", "ocr_batch_size"),
        ("PDF_TABLE_BATCH_SIZE", "table_batch_size"),
        ("PDF_QUEUE_MAX_SIZE", "queue_max_size"),
    ):
        value = os.environ.get(env_var)
        if value is not None:
            kwargs[field] = int(value)
    return PdfPipelineOptions(**kwargs)  # type: ignore[arg-type]


_converter: DocumentConverter = DocumentConverter(
    format_options={
        InputFormat.PDF: PdfFormatOption(pipeline_options=_pipeline_options()),
    }
)


def convert_pdf(path: Path) -> str:
    """Convert a PDF file to Markdown using Docling. Blocking call."""
    result = _converter.convert(str(path))
    return result.document.export_to_markdown(image_mode=ImageRefMode.EMBEDDED)


def count_pdf_pages(path: Path) -> int:
    pdf = pypdfium2.PdfDocument(str(path))
    try:
        return len(pdf)
    finally:
        pdf.close()
