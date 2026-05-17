import importlib
from collections.abc import Callable
from pathlib import Path

from docling_core.types.doc.document import DoclingDocument


class UnsupportedFormatError(Exception):
    pass


_HANDLERS = {
    ".pdf": "parser.formats.pdf:convert_pdf",
    ".html": "parser.formats.html:convert_html",
    ".htm": "parser.formats.html:convert_html",
}


def _get_handler(path: Path) -> Callable[[Path], DoclingDocument]:
    ext = path.suffix.lower()
    handler_ref = _HANDLERS.get(ext)
    if handler_ref is None:
        raise UnsupportedFormatError(f"No handler for extension: {ext}")
    module_path, func_name = handler_ref.split(":")
    module = importlib.import_module(module_path)
    handler: Callable[[Path], DoclingDocument] = getattr(module, func_name)
    return handler


def convert(path: Path) -> DoclingDocument:
    """Convert a document at the given path. Returns the DoclingDocument."""
    handler = _get_handler(path)
    return handler(path)
