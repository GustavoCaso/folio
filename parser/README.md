# Parser

Stateless document parser service. Converts PDF bytes to Markdown via Docling. Exposes a gRPC server and a CLI for direct file conversion.

## Setup

```bash
uv sync
```

## Running

### CLI (no server)

Convert a PDF directly to Markdown:

```bash
uv run parser convert path/to/file.pdf
uv run parser convert path/to/file.pdf --output path/to/output.md
```

### gRPC Server

```bash
uv run parser-server
```

Environment variables:

| Variable | Default | Description |
|---|---|---|
| `GRPC_PORT` | `50051` | gRPC server port |
| `NUM_WORKERS` | `2` | Parallel conversion workers |
| `PDF_LAYOUT_BATCH_SIZE` | docling default | Docling layout batch size |
| `PDF_OCR_BATCH_SIZE` | docling default | Docling OCR batch size |

### Docker

```bash
docker build -t parser .
docker run -p 50051:50051 parser
```

## Development

```bash
make test          # Run unit tests
make lint          # Lint and auto-fix
make format        # Format code
make typecheck     # Type-check with mypy
make proto         # Regenerate gRPC bindings
```

Integration tests (require `tests/fixtures/sample.pdf`):

```bash
uv run pytest -m integration
```
