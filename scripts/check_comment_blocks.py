"""Fail when a source file holds a comment block of eight lines or more.

A block is a maximal run of lines that hold a comment and nothing else, where only a
line of code ends the run and a blank line neither ends it nor counts; or a Python
docstring, sized by its span. Go files are scanned like TypeScript: `//` and `/* */`. Reasoning in `docs/decisions/comment-blocks.md`.
"""

from __future__ import annotations

import ast
import io
import sys
import tokenize
from dataclasses import dataclass
from enum import Enum, auto
from pathlib import Path, PurePosixPath

ROOT = Path(__file__).resolve().parent.parent
LIMIT = 8
# The directories holding this project's own source. A scope that does not exist is
# skipped, so one list serves a backend-only repo and a backend plus frontend one.
SCOPES = ("backend", "frontend/src", "src", "cmd", "internal")
SUFFIXES = {".py", ".ts", ".tsx", ".go"}
EXCLUDED_DIRS = {"tests", "migrations", ".venv", "node_modules", "__pycache__"}
EXCLUDED_PATHS = (
    "frontend/src/api/schema.d.ts",
    "frontend/src/i18n/messages/",
    "src/api/schema.d.ts",
    "src/i18n/messages/",
)
TRIVIAL_TOKENS = {
    tokenize.COMMENT,
    tokenize.NL,
    tokenize.NEWLINE,
    tokenize.INDENT,
    tokenize.DEDENT,
    tokenize.ENDMARKER,
    tokenize.ENCODING,
}
DOCSTRING_HOLDERS = (ast.Module, ast.ClassDef, ast.FunctionDef, ast.AsyncFunctionDef)
# What may precede a `/` that opens a regex literal rather than dividing.
REGEX_AFTER_CHARS = set("(,=:[!&|?{};+-*%<>~^")
REGEX_AFTER_WORDS = {
    "await",
    "case",
    "delete",
    "do",
    "else",
    "in",
    "instanceof",
    "new",
    "of",
    "return",
    "throw",
    "typeof",
    "void",
    "yield",
}
WHAT_TO_DO = (
    f"A comment block of {LIMIT} comment lines or more explains too much: keep the "
    "decision in a sentence and move the history to docs/decisions/."
)


class UnclosedJsx(ValueError):
    """A `.tsx` file that ends inside an element, so what followed was scanned wrong."""


class Jsx(Enum):
    """The two frames JSX adds to the scanner: inside `<Name ...>`, and among an
    element's children, where text is code however much it looks like a comment."""

    TAG = auto()
    CHILDREN = auto()


@dataclass(frozen=True, order=True)
class Block:
    start: int
    end: int
    lines: int


def in_scope(rel: PurePosixPath) -> bool:
    if rel.suffix not in SUFFIXES:
        return False
    if EXCLUDED_DIRS & set(rel.parts[:-1]):
        return False
    if any(rel.as_posix().startswith(prefix) for prefix in EXCLUDED_PATHS):
        return False
    if rel.suffix == ".py" and (
        rel.name.startswith("test_") or rel.name == "conftest.py"
    ):
        return False
    return not rel.name.endswith((".test.ts", ".test.tsx", "_test.go"))


def source_files(root: Path) -> list[Path]:
    files = []
    for scope in SCOPES:
        if not (root / scope).is_dir():
            continue
        for path in sorted((root / scope).rglob("*")):
            if path.is_file() and in_scope(
                PurePosixPath(path.relative_to(root).as_posix())
            ):
                files.append(path)
    return files


def comment_runs(comment_lines: set[int], code_lines: set[int]) -> list[Block]:
    """Group comment-only lines into runs: only code ends one; blanks do not count."""
    runs: list[Block] = []
    for line in sorted(comment_lines - code_lines):
        last = runs[-1] if runs else None
        if last and code_lines.isdisjoint(range(last.end + 1, line)):
            runs[-1] = Block(last.start, line, last.lines + 1)
        else:
            runs.append(Block(line, line, 1))
    return runs


def python_blocks(text: str) -> list[Block]:
    code_lines: set[int] = set()
    comment_lines: set[int] = set()
    for token in tokenize.generate_tokens(io.StringIO(text).readline):
        if token.type == tokenize.COMMENT:
            comment_lines.add(token.start[0])
        elif token.type not in TRIVIAL_TOKENS:
            code_lines.update(range(token.start[0], token.end[0] + 1))
    blocks = comment_runs(comment_lines, code_lines)
    for node in ast.walk(ast.parse(text)):
        if isinstance(node, DOCSTRING_HOLDERS) and node.body:
            first = node.body[0]
            if (
                isinstance(first, ast.Expr)
                and isinstance(first.value, ast.Constant)
                and isinstance(first.value.value, str)
                and first.end_lineno is not None
            ):
                blocks.append(
                    Block(
                        first.lineno,
                        first.end_lineno,
                        first.end_lineno - first.lineno + 1,
                    )
                )
    return sorted(blocks)


