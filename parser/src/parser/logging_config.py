"""JSON logging setup for the parser service.

Emits one JSON object per log record to stderr with a stable field order:

    {"ts": "...", "level": "info", "logger": "parser.servicer", "msg": "...", ...extras, "err": "..."}

Use ``extra={...}`` on log calls to attach structured fields (``job_id``,
``filename``, ``pages_done``, etc.).
"""

from __future__ import annotations

import json
import logging
import os
import sys
from datetime import UTC, datetime

_RESERVED = {
    "name",
    "msg",
    "args",
    "levelname",
    "levelno",
    "pathname",
    "filename",
    "module",
    "exc_info",
    "exc_text",
    "stack_info",
    "lineno",
    "funcName",
    "created",
    "msecs",
    "relativeCreated",
    "thread",
    "threadName",
    "processName",
    "process",
    "taskName",
    "message",
    "asctime",
}


class JSONFormatter(logging.Formatter):
    """Serialize a LogRecord as a single-line JSON object."""

    def format(self, record: logging.LogRecord) -> str:
        payload: dict[str, object] = {
            "ts": datetime.fromtimestamp(record.created, tz=UTC).isoformat(),
            "level": record.levelname.lower(),
            "logger": record.name,
            "msg": record.getMessage(),
        }

        for key, value in record.__dict__.items():
            if key in _RESERVED or key.startswith("_"):
                continue
            payload[key] = value

        if record.exc_info:
            payload["err"] = self.formatException(record.exc_info)
        elif record.exc_text:
            payload["err"] = record.exc_text

        return json.dumps(payload, default=str)


def _parse_level(value: str | None) -> int:
    if not value:
        return logging.INFO
    name = value.strip().upper()
    if name == "WARNING":
        name = "WARN"
    mapping = {
        "DEBUG": logging.DEBUG,
        "INFO": logging.INFO,
        "WARN": logging.WARNING,
        "ERROR": logging.ERROR,
    }
    return mapping.get(name, logging.INFO)


def configure(level: str | None = None) -> None:
    """Install the JSON formatter on the root logger.

    Reads ``LOG_LEVEL`` from the environment when ``level`` is not given.
    Replaces any existing handlers so repeated calls are idempotent.
    """
    resolved = _parse_level(level if level is not None else os.environ.get("LOG_LEVEL"))

    root = logging.getLogger()
    root.setLevel(resolved)
    # Replace handlers so calling configure() twice doesn't stack them.
    for h in list(root.handlers):
        root.removeHandler(h)

    handler = logging.StreamHandler(sys.stderr)
    handler.setFormatter(JSONFormatter())
    root.addHandler(handler)

    # Docling is very chatty at DEBUG. Cap to INFO unless we're explicitly
    # running the service at DEBUG.
    if resolved > logging.DEBUG:
        logging.getLogger("docling").setLevel(logging.INFO)
