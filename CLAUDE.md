# CLAUDE.md

Self-hosted EPUB reader with persistent highlights. Single Go binary: owns all job + highlight state in a SQLite DB and parses EPUB bytes in-process. User-facing setup and env vars live in `README.md`.

## Commands

```bash
make build       # go build ./...
make test        # Go tests
make test-js     # vitest (jsdom)
make test-race   # Go tests with -race
make templ       # regenerate templ templates
make css         # regenerate Tailwind output
make lint        # golangci-lint --fix
make format      # golangci-lint fmt
```

## Stack

- **Go** — HTTP server, SQLite store, EPUB conversion
- **[templ](https://templ.guide/)** — typed HTML templates; **templui** components; Tailwind CSS
- **golang-migrate** + `modernc.org/sqlite` — pure-Go SQLite, embedded migrations
- **raitucarp/epub** + `golang.org/x/net/html` — EPUB parsing and chapter HTML rendering
- **goldmark** — Markdown → HTML, only for legacy PDF jobs already in the DB
- **vanilla JS + vitest/jsdom** — highlight capture/render in the browser
- **mise** pins Go, Node, pnpm (`.mise.toml`); `engine-strict=true` in `.npmrc`

## Layout

```
main.go      entry point → cmd.Execute()
cmd/         server entry, flag/env parsing, export backend wiring
internal/
  db/                SQLite store, migrations, jobs + highlights + exports
  hub/               in-memory pub/sub: conversion status events → SSE per job
  converter/         Runner: dispatches conversions to a map[JobFormat]parser.Parser, owns cancel tracking
  converter/parser/  Parser interface + NewEPUB implementation
  renderer/epub/     HTML-tree walker: data-block-id injection + image data-URI rewriting per chapter
  renderer/markdown/ goldmark renderer with data-block-id injection (legacy PDF jobs)
  export/            highlight export worker + backends (Readwise)
  domain/            domain types
  repository/        repository interfaces
  handlers/          HTTP handlers (documents, reader, highlights, exports, SSE)
  handlers/static/   js/ (client JS), css/ (custom), tailwind/ (generated)
  templates/         templ templates (layout, documents, reader, edit, exports)
  logging/           structured JSON logger + HTTP middleware
```

## Data flow

```
Browser uploads EPUB
  → POST /documents              # non-.epub uploads rejected with 400
  → store.CreateJob(..., format="epub")
  → go converter.Run(..., format)
      → parsers["epub"].Convert()     # parser.NewEPUB, synchronous
          → epub.NewReader(bytes), extract Title/Author/Cover
          → per spine item: renderer/epub.Render() → data-block-id + base64 images
          → write DATA_DIR/{jobID}/chapter-N.html + toc.json
          → store.MarkJobDone(outputPath=dir)
          → hub.Publish(DONE) → SSE → browser
```

`GET /read/{jobID}?chapter=N&full=1` serves one chapter or the whole book concatenated. `Job.Format` selects the reader path (`epub` vs legacy `pdf`).

## Rules

- **Only EPUB is converted.** Only `NewEPUB` is registered in the Runner. An unregistered format marks the job failed rather than panicking. `domain.PdfFormat` + `renderer/markdown/` remain only so existing PDF jobs still render. See ADR 0007.
- **Parsers own the job lifecycle.** Each `parser.Parser` writes output, calls `MarkJobDone`/`MarkJobFailed`, publishes hub events, and must mark the job failed on `context.Canceled`. See ADR 0002.
- **Highlights anchor to `data-block-id`.** EPUB IDs: `ch{N}-{tag}-{M}` (e.g. `ch0-p-1`); legacy Markdown IDs: `kind-N` (e.g. `paragraph-3`). `StartPos`/`EndPos` are character offsets within the block (each `<img>` counts as 1 char). Multi-block highlights use `start_block_id`/`end_block_id`. Don't change ID shape without a migration plan. See ADR 0001.
- **Generated files are checked in.** After editing `.templ` run `make templ`; after class changes run `make css`. Don't hand-edit `*_templ.go` or `static/tailwind/output.css`.
- **Static assets are embedded.** After editing JS, rebuild the binary/image. Browsers cache module scripts aggressively; hard reload after restart.
- **Handler tests inject parsers** via `handlers.RegisterWithParsers` (exported from `internal/handlers/export_test.go`).

## Issue tracker

Issues tracked in GitHub Issues (GustavoCaso/folio), via `gh` CLI. See `docs/agents/issue-tracker.md`.
