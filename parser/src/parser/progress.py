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
    message: str


class _ProgressLogHandler(logging.Handler):
    def __init__(
        self, pages_total: int, queue: asyncio.Queue[ProgressEvent], loop: asyncio.AbstractEventLoop
    ) -> None:
        super().__init__(level=logging.DEBUG)
        self._pages_total = pages_total
        self._queue = queue
        self._loop = loop

    def emit(self, record: logging.LogRecord) -> None:
        evt = self._parse(record)
        if evt is None:
            return
        with contextlib.suppress(RuntimeError):
            self._loop.call_soon_threadsafe(self._queue.put_nowait, evt)

    def _parse(self, record: logging.LogRecord) -> ProgressEvent | None:
        msg = record.getMessage()
        name = record.name

        if name.startswith("docling") and "Stage assemble" in msg:
            m = _STAGE_ASSEMBLE.search(msg)
            if m:
                done = int(m.group(1))
                return ProgressEvent(message=f"converted page {done}/{self._pages_total}")
        return None


@contextmanager
def attach(
    pages_total: int, queue: asyncio.Queue[ProgressEvent], loop: asyncio.AbstractEventLoop
) -> Iterator[None]:
    handler = _ProgressLogHandler(pages_total, queue, loop)
    docling_logger = logging.getLogger("docling")
    prev_level = docling_logger.level
    docling_logger.setLevel(logging.DEBUG)
    docling_logger.addHandler(handler)
    try:
        yield
    finally:
        docling_logger.removeHandler(handler)
        docling_logger.setLevel(prev_level)
