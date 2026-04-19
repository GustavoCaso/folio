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

## Status

Single-user dev project. Migrations not backwards compatible — wipe `data/folio.db` between schema changes.
