# Graph Report - .  (2026-08-04)

## Corpus Check
- 213 files · ~132,004 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 1159 nodes · 2341 edges · 68 communities (56 shown, 12 thin omitted)
- Extraction: 91% EXTRACTED · 9% INFERRED · 0% AMBIGUOUS · INFERRED: 210 edges (avg confidence: 0.8)
- Token cost: 65,300 input · 0 output

## Community Hubs (Navigation)
- Parser gRPC Servicer & Docling Init
- Document HTTP Handlers
- Export Backends & Worker
- Parser Test Helpers & HTML Metadata
- Docling Converter Core
- UI Repository Test Suite
- Job Domain & Templates
- SQLite DB Access Layer
- Parser CLI
- Root Architecture Docs
- Code Block Postprocessing
- Export & Edit HTTP Handlers
- PDF Pipeline Options
- Highlight Domain & Reader View
- Reader.js Frontend Highlighting
- Deployment & Compose Docs
- PDF Image Extraction
- UI JS Dev Dependencies
- Proto ConvertMeta Messages
- Parser gRPC Client Core
- Markdown Block-ID Renderer
- Hub Pub/Sub & Fake Parsers
- UI JS Formatting Dependencies
- Proto ConvertChunk Messages
- gRPC Client Wrapper
- PDF Metadata & Thumbnail
- Proto Parser Server Interface
- Converter Job Runner Setup
- Documents.js Frontend
- Convert & Health Test Suite
- Fake Parser Server for Tests
- Converter Job Runner
- Fake Parser Client for Tests
- Progress Logging Handler
- Hub Pub/Sub Tests
- Proto ConvertResult Metadata
- Proto ConvertResult Status
- Highlight Export Queries
- Proto gRPC Client Interfaces
- Proto Message Reflection
- Proto Descriptors
- Health Check Tests
- Readwise API Types
- UI CI Workflow
- Local Deploy Script
- Readwise Export Backend
- Parser CI Workflow
- Docker Compose Project
- Highlight Create Request
- Error Message Type
- Entrypoint Script A
- Entrypoint Script B
- UI Go Module
- Parser Python Project
- Structured Logging Note

## God Nodes (most connected - your core abstractions)
1. `newTestStore()` - 33 edges
2. `ParserServicer` - 32 edges
3. `Hub` - 32 edges
4. `New()` - 31 edges
5. `pdf_pipeline_options()` - 30 edges
6. `newTestMux()` - 30 edges
7. `ConvertResult` - 28 edges
8. `TestPDFPipelineOptions` - 26 edges
9. `extract_metadata()` - 21 edges
10. `_fake_save_as_markdown()` - 21 edges

## Surprising Connections (you probably didn't know these)
- `_make_url_request()` --calls--> `ConvertURLRequest`  [EXTRACTED]
  parser/tests/test_servicer.py → ui/internal/parser/proto/parser.pb.go
- `parser/Dockerfile` --references--> `Docling (PDF/HTML to Markdown)`  [INFERRED]
  .github/workflows/docker-publish.yml → README.md
- `parser service (Python gRPC server)` --references--> `Docling (PDF/HTML to Markdown)`  [EXTRACTED]
  CLAUDE.md → README.md
- `formats/pdf/pdf.py (DocumentConverter)` --references--> `Docling (PDF/HTML to Markdown)`  [EXTRACTED]
  parser/CLAUDE.md → README.md
- `logging_config.py` --references--> `Docling (PDF/HTML to Markdown)`  [EXTRACTED]
  parser/CLAUDE.md → README.md

## Import Cycles
- None detected.

## Hyperedges (group relationships)
- **PDF conversion pipeline via bidirectional gRPC stream** — ui_internal_parser_client, parser_servicer_py, proto_parser_proto, docling_dependency [EXTRACTED 0.95]
- **Proto regeneration touches both services** — proto_parser_proto, make_proto_command, ui_internal_parser_proto, parser_grpc_generated [EXTRACTED 0.95]
- **CI workflows gate main + tags across services** — github_workflows_ui_workflow, github_workflows_parser_workflow, github_workflows_docker_publish_docker_workflow [INFERRED 0.85]

## Communities (68 total, 12 thin omitted)

### Community 0 - "Parser gRPC Servicer & Docling Init"
Cohesion: 0.07
Nodes (55): AbstractEventLoop, object, Force Docling model initialization. Blocks until all models are loaded., warmup(), add_ParserServiceServicer_to_server(), ParserService, ParserServiceServicer, ParserServiceStub (+47 more)

