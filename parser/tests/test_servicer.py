from unittest.mock import MagicMock, patch

import pytest

from parser.grpc import parser_pb2
from parser.servicer import ParserServicer


def _make_stream(filename: str, data: bytes, request_id: str = ""):
    """Build an async iterator simulating a gRPC request stream."""
    chunks = [
        parser_pb2.ConvertChunk(
            meta=parser_pb2.ConvertMeta(filename=filename, request_id=request_id)
        ),
        parser_pb2.ConvertChunk(data=data),
    ]

    async def _iter():
        for chunk in chunks:
            yield chunk

    return _iter()


@pytest.mark.asyncio
async def test_convert_document_yields_processing_then_done():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    stream = _make_stream("doc.pdf", b"%PDF")

    with patch("parser.servicer.convert", return_value="# Hello"):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    statuses = [r.status.status for r in results if r.HasField("status")]
    assert "PROCESSING" in statuses
    assert "DONE" in statuses


@pytest.mark.asyncio
async def test_convert_document_yields_markdown_chunk():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    stream = _make_stream("doc.pdf", b"%PDF")

    with patch("parser.servicer.convert", return_value="# Hello World"):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    chunks = [r.markdown_chunk for r in results if r.HasField("markdown_chunk")]
    combined = b"".join(chunks).decode("utf-8")
    assert combined == "# Hello World"


@pytest.mark.asyncio
async def test_convert_document_emits_stage_transitions():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    stream = _make_stream("doc.pdf", b"%PDF")

    with patch("parser.servicer.convert", return_value="# Hi"), \
         patch("parser.servicer.count_pdf_pages", return_value=7):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    stages = [r.status.stage for r in results if r.HasField("status")]
    assert "received" in stages
    assert "loading" in stages
    assert "exporting" in stages
    assert "done" in stages

    loading = next(r.status for r in results if r.HasField("status") and r.status.stage == "loading")
    assert loading.pages_total == 7


@pytest.mark.asyncio
async def test_convert_document_logs_request_id_from_meta(caplog):
    import json
    import logging

    from parser.logging_config import JSONFormatter

    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF", request_id="req-xyz")

    fmt = JSONFormatter()
    with caplog.at_level(logging.INFO, logger="parser.servicer"):
        with patch("parser.servicer.convert", return_value="# Hi"):
            async for _ in servicer.ConvertDocument(stream, context):
                pass

    matching = [
        json.loads(fmt.format(rec))
        for rec in caplog.records
        if rec.name == "parser.servicer" and getattr(rec, "request_id", None) == "req-xyz"
    ]
    assert matching, "expected at least one log with request_id=req-xyz"
    assert any(m["msg"] == "convert received" for m in matching)
    assert any(m["msg"] == "convert done" for m in matching)


@pytest.mark.asyncio
async def test_convert_document_yields_failed_on_error():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    stream = _make_stream("doc.pdf", b"%PDF")

    with patch("parser.servicer.convert", side_effect=RuntimeError("boom")):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    statuses = {r.status.status for r in results if r.HasField("status")}
    assert "FAILED" in statuses
