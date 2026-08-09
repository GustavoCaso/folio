# Format-keyed parser dispatch; each parser owns its full completion lifecycle

`converter.Runner` dispatches conversions through a `map[string]parser.Parser` keyed by `domain.Format`, and the `Parser` interface (`Convert`/`ConvertFromURL`) returns only `error` — not a result struct for the Runner to write out. Each implementation (`NewPDF`, `NewEPUB`) is responsible for writing its own output, calling `MarkJobDone`/`MarkJobFailed`, and publishing hub events itself.

This replaced an earlier shape where `converter.Runner` held a single hardcoded PDF `client.Client` plus a separate `epubconvert.Runner`, with format branching happening in the handlers. The chosen shape duplicates completion boilerplate across parsers, but it's the right trade because formats don't share a completion shape: PDF writes one markdown file via a gRPC round-trip, EPUB writes multiple chapter HTML files plus `toc.json` synchronously with no gRPC. Forcing both through one shared "write result, then complete" path would mean the shared path constantly branches on format anyway.

An unregistered format marks the job failed rather than panicking.

## Considered options

- Keep per-format branching in handlers with separate Runner types per format (`epubConverter` field) — rejected, this is what came before and it doesn't scale past two formats.
- Runner owns writing output centrally, parsers just return converted bytes/paths — rejected, formats' output shapes differ too much (single file vs. directory of files) for one central writer to stay simple.