def string_end(text: str, start: int, quote: str) -> int:
    """Index just past the closing quote, or of the newline that ends an open string."""
    i = start + 1
    while i < len(text):
        char = text[i]
        if char == "\\":
            i += 2
        elif char == quote:
            return i + 1
        elif char == "\n":
            return i
        else:
            i += 1
    return i


def regex_end(text: str, start: int) -> int | None:
    """Index just past a regex literal's closing `/`, or None if the line ends first."""
    i = start + 1
    in_class = False
    while i < len(text):
        char = text[i]
        if char == "\n":
            return None
        if char == "\\":
            i += 2
            continue
        if in_class:
            in_class = char != "]"
        elif char == "[":
            in_class = True
        elif char == "/":
            return i + 1
        i += 1
    return None


def block_comment(text: str, start: int) -> tuple[int, int, int] | None:
    """The comment's own `[start, end)` and the index to resume at, for a `/* */` or JSX
    `{/* */}` comment opening at `start`; None where there is none."""
    i = start
    if text[i] == "{":
        i += 1
        while i < len(text) and text[i].isspace():
            i += 1
    if not text.startswith("/*", i):
        return None
    close = text.find("*/", i + 2)
    end = len(text) if close == -1 else close + 2
    if text[start] != "{":
        return i, end, end
    resume = end
    while resume < len(text) and text[resume].isspace():
        resume += 1
    return (i, end, resume + 1) if text.startswith("}", resume) else None


def opens_element(text: str, i: int) -> bool:
    """Whether the `<` at `i` starts a JSX element or fragment: `<>`, or a name followed
    by whitespace, `>`, `/` or `{`, so `<T,>(x) => x` and `a < b` stay operators."""
    j = i + 1
    if text.startswith(">", j):
        return True
    if j >= len(text) or not (text[j].isalpha() or text[j] in "_$"):
        return False
    while j < len(text) and (text[j].isalnum() or text[j] in "_$.-:"):
        j += 1
    return j < len(text) and (text[j].isspace() or text[j] in ">/{")


@dataclass(frozen=True)
class Scan:
    code_lines: set[int]
    comment_lines: set[int]
    # `[start, end)` of each comment's own text, the braces of a JSX comment excluded.
    comments: list[tuple[int, int]]
    unclosed_jsx: bool = False


