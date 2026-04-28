# Folio

Self-hosted PDF reader with persistent highlights. Upload a PDF, get rendered Markdown in the browser, and annotate it across paragraphs and images. Highlights survive re-renders by anchoring to stable block IDs (paragraph/heading/code-block).

## Architecture

Two services, one bidirectional gRPC stream between them.

```
Browser ──HTTP/SSE──▶ ui (Go)  ──gRPC stream──▶ parser (Python)
                       │                              │
                       ▼                              ▼
               SQLite (jobs+highlights)       Docling (PDF→Markdown)
```

- **`ui/`** — Go web app. Owns all state (jobs, highlights) in a single SQLite DB. Serves uploads, the reader UI, SSE progress, and the highlights API. `templ` for templates, vanilla JS for highlight capture/render.
- **`parser/`** — Python gRPC server wrapping [Docling](https://github.com/docling-project/docling). Stateless: accepts PDF bytes, streams back progress + Markdown. No file paths exchanged.
- **`proto/parser.proto`** — wire contract.

## Run

```bash
make up      # docker compose up --build -d
make down
```

UI on `http://localhost:8080`. SQLite at `data/folio.db`. Docling model cache at `models/`.

## Test

```bash
make test    # Go unit tests + JS (vitest) + Python tests
```

## Project layout

| Path | Purpose |
|------|---------|
| `ui/` | Go web UI, SQLite, gRPC client |
| `parser/` | Python gRPC server, Docling integration |
| `proto/` | Shared protobuf definitions |
| `data/` | SQLite + parsed Markdown output (gitignored) |
| `models/` | Docling model cache (gitignored) |
| `compose.yaml` | Docker Compose stack |

## Configuration

Both services are configured via environment variables. The `compose.yaml` is the canonical place to set them.

### UI (`ui/`)

| Variable | Default | Description |
|---|---|---|
| `DB_PATH` | `/data/folio.db` | SQLite path for jobs and highlights |
| `PARSER_GRPC_ADDR` | `localhost:50051` | Parser gRPC address |
| `DATA_DIR` | `/data` | Where Markdown files are written and read |
| `LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |

### Parser (`parser/`)

| Variable | Default | Description |
|---|---|---|
| `GRPC_PORT` | `50051` | gRPC server port |
| `NUM_WORKERS` | `2` | Parallel Docling conversions |
| `PDF_IMAGE_MODE` | `placeholder` | Markdown image mode: `embedded`, `placeholder`, `referenced` |
| `PDF_DO_OCR` | `true` | Run OCR on scanned pages |
| `PDF_DO_TABLE_STRUCTURE` | `true` | Recognize table structure |
| `PDF_GENERATE_IMAGES` | `true` | Extract picture images from PDF |
| `PDF_POST_PROCESS_CODE_BLOCKS` | `true` | Pattern-based code language detection (fast, CPU-only) |
| `PDF_DO_CODE_ENRICHMENT` | `false` | VLM-based code enrichment — accurate but very slow on CPU (~100s/image); requires GPU for practical use |
| `PDF_CODE_FORMULA_PRESET` | `codeformulav2` | VLM preset when `PDF_DO_CODE_ENRICHMENT=true`: `codeformulav2` or `granite_docling` (258M, faster on CPU) |
| `PDF_LAYOUT_BATCH_SIZE` | docling default | Docling layout batch size |
| `PDF_OCR_BATCH_SIZE` | docling default | Docling OCR batch size |
| `PDF_TABLE_BATCH_SIZE` | docling default | Docling table batch size |
| `PDF_QUEUE_MAX_SIZE` | docling default | Docling pipeline queue depth |
| `LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |

