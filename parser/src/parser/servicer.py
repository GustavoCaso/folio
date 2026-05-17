from __future__ import annotations

import asyncio
import logging
import tempfile
import time
import uuid
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import AsyncGenerator, AsyncIterator

import grpc

from parser.converter import convert
from parser.formats.helpers import extract_metadata, get_format_settings
from parser.formats.pdf import count_pdf_pages
from parser.grpc import parser_pb2, parser_pb2_grpc
from parser.postprocess import enrich_code_blocks
from parser.progress import ProgressEvent, attach

logger = logging.getLogger(__name__)

_CHUNK_SIZE = 64 * 1024  # 64 KB per markdown chunk
_DRAIN_POLL_INTERVAL = 0.5  # seconds


class ParserServicer(parser_pb2_grpc.ParserServiceServicer):  # type: ignore[misc]
    """Stateless gRPC servicer. No state is retained between requests."""

    def __init__(self, num_workers: int = 2) -> None:
        self._executor = ThreadPoolExecutor(max_workers=num_workers)

    async def ConvertDocument(  # noqa: N802
        self,
        request_iterator: AsyncIterator[parser_pb2.ConvertChunk],
        context: grpc.aio.ServicerContext[parser_pb2.ConvertChunk, parser_pb2.ConvertResult],
    ) -> AsyncGenerator[parser_pb2.ConvertResult, None]:
        # Per-RPC correlation id so every log line for a single conversion
        # shares the same job_id field.
        job_id = uuid.uuid4().hex[:8]
        started = time.monotonic()

        # --- Receive all chunks from the caller ---
        meta = None
        buf = bytearray()

        async for chunk in request_iterator:
            kind = chunk.WhichOneof("payload")
            if kind == "meta":
                meta = chunk.meta
            elif kind == "data":
                buf.extend(chunk.data)

        if meta is None:
            logger.warning("missing ConvertMeta", extra={"job_id": job_id})
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Missing ConvertMeta")
            return

        ctx: dict[str, object] = {
            "job_id": job_id,
            "file": meta.filename,
            "bytes": len(buf),
        }
        if meta.request_id:
            ctx["request_id"] = meta.request_id
        logger.info("convert received", extra=ctx)

        suffix = Path(meta.filename).suffix.lower() or ".pdf"

        yield parser_pb2.ConvertResult(
            status=parser_pb2.StatusUpdate(
                status="PROCESSING",
                stage="received",
                message=f"received {len(buf)} bytes",
            )
        )

        # --- Write to temp file and convert (blocking Docling call in executor) ---
        tmp_path: Path | None = None
        try:
            with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as f:
                f.write(buf)
                tmp_path = Path(f.name)

            pages_total = 0
            if suffix == ".pdf":
                try:
                    pages_total = count_pdf_pages(tmp_path)
                except Exception:
                    logger.exception(
                        "count pages failed",
                        extra={**ctx, "tmp_path": str(tmp_path)},
                    )

            logger.info("convert loading", extra={**ctx, "pages_total": pages_total})

            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(
                    status="PROCESSING",
                    stage="loading",
                    message=f"loading document ({pages_total} pages)",
                    pages_total=pages_total,
                )
            )

            loop = asyncio.get_running_loop()
            progress_q: asyncio.Queue[ProgressEvent] = asyncio.Queue()

            with attach(progress_q, loop):
                convert_task = loop.run_in_executor(self._executor, convert, tmp_path)

                while not convert_task.done():
                    try:
                        evt = await asyncio.wait_for(progress_q.get(), timeout=_DRAIN_POLL_INTERVAL)
                    except TimeoutError:
                        continue
                    logger.debug(
                        "convert progress",
                        extra={
                            **ctx,
                            "stage": evt.stage,
                            "pages_done": evt.pages_done,
                            "pages_total": pages_total,
                        },
                    )
                    yield parser_pb2.ConvertResult(
                        status=parser_pb2.StatusUpdate(
                            status="PROCESSING",
                            stage=evt.stage,
                            message=evt.message,
                            pages_done=evt.pages_done,
                            pages_total=pages_total,
                        )
                    )

                # Drain any events queued just before completion
                while not progress_q.empty():
                    evt = progress_q.get_nowait()
                    yield parser_pb2.ConvertResult(
                        status=parser_pb2.StatusUpdate(
                            status="PROCESSING",
                            stage=evt.stage,
                            message=evt.message,
                            pages_done=evt.pages_done,
                            pages_total=pages_total,
                        )
                    )

                doc = await convert_task

            image_mode, do_post_process = get_format_settings(suffix)
            markdown = doc.export_to_markdown(image_mode=image_mode)
            if do_post_process:
                markdown = enrich_code_blocks(markdown)

            encoded = markdown.encode("utf-8")
            logger.info(
                "convert exporting",
                extra={**ctx, "md_bytes": len(encoded), "pages_total": pages_total},
            )

            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(
                    status="PROCESSING",
                    stage="exporting",
                    message="streaming markdown",
                    pages_done=pages_total,
                    pages_total=pages_total,
                )
            )

            # --- Stream markdown back in chunks ---
            md_chunks = 0
            for i in range(0, len(encoded), _CHUNK_SIZE):
                yield parser_pb2.ConvertResult(markdown_chunk=encoded[i : i + _CHUNK_SIZE])
                md_chunks += 1

            title, author, cover = extract_metadata(tmp_path, suffix, generate_cover=True)
            yield parser_pb2.ConvertResult(
                metadata=parser_pb2.DocumentMetadata(
                    title=title,
                    author=author,
                    cover=cover,
                )
            )

            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(
                    status="DONE",
                    stage="done",
                    pages_done=pages_total,
                    pages_total=pages_total,
                )
            )
            logger.info(
                "convert done",
                extra={
                    **ctx,
                    "pages_total": pages_total,
                    "md_bytes": len(encoded),
                    "md_chunks": md_chunks,
                    "dur_ms": int((time.monotonic() - started) * 1000),
                },
            )

        except Exception as exc:
            logger.exception("convert failed", extra=ctx)
            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(
                    status="FAILED",
                    error=str(exc),
                )
            )
        finally:
            if tmp_path is not None:
                tmp_path.unlink(missing_ok=True)

    def shutdown(self) -> None:
        self._executor.shutdown(wait=False)
