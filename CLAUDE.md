# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

Self-hosted PDF reader with persistent highlights. Two services, one bidirectional gRPC stream:

- `ui/` — Go web app. Owns all state (jobs + highlights) in SQLite. See `ui/CLAUDE.md`.
- `parser/` — Python gRPC server wrapping Docling. Stateless. See `parser/README.md`.
- `proto/parser.proto` — wire contract between them.

For service internals (architecture, env vars, commands, conventions), read the per-service docs. Do not duplicate them here.

## Top-level commands

```bash
make up       # docker compose up --build -d   (ui :8080, parser :50051)
make down
make test     # ui Go + ui vitest + parser pytest
make proto    # regen gRPC bindings in BOTH ui/ and parser/
```

Per-service commands (`lint`, `format`, `typecheck`, `templ`, `test-race`, ...) live in `ui/Makefile` and `parser/Makefile`.

## Cross-cutting rules

- **Proto changes touch both services.** Edit `proto/parser.proto`, then `make proto` from root — regenerates `ui/internal/parser/proto/` and `parser/src/parser/grpc/`. Generated files are checked in; do not hand-edit.
- **Only the Go UI touches SQLite.** Python is stateless; no file paths cross the wire.
- **Docker compose project name is `folio`** (`docker compose -p folio ...`). Models cached in `models/`, data in `data/` (both gitignored, mounted into containers).

## Agent skills

### Issue tracker

Issues tracked in GitHub Issues (GustavoCaso/folio), via `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: `CONTEXT.md` + `docs/adr/` at repo root. See `docs/agents/domain.md`.
