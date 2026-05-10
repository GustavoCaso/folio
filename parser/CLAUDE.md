# CLAUDE.md

## Commands

```bash
uv sync              # install deps
uv run parser convert path/to/file.pdf [-o out.md]   # CLI, no server
uv run parser-server # start gRPC server

make test            # pytest
make lint            # ruff check --fix
make format          # ruff format
make typecheck       # mypy --strict
make proto           # regen gRPC bindings from ../proto/parser.proto
```

Integration tests (skipped by default; need `tests/fixtures/sample.pdf`):

```bash
uv run pytest -m integration
```

## Architecture

Stateless document parser. Wraps [Docling](https://github.com/docling-project/docling) behind a gRPC `ParserService` and a `convert` CLI. No state is retained between requests.

**Entry points:**
- `parser.server:serve` → `parser-server` script. Starts `grpc.aio.server` on `GRPC_PORT`.
- `parser.cli:main` → `parser` script. One-shot file conversion.

**Components (`src/parser/`):**
- `servicer.py` — `ParserServicer.ConvertDocument`. Reads all `ConvertChunk`s into a buffer, writes to a temp file, runs Docling in a `ThreadPoolExecutor` (blocking → executor), streams progress + 64KB markdown chunks back on the same stream.
- `converter.py` — dispatcher. Maps file extension → handler in `parser.formats`. Raises `UnsupportedFormatError` for unknown extensions.
- `formats/pdf.py` — Docling `DocumentConverter` instance (module-level, shared across requests). `PdfPipelineOptions` tuned via `PDF_*_BATCH_SIZE` env vars. Exports markdown with `ImageRefMode.EMBEDDED`. `count_pdf_pages` via `pypdfium2`.
- `progress.py` — `attach(queue, loop)` context manager adds a `logging.Handler` to the `docling` logger, parses lines like `Finished converting pages X/Y` into `ProgressEvent`s, and forwards them to an asyncio queue from the executor thread (`loop.call_soon_threadsafe`).
- `postprocess.py` — `enrich_code_blocks`: pattern-based language detection for fenced code blocks; called from `formats/pdf.py` when `PDF_POST_PROCESS_CODE_BLOCKS=true`.
- `logging_config.py` — JSON-to-stderr formatter. `extra={...}` on log calls becomes top-level fields. Caps `docling` logger at INFO unless root is DEBUG.
- `grpc/` — generated protobuf bindings (do not edit manually; the `make proto` step post-processes the import).

**Progress flow:** Docling is synchronous and logs per-page. `progress.py` hooks its logger, `servicer.py` polls the queue every 500ms while the executor future runs, drains remaining events after completion, then streams markdown.

## Adding dependencies 
Using `uv add <dependency>` or uv add --dev <dependency>. To avoid getting unwanted updates and to increase security, we pin all of our dependencies, meaning we use `==` rather than `>=`

## Adding a new format

1. New module in `src/parser/formats/` with a `convert_<fmt>(path: Path) -> str` function.
2. Register it in `parser/converter.py::_HANDLERS` (`".ext": "parser.formats.mod:convert_fn"`).
3. Page counting: `servicer.py` only calls `count_pdf_pages` for `.pdf`; extend if needed.

## Configuration (env vars)

| Variable | Default | Description |
|---|---|---|
| `GRPC_PORT` | `50051` | gRPC server port |
| `NUM_WORKERS` | `2` | `ThreadPoolExecutor` size (parallel Docling conversions) |
| `PDF_GENERATE_IMAGES` | `false` | Extract picture images from PDF |
| `PDF_DO_OCR` | `true` | Run OCR on scanned pages |
| `PDF_DO_TABLE_STRUCTURE` | `true` | Recognize table structure |
| `PDF_DO_CODE_ENRICHMENT` | `false` | Docling VLM-based code enrichment — accurate but very slow on CPU (~100s/image); requires GPU for practical use |
| `PDF_CODE_FORMULA_PRESET` | `codeformulav2` | VLM preset when `PDF_DO_CODE_ENRICHMENT=true`: `codeformulav2` (accurate, larger) or `granite_docling` (258M params, faster on CPU). Note: MLX variant of granite_docling requires native Apple Silicon access — not available inside Docker |
| `PDF_POST_PROCESS_CODE_BLOCKS` | `true` | Post-process code blocks with pattern-based language detection (fast, CPU-only, no extra models); mutually exclusive with `PDF_DO_CODE_ENRICHMENT` |
| `PDF_IMAGE_MODE` | `placeholder` | Markdown image mode: `embedded`, `placeholder`, `referenced` |
| `PDF_LAYOUT_BATCH_SIZE` | docling default | Docling layout batch size |
| `PDF_OCR_BATCH_SIZE` | docling default | Docling OCR batch size |
| `PDF_TABLE_BATCH_SIZE` | docling default | Docling table batch size |
| `PDF_QUEUE_MAX_SIZE` | docling default | Docling pipeline queue depth |
| `LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |

## Tooling

- Python ≥ 3.12, deps + venv via `uv` (`uv sync`).
- `ruff` for lint + format; excludes `src/parser/grpc/` (generated).
- `mypy --strict` on `src/`.
- `pytest` with `asyncio_mode = "auto"`; `integration` marker opts into fixture-dependent tests.

## Regenerating gRPC bindings

```bash
make proto
```

Uses `grpc_tools.protoc` + [`protoletariat`](https://github.com/cpcloud/protoletariat) to rewrite the generated top-level import into a package-relative one (`from . import parser_pb2`). Do not hand-edit the files under `src/parser/grpc/`.
