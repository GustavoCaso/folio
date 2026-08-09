# Restore heading hierarchy from the PDF's own TOC, not Docling's output

Docling flattens all headings to H2 in its markdown output, losing the original H1-H6 structure. Rather than fixing this in Docling or accepting flat headings, a second library (PyMuPDF/`fitz`) opens the same PDF, extracts its table of contents via `get_toc()`, builds a normalized-text-to-level map, and rewrites the `#` prefixes in Docling's markdown to match.

This introduces a second PDF-parsing library alongside Docling specifically for this one task, and depends on the source PDF actually having an embedded TOC — PDFs without one get no correction. Text matching between TOC entries and markdown headings is case/punctuation-insensitive by design, since exact-match would fail on trivial formatting differences between the two extraction paths. Gated by `PDF_POST_PROCESS_HEADINGS` (default `true`), PDF-only.
