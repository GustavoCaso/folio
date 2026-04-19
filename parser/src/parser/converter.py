import importlib
from pathlib import Path


class UnsupportedFormatError(Exception):
    pass


_HANDLERS = {
    ".pdf": "parser.formats.pdf:convert_pdf",
}


def _get_handler(path: Path):  # type: ignore[return]
    ext = path.suffix.lower()
    handler_ref = _HANDLERS.get(ext)
    if handler_ref is None:
        raise UnsupportedFormatError(f"No handler for extension: {ext}")
    module_path, func_name = handler_ref.split(":")
    module = importlib.import_module(module_path)
    return getattr(module, func_name)


def convert(path: Path) -> str:
    """Convert a document at the given path to Markdown. Returns the Markdown string."""
    handler = _get_handler(path)
    return handler(path)
