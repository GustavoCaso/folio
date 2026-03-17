from pathlib import Path

from docling.datamodel.base_models import InputFormat
from docling.datamodel.pipeline_options import (
    CodeFormulaVlmOptions,
    PdfPipelineOptions,
)
from docling.document_converter import DocumentConverter, PdfFormatOption

_converter: DocumentConverter = DocumentConverter(
    format_options={
        InputFormat.PDF: PdfFormatOption(
            pipeline_options=PdfPipelineOptions(
                do_code_enrichment=True,
                do_formula_enrichment=True,
                code_formula_options=CodeFormulaVlmOptions.from_preset("codeformulav2"),
            )
        )
    }
)


def convert(input_path: Path) -> str:
    """Convert a PDF file to a Markdown string using docling."""
    result = _converter.convert(str(input_path))
    return result.document.export_to_markdown()
