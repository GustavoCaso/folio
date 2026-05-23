import os
from pathlib import Path

from docling.datamodel.accelerator_options import AcceleratorOptions
from docling.datamodel.backend_options import HTMLBackendOptions
from docling.datamodel.base_models import InputFormat
from docling.datamodel.pipeline_options import (
    CodeFormulaVlmOptions,
    PdfPipelineOptions,
)
from docling.document_converter import DocumentConverter, HTMLFormatOption, PdfFormatOption
from docling_core.types.doc.document import DoclingDocument

from parser.formats.helpers import bool_env

_VALID_CODE_FORMULA_PRESETS = {"codeformulav2", "granite_docling"}


def _code_formula_options() -> CodeFormulaVlmOptions:
    preset = os.environ.get("PDF_CODE_FORMULA_PRESET", "codeformulav2").lower()
    if preset not in _VALID_CODE_FORMULA_PRESETS:
        preset = "codeformulav2"
    result: CodeFormulaVlmOptions = CodeFormulaVlmOptions.from_preset(preset)
    return result


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


def html_backend_options() -> HTMLBackendOptions:
    opts = HTMLBackendOptions(  # type: ignore[call-arg]
        fetch_images=bool_env("HTML_FETCH_IMAGES", False),
        render_page=bool_env("HTML_RENDER_PAGE", False),
        add_title=bool_env("HTML_ADD_TITLE", True),
        infer_furniture=bool_env("HTML_INFER_FURNITURE", True),
    )
    render_dpi = os.environ.get("HTML_RENDER_DPI")
    if render_dpi is not None:
        opts.render_dpi = int(render_dpi)
    render_device_scale = os.environ.get("HTML_RENDER_DEVICE_SCALE")
    if render_device_scale is not None:
        opts.render_device_scale = float(render_device_scale)
    return opts


_converter: DocumentConverter = DocumentConverter(
    format_options={
        InputFormat.PDF: PdfFormatOption(pipeline_options=pdf_pipeline_options()),
        InputFormat.HTML: HTMLFormatOption(backend_options=html_backend_options()),
    }
)


def convert(source: Path | str) -> DoclingDocument:
    result = _converter.convert(source=source)
    return result.document


def warmup() -> None:
    """Force Docling model initialization. Blocks until all models are loaded."""
    _converter.initialize_pipeline(InputFormat.PDF)
