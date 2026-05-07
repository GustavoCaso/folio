import logging
import re
import shutil
import subprocess

_FENCED_BLOCK = re.compile(r"^```(\w*)\n(.*?)^```", re.MULTILINE | re.DOTALL)

_log = logging.getLogger(__name__)

_SUPPORTED_PRETTIER_EXTENSIONS: list[str] = [
    "js",
    "ts",
    "json",
    "yaml",
    "html",
    "css",
    "sql",
    "sh",
    "java",
    "toml",
]

_PRETTIER_BIN: str | None = shutil.which("prettier")
if _PRETTIER_BIN is None:
    _log.warning("prettier not found in PATH — code block formatting will be skipped")

_FLAGS = re.IGNORECASE | re.MULTILINE
_FLAGS_CS = re.MULTILINE  # case-sensitive


# (language, patterns, flags) — ordered by specificity; first strong match wins.
# SQL uses case-sensitive flags so only uppercase keywords match.
_PATTERNS: list[tuple[str, list[str], int]] = [
    (
        "sh",
        [
            r"^#!\s*/bin/(ba)?sh",
            r"\[\[",
            r"(?:^|[\s;|&])\$\{?\w+\}?",  # $var not inside a quoted string
            r"\bfi\b",
            r"\bdone\b",
            r"\b(export|source|alias|chmod|mkdir|cd|rm|cp|mv|grep|awk|sed|\|_)\b",
            r"^\w+\s*\(\)\s*\{",  # function definition: foo() { at line start
        ],
        _FLAGS,
    ),
    (
        "py",
        [
            r"\bdef \w+\s*\(",
            r"\bimport \w",
            r"\bfrom \w+ import",
            r"print\s*\(",
            r"\bself\b",
            r"\bNone\b",
        ],
        _FLAGS,
    ),
    (
        "sql",
        [
            r"\bSELECT\b",
            r"\bINSERT\b",
            r"\bUPDATE\b",
            r"\bDELETE\b",
            r"\bFROM\b",
            r"\bWHERE\b",
            r"\bJOIN\b",
            r"\bGROUP\b",
            r"\bORDER\b",
        ],
        _FLAGS_CS,
    ),
    (
        "js",
        [
            r"\bconsole\.log\b",
            r"\brequire\s*\(",
            r"=>",
            r"\bfunction\b",
            r"\bdocument\b",
            r"\bwindow\b",
        ],
        _FLAGS,
    ),
    (
        "ts",
        [
            r":\s*(string|number|boolean|void|never|unknown)\b",
            r"\binterface\b",
            r"\btype\b\s+\w+\s*=",
            r"\bas\s+\w+\b",
            r"\benum\b",
        ],
        _FLAGS,
    ),
    (
        "go",
        [
            r"\bpackage\s+\w+",
            r"\bfmt\.\w+\s*\(",
            r'import\s+"[^"]+"\s*$',
            r"\bgoroutine\b",
            r"\bchan\b",
            r":=",
        ],
        _FLAGS,
    ),
    (
        "rs",
        [
            r"\bfn\s+\w+\s*\(",
            r"\blet\s+mut\b",
            r"\bimpl\b",
            r"\bmatch\b.*\{",
            r"::<",
            r"\bOption<",
            r"\bResult<",
        ],
        _FLAGS,
    ),
    (
        "java",
        [
            r"\bpublic\s+(class|interface|enum)\b",
            r"\bSystem\.out\.",
            r"@Override\b",
            r"\bvoid\s+\w+\s*\(",
            r"\bnew\s+\w+\s*\(",
            r"\bimport\s+java\.",
        ],
        _FLAGS,
    ),
    (
        "kt",
        [
            r"\bfun\s+\w+\s*\(",
            r"\bval\s+\w+",
            r"\bvar\s+\w+\s*:",
            r"\bdata\s+class\b",
            r"\bobject\s*\{",
            r"\bcompanion\s+object\b",
        ],
        _FLAGS,
    ),
    (
        "toml",
        [r"^\[[\w.]+\]", r"^\w[\w-]*\s*=\s*", r"^\[\[[\w.]+\]\]"],
        _FLAGS,
    ),
    ("json", [r'^\s*\{\s*"', r'":\s*("|\d|\[|\{|true|false|null)'], _FLAGS),
    ("yaml", [r"^---", r"^\w[\w-]*:\s+\S", r"^-\s+\w"], _FLAGS),
    ("html", [r"<!DOCTYPE\s+html", r"<html[\s>]", r"<(div|span|p|a|ul|li|head|body)[\s>]"], _FLAGS),
    (
        "css",
        [
            r"::-webkit-\w+",
            r"\b\w[\w-]*\s*:\s*\d+(?:px|em|rem|%|vh|vw|pt)\b",
            r"[.#][\w-]+\s*\{",
            r"\b(background-color|border-radius|font-size|z-index|max-width|min-height|overflow|position|display)\s*:",
            r":\s*#[0-9a-fA-F]{3,6}\b",
            r"\brgba?\(\s*\d+",
        ],
        _FLAGS,
    ),
]


def _detect_language(code: str) -> str:
    if not code.strip():
        return ""

    scored: list[tuple[str, int]] = []
    for lang, patterns, flags in _PATTERNS:
        hits = sum(1 for p in patterns if re.search(p, code, flags))
        if hits:
            scored.append((lang, hits))
    if not scored:
        return ""
    scored.sort(key=lambda x: -x[1])
    return scored[0][0]


# Languages that require an explicit --plugin flag
_PRETTIER_PLUGINS: dict[str, str] = {
    "sql": "prettier-plugin-sql",
    "sh": "prettier-plugin-sh",
    "java": "prettier-plugin-java",
    "toml": "prettier-plugin-toml",
}


def _prettier_format(code: str, lang: str) -> str:
    supported = lang in _SUPPORTED_PRETTIER_EXTENSIONS
    if not supported or not _PRETTIER_BIN:
        return code
    cmd = [_PRETTIER_BIN, "--stdin-filepath", f"file.{lang}"]
    if plugin := _PRETTIER_PLUGINS.get(lang):
        cmd += ["--plugin", plugin]
    try:
        result = subprocess.run(
            cmd,
            input=code,
            capture_output=True,
            text=True,
            timeout=10,
        )
        if result.returncode == 0:
            return result.stdout
    except Exception:
        _log.debug("prettier failed for %s", lang, exc_info=True)
    return code


def enrich_code_blocks(markdown: str) -> str:
    def _replace(m: re.Match[str]) -> str:
        tag = m.group(1)
        code = m.group(2)
        if tag:
            formatted = _prettier_format(code, tag)
            return f"```{tag}\n{formatted}```"
        lang = _detect_language(code)
        formatted = _prettier_format(code, lang)
        return f"```{lang}\n{formatted}```"

    return _FENCED_BLOCK.sub(_replace, markdown)
