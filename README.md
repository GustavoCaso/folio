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


## Project layout

| Path | Purpose |
|------|---------|
| `ui/` | Go web UI, SQLite, gRPC client |
| `parser/` | Python gRPC server, Docling integration |
| `proto/` | Shared protobuf definitions |


## Local dedelopment

To install all require languages and tools run `mise install` 

### Run Locally
Requirements: Docker and Docker compose

In the root folder run `make up` this would create two containers: ui and parser. 

Folio would be accessible on `http://localhost:8080`. Any documents you convert would be available locally in the `data` as well as the SQLite fiel `folio.db`. 

To stop everything run `make down`

### Test

In the root folder run `make test`. This commands takes care of executing tests in both projects

### Tasks 

Each projects `ui` and `parser` define their task in their `Makefile`.

## Configuration

Both services are configured via environment variables. 
The `compose.yaml` serves as an example way of configuring Folio.

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
| `PDF_POST_PROCESS_CODE_BLOCKS` | `true` | Pattern-based language detection + Prettier formatting for code blocks (see below) |
| `PDF_DO_CODE_ENRICHMENT` | `false` | VLM-based code enrichment — accurate but very slow on CPU (~100s/image); requires GPU for practical use |
| `PDF_CODE_FORMULA_PRESET` | `codeformulav2` | VLM preset when `PDF_DO_CODE_ENRICHMENT=true`: `codeformulav2` or `granite_docling` (258M, faster on CPU) |
| `PDF_LAYOUT_BATCH_SIZE` | docling default | Docling layout batch size |
| `PDF_OCR_BATCH_SIZE` | docling default | Docling OCR batch size |
| `PDF_TABLE_BATCH_SIZE` | docling default | Docling table batch size |
| `PDF_QUEUE_MAX_SIZE` | docling default | Docling pipeline queue depth |
| `LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error` |

## Code block processing

Docling extracts code blocks from PDFs as plain text with no language tag. The parser offers three approaches to enrich them, ordered by speed vs. accuracy:

### 1. Pattern-based detection + Prettier formatting (`PDF_POST_PROCESS_CODE_BLOCKS=true`)

The default. Two stages:

**Detection** — regex patterns score each block against known languages (Python, JavaScript, TypeScript, Go, Rust, Java, Kotlin, SQL, Shell, JSON, YAML, TOML, HTML, CSS). The language with the most pattern hits wins. 

**Formatting** — Using [Prettier](https://prettier.io) the block is piped through it via `prettier --stdin-filepath file.<ext>`. Languages with community plugins use `--plugin`:

| Language | Prettier support | Plugin |
|---|---|---|
| JS, TS, JSON, YAML, HTML, CSS | Native | — |
| SQL | Plugin | `prettier-plugin-sql` |
| Shell / Bash | Plugin | `prettier-plugin-sh` |
| Java | Plugin | `prettier-plugin-java` |
| TOML | Plugin | `prettier-plugin-toml` |
| Python, Go, Rust, Kotlin | None | Skipped (tag only) |

Prettier failures fall back silently to the original code.

**Limitations:**
- Regex patterns can misdetect ambiguous snippets (e.g. short fragments that match multiple languages).
- Docling sometimes strips internal newlines from code blocks, which breaks both detection and formatting.
- Languages with no prettier support (Python, Go, Rust, Kotlin) get a language tag but no formatting.

### 2. VLM-based enrichment (`PDF_DO_CODE_ENRICHMENT=true`)

Uses a vision-language model (Docling's `CodeFormulaVlmOptions`) to read the code visually from the PDF page. More accurate for language detection and can reconstruct formatting lost during text extraction.

**Limitations:**
- Very slow on CPU (~100s per image). Requires a GPU for practical throughput.
- Mutually exclusive with `PDF_POST_PROCESS_CODE_BLOCKS` — enabling both is wasteful; VLM takes precedence in Docling's pipeline.
- Two presets via `PDF_CODE_FORMULA_PRESET`: `codeformulav2` (more accurate, larger model) and `granite_docling` (258M params, faster on CPU; MLX variant requires native Apple Silicon, unavailable in Docker).

### 3. No enrichment (both disabled)

Code blocks are emitted as untagged fenced blocks.
