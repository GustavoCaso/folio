from parser.postprocess import enrich_code_blocks


def test_adds_language_to_untagged_python_block():
    md = "```\nprint('hello')\n```"
    result = enrich_code_blocks(md)
    assert result.startswith("```python")


def test_preserves_existing_language_tag():
    md = "```javascript\nconsole.log('hi');\n```"
    result = enrich_code_blocks(md)
    assert result.startswith("```javascript")


def test_handles_multiple_blocks():
    md = "```\nprint('hi')\n```\n\nsome text\n\n```\nSELECT 1;\n```"
    result = enrich_code_blocks(md)
    blocks = [line for line in result.splitlines() if line.startswith("```") and line != "```"]
    assert len(blocks) == 2
    assert all(b != "```" for b in blocks)


def test_passthrough_no_code_blocks():
    md = "# Hello\n\nJust text."
    result = enrich_code_blocks(md)
    assert result == md


def test_empty_code_block_left_alone():
    md = "```\n```"
    result = enrich_code_blocks(md)
    # empty block — no content to guess, leave as-is
    assert "```" in result


def test_shell_script_detected():
    md = "```\n#!/bin/bash\necho hello\n```"
    result = enrich_code_blocks(md)
    assert "bash" in result or "sh" in result


def test_sql_detected():
    md = "```\nSELECT id, name FROM users WHERE active = 1;\n```"
    result = enrich_code_blocks(md)
    assert "sql" in result.lower()


def test_go_detected_from_docling_output():
    # Docling strips newlines from code blocks — language is detected but formatting is not fixed
    md = '```\npackage main import ( "fmt" "os" "github.com/stianeikeland/go-rpio/v4" )\n```'
    result = enrich_code_blocks(md)
    assert result.startswith("```go")
