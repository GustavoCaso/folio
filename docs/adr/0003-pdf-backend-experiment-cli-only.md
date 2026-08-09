# Alternative PDF backends stay CLI-only; gRPC server always uses Docling

`parser convert --backend` supports `docling` (default), `pymupdf4llm`, and `pdfmux` for evaluating faster/lighter PDF→markdown conversion against Docling. This is scoped to the CLI only — the gRPC server keeps using Docling unconditionally, with no `--backend`-equivalent wired through the proto. If an alternative backend wins the comparison, promoting it to the server is separate future work.

`marker` and `mineru` were rejected outright (marker: 5-10s/page on CPU with ~2GB models; mineru: GPU-oriented) — heavier than the Docling baseline they'd be replacing. `pdfmux`'s `[tables]` extra was rejected because it installs Docling as a dependency, defeating the point of the comparison; only pdfmux's core (pymupdf4llm + confidence scoring) is used.

All backends run through the same post-processing pipeline (`normalize_heading_hierarchy`, `enrich_code_blocks`) so the comparison isolates backend quality rather than pipeline differences. New backends skip image extraction (text-only markdown); Docling's path keeps its existing image extraction.

This is a deliberately reversible experiment, not a permanent architecture split — but it's the reason the CLI and server support different sets of backends, which would otherwise look like an oversight.
