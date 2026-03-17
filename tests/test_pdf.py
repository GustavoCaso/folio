from pathlib import Path

import pytest

from pdf_to_md.formats.pdf import convert

FIXTURE = Path(__file__).parent / "fixtures" / "sample.pdf"


@pytest.mark.skipif(not FIXTURE.exists(), reason="sample.pdf fixture not present")
def test_convert_returns_markdown_string() -> None:
    result = convert(FIXTURE)
    assert isinstance(result, str)
    assert len(result) > 0


@pytest.mark.skipif(not FIXTURE.exists(), reason="sample.pdf fixture not present")
def test_convert_contains_content() -> None:
    result = convert(FIXTURE)
    assert "#" in result or len(result) > 100