def scan_typescript(text: str, *, jsx: bool) -> Scan:
    """Which lines carry code and which a comment, by lexing strings, regexes and, in a
    `.tsx` file, JSX elements."""
    code_lines: set[int] = set()
    comment_lines: set[int] = set()
    comments: list[tuple[int, int]] = []
    # One frame per open template literal (None), `${` or `{` expression (its brace
    # depth), JSX tag or element body.
    frames: list[int | Jsx | None] = [0]
    last_char = ""
    last_word = ""
    line = 1
    i = 0
    n = len(text)

    def consume(end: int) -> None:
        nonlocal i, line
        code_lines.update(range(line, line + text.count("\n", i, end) + 1))
        line += text.count("\n", i, end)
        i = end

    def record_comment(found: tuple[int, int, int]) -> None:
        nonlocal i, line
        start, end, resume = found
        comment_lines.update(range(line, line + text.count("\n", i, resume) + 1))
        comments.append((start, end))
        line += text.count("\n", i, resume)
        i = resume

    def open_element() -> None:
        nonlocal i
        code_lines.add(line)
        if text.startswith("<>", i):
            frames.append(Jsx.CHILDREN)
            i += 2
        else:
            frames.append(Jsx.TAG)
            i += 1

    def close_element() -> None:
        # Back in code, the element is an operand: a `/` after it divides.
        nonlocal last_char, last_word
        frames.pop()
        last_char, last_word = "a", ""

    while i < n:
        char = text[i]
        depth = frames[-1]
        if char == "\n":
            line += 1
            i += 1
        elif depth is None:
            code_lines.add(line)
            if char == "\\":
                consume(i + 2)
            elif char == "`":
                frames.pop()
                i += 1
            elif text.startswith("${", i):
                frames.append(0)
                i += 2
            else:
                i += 1
        elif char in " \t\r":
            i += 1
        elif depth is Jsx.CHILDREN:
            if text.startswith("</", i):
                end = text.find(">", i)
                consume(n if end == -1 else end + 1)
                close_element()
            elif char == "<" and opens_element(text, i):
                open_element()
            elif char == "{" and (found := block_comment(text, i)) is not None:
                record_comment(found)
            elif char == "{":
                code_lines.add(line)
                frames.append(0)
                i += 1
                last_char, last_word = char, ""
            else:
                code_lines.add(line)
                i += 1
        elif text.startswith("//", i):
            comment_lines.add(line)
            end = text.find("\n", i)
            end = n if end == -1 else end
            comments.append((i, end))
            i = end
        elif (found := block_comment(text, i)) is not None:
            record_comment(found)
        elif depth is Jsx.TAG:
            # A comment between two attributes is one, so the tag is checked after them.
            code_lines.add(line)
            if char in "'\"":
                # A JSX attribute string has no escapes and may span lines.
                quote_end = text.find(char, i + 1)
                consume(n if quote_end == -1 else quote_end + 1)
            elif char == "{":
                frames.append(0)
                i += 1
                last_char, last_word = char, ""
            elif text.startswith("/>", i):
                i += 2
                close_element()
            elif char == ">":
                frames[-1] = Jsx.CHILDREN
                i += 1
            else:
                i += 1
        else:
            code_lines.add(line)
            if char in "'\"":
                consume(string_end(text, i, char))
                last_char, last_word = "a", ""
            elif char == "`":
                frames.append(None)
                i += 1
                last_char, last_word = "a", ""
            elif char == "{":
                frames[-1] = depth + 1
                i += 1
                last_char, last_word = char, ""
            elif char == "}":
                if depth == 0 and len(frames) > 1:
                    frames.pop()
                else:
                    frames[-1] = max(depth - 1, 0)
                i += 1
                last_char, last_word = char, ""
            elif char.isalnum() or char in "_$":
                end = i + 1
                while end < n and (text[end].isalnum() or text[end] in "_$"):
                    end += 1
                last_word = text[i:end]
                last_char = "a"
                i = end
            elif char == "/" and (
                last_char == ""
                or last_char in REGEX_AFTER_CHARS
                or last_word in REGEX_AFTER_WORDS
            ):
                close = regex_end(text, i)
                i = close if close is not None else i + 1
                last_char, last_word = ("a", "") if close is not None else (char, "")
            elif (
                jsx
                and char == "<"
                and opens_element(text, i)
                and (
                    last_char == ""
                    or last_char in REGEX_AFTER_CHARS
                    or last_word in REGEX_AFTER_WORDS
                )
            ):
                open_element()
            else:
                i += 1
                last_char, last_word = char, ""
    unclosed = any(isinstance(frame, Jsx) for frame in frames)
    return Scan(code_lines, comment_lines, comments, unclosed)


def typescript_blocks(text: str, *, jsx: bool) -> list[Block]:
    scan = scan_typescript(text, jsx=jsx)
    if scan.unclosed_jsx:
        # A generic arrow such as `<T extends X>(x) => x` opens an element that never
        # closes, and every comment after it would silently go uncounted.
        raise UnclosedJsx(
            "ends inside a JSX element, so the rest of it was not scanned"
        )
    return comment_runs(scan.comment_lines, scan.code_lines)


def long_blocks(path: Path) -> list[Block]:
    text = path.read_text(encoding="utf-8")
    if path.suffix == ".py":
        blocks = python_blocks(text)
    else:
        blocks = typescript_blocks(text, jsx=path.suffix == ".tsx")
    return [block for block in blocks if block.lines >= LIMIT]


def check(root: Path) -> list[str]:
    findings: list[str] = []
    for path in source_files(root):
        rel = path.relative_to(root).as_posix()
        try:
            blocks = long_blocks(path)
        except (SyntaxError, tokenize.TokenError) as error:
            findings.append(f"{rel}: does not parse as Python ({error})")
            continue
        except UnclosedJsx as error:
            findings.append(f"{rel}: {error}")
            continue
        if not blocks:
            continue
        findings.append(f"{rel}: {len(blocks)} long comment block(s)")
        findings.extend(
            f"  lines {block.start}-{block.end} ({block.lines} comment lines)"
            for block in blocks
        )
    return findings


def main(argv: list[str]) -> int:
    if argv:
        raise SystemExit(f"usage: {Path(__file__).name}")
    findings = check(ROOT)
    if findings:
        print("\n".join(findings))
        print()
        print(WHAT_TO_DO)
        return 1
    print(f"comment blocks: {len(source_files(ROOT))} files checked, none too long")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
