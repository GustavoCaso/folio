import logging
import os
from pathlib import Path
from urllib.parse import urlparse

from docling.datamodel.accelerator_options import AcceleratorOptions
from docling.datamodel.backend_options import HTMLBackendOptions
from docling.datamodel.base_models import InputFormat
from docling.datamodel.pipeline_options import (
    CodeFormulaVlmOptions,
    PdfPipelineOptions,
    TableFormerMode,
    TableStructureOptions,
)
from docling.document_converter import DocumentConverter, HTMLFormatOption, PdfFormatOption
from docling_core.types.doc.document import DoclingDocument

from parser.formats.helpers import bool_env

logger = logging.getLogger(__name__)

_VALID_CODE_FORMULA_PRESETS = {"codeformulav2", "granite_docling"}
_VALID_TABLE_STRUCTURE_MODES = {"accurate", "fast"}

PDF_BATCH_SIZE: int = int(os.environ.get("PDF_BATCH_SIZE", "100"))


def _code_formula_options() -> CodeFormulaVlmOptions:
    preset = os.environ.get("PDF_CODE_FORMULA_PRESET", "codeformulav2").lower()
    if preset not in _VALID_CODE_FORMULA_PRESETS:
        preset = "codeformulav2"
    result: CodeFormulaVlmOptions = CodeFormulaVlmOptions.from_preset(preset)
    return result


def _table_structure_options() -> TableStructureOptions:
    mode_str = os.environ.get("PDF_TABLE_STRUCTURE_MODE", "accurate").lower()
    if mode_str not in _VALID_TABLE_STRUCTURE_MODES:
        mode_str = "accurate"
    mode = TableFormerMode.FAST if mode_str == "fast" else TableFormerMode.ACCURATE
    return TableStructureOptions(mode=mode)


def pdf_pipeline_options() -> PdfPipelineOptions:
    kwargs: dict[str, object] = {
        "generate_picture_images": False,
        "generate_page_images": bool_env("PDF_GENERATE_PAGE_IMAGES", False),
        "do_ocr": bool_env("PDF_DO_OCR", True),
        "do_table_structure": bool_env("PDF_DO_TABLE_STRUCTURE", True),
        "do_code_enrichment": bool_env("PDF_DO_CODE_ENRICHMENT", False),
        "do_formula_enrichment": bool_env("PDF_DO_FORMULA_ENRICHMENT", False),
        "force_backend_text": bool_env("PDF_FORCE_BACKEND_TEXT", False),
        "code_formula_options": _code_formula_options(),
        "table_structure_options": _table_structure_options(),
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
    for env_var, field in (
        ("PDF_IMAGES_SCALE", "images_scale"),
        ("PDF_DOCUMENT_TIMEOUT", "document_timeout"),
    ):
        value = os.environ.get(env_var)
        if value is not None:
            kwargs[field] = float(value)
    num_threads = os.environ.get("PDF_NUM_THREADS")
    if num_threads is not None:
        kwargs["accelerator_options"] = AcceleratorOptions(num_threads=int(num_threads))
    return PdfPipelineOptions(**kwargs)  # type: ignore[arg-type]


def html_backend_options(source_uri: str = "") -> HTMLBackendOptions:
    opts = HTMLBackendOptions(  # type: ignore[call-arg]
        fetch_images=bool_env("HTML_FETCH_IMAGES", False),
        render_page=bool_env("HTML_RENDER_PAGE", False),
        add_title=bool_env("HTML_ADD_TITLE", True),
        infer_furniture=bool_env("HTML_INFER_FURNITURE", True),
        enable_remote_fetch=bool_env("HTML_FETCH_IMAGES", False),
        source_uri=source_uri or None,  # type: ignore[arg-type]
    )
    render_dpi = os.environ.get("HTML_RENDER_DPI")
    if render_dpi is not None:
        opts.render_dpi = int(render_dpi)
    render_device_scale = os.environ.get("HTML_RENDER_DEVICE_SCALE")
    if render_device_scale is not None:
        opts.render_device_scale = float(render_device_scale)
    return opts


# PDF converter: single singleton — PDF pipeline construction is expensive
# (OCR, table structure, and layout models load on first use).
_pdf_converter: DocumentConverter = DocumentConverter(
    format_options={
        InputFormat.PDF: PdfFormatOption(pipeline_options=pdf_pipeline_options()),
    }
)


def _url_origin(url: str) -> str:
    parsed = urlparse(url)
    return f"{parsed.scheme}://{parsed.netloc}"


def _make_html_converter(source_uri: str) -> DocumentConverter:
    return DocumentConverter(
        format_options={
            InputFormat.HTML: HTMLFormatOption(backend_options=html_backend_options(source_uri)),
        }
    )


def convert(source: Path | str) -> DoclingDocument:
    if isinstance(source, str):
        if Path(source).suffix.lower() == ".pdf":
            logger.debug("converting pdf url", extra={"url": source})
            converter = _pdf_converter
        else:
            origin = _url_origin(source)
            logger.debug("converting html url", extra={"url": source, "origin": origin})
            converter = _make_html_converter(origin)
    else:
        converter = _pdf_converter
    result = converter.convert(source=source)
    return result.document


def convert_pdf_page_range(path: Path, start: int, end: int) -> DoclingDocument:
    """Convert a single page range of a PDF."""
    result = _pdf_converter.convert(source=path, page_range=(start, end))
    return result.document


def pdf_page_batches(page_count: int) -> list[tuple[int, int]]:
    """Return (start, end) page ranges for batching a document of page_count pages."""
    return [
        (start, min(start + PDF_BATCH_SIZE - 1, page_count))
        for start in range(1, page_count + 1, PDF_BATCH_SIZE)
    ]


def convert_pdf_batched(path: Path, page_count: int) -> DoclingDocument:
    """Convert a PDF in page batches of PDF_BATCH_SIZE, concatenating results."""
    docs: list[DoclingDocument] = []
    for start, end in pdf_page_batches(page_count):
        logger.debug(
            "convert pdf batch",
            extra={"path": str(path), "start": start, "end": end, "total": page_count},
        )
        docs.append(convert_pdf_page_range(path, start, end))
    return DoclingDocument.concatenate(docs)


def warmup() -> None:
    """Force Docling model initialization. Blocks until all models are loaded."""
    _pdf_converter.initialize_pipeline(InputFormat.PDF)
