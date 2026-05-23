from pathlib import Path

import pypdfium2

from parser.formats.helpers import bool_env

post_process_code_blocks: bool = bool_env("PDF_POST_PROCESS_CODE_BLOCKS", True)


def count_pdf_pages(path: Path) -> int:
    pdf = pypdfium2.PdfDocument(str(path))
    try:
        return len(pdf)
    finally:
        pdf.close()
