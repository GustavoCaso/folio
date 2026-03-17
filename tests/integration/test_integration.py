from pathlib import Path

import pytest

from pdf_to_md.converter import convert


FIXTURE = Path(__file__).parent.parent / "fixtures" / "sample.pdf"


@pytest.mark.integration
@pytest.mark.skipif(not FIXTURE.exists(), reason="sample.pdf fixture not present")
def test_convert_pdf_produces_markdown(tmp_path: Path) -> None:
    output = convert(FIXTURE, tmp_path / "output.md")
    content = output.read_text()
    assert isinstance(content, str)
    assert len(content) > 0
    assert output.suffix == ".md"