### Community 1 - "Document HTTP Handlers"
Cohesion: 0.05
Nodes (49): Attr, Level, ctxKey, flushableRecorder, reqIDKey, statusRecorder, ResponseRecorder, envOr() (+41 more)

### Community 2 - "Export Backends & Worker"
Cohesion: 0.06
Nodes (54): Backend, ExportRecord, fakeBackend, Worker, Config, ReadwiseBackend, ExportRepository, Server (+46 more)

### Community 3 - "Parser Test Helpers & HTML Metadata"
Cohesion: 0.06
Nodes (24): HTMLParser, parametrize, bool_env(), extract_metadata(), Path, Extract (title, author, cover) from a document. source is a Path for file-…, _decode_cover(), extract_metadata() (+16 more)

### Community 4 - "Docling Converter Core"
Cohesion: 0.05
Nodes (20): DocumentConverter, HTMLBackendOptions, convert(), convert_pdf_page_range(), html_backend_options(), _make_html_converter(), pdf_page_batches(), DoclingDocument (+12 more)

### Community 5 - "UI Repository Test Suite"
Cohesion: 0.09
Nodes (50): recordingBackend, HighlightRepository, JobRepository, Store, ServeMux, emptyMultipartRequest(), Handler, Request (+42 more)

### Community 6 - "Job Domain & Templates"
Cohesion: 0.10
Nodes (36): scanner, Job, Context, db, scanJob(), coverBase64(), DocumentList(), Documents() (+28 more)

### Community 7 - "SQLite DB Access Layer"
Cohesion: 0.12
Nodes (38): sqlExecutor, Context, db, New(), runMigrations(), T, TestWithTx_CommitsOnSuccess(), TestWithTx_MultipleOpsAtomic() (+30 more)

### Community 8 - "Parser CLI"
Cohesion: 0.10
Nodes (35): ArgumentParser, Namespace, build_parser(), _convert(), main(), _metadata(), get_format_settings(), Return post_process_code_blocks for the given file extension. (+27 more)

### Community 9 - "Root Architecture Docs"
Cohesion: 0.06
Nodes (35): Bidirectional gRPC stream, Highlight anchoring via stable block IDs, Root CLAUDE.md, Folio, golang-migrate + modernc.org/sqlite, data-block-id anchoring survives goldmark/Docling changes, goldmark, make proto (+27 more)

### Community 10 - "Code Block Postprocessing"
Cohesion: 0.11
Nodes (7): integration, _detect_language(), enrich_code_blocks(), _prettier_format(), TestIntegration, TestLanguageDetection, TestPrettierArguments

### Community 11 - "Export & Edit HTTP Handlers"
Cohesion: 0.11
Nodes (28): Handlers, Request, ResponseWriter, EditDocument(), Component, Exports(), exportStatusBadge(), Component (+20 more)

### Community 12 - "PDF Pipeline Options"
Cohesion: 0.11
Nodes (7): CodeFormulaVlmOptions, _code_formula_options(), pdf_pipeline_options(), _table_structure_options(), TestPDFPipelineOptions, PdfPipelineOptions, TableStructureOptions

### Community 13 - "Highlight Domain & Reader View"
Cohesion: 0.17
Nodes (21): ExportRecord, Highlight, Context, db, Time, Component, headerActions(), highlightCard() (+13 more)

### Community 14 - "Reader.js Frontend Highlighting"
Cohesion: 0.18
Nodes (19): applyHighlight(), applyHighlights(), blockWalker(), bootstrap(), buildPendingSelection(), calcPopoverPosition(), computeProgress(), findBlockAncestor() (+11 more)

### Community 15 - "Deployment & Compose Docs"
Cohesion: 0.11
Nodes (22): Pattern-based detection + Prettier formatting, VLM-based code enrichment (CodeFormulaVlmOptions), compose parser service definition, compose ui service definition, gustavocaso/folio-parser image, gustavocaso/folio-ui image, Docling (PDF/HTML to Markdown), build-parser job (+14 more)

