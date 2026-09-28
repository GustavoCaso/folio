# Remove the Python parser; Folio is a single Go service that converts EPUB only

Folio used to run two services: the Go UI, and a stateless Python gRPC server wrapping Docling that turned PDFs and imported URLs into Markdown. We removed the Python parser and everything that depended on it: `proto/parser.proto`, the gRPC client, `NewPDF`, URL import (`POST /documents/import`, `RunFromURL`/`ConvertFromURL`) and the parser health badge. The goal is a smaller codebase. We now ship one binary, one Docker image, one language toolchain and one test suite, instead of keeping two services and a wire contract in sync. EPUB is already parsed in-process by Go (`NewEPUB`), so it is the only supported input for now. Uploads without `.epub` are rejected with 400.

## Consequences

- `converter.Runner` still dispatches through a format-keyed parser map ([ADR 0002](0002-format-keyed-parser-dispatch.md)), with only EPUB registered. We can add a format again by registering a new `parser.Parser` without reshaping the Runner.
- Existing PDF jobs remain readable. `domain.PdfFormat` and `renderer/markdown/` stay so already-converted Markdown still renders, and highlights anchored to it still work ([ADR 0001](0001-highlight-anchoring-via-block-id.md)). Retrying a failed PDF job fails with "no parser registered".
- ADRs 0003–0006 describe the Python PDF pipeline and are historical only.

## Considered options

- Keep the Python parser running next to the Go service. Rejected: running two services, a gRPC contract and Docling's model downloads cost more than PDF support is worth right now.
- Port PDF conversion to Go. Deferred: it is out of scope for this simplification, and a future ADR can revisit it.
