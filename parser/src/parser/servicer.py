import asyncio
import logging
import tempfile
from collections.abc import AsyncIterator
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import grpc

from parser import progress
from parser.converter import convert
from parser.formats.pdf import count_pdf_pages
from parser.grpc import parser_pb2, parser_pb2_grpc

logger = logging.getLogger(__name__)

_CHUNK_SIZE = 64 * 1024  # 64 KB per markdown chunk
_DRAIN_POLL_INTERVAL = 0.5  # seconds


class ParserServicer(parser_pb2_grpc.ParserServiceServicer):
    """Stateless gRPC servicer. No state is retained between requests."""

    def __init__(self, num_workers: int = 2) -> None:
        self._executor = ThreadPoolExecutor(max_workers=num_workers)

    async def ConvertDocument(  # noqa: N802
        self,
        request_iterator: AsyncIterator[parser_pb2.ConvertChunk],
        context: grpc.aio.ServicerContext,
    ):
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
            await context.abort(grpc.StatusCode.INVALID_ARGUMENT, "Missing ConvertMeta")
            return

        suffix = Path(meta.filename).suffix or ".pdf"

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
            if suffix.lower() == ".pdf":
                try:
                    pages_total = count_pdf_pages(tmp_path)
                except Exception:
                    logger.exception("failed to count pages for %s", meta.filename)

            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(
                    status="PROCESSING",
                    stage="loading",
                    message=f"loading document ({pages_total} pages)",
                    pages_total=pages_total,
                )
            )

            loop = asyncio.get_running_loop()
            progress_q: asyncio.Queue[progress.ProgressEvent] = asyncio.Queue()

            with progress.attach(progress_q, loop):
                convert_task = loop.run_in_executor(
                    self._executor, convert, tmp_path
                )

                while not convert_task.done():
                    try:
                        evt = await asyncio.wait_for(
                            progress_q.get(), timeout=_DRAIN_POLL_INTERVAL
                        )
                    except asyncio.TimeoutError:
                        continue
                    yield parser_pb2.ConvertResult(
                        status=parser_pb2.StatusUpdate(
                            status="PROCESSING",
                            stage=evt.stage,
                            message=evt.message,
                            pages_done=evt.pages_done,
                            pages_total=evt.pages_total or pages_total,
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
                            pages_total=evt.pages_total or pages_total,
                        )
                    )

                markdown: str = await convert_task

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
            encoded = markdown.encode("utf-8")
            for i in range(0, len(encoded), _CHUNK_SIZE):
                yield parser_pb2.ConvertResult(
                    markdown_chunk=encoded[i : i + _CHUNK_SIZE]
                )

            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(
                    status="DONE",
                    stage="done",
                    pages_done=pages_total,
                    pages_total=pages_total,
                )
            )

        except Exception as exc:
            logger.exception("Conversion failed for %s", meta.filename)
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