### Community 16 - "PDF Image Extraction"
Cohesion: 0.17
Nodes (20): BoundingBox, _bbox_to_crop(), extract_images(), DoclingDocument, Path, Convert a BOTTOMLEFT bbox to pypdfium2 crop margins (left, bottom, right, top)., Render each picture bbox from the PDF using pypdfium2. Yields (filename,…, Replace <!-- image --> placeholders with ![Image](filename) in document order. (+12 more)

### Community 17 - "UI JS Dev Dependencies"
Cohesion: 0.09
Nodes (21): jsdom, tailwindcss, @tailwindcss/cli, devDependencies, jsdom, tailwindcss, @tailwindcss/cli, vitest (+13 more)

### Community 18 - "Proto ConvertMeta Messages"
Cohesion: 0.11
Nodes (6): MessageState, ConvertMeta, ConvertURLRequest, ImageChunk, SizeCache, UnknownFields

### Community 19 - "Parser gRPC Client Core"
Cohesion: 0.18
Nodes (6): ConversionResult, ImageFile, blockingParser, failParser, successParser, Context

### Community 20 - "Markdown Block-ID Renderer"
Cohesion: 0.19
Nodes (16): Document, Markdown, Node, blockIDTransformer, codeBlockIDs(), Context, newMarkdownRendered(), Render() (+8 more)

### Community 21 - "Hub Pub/Sub & Fake Parsers"
Cohesion: 0.18
Nodes (8): blockingParser, errorParser, Hub, StatusEvent, Mutex, Context, Logger, New()

### Community 22 - "UI JS Formatting Dependencies"
Cohesion: 0.11
Nodes (18): dependencies, prettier, prettier-plugin-java, prettier-plugin-sh, prettier-plugin-sql, prettier-plugin-toml, engines, node (+10 more)

### Community 23 - "Proto ConvertChunk Messages"
Cohesion: 0.12
Nodes (8): ConvertChunk, ConvertChunk_Data, ConvertChunk_Meta, ConvertResult_ImageChunk, ConvertResult_MarkdownChunk, isConvertChunk_Payload, file_parser_proto_init(), init()

### Community 24 - "gRPC Client Wrapper"
Cohesion: 0.19
Nodes (10): Client, ClientConn, Context, Logger, ServerStreamingClient, Time, New(), T (+2 more)

### Community 25 - "PDF Metadata & Thumbnail"
Cohesion: 0.24
Nodes (13): extract_metadata(), Path, Return (title, author, cover) from PDF file. Empty strings if absent., Render PDF page as PNG at 72 DPI. Returns empty bytes on failure., render_pdf_page(), _mock_pdf(), _mock_pdf_with_cover(), test_extract_metadata_empty_when_missing() (+5 more)

### Community 26 - "Proto Parser Server Interface"
Cohesion: 0.18
Nodes (10): ParserServiceServer, UnimplementedParserServiceServer, UnsafeParserServiceServer, ServerStream, ServiceRegistrar, BidiStreamingServer, ServerStreamingServer, _ParserService_ConvertDocument_Handler() (+2 more)

### Community 27 - "Converter Job Runner Setup"
Cohesion: 0.58
Nodes (12): New(), Logger, T, newHub(), newLogger(), newStore(), TestRun_CancelledContext_MarksJobCancelledByUser(), TestRun_MarksDone() (+4 more)

### Community 28 - "Documents.js Frontend"
Cohesion: 0.26
Nodes (8): badgeHTML(), bootstrap(), cancelButtonHTML(), deleteFormButton(), initDropZone(), readButtonHTML(), retryFormHTML(), watchJob()

### Community 29 - "Convert & Health Test Suite"
Cohesion: 0.50
Nodes (12): collectBuffered(), T, newHub(), startFakeParser(), startFakeParserURL(), TestConvert_CollectsImageChunks(), TestConvert_DoesNotPublishDONEEvent(), TestConvert_NoImagesWhenNoneStreamed() (+4 more)

### Community 30 - "Fake Parser Server for Tests"
Cohesion: 0.21
Nodes (5): fakeParserServer, ConvertResult, isConvertResult_Payload, BidiStreamingServer, ServerStreamingServer

### Community 31 - "Converter Job Runner"
Cohesion: 0.30
Nodes (5): Runner, Map, Logger, Time, safeSlug()

### Community 32 - "Fake Parser Client for Tests"
Cohesion: 0.26
Nodes (7): fakeParserClient, Context, Handler, T, newHealthMux(), TestParserHealth_Healthy(), TestParserHealth_Unhealthy()

### Community 33 - "Progress Logging Handler"
Cohesion: 0.33
Nodes (9): _handler(), asyncio, LogRecord, _record(), test_attach_emits_events_to_queue(), test_attach_removes_handler_on_exit(), test_parse_docling_unmatched_returns_none(), test_parse_finished_pages() (+1 more)

### Community 34 - "Hub Pub/Sub Tests"
Cohesion: 0.44
Nodes (10): T, newHub(), TestNewRequiresLogger(), TestPublishCarriesMessage(), TestPublishDoesNotReachUnrelatedJob(), TestPublishReachesSubscriber(), TestReplayBufferClearedOnLastUnsubscribe(), TestStatusEventJSONShape() (+2 more)

### Community 37 - "Highlight Export Queries"
Cohesion: 0.42
Nodes (3): Context, db, ExportRecord

### Community 38 - "Proto gRPC Client Interfaces"
Cohesion: 0.32
Nodes (6): BidiStreamingClient, ClientConnInterface, ParserServiceClient, Context, ServerStreamingClient, NewParserServiceClient()

### Community 41 - "Health Check Tests"
Cohesion: 0.60
Nodes (5): HealthCheckResponse_ServingStatus, T, startHealthServer(), TestHealth_NotServing(), TestHealth_Serving()

### Community 42 - "Readwise API Types"
Cohesion: 0.50
Nodes (4): ReadwiseCreateRequest, ReadwiseHighlightInput, ReadwiseHighlightResponse, ReadwiseTagRequest

### Community 43 - "UI CI Workflow"
Cohesion: 0.67
Nodes (4): go job (test+lint), js job (vitest), ui/Makefile, ui CI Workflow

### Community 44 - "Local Deploy Script"
Cohesion: 0.83
Nodes (3): build_and_push(), deploy-local.sh script, usage()

### Community 45 - "Readwise Export Backend"
Cohesion: 0.67
Nodes (3): export.Backend interface, Readwise export backend, ui/cmd/server.go

### Community 46 - "Parser CI Workflow"
Cohesion: 0.67
Nodes (3): parser/Makefile, python job (pytest+lint+typecheck), parser CI Workflow

## Knowledge Gaps
- **60 isolated node(s):** `entrypoint.sh script`, `name`, `private`, `node`, `pnpm` (+55 more)
  These have ≤1 connection - possible missing edges or undocumented components.
- **12 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `ConvertResult` connect `Fake Parser Server for Tests` to `Proto ConvertResult Metadata`, `Docling Converter Core`, `Proto ConvertResult Status`, `Proto gRPC Client Interfaces`, `Proto Message Reflection`, `Proto Descriptors`, `Proto ConvertMeta Messages`, `Proto ConvertChunk Messages`, `gRPC Client Wrapper`, `Proto Parser Server Interface`, `Convert & Health Test Suite`?**
  _High betweenness centrality (0.375) - this node is a cross-community bridge._
- **Why does `Hub` connect `Hub Pub/Sub & Fake Parsers` to `Fake Parser Client for Tests`, `Export Backends & Worker`, `Hub Pub/Sub Tests`, `UI Repository Test Suite`, `Parser gRPC Client Core`, `gRPC Client Wrapper`, `Converter Job Runner Setup`, `Convert & Health Test Suite`, `Converter Job Runner`?**
  _High betweenness centrality (0.167) - this node is a cross-community bridge._
- **Why does `Store` connect `UI Repository Test Suite` to `Export Backends & Worker`, `Converter Job Runner Setup`, `SQLite DB Access Layer`, `Converter Job Runner`?**
  _High betweenness centrality (0.150) - this node is a cross-community bridge._
- **Are the 15 inferred relationships involving `newTestStore()` (e.g. with `TestDeleteHighlight_CallsBackendDelete()` and `TestListExports_Empty()`) actually correct?**
  _`newTestStore()` has 15 INFERRED edges - model-reasoned connections that need verification._
- **Are the 27 inferred relationships involving `New()` (e.g. with `TestWithTx_CommitsOnSuccess()` and `TestWithTx_MultipleOpsAtomic()`) actually correct?**
  _`New()` has 27 INFERRED edges - model-reasoned connections that need verification._
- **What connects `entrypoint.sh script`, `name`, `private` to the rest of the system?**
  _60 weakly-connected nodes found - possible documentation gaps or missing edges._
- **Should `Parser gRPC Servicer & Docling Init` be split into smaller, more focused modules?**
  _Cohesion score 0.06924882629107981 - nodes in this community are weakly interconnected._