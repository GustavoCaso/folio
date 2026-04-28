import re

_FENCED_BLOCK = re.compile(r"^```(\w*)\n(.*?)^```", re.MULTILINE | re.DOTALL)

# (language, patterns) — ordered by specificity; first strong match wins
_PATTERNS: list[tuple[str, list[str]]] = [
    ("python", [r"\bdef \w+\s*\(", r"\bimport \w", r"\bfrom \w+ import", r"print\s*\(", r":\s*$"]),
    (
        "sql",
        [r"\bSELECT\b", r"\bINSERT\b", r"\bUPDATE\b", r"\bDELETE\b", r"\bFROM\b", r"\bWHERE\b"],
    ),
    ("bash", [r"^#!\s*/bin/(ba)?sh", r"\becho\b", r"\$\w+", r"\bfi\b", r"\bdone\b"]),
    ("javascript", [r"\bconst\b", r"\blet\b", r"\bvar\b", r"console\.log", r"=>", r"\bfunction\b"]),
    (
        "typescript",
        [r":\s*(string|number|boolean|void)\b", r"\binterface\b", r"\btype\b.*=", r"<\w+>"],
    ),
    ("go", [r"\bfunc\b", r"\bpackage\b", r":=", r"\bfmt\."]),
    ("rust", [r"\bfn\b", r"\blet\b.*\bmut\b", r"\bimpl\b", r"::<"]),
    ("json", [r"^\s*\{", r'":\s*("|\d|\[|\{)']),
    ("yaml", [r"^\w+:\s*\S", r"^-\s+\w"]),
    ("html", [r"<html", r"<div", r"<p>", r"<\w+\s+\w+="]),
    ("css", [r"\w+\s*\{[^}]*:[^}]*\}", r"#\w+\s*\{", r"\.\w+\s*\{"]),
]

_FLAGS = re.IGNORECASE | re.MULTILINE


def _detect_language(code: str) -> str:
    if not code.strip():
        return ""
    scored: list[tuple[str, int]] = []
    for lang, patterns in _PATTERNS:
        hits = sum(1 for p in patterns if re.search(p, code, _FLAGS))
        if hits:
            scored.append((lang, hits))
    if not scored:
        return ""
    scored.sort(key=lambda x: -x[1])
    return scored[0][0]


def enrich_code_blocks(markdown: str) -> str:
    def _replace(m: re.Match[str]) -> str:
        tag = m.group(1)
        code = m.group(2)
        if tag:
            return m.group(0)
        lang = _detect_language(code)
        return f"```{lang}\n{code}```"

    return _FENCED_BLOCK.sub(_replace, markdown)
