# Folio

Self-hosted EPUB reader with persistent highlights.

## Features

- Upload EPUB books and read them in the browser, chapter by chapter, with a table of contents
- Highlight passages and add optional tags and notes
- Reading progress is saved per book
- Edit book metadata: title, author, tags, cover
- Export highlights to [Readwise](https://readwise.io) automatically

## Running

### Docker

```bash
docker run -d \
  --name folio \
  -p 8080:8080 \
  -v ./data:/data \
  gustavocaso/folio:latest
```

Then open http://localhost:8080.

### Docker Compose

```bash
cp .env.example .env   # optional: add READWISE_API_TOKEN
docker compose up --build -d
```

All state (SQLite database + converted books) lives in the mounted `/data` volume.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `DB_PATH` | `/data/folio.db` | SQLite database path |
| `DATA_DIR` | `/data` | Where converted books are stored |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |
| `READWISE_API_TOKEN` | — | Enables Readwise highlight export |
| `READWISE_TIMEOUT` | `30s` | Readwise HTTP timeout |
| `EXPORT_INTERVAL` | `1m` | How often highlights are exported |
| `PUID` / `PGID` | `1000` | User/group IDs the container runs as (match your host for volume permissions) |

## Development

Requires [mise](https://mise.jdx.dev/) for Go, Node and pnpm.

```bash
mise install
pnpm install
make build
make test && make test-js
go run . -data ./data -db ./data/folio.db
```
