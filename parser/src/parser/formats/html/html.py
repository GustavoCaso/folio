from pathlib import Path

from docling.datamodel.backend_options import HTMLBackendOptions
from docling.datamodel.base_models import InputFormat
from docling.document_converter import DocumentConverter, HTMLFormatOption
from docling_core.types.doc.base import ImageRefMode
from docling_core.types.doc.document import DoclingDocument

from parser.formats.helpers import bool_env
from parser.formats.helpers import image_mode as _image_mode_from_env


def _backend_options() -> HTMLBackendOptions:
    return HTMLBackendOptions(
        fetch_images=bool_env("HTML_FETCH_IMAGES", False),
    )  # type: ignore[call-arg]


_converter: DocumentConverter = DocumentConverter(
    format_options={
        InputFormat.HTML: HTMLFormatOption(backend_options=_backend_options()),
    }
)

image_mode_value: ImageRefMode = _image_mode_from_env("HTML_IMAGE_MODE")

post_process_code_blocks: bool = bool_env("HTML_POST_PROCESS_CODE_BLOCKS", True)


def convert_html(path: Path) -> DoclingDocument:
    """Convert an HTML file using Docling. Returns the DoclingDocument."""
    result = _converter.convert(str(path))
    return result.document
