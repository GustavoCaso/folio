from unittest.mock import MagicMock, patch

import pytest
from docling_core.types.doc.base import ImageRefMode
from docling_core.types.doc.document import DoclingDocument

from parser.grpc import parser_pb2
from parser.servicer import ParserServicer


def _make_stream(filename: str, data: bytes, request_id: str = ""):
    """Build an async iterator simulating a gRPC ConvertDocument request stream."""
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


def _make_url_request(url: str, request_id: str = "") -> parser_pb2.ConvertURLRequest:
    """Build a ConvertURLRequest for ConvertURL RPC."""
    return parser_pb2.ConvertURLRequest(url=url, request_id=request_id)


def _fake_save_as_markdown(markdown: str = "# Hello"):
    """Return a side_effect for mock_doc.save_as_markdown that writes the markdown file."""

    def _impl(filename, image_mode, image_placeholder: str = "<!-- image -->"):
        filename.write_text(markdown, encoding="utf-8")

    return _impl


# --- ConvertDocument (file upload) tests ---


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
async def test_convert_document_emits_messages():
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

    messages = [r.status.message for r in results if r.HasField("status")]
    assert any("received" in m for m in messages)
    assert any("loading document (7 pages)" in m for m in messages)


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


# --- Batched conversion tests ---


@pytest.mark.asyncio
async def test_batched_path_used_when_pages_exceed_batch_size():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF")

    mock_doc = MagicMock(spec=DoclingDocument)
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Batched")

    with (
        patch("parser.servicer.count_pdf_pages", return_value=150),
        patch("parser.servicer.PDF_BATCH_SIZE", 100),
        patch(
            "parser.servicer.pdf_page_batches", return_value=[(1, 100), (101, 150)]
        ) as mock_batches,
        patch("parser.servicer.convert_pdf_page_range", return_value=mock_doc) as mock_range,
        patch("parser.servicer.DoclingDocument") as mock_cls,
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter([])),
    ):
        mock_cls.concatenate.return_value = mock_doc
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    mock_batches.assert_called_once_with(150)
    assert mock_range.call_count == 2


@pytest.mark.asyncio
async def test_batched_path_yields_progress_per_batch():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF")

    mock_doc = MagicMock(spec=DoclingDocument)
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Batched")

    with (
        patch("parser.servicer.count_pdf_pages", return_value=150),
        patch("parser.servicer.PDF_BATCH_SIZE", 100),
        patch("parser.servicer.pdf_page_batches", return_value=[(1, 100), (101, 150)]),
        patch("parser.servicer.convert_pdf_page_range", return_value=mock_doc),
        patch("parser.servicer.DoclingDocument") as mock_cls,
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter([])),
    ):
        mock_cls.concatenate.return_value = mock_doc
        results = []
        async for result in servicer.ConvertDocument(stream, context):
            results.append(result)

    converting_msgs = [
        r.status.message
        for r in results
        if r.HasField("status") and "converting batch" in r.status.message
    ]
    assert len(converting_msgs) == 2
    assert "1" in converting_msgs[0] and "100" in converting_msgs[0]
    assert "101" in converting_msgs[1] and "150" in converting_msgs[1]


@pytest.mark.asyncio
async def test_single_path_used_when_pages_within_batch_size():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()
    stream = _make_stream("doc.pdf", b"%PDF")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Small")

    with (
        patch("parser.servicer.count_pdf_pages", return_value=50),
        patch("parser.servicer.PDF_BATCH_SIZE", 100),
        patch("parser.servicer.convert", return_value=mock_doc) as mock_convert,
        patch("parser.servicer.convert_pdf_page_range") as mock_range,
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
    ):
        async for _ in servicer.ConvertDocument(stream, context):
            pass

    mock_convert.assert_called_once()
    mock_range.assert_not_called()


# --- ConvertURL tests ---


@pytest.mark.asyncio
async def test_convert_url_yields_processing_then_done():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    request = _make_url_request("https://example.com/doc.pdf")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# From URL")

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter([])),
    ):
        results = []
        async for result in servicer.ConvertURL(request, context):
            results.append(result)

    statuses = [r.status.status for r in results if r.HasField("status")]
    assert "PROCESSING" in statuses
    assert "DONE" in statuses


@pytest.mark.asyncio
async def test_convert_url_passes_url_to_convert():
    """convert must be called with the URL string, not a file path."""
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    url = "https://example.com/paper.pdf"
    request = _make_url_request(url)

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# URL doc")

    with (
        patch("parser.servicer.convert", return_value=mock_doc) as mock_convert,
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter([])),
    ):
        async for _ in servicer.ConvertURL(request, context):
            pass

    mock_convert.assert_called_once_with(url)


@pytest.mark.asyncio
async def test_convert_url_uses_placeholder_mode_for_pdf():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    request = _make_url_request("https://example.com/paper.pdf")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Hi\n\n<!-- image -->\n")

    fake_images = [("image_000000_aabb.png", b"\x89PNG")]

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter(fake_images)),
    ):
        results = []
        async for result in servicer.ConvertURL(request, context):
            results.append(result)

    call_kwargs = mock_doc.save_as_markdown.call_args
    assert call_kwargs.kwargs["image_mode"] == ImageRefMode.PLACEHOLDER


@pytest.mark.asyncio
async def test_convert_url_html_uses_referenced_mode():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    request = _make_url_request("https://example.com/page.html")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# HTML from URL")

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
    ):
        results = []
        async for result in servicer.ConvertURL(request, context):
            results.append(result)

    call_kwargs = mock_doc.save_as_markdown.call_args
    assert call_kwargs.kwargs["image_mode"] == ImageRefMode.REFERENCED


@pytest.mark.asyncio
async def test_convert_url_yields_failed_on_error():
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    request = _make_url_request("https://example.com/doc.pdf")

    with patch("parser.servicer.convert", side_effect=RuntimeError("network error")):
        results = []
        async for result in servicer.ConvertURL(request, context):
            results.append(result)

    statuses = {r.status.status for r in results if r.HasField("status")}
    assert "FAILED" in statuses


@pytest.mark.asyncio
async def test_convert_url_loading_status_message():
    """URL conversions emit a loading message."""
    servicer = ParserServicer(num_workers=1)
    context = MagicMock()

    request = _make_url_request("https://example.com/doc.pdf")

    mock_doc = MagicMock()
    mock_doc.save_as_markdown.side_effect = _fake_save_as_markdown("# Hi")

    with (
        patch("parser.servicer.convert", return_value=mock_doc),
        patch("parser.servicer.extract_metadata", return_value=("", "", b"")),
        patch("parser.servicer.extract_images", return_value=iter([])),
    ):
        results = []
        async for result in servicer.ConvertURL(request, context):
            results.append(result)

    messages = [r.status.message for r in results if r.HasField("status")]
    assert any("fetching" in m or "loading" in m for m in messages)
