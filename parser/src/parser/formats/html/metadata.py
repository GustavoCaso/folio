from __future__ import annotations

import base64
import logging
from html.parser import HTMLParser
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from pathlib import Path

logger = logging.getLogger(__name__)

_OG_IMAGE = "og:image"


class _MetadataParser(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.title = ""
        self.author = ""
        self.og_image = ""
        self._in_title = False

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        attr_dict = {k.lower(): v for k, v in attrs}
        if tag.lower() == "title":
            self._in_title = True
        elif tag.lower() == "meta":
            name = (attr_dict.get("name") or "").lower()
            prop = (attr_dict.get("property") or "").lower()
            content = attr_dict.get("content") or ""
            if name == "author":
                self.author = content
            elif prop == _OG_IMAGE or name == _OG_IMAGE:
                self.og_image = content

    def handle_data(self, data: str) -> None:
        if self._in_title and not self.title:
            self.title = data.strip()

    def handle_endtag(self, tag: str) -> None:
        if tag.lower() == "title":
            self._in_title = False


def _decode_cover(og_image: str) -> bytes:
    """Decode a data: URI cover image. Returns empty bytes for non-data URIs."""
    if not og_image.startswith("data:"):
        return b""
    try:
        _header, encoded = og_image.split(",", 1)
        return base64.b64decode(encoded)
    except Exception:
        return b""


def extract_metadata(html_path: Path, generate_cover: bool) -> tuple[str, str, bytes]:
    """Return (title, author, cover) from HTML file. Empty strings/bytes if absent."""
    try:
        content = html_path.read_text(encoding="utf-8", errors="replace")
        parser = _MetadataParser()
        parser.feed(content)
        cover = b""
        if generate_cover:
            cover = _decode_cover(parser.og_image)
        return parser.title, parser.author, cover
    except Exception:
        logger.warning("HTML metadata extraction failed", exc_info=True)
        return "", "", b""
