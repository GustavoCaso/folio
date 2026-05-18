import os
from pathlib import Path

from docling.datamodel.backend_options import HTMLBackendOptions
from docling.datamodel.base_models import InputFormat
from docling.document_converter import DocumentConverter, HTMLFormatOption
from docling_core.types.doc.base import ImageRefMode
from docling_core.types.doc.document import DoclingDocument

from parser.formats.helpers import bool_env
from parser.formats.helpers import image_mode as _image_mode_from_env


def _backend_options() -> HTMLBackendOptions:
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
        InputFormat.HTML: HTMLFormatOption(backend_options=_backend_options()),
    }
)

image_mode_value: ImageRefMode = _image_mode_from_env("HTML_IMAGE_MODE")

post_process_code_blocks: bool = bool_env("HTML_POST_PROCESS_CODE_BLOCKS", True)


def convert_html(path: Path) -> DoclingDocument:
    """Convert an HTML file using Docling. Returns the DoclingDocument."""
    result = _converter.convert(str(path))
    return result.document
