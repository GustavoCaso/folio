import logging
import os
import tempfile
from collections.abc import Callable
from pathlib import Path
from types import MappingProxyType

from fastapi import FastAPI, File, HTTPException, UploadFile
from pydantic import BaseModel

from pdf_to_md.converter import UnsupportedFormatError, _get_handlers

logger = logging.getLogger("pdf_to_md")

app = FastAPI(title="pdf-to-md", description="Convert documents to Markdown via HTTP")


class ConvertResponse(BaseModel):
    filename: str
    markdown: str


def convert_bytes(filename: str, content: bytes) -> str:
    """Write content to a temp file, convert, return markdown string."""
    handlers: MappingProxyType[str, Callable[[Path], str]] = _get_handlers()
    suffix = Path(filename).suffix.lower()
    handler = handlers.get(suffix)
    if handler is None:
        raise UnsupportedFormatError(
            f"No handler for format '{suffix}'. Supported: {list(handlers)}"
        )
    with tempfile.NamedTemporaryFile(suffix=suffix, delete=False, delete_on_close=False) as f:
        tmp_path = Path(f.name)
    try:
        tmp_path.write_bytes(content)
        return handler(tmp_path)
    finally:
        tmp_path.unlink(missing_ok=True)


@app.post("/convert", response_model=ConvertResponse)
async def convert_endpoint(file: UploadFile = File(...)) -> ConvertResponse:
    filename = file.filename or ""
    logger.info("Received file: %s", filename)
    content = await file.read()
    try:
        markdown = convert_bytes(filename, content)
    except UnsupportedFormatError as e:
        logger.warning("Unsupported format: %s", e)
        raise HTTPException(status_code=422, detail=str(e))
    logger.info("Converted: %s", filename)
    return ConvertResponse(filename=filename, markdown=markdown)


def serve() -> None:
    import uvicorn

    log_level = os.environ.get("LOG_LEVEL", "INFO").upper()
    logging.basicConfig(
        level=getattr(logging, log_level, logging.INFO),
        format="%(levelname)s: %(message)s",
    )
    uvicorn.run(app, host="0.0.0.0", port=8000)
