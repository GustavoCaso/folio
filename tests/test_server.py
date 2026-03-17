import io
import logging
from unittest.mock import patch

import pytest
from httpx import ASGITransport, AsyncClient

from pdf_to_md.server import app

MOCK_HANDLERS = {".pdf": lambda p: "# Hello\n\nContent"}


@pytest.mark.asyncio
async def test_convert_endpoint_returns_markdown() -> None:
    fake_pdf = io.BytesIO(b"%PDF-1.4 fake content")

    with patch("pdf_to_md.server.convert_bytes", return_value="# Hello\n\nContent"):
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            response = await client.post(
                "/convert",
                files={"file": ("book.pdf", fake_pdf, "application/pdf")},
            )

    assert response.status_code == 200
    assert response.json()["markdown"] == "# Hello\n\nContent"
    assert response.json()["filename"] == "book.pdf"


@pytest.mark.asyncio
async def test_convert_endpoint_unsupported_format() -> None:
    fake_file = io.BytesIO(b"fake epub content")

    with patch("pdf_to_md.server._get_handlers", return_value=MOCK_HANDLERS):
        async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as client:
            response = await client.post(
                "/convert",
                files={"file": ("book.epub", fake_file, "application/epub+zip")},
            )

    assert response.status_code == 422
    assert (
        "epub" in response.json()["detail"].lower()
        or "handler" in response.json()["detail"].lower()
    )


@pytest.mark.asyncio
async def test_server_logs_info_on_successful_convert(caplog: pytest.LogCaptureFixture) -> None:
    fake_pdf = io.BytesIO(b"%PDF-1.4 fake content")

    with patch("pdf_to_md.server.convert_bytes", return_value="# Hello\n\nContent"):
        with caplog.at_level(logging.INFO, logger="pdf_to_md"):
            async with AsyncClient(
                transport=ASGITransport(app=app), base_url="http://test"
            ) as client:
                response = await client.post(
                    "/convert",
                    files={"file": ("book.pdf", fake_pdf, "application/pdf")},
                )

    assert response.status_code == 200
    assert any("book.pdf" in r.message for r in caplog.records if r.levelno == logging.INFO)


@pytest.mark.asyncio
async def test_server_logs_warning_on_unsupported_format(caplog: pytest.LogCaptureFixture) -> None:
    fake_file = io.BytesIO(b"fake epub content")

    with patch("pdf_to_md.server._get_handlers", return_value=MOCK_HANDLERS):
        with caplog.at_level(logging.WARNING, logger="pdf_to_md"):
            async with AsyncClient(
                transport=ASGITransport(app=app), base_url="http://test"
            ) as client:
                response = await client.post(
                    "/convert",
                    files={"file": ("book.epub", fake_file, "application/epub+zip")},
                )

    assert response.status_code == 422
    assert any(r.levelno == logging.WARNING and "epub" in r.message.lower() for r in caplog.records)
