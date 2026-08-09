# Folio UI

Go web UI for [Folio](../README.md). Owns all job + highlight state in a single SQLite DB. Sends PDF bytes to the Python parser via bidirectional gRPC stream; parses EPUB bytes in-process (no gRPC).

## Stack

- **Go** — HTTP server, SQLite store, gRPC client
- **[templ](https://templ.guide/)** — typed HTML templates
- **[golang-migrate](https://github.com/golang-migrate/migrate)** + `modernc.org/sqlite` — pure-Go SQLite, embedded migrations
- **[goldmark](https://github.com/yuin/goldmark)** — Markdown → HTML rendering with `data-block-id` injection for highlight anchoring
- **[raitucarp/epub](https://github.com/raitucarp/epub)** + `golang.org/x/net/html` — EPUB parsing and chapter HTML rendering with `data-block-id` injection
- **vanilla JS + vitest/jsdom** — highlight capture/render in the browser

## Layout

```
cmd/         server entry, route wiring
internal/
  db/          SQLite store, migrations, jobs + highlights
  hub/         in-memory pub/sub: gRPC status events → SSE per job
  parser/
    client/    gRPC client to Python parser
    proto/     generated protobuf bindings (do not edit)
  converter/   Runner: dispatches conversions to a map[string]parser.Parser keyed by format
  converter/parser/  Parser interface + per-format implementations (NewPDF, NewEPUB)
  renderer/markdown/ goldmark renderer with data-block-id injection
  renderer/epub/     HTML-tree walker: data-block-id injection + image data-URI rewriting per chapter
  domain/      domain types
  repository/  repository interfaces
  handlers/    HTTP handlers (documents, reader, highlights, SSE)
  templates/   templ templates (layout, documents, reader)
static/      highlight.js, CSS
```

## Data flow

```
Browser uploads PDF
  → POST /documents
  → store.CreateJob(..., format="pdf")
  → go converter.Run(..., format)     # converter.Runner dispatches by format
      → parsers["pdf"].Convert()      # parser.NewPDF, opens gRPC stream
          → sends PDF chunks
          ← StatusUpdate{PROCESSING}  → hub.Publish() → SSE → browser
          ← markdown_chunk × N
          ← StatusUpdate{DONE}        → hub.Publish() → SSE → browser
          → os.WriteFile(markdown)
          → store.MarkJobDone()

Browser uploads EPUB
  → POST /documents (same route, format detected)
  → store.CreateJob(..., format="epub")
  → go converter.Run(..., format)
      → parsers["epub"].Convert()     # parser.NewEPUB, synchronous, no gRPC
          → epub.NewReader(bytes), extract Title/Author/Cover
          → per spine item: renderer/epub.Render() → data-block-id + base64 images
          → write DATA_DIR/{jobID}/chapter-N.html + toc.json
          → store.MarkJobDone(outputPath=dir)
```

Each `parser.Parser` implementation owns its full completion lifecycle (output writes, `MarkJobDone`/`MarkJobFailed`, hub publish) — see [ADR 0002](../docs/adr/0002-format-keyed-parser-dispatch.md).

## Highlight anchoring

Highlights anchor to `data-block-id` attributes. PDF/Markdown IDs are injected by the goldmark renderer (`renderer/markdown/`), of the form `kind-N` (e.g. `heading-1`, `paragraph-3`). EPUB IDs are injected by `renderer/epub/`, chapter-prefixed as `ch{N}-{tag}-{M}`. `StartPos`/`EndPos` are character offsets within the block (each `<img>` counts as 1 char), not the whole document. Highlights span multiple blocks via `start_block_id`/`end_block_id`.

This survives goldmark version upgrades and minor Docling output changes as long as block structure is preserved. See [ADR 0001](../docs/adr/0001-highlight-anchoring-via-block-id.md).

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
