import io
import json
import logging

from parser.logging_config import JSONFormatter, configure


def _make_record(level: int = logging.INFO, extras: dict | None = None) -> logging.LogRecord:
    rec = logging.LogRecord(
        name="parser.servicer",
        level=level,
        pathname=__file__,
        lineno=1,
        msg="hello %s",
        args=("world",),
        exc_info=None,
    )
    for k, v in (extras or {}).items():
        setattr(rec, k, v)
    return rec


def test_json_formatter_core_fields():
    out = JSONFormatter().format(_make_record())
    decoded = json.loads(out)
    assert decoded["level"] == "info"
    assert decoded["logger"] == "parser.servicer"
    assert decoded["msg"] == "hello world"
    assert "ts" in decoded


def test_json_formatter_includes_extras():
    out = JSONFormatter().format(_make_record(extras={"job_id": "abc", "pages_done": 3}))
    decoded = json.loads(out)
    assert decoded["job_id"] == "abc"
    assert decoded["pages_done"] == 3


def test_json_formatter_captures_exception():
    try:
        raise ValueError("boom")
    except ValueError:
        import sys

        rec = logging.LogRecord(
            name="parser.x",
            level=logging.ERROR,
            pathname=__file__,
            lineno=1,
            msg="failed",
            args=(),
            exc_info=sys.exc_info(),
        )
    decoded = json.loads(JSONFormatter().format(rec))
    assert decoded["level"] == "error"
    assert "boom" in decoded["err"]


def test_configure_level_filters(capsys):
    configure("warn")
    log = logging.getLogger("parser.test_level")
    log.debug("debug-line")
    log.info("info-line")
    log.warning("warn-line", extra={"job_id": "j1"})

    err = capsys.readouterr().err.strip().splitlines()
    assert len(err) == 1
    decoded = json.loads(err[0])
    assert decoded["msg"] == "warn-line"
    assert decoded["job_id"] == "j1"


def test_configure_is_idempotent():
    configure("info")
    configure("info")
    root = logging.getLogger()
    # Exactly one handler after re-configure.
    stream_handlers = [h for h in root.handlers if isinstance(h, logging.StreamHandler)]
    assert len(stream_handlers) == 1


def test_configure_writes_to_stderr_by_default(capsys):
    configure("info")
    logging.getLogger("parser.flow").info("ping", extra={"k": "v"})
    captured = capsys.readouterr()
    assert captured.out == ""
    decoded = json.loads(captured.err.strip())
    assert decoded["msg"] == "ping"
    assert decoded["k"] == "v"


def test_json_formatter_accepts_non_serializable_extras():
    class Weird:
        def __repr__(self) -> str:
            return "<weird>"

    out = JSONFormatter().format(_make_record(extras={"thing": Weird()}))
    decoded = json.loads(out)
    assert decoded["thing"] == "<weird>"


def test_configure_defaults_to_info_when_unset(monkeypatch):
    monkeypatch.delenv("LOG_LEVEL", raising=False)
    configure()
    assert logging.getLogger().level == logging.INFO


def test_configure_writes_to_buffer_when_stream_replaced():
    buf = io.StringIO()
    # Directly using JSONFormatter on a custom handler for coverage of manual wiring.
    h = logging.StreamHandler(buf)
    h.setFormatter(JSONFormatter())
    log = logging.getLogger("parser.manual")
    log.handlers = [h]
    log.setLevel(logging.INFO)
    log.propagate = False
    log.info("direct", extra={"x": 1})
    log.handlers = []
    decoded = json.loads(buf.getvalue().strip())
    assert decoded["x"] == 1
