import asyncio
import logging

import pytest

from parser import progress


def _record(name: str, msg: str, level: int = logging.DEBUG) -> logging.LogRecord:
    return logging.LogRecord(
        name=name,
        level=level,
        pathname=__file__,
        lineno=1,
        msg=msg,
        args=None,
        exc_info=None,
    )


def _handler(pages_total: int = 10) -> progress._ProgressLogHandler:
    loop = asyncio.new_event_loop()
    queue: asyncio.Queue[progress.ProgressEvent] = asyncio.Queue()
    return progress._ProgressLogHandler(pages_total, queue, loop)


def test_parse_finished_pages():
    h = _handler(pages_total=10)
    evt = h._parse(
        _record("docling.pipeline.base_pipeline", "Stage assemble: run_id=1 pages=[3] time=1.234")
    )
    assert evt is not None
    assert evt.message == "converted page 3/10"


def test_parse_unrelated_logger_returns_none():
    h = _handler()
    assert h._parse(_record("urllib3.connectionpool", "anything")) is None


def test_parse_docling_unmatched_returns_none():
    h = _handler()
    assert h._parse(_record("docling.foo", "some random message")) is None


@pytest.mark.asyncio
async def test_attach_emits_events_to_queue():
    loop = asyncio.get_running_loop()
    queue: asyncio.Queue[progress.ProgressEvent] = asyncio.Queue()

    with progress.attach(10, queue, loop):
        logging.getLogger("docling.pipeline.base_pipeline").debug(
            "Stage assemble: run_id=1 pages=[5] time=2.0"
        )
        evt = await asyncio.wait_for(queue.get(), timeout=1.0)

    assert evt.message == "converted page 5/10"


@pytest.mark.asyncio
async def test_attach_removes_handler_on_exit():
    loop = asyncio.get_running_loop()
    queue: asyncio.Queue[progress.ProgressEvent] = asyncio.Queue()
    docling_logger = logging.getLogger("docling")
    handlers_before = list(docling_logger.handlers)

    with progress.attach(0, queue, loop):
        assert len(docling_logger.handlers) == len(handlers_before) + 1

    assert docling_logger.handlers == handlers_before
