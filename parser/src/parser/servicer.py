from __future__ import annotations

import asyncio
import logging
import os
import tempfile
import time
import uuid
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from collections.abc import AsyncGenerator, AsyncIterator

import grpc
from docling_core.types.doc.base import ImageRefMode
from docling_core.types.doc.document import DoclingDocument

from parser.formats.converter import (
    PDF_BATCH_SIZE,
    convert,
    convert_pdf_page_range,
    pdf_page_batches,
)
from parser.formats.helpers import extract_metadata, get_format_settings
from parser.formats.pdf.images import PLACEHOLDER, extract_images
from parser.formats.pdf.pdf import count_pdf_pages
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
        job_id = uuid.uuid4().hex[:8]
        started = time.monotonic()

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

        suffix = Path(meta.filename).suffix.lower() or ".pdf"
        ctx: dict[str, object] = {"job_id": job_id, "file": meta.filename, "bytes": len(buf)}
        if meta.request_id:
            ctx["request_id"] = meta.request_id

        logger.info("convert received", extra=ctx)

        tmp_path: Path | None = None
        try:
            with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as f:
                f.write(buf)
                tmp_path = Path(f.name)

            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(
                    status="PROCESSING",
                    stage="received",
                    message=f"received {len(buf)} bytes",
                )
            )

            pages_total = 0
            if suffix == ".pdf":
                try:
                    pages_total = count_pdf_pages(tmp_path)
                except Exception:
                    logger.exception("count pages failed", extra={**ctx, "tmp_path": str(tmp_path)})

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

            if suffix == ".pdf" and pages_total > PDF_BATCH_SIZE:
                docs = []
                for start, end in pdf_page_batches(pages_total):
                    logger.debug(
                        "convert batch",
                        extra={**ctx, "start": start, "end": end, "pages_total": pages_total},
                    )
                    batch_doc = await loop.run_in_executor(
                        self._executor, convert_pdf_page_range, tmp_path, start, end
                    )
                    docs.append(batch_doc)
                    yield parser_pb2.ConvertResult(
                        status=parser_pb2.StatusUpdate(
                            status="PROCESSING",
                            stage="converting",
                            message=f"converted pages {end}/{pages_total}",
                            pages_done=end,
                            pages_total=pages_total,
                        )
                    )
                doc = DoclingDocument.concatenate(docs)
            else:
                with attach(progress_q, loop):
                    convert_task = loop.run_in_executor(self._executor, convert, tmp_path)

                    while not convert_task.done():
                        try:
                            evt = await asyncio.wait_for(
                                progress_q.get(), timeout=_DRAIN_POLL_INTERVAL
                            )
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

            async for result in self._export_doc(ctx, doc, suffix, tmp_path, pages_total, started):
                yield result

        except Exception as exc:
            logger.exception("convert failed", extra=ctx)
            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(status="FAILED", error=str(exc))
            )
        finally:
            if tmp_path is not None:
                tmp_path.unlink(missing_ok=True)

    async def ConvertURL(  # noqa: N802
        self,
        request: parser_pb2.ConvertURLRequest,
        context: grpc.aio.ServicerContext[parser_pb2.ConvertURLRequest, parser_pb2.ConvertResult],
    ) -> AsyncGenerator[parser_pb2.ConvertResult, None]:
        job_id = uuid.uuid4().hex[:8]
        started = time.monotonic()

        suffix = Path(request.url).suffix.lower() or ".html"
        ctx: dict[str, object] = {"job_id": job_id, "url": request.url}
        if request.request_id:
            ctx["request_id"] = request.request_id

        # Defense-in-depth: validate scheme even though the Go caller does it too.
        if not request.url.startswith(("http://", "https://")):
            await context.abort(
                grpc.StatusCode.INVALID_ARGUMENT, "URL must use http or https scheme"
            )
            return

        logger.info("convert url received", extra=ctx)

        tmp_path: Path | None = None
        try:
            with tempfile.NamedTemporaryFile(suffix=suffix, delete=False) as f:
                tmp_path = Path(f.name)

            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(
                    status="PROCESSING",
                    stage="received",
                    message=f"received url: {request.url}",
                )
            )

            logger.info("convert loading", extra={**ctx, "pages_total": 0})

            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(
                    status="PROCESSING",
                    stage="loading",
                    message="fetching and loading document",
                )
            )

            loop = asyncio.get_running_loop()
            convert_task = loop.run_in_executor(self._executor, convert, request.url)
            doc = await convert_task

            async for result in self._export_doc(
                ctx, doc, suffix, tmp_path, 0, started, is_url=True, source_url=request.url
            ):
                yield result

        except Exception as exc:
            logger.exception("convert url failed", extra=ctx)
            yield parser_pb2.ConvertResult(
                status=parser_pb2.StatusUpdate(status="FAILED", error=str(exc))
            )
        finally:
            if tmp_path is not None:
                tmp_path.unlink(missing_ok=True)

    async def _export_doc(
        self,
        ctx: dict[str, object],
        doc: DoclingDocument,
        suffix: str,
        tmp_path: Path,
        pages_total: int,
        started: float,
        is_url: bool = False,
        source_url: str = "",
    ) -> AsyncGenerator[parser_pb2.ConvertResult, None]:
        do_post_process = get_format_settings(suffix)

        with tempfile.TemporaryDirectory() as export_dir:
            md_path = Path(export_dir) / "doc.md"

            if suffix == ".pdf":
                doc.save_as_markdown(
                    filename=md_path,
                    image_mode=ImageRefMode.PLACEHOLDER,
                    image_placeholder=PLACEHOLDER,
                )
                markdown = md_path.read_text(encoding="utf-8")
                image_files = []
                if not is_url:
                    scale_val = float(os.environ.get("PDF_IMAGES_SCALE", "0.5")) * 150 / 72
                    for name, data in extract_images(doc, tmp_path, scale=scale_val):
                        markdown = markdown.replace(PLACEHOLDER, f"![Image]({name})", 1)
                        image_files.append((name, data))
            else:
                doc.save_as_markdown(filename=md_path, image_mode=ImageRefMode.REFERENCED)
                markdown = md_path.read_text(encoding="utf-8")
                artifacts_dir = md_path.with_name(md_path.stem + "_artifacts")
                image_files = [
                    (p.name, p.read_bytes())
                    for p in (sorted(artifacts_dir.glob("*.png")) if artifacts_dir.exists() else [])
                ]

            logger.debug(
                "images collected",
                extra={**ctx, "count": len(image_files), "names": [n for n, _ in image_files]},
            )

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

            md_chunks = 0
            for i in range(0, len(encoded), _CHUNK_SIZE):
                yield parser_pb2.ConvertResult(markdown_chunk=encoded[i : i + _CHUNK_SIZE])
                md_chunks += 1

            for name, data in image_files:
                yield parser_pb2.ConvertResult(
                    image_chunk=parser_pb2.ImageChunk(filename=name, data=data)
                )

        source: Path | str = source_url if source_url else tmp_path
        title, author, cover = extract_metadata(source, suffix, generate_cover=True)
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

    def shutdown(self) -> None:
        self._executor.shutdown(wait=False)
