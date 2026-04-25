import asyncio
import contextlib
import logging
import re
from contextlib import contextmanager
from dataclasses import dataclass

_PAGE_RE = re.compile(r"Finished converting pages (\d+)/(\d+)")


@dataclass
class ProgressEvent:
    stage: str
    message: str
    pages_done: int = 0
    pages_total: int = 0


class _ProgressLogHandler(logging.Handler):
    def __init__(self, queue: asyncio.Queue, loop: asyncio.AbstractEventLoop) -> None:
        super().__init__(level=logging.DEBUG)
        self._queue = queue
        self._loop = loop

    def emit(self, record: logging.LogRecord) -> None:
        evt = _parse(record)
        if evt is None:
            return
        with contextlib.suppress(RuntimeError):
            self._loop.call_soon_threadsafe(self._queue.put_nowait, evt)


def _parse(record: logging.LogRecord) -> ProgressEvent | None:
    msg = record.getMessage()
    name = record.name

    if name.startswith("docling"):
        m = _PAGE_RE.search(msg)
        if m:
            done, total = int(m.group(1)), int(m.group(2))
            return ProgressEvent(
                stage="processing",
                message=f"converted page {done}/{total}",
                pages_done=done,
                pages_total=total,
            )
        if "Initializing pipeline" in msg:
            return ProgressEvent(stage="loading", message="initializing pipeline")
        if "Auto OCR model selected" in msg:
            return ProgressEvent(stage="ocr_init", message=msg)
        if "Processing document" in msg:
            return ProgressEvent(stage="processing", message=msg)
    return None


@contextmanager
def attach(queue: asyncio.Queue, loop: asyncio.AbstractEventLoop):
    handler = _ProgressLogHandler(queue, loop)
    docling_logger = logging.getLogger("docling")
    prev_level = docling_logger.level
    docling_logger.setLevel(logging.DEBUG)
    docling_logger.addHandler(handler)
    try:
        yield
    finally:
        docling_logger.removeHandler(handler)
        docling_logger.setLevel(prev_level)
