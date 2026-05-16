import os
from pathlib import Path

from docling.datamodel.backend_options import HTMLBackendOptions
from docling.datamodel.base_models import InputFormat
from docling.document_converter import DocumentConverter, HTMLFormatOption
from docling_core.types.doc.base import ImageRefMode
from docling_core.types.doc.document import DoclingDocument


def _bool_env(var: str, default: bool) -> bool:
    val = os.environ.get(var)
    if val is None:
        return default
    return val.lower() in ("1", "true", "yes")


def _image_mode() -> ImageRefMode:
    val = os.environ.get("HTML_IMAGE_MODE", "placeholder").lower()
    return {"embedded": ImageRefMode.EMBEDDED, "referenced": ImageRefMode.REFERENCED}.get(
        val, ImageRefMode.PLACEHOLDER
    )


def _backend_options() -> HTMLBackendOptions:
    return HTMLBackendOptions(
        fetch_images=_bool_env("HTML_FETCH_IMAGES", False),
    )  # type: ignore[call-arg]


_converter: DocumentConverter = DocumentConverter(
    format_options={
        InputFormat.HTML: HTMLFormatOption(backend_options=_backend_options()),
    }
)

image_mode_value: ImageRefMode = _image_mode()

post_process_code_blocks: bool = _bool_env("HTML_POST_PROCESS_CODE_BLOCKS", True)


def convert_html(path: Path) -> DoclingDocument:
    """Convert an HTML file using Docling. Returns the DoclingDocument."""
    result = _converter.convert(str(path))
    return result.document
