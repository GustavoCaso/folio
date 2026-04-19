# Folio UI

Go web UI for [Folio](../README.md). Owns all job + highlight state in a single SQLite DB. Sends PDFs to the Python parser via bidirectional gRPC stream.

## Stack

- **Go** — HTTP server, SQLite store, gRPC client
- **[templ](https://templ.guide/)** — typed HTML templates
- **[golang-migrate](https://github.com/golang-migrate/migrate)** + `modernc.org/sqlite` — pure-Go SQLite, embedded migrations
- **[goldmark](https://github.com/yuin/goldmark)** — Markdown → HTML rendering with `data-block-id` injection for highlight anchoring
- **vanilla JS + vitest/jsdom** — highlight capture/render in the browser

## Layout

```
cmd/         server entry, route wiring
internal/
  db/        SQLite store, migrations, jobs + highlights
  hub/       in-memory pub/sub: gRPC status events → SSE per job
  parser/
    client/  gRPC client to Python parser
    proto/   generated protobuf bindings (do not edit)
  renderer/  goldmark renderer with data-block-id injection
  handlers/  HTTP handlers (documents, reader, highlights, SSE)
  templates/ templ templates (layout, documents, reader)
static/      highlight.js, CSS
```

## Data flow

```
Browser uploads PDF
  → POST /documents
  → store.CreateJob()
  → go runConversion()                 # background goroutine
      → parser.Convert()               # opens gRPC stream
          → sends PDF chunks
          ← StatusUpdate{PROCESSING}   → hub.Publish() → SSE → browser
          ← markdown_chunk × N
          ← StatusUpdate{DONE}         → hub.Publish() → SSE → browser
      → os.WriteFile(markdown)
      → store.MarkJobDone()
```

## Highlight anchoring

Highlights anchor to `data-block-id` attributes injected by the goldmark renderer. Each block-level element gets a stable ID of the form `kind-N` (e.g. `heading-1`, `paragraph-3`). `StartPos`/`EndPos` are character offsets within the block (each `<img>` counts as 1 char), not the whole document. Highlights span multiple blocks via `start_block_id`/`end_block_id`.

This survives goldmark version upgrades and minor Docling output changes as long as block structure is preserved.

## Commands

```bash
make build       # go build ./...
make test        # Go tests
make test-js     # vitest (jsdom)
make test-race   # Go tests with -race
make proto       # regenerate gRPC bindings from ../proto/parser.proto
make templ       # regenerate templ templates
make lint        # golangci-lint --fix
make format      # golangci-lint fmt
```

## Configuration (env vars)

| Variable | Default | Description |
|---|---|---|
| `DB_PATH` | `/data/folio.db` | SQLite path |
| `PARSER_GRPC_ADDR` | `localhost:50051` | Parser gRPC address |
| `DATA_DIR` | `/data` | Where parsed Markdown is written/read |

## Tooling

Node + pnpm versions pinned via [mise](https://mise.jdx.dev/) (`.mise.toml`). `engine-strict=true` in `.npmrc` enforces them.

## Notes

- Migrations are not backwards compatible (single-user dev). Wipe `data/folio.db` between schema changes.
- After editing JS, rebuild the Docker image — `static/` is baked at build time. Browsers cache module scripts aggressively; hard reload (Cmd+Shift+R) after restart.
