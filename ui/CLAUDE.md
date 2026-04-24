# CLAUDE.md

## Commands

```bash
make build    # Build the binary
make test     # Run all tests
make proto    # Regenerate gRPC bindings from proto/parser.proto
templ generate # Regenerate Templ templates
```

## Architecture

Go web UI for Folio. Owns all job state and highlights in a single SQLite DB.
Sends PDF bytes to the Python parser via a single bidirectional gRPC stream.
Python is treated as stateless — no file paths are exchanged.

**Entry point:** `main.go` → `cmd/Execute()`

**Components (`internal/`):**
- `db/` — SQLite store (jobs + highlights), golang-migrate migrations, modernc.org/sqlite driver
- `hub/` — in-memory pub/sub: routes gRPC stream status events to SSE connections per job ID
- `parser/client/` — gRPC client: sends PDF chunks, receives StatusUpdates + markdown, publishes to Hub
- `parser/proto/` — generated protobuf bindings (do not edit manually)
- `renderer/` — goldmark Markdown renderer with data-block-id injection for highlight anchoring
- `handlers/` — HTTP handlers (documents, reader, highlights CRUD, SSE)
- `templates/` — Templ templates (layout, documents, reader)

**Static:** `static/highlight.js` — TreeWalker-based highlight application and capture

## Data flow

```
Browser uploads PDF
  → POST /documents
  → store.CreateJob()
  → go runConversion()       # background goroutine
      → parser.Convert()     # opens gRPC stream
          → sends PDF chunks
          ← receives StatusUpdate{PROCESSING}  → hub.Publish() → SSE → browser
          ← receives markdown_chunk × N
          ← receives StatusUpdate{DONE}        → hub.Publish() → SSE → browser
      → os.WriteFile(markdown)
      → store.MarkJobDone()
```

## Highlight anchoring

Highlights are anchored to `data-block-id` attributes injected by the goldmark
renderer. Each block-level element (heading, paragraph, code block) gets a unique
ID of the form `kind-N` (e.g. `heading-1`, `paragraph-3`). StartPos/EndPos are
character offsets within that block's text content, not the whole document.

This means highlights survive goldmark version upgrades and minor Docling output
changes, as long as the block structure is preserved.

## Configuration (env vars)

| Variable | Default | Description |
|---|---|---|
| `DB_PATH` | `/data/folio.db` | SQLite path for jobs and highlights |
| `PARSER_GRPC_ADDR` | `localhost:50051` | Parser gRPC address |
| `DATA_DIR` | `/data` | Where Markdown files are written and read |
| `LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

## Regenerating gRPC bindings

```bash
make proto
```

## Regenerating templates

```bash
templ generate
```
