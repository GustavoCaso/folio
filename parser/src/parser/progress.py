import asyncio
import contextlib
import logging
import re
from collections.abc import Iterator
from contextlib import contextmanager
from dataclasses import dataclass

_STAGE_ASSEMBLE = re.compile(r"Stage assemble: run_id=\d+ pages=\[(\d+)\] .*")


@dataclass
class ProgressEvent:
    stage: str
    message: str
    pages_done: int = 0


class _ProgressLogHandler(logging.Handler):
    def __init__(
        self, queue: asyncio.Queue[ProgressEvent], loop: asyncio.AbstractEventLoop
    ) -> None:
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
        if "Stage assemble" in msg:
            m = _STAGE_ASSEMBLE.search(msg)
            if m:
                done = int(m.group(1))
                return ProgressEvent(
                    stage="processing",
                    message=f"converted page {done}",
                    pages_done=done,
                )
        if "Initializing pipeline" in msg:
            return ProgressEvent(stage="loading", message="initializing pipeline")
        if "Auto OCR model selected" in msg:
            return ProgressEvent(stage="ocr_init", message=msg)
        if "Processing document" in msg:
            return ProgressEvent(stage="processing", message=msg)
    return None


@contextmanager
def attach(queue: asyncio.Queue[ProgressEvent], loop: asyncio.AbstractEventLoop) -> Iterator[None]:
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
