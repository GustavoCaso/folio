# Extract PDF images via pypdfium2 crops, not Docling's in-memory image cache

Docling converts with `generate_picture_images=False`, so it never builds an in-memory image cache. After conversion, each `PictureItem`'s bounding box (from `doc.pictures`) is used to crop-render that one region from the source PDF via pypdfium2, one image at a time, and `<!-- image -->` placeholders in the markdown are rewritten in order to `![Image](filename.png)`.

Docling's built-in `generate_picture_images=True` path holds every extracted image in memory simultaneously — peak memory scales with the number and size of images in the document. Rendering crops on demand from the original PDF keeps peak memory flat regardless of document size, at the cost of a second pass over the document's picture provenance data (bounding box + page number) instead of reading a ready-made cache.
