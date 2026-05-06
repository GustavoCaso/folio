from unittest.mock import MagicMock, patch

from parser.postprocess import _detect_language, enrich_code_blocks


def test_adds_language_to_untagged_python_block():
    md = "```\nprint('hello')\n```"
    result = enrich_code_blocks(md)
    assert result.startswith("```py")


def test_preserves_existing_language_tag():
    md = "```js\nconsole.log('hi');\n```"
    result = enrich_code_blocks(md)
    assert result.startswith("```js")


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


# --- language detection: false positive regression tests ---


def test_bash_without_shebang_not_detected_as_go():
    code = "export FOO=bar\nif [[ $FOO ]]; then\n  echo yes\nfi\n"
    assert _detect_language(code) == "sh"


def test_bash_with_shebang_not_detected_as_go():
    code = "#!/bin/bash\necho hello\nexport PATH=$PATH:/usr/local/bin\n"
    assert _detect_language(code) == "sh"


def test_go():
    # `:=` and `func` alone should not be enough to trigger go
    code = "func foo() {\n  x := 1\n  return x\n}\n"
    assert _detect_language(code) == "go"


def test_sql_lowercase_not_detected():
    code = "select id from users where active = 1;\n"
    assert _detect_language(code) != "sql"


def test_sql_uppercase_detected():
    code = "SELECT id FROM users WHERE active = 1;\n"
    assert _detect_language(code) == "sql"


def test_rust_detected():
    code = 'fn main() {\n    let mut x = 5;\n    println!("{}", x);\n}\n'
    assert _detect_language(code) == "rs"


def test_typescript_detected():
    code = "interface User {\n  name: string;\n  age: number;\n}\n"
    assert _detect_language(code) == "ts"


# --- prettier integration ---


def test_prettier_formats_javascript_block():
    mock_result = MagicMock()
    mock_result.returncode = 0
    mock_result.stdout = 'console.log("hello");\n'

    with (
        patch("parser.postprocess._PRETTIER_BIN", "/usr/bin/prettier"),
        patch("parser.postprocess.subprocess.run", return_value=mock_result) as mock_run,
    ):
        result = enrich_code_blocks("```js\nconsole.log('hello')\n```")

    mock_run.assert_called_once()
    args = mock_run.call_args[0][0]
    assert "--stdin-filepath" in args
    assert "file.js" in args
    assert 'console.log("hello");\n' in result


def test_prettier_fallback_on_error():
    mock_result = MagicMock()
    mock_result.returncode = 1
    mock_result.stdout = ""

    with (
        patch("parser.postprocess._PRETTIER_BIN", "/usr/bin/prettier"),
        patch("parser.postprocess.subprocess.run", return_value=mock_result),
    ):
        result = enrich_code_blocks("```js\nconsole.log('hello')\n```")

    # original code preserved
    assert "console.log('hello')" in result


def test_prettier_not_called_for_unsupported_language():
    with (
        patch("parser.postprocess._PRETTIER_BIN", "/usr/bin/prettier"),
        patch("parser.postprocess.subprocess.run") as mock_run,
    ):
        enrich_code_blocks("```python\nprint('hi')\n```")

    mock_run.assert_not_called()


def test_prettier_passes_plugin_for_sql():
    mock_result = MagicMock()
    mock_result.returncode = 0
    mock_result.stdout = "SELECT id\nFROM users\nWHERE active = 1;\n"

    with (
        patch("parser.postprocess._PRETTIER_BIN", "/usr/bin/prettier"),
        patch("parser.postprocess.subprocess.run", return_value=mock_result) as mock_run,
    ):
        enrich_code_blocks("```sql\nSELECT id FROM users WHERE active = 1;\n```")

    args = mock_run.call_args[0][0]
    assert "--plugin" in args
    assert "prettier-plugin-sql" in args


# --- new language detection ---


def test_java_detected():
    code = (
        'public class Foo {\n    public void bar() {\n        System.out.println("hi");\n    }\n}\n'
    )
    assert _detect_language(code) == "java"


def test_kotlin_detected():
    code = 'fun main() {\n    val name: String = "world"\n    println("Hello $name")\n}\n'
    assert _detect_language(code) == "kt"


def test_toml_detected():
    code = '[package]\nname = "my-app"\nversion = "0.1.0"\n\n[dependencies]\nserde = "1.0"\n'
    assert _detect_language(code) == "toml"


def test_prettier_passes_plugin_for_bash():
    mock_result = MagicMock()
    mock_result.returncode = 0
    mock_result.stdout = "#!/bin/bash\nexport FOO=bar\n"

    with (
        patch("parser.postprocess._PRETTIER_BIN", "/usr/bin/prettier"),
        patch("parser.postprocess.subprocess.run", return_value=mock_result) as mock_run,
    ):
        enrich_code_blocks("```sh\n#!/bin/bash\nexport FOO=bar\n```")

    args = mock_run.call_args[0][0]
    assert "--plugin" in args
    assert "prettier-plugin-sh" in args
    assert "file.sh" in args


def test_prettier_passes_plugin_for_java():
    mock_result = MagicMock()
    mock_result.returncode = 0
    mock_result.stdout = (
        'public class Foo {\n  public void bar() {\n    System.out.println("hi");\n  }\n}\n'
    )

    with (
        patch("parser.postprocess._PRETTIER_BIN", "/usr/bin/prettier"),
        patch("parser.postprocess.subprocess.run", return_value=mock_result) as mock_run,
    ):
        enrich_code_blocks(
            '```java\npublic class Foo { public void bar() { System.out.println("hi"); } }\n```'
        )

    args = mock_run.call_args[0][0]
    assert "--plugin" in args
    assert "prettier-plugin-java" in args
    assert "file.java" in args


def test_prettier_passes_plugin_for_toml():
    mock_result = MagicMock()
    mock_result.returncode = 0
    mock_result.stdout = '[package]\nname = "my-app"\n'

    with (
        patch("parser.postprocess._PRETTIER_BIN", "/usr/bin/prettier"),
        patch("parser.postprocess.subprocess.run", return_value=mock_result) as mock_run,
    ):
        enrich_code_blocks('[package]\nname = "my-app"\n```toml\n[package]\nname = "my-app"\n```')

    args = mock_run.call_args[0][0]
    assert "--plugin" in args
    assert "prettier-plugin-toml" in args
    assert "file.toml" in args
