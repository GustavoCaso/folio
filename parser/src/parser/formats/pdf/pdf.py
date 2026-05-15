import os
from pathlib import Path

import pypdfium2
from docling.datamodel.base_models import InputFormat
from docling.datamodel.pipeline_options import CodeFormulaVlmOptions, PdfPipelineOptions
from docling.document_converter import DocumentConverter, PdfFormatOption
from docling_core.types.doc.base import ImageRefMode
from docling_core.types.doc.document import DoclingDocument


def _bool_env(var: str, default: bool) -> bool:
    val = os.environ.get(var)
    if val is None:
        return default
    return val.lower() in ("1", "true", "yes")


def _image_mode() -> ImageRefMode:
    val = os.environ.get("PDF_IMAGE_MODE", "placeholder").lower()
    return {"embedded": ImageRefMode.EMBEDDED, "referenced": ImageRefMode.REFERENCED}.get(
        val, ImageRefMode.PLACEHOLDER
    )


_VALID_CODE_FORMULA_PRESETS = {"codeformulav2", "granite_docling"}


def _code_formula_options() -> CodeFormulaVlmOptions:
    preset = os.environ.get("PDF_CODE_FORMULA_PRESET", "codeformulav2").lower()
    if preset not in _VALID_CODE_FORMULA_PRESETS:
        preset = "codeformulav2"
    result: CodeFormulaVlmOptions = CodeFormulaVlmOptions.from_preset(preset)
    return result


def _pipeline_options() -> PdfPipelineOptions:
    kwargs: dict[str, object] = {
        "generate_picture_images": _bool_env("PDF_GENERATE_IMAGES", True),
        "do_ocr": _bool_env("PDF_DO_OCR", True),
        "do_table_structure": _bool_env("PDF_DO_TABLE_STRUCTURE", True),
        "do_code_enrichment": _bool_env("PDF_DO_CODE_ENRICHMENT", False),
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
    return PdfPipelineOptions(**kwargs)  # type: ignore[arg-type]


_converter: DocumentConverter = DocumentConverter(
    format_options={
        InputFormat.PDF: PdfFormatOption(pipeline_options=_pipeline_options()),
    }
)

image_mode_value: ImageRefMode = _image_mode()

post_process_code_blocks: bool = _bool_env("PDF_POST_PROCESS_CODE_BLOCKS", True)


def convert_pdf(path: Path) -> DoclingDocument:
    """Convert a PDF file using Docling. Returns the DoclingDocument."""
    result = _converter.convert(str(path))
    return result.document


def count_pdf_pages(path: Path) -> int:
    pdf = pypdfium2.PdfDocument(str(path))
    try:
        return len(pdf)
    finally:
        pdf.close()


def warmup() -> None:
    """Force Docling model initialization. Blocks until all models are loaded."""
    _converter.initialize_pipeline(InputFormat.PDF)
