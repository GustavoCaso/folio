from __future__ import annotations

import base64
import logging
import urllib.request
from html.parser import HTMLParser
from typing import TYPE_CHECKING

if TYPE_CHECKING:
    from pathlib import Path

logger = logging.getLogger(__name__)

_OG_IMAGE = "og:image"
_OG_TITLE = "og:title"
_FETCH_TIMEOUT = 10


class _MetadataParser(HTMLParser):
    def __init__(self) -> None:
        super().__init__()
        self.title = ""
        self.og_title = ""
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
            elif prop == _OG_TITLE or name == _OG_TITLE:
                self.og_title = content

    def handle_data(self, data: str) -> None:
        if self._in_title and not self.title:
            self.title = data.strip()

    def handle_endtag(self, tag: str) -> None:
        if tag.lower() == "title":
            self._in_title = False


def _decode_cover(og_image: str) -> bytes:
    """Decode a cover image from a data: URI or fetch from an https:// URL."""
    if not og_image:
        return b""
    if og_image.startswith("data:"):
        try:
            _header, encoded = og_image.split(",", 1)
            return base64.b64decode(encoded)
        except Exception:
            return b""
    if og_image.startswith("http://") or og_image.startswith("https://"):
        try:
            req = urllib.request.Request(
                og_image,
                headers={"User-Agent": "Mozilla/5.0 (compatible; Folio/1.0)"},
            )
            with urllib.request.urlopen(req, timeout=_FETCH_TIMEOUT) as resp:  # noqa: S310
                data: bytes = resp.read()
                return data
        except Exception:
            logger.debug("Failed to fetch og:image %s", og_image)
            return b""
    return b""


def _parse_and_build(content: str, generate_cover: bool) -> tuple[str, str, bytes]:
    parser = _MetadataParser()
    parser.feed(content)
    title = parser.og_title or parser.title
    cover = _decode_cover(parser.og_image) if generate_cover else b""
    return title, parser.author, cover


def extract_metadata(html_path: Path, generate_cover: bool) -> tuple[str, str, bytes]:
    """Return (title, author, cover) from HTML file. Empty strings/bytes if absent."""
    try:
        content = html_path.read_text(encoding="utf-8", errors="replace")
        return _parse_and_build(content, generate_cover)
    except Exception:
        logger.warning("HTML metadata extraction failed", exc_info=True)
        return "", "", b""


def extract_metadata_from_url(url: str, generate_cover: bool) -> tuple[str, str, bytes]:
    """Return (title, author, cover) by fetching HTML from a URL."""
    try:
        req = urllib.request.Request(
            url,
            headers={"User-Agent": "Mozilla/5.0 (compatible; Folio/1.0)"},
        )
        with urllib.request.urlopen(req, timeout=_FETCH_TIMEOUT) as resp:  # noqa: S310
            raw: bytes = resp.read()
        content = raw.decode("utf-8", errors="replace")
        return _parse_and_build(content, generate_cover)
    except Exception:
        logger.warning("HTML metadata extraction from URL failed", exc_info=True)
        return "", "", b""
