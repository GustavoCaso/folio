from unittest.mock import MagicMock, patch

import pytest
from docling_core.types.doc.base import ImageRefMode

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


def _fake_save_as_markdown(markdown: str = "# Hello"):
    """Return a side_effect for mock_doc.save_as_markdown that writes the markdown file."""

    def _impl(
        filename,
        image_mode,
        image_placeholder: str = "<!-- image -->",
    ):
        filename.write_text(markdown, encoding="utf-8")

    return _impl


@pytest.mark.asyncio
async def test_convert_document_yields_processing_then_done():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    stream = _make_stream("doc.pdf", b"%PDF")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown()

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
    ):
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

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Hello World")

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
    ):
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

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Hi")

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.count_pdf_pages", return_value=7),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
    ):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    stages = [r.status.stage for r in results if r.HasField("status")]
    assert "received" in stages
    assert "loading" in stages
    assert "exporting" in stages
    assert "done" in stages

    loading = next(
        r.status for r in results if r.HasField("status") and r.status.stage == "loading"
    )
    assert loading.pages_total == 7


@pytest.mark.asyncio
async def test_convert_document_logs_request_id_from_meta(caplog):
    import json
    import logging

    from parser.logging_config import JSONFormatter

    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF", request_id="req-xyz")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Hi")

    fmt = JSONFormatter()
    with (
        caplog.at_level(logging.INFO, logger="parser.servicer"),
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
    ):
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


@pytest.mark.asyncio
async def test_convert_document_yields_metadata_on_success():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown()

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("My Title", "Jane", b"\x89PNG")),
    ):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    metadata_msgs = [r for r in results if r.HasField("metadata")]
    assert len(metadata_msgs) == 1
    meta = metadata_msgs[0].metadata
    assert meta.title == "My Title"
    assert meta.author == "Jane"
    assert meta.cover == b"\x89PNG"

    # metadata must arrive before DONE
    metadata_idx = next(i for i, r in enumerate(results) if r.HasField("metadata"))
    done_idx = next(
        i for i, r in enumerate(results) if r.HasField("status") and r.status.status == "DONE"
    )
    assert metadata_idx < done_idx


@pytest.mark.asyncio
async def test_convert_document_no_metadata_on_failure():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF")

    with patch("parser.servicer.convert", side_effect=RuntimeError("boom")):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    metadata_msgs = [r for r in results if r.HasField("metadata")]
    assert len(metadata_msgs) == 0


@pytest.mark.asyncio
async def test_convert_document_yields_image_chunks():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF")

    png_bytes = b"\x89PNG\r\n\x1a\n"
    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Hello\n\n<!-- image -->\n")
    mock_doc.pictures = []

    fake_images = [("image_000000_abc.png", png_bytes)]

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter(fake_images)),
    ):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    image_chunks = [r for r in results if r.HasField("image_chunk")]
    assert len(image_chunks) == 1
    assert image_chunks[0].image_chunk.filename == "image_000000_abc.png"
    assert image_chunks[0].image_chunk.data == png_bytes


@pytest.mark.asyncio
async def test_convert_document_no_image_chunks_when_no_images():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Hello")
    mock_doc.pictures = []

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter([])),
    ):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    image_chunks = [r for r in results if r.HasField("image_chunk")]
    assert len(image_chunks) == 0


@pytest.mark.asyncio
async def test_convert_document_image_chunks_arrive_after_markdown():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Hello\n\n<!-- image -->\n")
    mock_doc.pictures = []

    fake_images = [("image_000000_abc.png", b"\x89PNG")]

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter(fake_images)),
    ):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    last_md_idx = max(i for i, r in enumerate(results) if r.HasField("markdown_chunk"))
    first_img_idx = next(i for i, r in enumerate(results) if r.HasField("image_chunk"))
    assert first_img_idx > last_md_idx


@pytest.mark.asyncio
async def test_pdf_conversion_uses_placeholder_mode_and_extract_images():
    """Servicer must use PLACEHOLDER image mode and extract images via pypdfium2."""

    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Hi\n\n<!-- image -->\n")
    mock_doc.pictures = []

    png = b"\x89PNG\r\n\x1a\n"
    fake_images = [("image_000000_aabbccdd.png", png)]

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter(fake_images)),
    ):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    call_kwargs = mock_doc.save_as_markdown.call_args
    assert call_kwargs.kwargs["image_mode"] == ImageRefMode.PLACEHOLDER

    image_chunks = [r for r in results if r.HasField("image_chunk")]
    assert len(image_chunks) == 1
    assert image_chunks[0].image_chunk.filename == "image_000000_aabbccdd.png"
    assert image_chunks[0].image_chunk.data == png


@pytest.mark.asyncio
async def test_pdf_markdown_has_image_refs_not_placeholders():
    """After rewrite, streamed markdown must contain filename refs, not <!-- image -->."""
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Hi\n\n<!-- image -->\n")
    mock_doc.pictures = []

    fake_images = [("image_000000_aabbccdd.png", b"\x89PNG")]

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter(fake_images)),
    ):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    chunks = [r.markdown_chunk for r in results if r.HasField("markdown_chunk")]
    combined = b"".join(chunks).decode("utf-8")
    assert "<!-- image -->" not in combined
    assert "![Image](image_000000_aabbccdd.png)" in combined


@pytest.mark.asyncio
async def test_convert_html_document_yields_processing_then_done():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    stream = _make_stream("page.html", b"<html></html>")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown()

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("My Title", "Author", b"")),
    ):
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    call_kwargs = mock_doc.save_as_markdown.call_args
    assert call_kwargs.kwargs["image_mode"] == ImageRefMode.REFERENCED

    statuses = [r.status.status for r in results if r.HasField("status")]
    assert "PROCESSING" in statuses
    assert "DONE" in statuses

    metadata_msgs = [r for r in results if r.HasField("metadata")]
    assert len(metadata_msgs) == 1
    assert metadata_msgs[0].metadata.title == "My Title"
    assert metadata_msgs[0].metadata.author == "Author"
