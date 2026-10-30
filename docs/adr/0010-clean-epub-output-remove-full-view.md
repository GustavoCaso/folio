# Clean EPUB chapter output and remove the full-book view

Converted chapters were written as whole XHTML documents, with the publisher's `<head>`, stylesheet links, calibre classes and tooling attributes, and the reader pasted them into its page. Images and links were broken. A chapter's references are relative to the chapter file, but the parser looked them up as manifest hrefs, which are relative to the OPF. Cross-chapter links kept the source's `file.xhtml#id` hrefs, and those files don't exist on the server.

## Decision

- The parser resolves every image and link reference against the chapter's own href before looking it up in the manifest or spine. The resolvers stay in the parser, and `renderer/epub.Render` takes them as callbacks.
- Links: `http(s)` and `mailto` links and bare `#id` links are kept. A link into another spine document becomes `?chapter=N#id`, and a link into the same chapter becomes `#id`. Any other link (unknown scheme, a file not in the spine) loses its `href`, so it doesn't render as a broken link.
- Output is the `<body>` children only. `class`, `style`, `data-*`, `on*`, `xmlns*` and prefixed attributes like `epub:type` are dropped, as are `<link>`, `<meta>`, `<style>`, `<script>` and comments. `id` is kept because it's the target of links.
- The `?full=1` view is removed. It had no entry point left in the UI, and a cross-chapter link can't be written once at conversion time and still work in both views.

## Consequences

- Only attributes and non-text elements are removed, so block numbering and character offsets are unchanged ([ADR 0001](0001-highlight-anchoring-via-block-id.md)).
- Books converted before this change keep their old output until they are uploaded again.
- Publisher styling is gone. The reader's own CSS styles the output.
