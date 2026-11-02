import tempfile
import unittest
from pathlib import Path, PurePosixPath

import check_comment_blocks as guard

Block = guard.Block


def hash_lines(count: int) -> str:
    return "\n".join(f"# line {n}" for n in range(count)) + "\n"


def slash_lines(count: int) -> str:
    return "\n".join(f"// line {n}" for n in range(count)) + "\n"


def star_block(count: int) -> str:
    inner = "\n".join(" * line" for _ in range(count - 2))
    return f"/**\n{inner}\n */\n"


def long_blocks(text: str, suffix: str) -> list[Block]:
    if suffix == ".py":
        blocks = guard.python_blocks(text)
    else:
        blocks = guard.typescript_blocks(text, jsx=suffix == ".tsx")
    return [block for block in blocks if block.lines >= guard.LIMIT]


def comment_texts(text: str, suffix: str) -> list[str]:
    scan = guard.scan_typescript(text, jsx=suffix == ".tsx")
    return [text[start:end] for start, end in scan.comments]


class PythonBlocks(unittest.TestCase):
    def test_seven_hash_lines_pass(self):
        self.assertEqual(long_blocks(hash_lines(7) + "x = 1\n", ".py"), [])

    def test_eight_hash_lines_fail(self):
        self.assertEqual(
            long_blocks(hash_lines(8) + "x = 1\n", ".py"), [Block(1, 8, 8)]
        )

    def test_bare_hash_line_counts_as_part_of_the_run(self):
        text = hash_lines(4) + "#\n" + hash_lines(3) + "x = 1\n"
        self.assertEqual(long_blocks(text, ".py"), [Block(1, 8, 8)])

    def test_chunks_split_by_a_blank_line_are_one_run(self):
        text = hash_lines(5) + "\n" + hash_lines(5) + "x = 1\n"
        self.assertEqual(long_blocks(text, ".py"), [Block(1, 11, 10)])

    def test_blank_lines_inside_a_run_do_not_count(self):
        text = "\n".join(f"# line {n}\n" for n in range(7)) + "x = 1\n"
        self.assertEqual(long_blocks(text, ".py"), [])

    def test_code_between_two_chunks_keeps_them_separate(self):
        text = hash_lines(6) + "x = 1\n" + hash_lines(6)
        self.assertEqual(long_blocks(text, ".py"), [])

    def test_docstring_does_not_merge_with_the_hash_comments_around_it(self):
        text = hash_lines(5) + "\n" + '"""One.\n\nTwo.\n"""\n' + "\n" + hash_lines(5)
        self.assertEqual(
            guard.python_blocks(text),
            [Block(1, 5, 5), Block(7, 10, 4), Block(12, 16, 5)],
        )

    def test_trailing_comment_after_code_ends_a_run(self):
        text = hash_lines(4) + "x = 1  # trailing\n" + hash_lines(4)
        self.assertEqual(long_blocks(text, ".py"), [])

    def test_hash_inside_a_string_is_code(self):
        lines = "\n".join(f'    "# {n}",' for n in range(8))
        self.assertEqual(long_blocks(f"COLOURS = [\n{lines}\n]\n", ".py"), [])

    def test_seven_line_docstring_passes(self):
        text = 'def f():\n    """One.\n\n    Two.\n    Three.\n    Four.\n    Five.\n'
        self.assertEqual(long_blocks(text + '    """\n', ".py"), [])

    def test_eight_line_docstring_fails(self):
        text = 'def f():\n    """One.\n\n    Two.\n    Three.\n    Four.\n    Five.\n'
        text += '    Six.\n    """\n'
        self.assertEqual(long_blocks(text, ".py"), [Block(2, 9, 8)])

    def test_module_and_class_docstrings_count(self):
        module = '"""' + "\n" * 7 + '"""\n'
        klass = 'class A:\n    """' + "\n" * 7 + '    """\n'
        self.assertEqual(
            long_blocks(module + klass, ".py"), [Block(1, 8, 8), Block(10, 17, 8)]
        )

    def test_async_function_docstring_counts(self):
        text = 'async def f():\n    """' + "\n" * 7 + '    """\n'
        self.assertEqual(long_blocks(text, ".py"), [Block(2, 9, 8)])

    def test_decorated_function_docstring_counts(self):
        text = '@cache\ndef f():\n    """' + "\n" * 7 + '    """\n'
        self.assertEqual(long_blocks(text, ".py"), [Block(3, 10, 8)])

    def test_decorated_class_docstring_counts(self):
        text = '@dataclass\nclass A:\n    """' + "\n" * 7 + '    """\n'
        self.assertEqual(long_blocks(text, ".py"), [Block(3, 10, 8)])

    def test_triple_quoted_non_docstring_is_code(self):
        page = 'PAGE = """<!doctype html>' + "\n<p>line</p>" * 8 + '\n"""\n'
        returned = (
            'def f():\n    x = 1\n    return f"""prompt' + "\nline" * 8 + '\n"""\n'
        )
        self.assertEqual(long_blocks(page + returned, ".py"), [])

    def test_comment_lines_inside_a_string_are_code(self):
        sql = (
            "SQL = '''\n"
            + "\n".join("-- # not a comment" for _ in range(8))
            + "\n'''\n"
        )
        self.assertEqual(long_blocks(sql, ".py"), [])


class TypeScriptBlocks(unittest.TestCase):
    def test_seven_slash_lines_pass(self):
        self.assertEqual(long_blocks(slash_lines(7) + "const x = 1\n", ".ts"), [])

    def test_eight_slash_lines_fail(self):
        self.assertEqual(
            long_blocks(slash_lines(8) + "const x = 1\n", ".ts"), [Block(1, 8, 8)]
        )

    def test_seven_line_star_block_passes(self):
        self.assertEqual(long_blocks(star_block(7) + "const x = 1\n", ".ts"), [])

    def test_eight_line_star_block_fails(self):
        self.assertEqual(
            long_blocks(star_block(8) + "const x = 1\n", ".ts"), [Block(1, 8, 8)]
        )

    def test_star_block_and_slash_lines_form_one_run(self):
        self.assertEqual(
            long_blocks(star_block(4) + slash_lines(4), ".ts"), [Block(1, 8, 8)]
        )

    def test_chunks_split_by_a_blank_line_are_one_run(self):
        text = slash_lines(5) + "\n" + slash_lines(5) + "const x = 1\n"
        self.assertEqual(long_blocks(text, ".ts"), [Block(1, 11, 10)])

    def test_star_blocks_split_by_a_blank_line_are_one_run(self):
        text = star_block(5) + "\n" + star_block(5) + "const x = 1\n"
        self.assertEqual(long_blocks(text, ".ts"), [Block(1, 11, 10)])

    def test_blank_lines_inside_a_run_do_not_count(self):
        text = "\n".join(f"// line {n}\n" for n in range(7)) + "const x = 1\n"
        self.assertEqual(long_blocks(text, ".ts"), [])

    def test_code_between_two_chunks_keeps_them_separate(self):
        text = slash_lines(6) + "const x = 1\n" + slash_lines(6)
        self.assertEqual(long_blocks(text, ".ts"), [])

    def test_trailing_comment_after_code_ends_a_run(self):
        text = slash_lines(4) + "const x = 1 // trailing\n" + slash_lines(4)
        self.assertEqual(long_blocks(text, ".ts"), [])

    def test_jsx_comment_counts_whole(self):
        text = "const el = (\n  <div>\n    {/* one\n        two\n        three\n"
        text += "        four\n        five\n        six\n        seven\n"
        text += "        eight */}\n"
        text += "    <p />\n  </div>\n)\n"
        self.assertEqual(long_blocks(text, ".tsx"), [Block(3, 10, 8)])

    def test_comment_opener_inside_a_string_does_not_swallow_code(self):
        text = "const glob = 'src/**/*.test.ts'\nconst dist = 'dist/*.js'\n"
        text += 'const url = "https://example.com//x"\n' + "const x = 1\n" * 8
        self.assertEqual(long_blocks(text, ".ts"), [])

    def test_template_literal_lines_are_code(self):
        text = (
            "const sql = `\n"
            + "\n".join("// not a comment" for _ in range(8))
            + "\n`\n"
        )
        self.assertEqual(long_blocks(text, ".ts"), [])

    def test_template_expression_holds_strings_and_templates(self):
        text = "const s = `a ${cond ? '/*' : `//${x}`} b\n" + "\n".join(
            "/* x" for _ in range(8)
        )
        self.assertEqual(long_blocks(text + "\n`\n", ".ts"), [])

    def test_comment_opener_inside_a_regex_does_not_swallow_code(self):
        text = (
            "const re = /[/*]+/g\nconst m = s.replace(/^\\/\\//, '')\n"
            + "const x = 1\n" * 8
        )
        self.assertEqual(long_blocks(text, ".ts"), [])

    def test_regex_after_a_keyword_does_not_open_a_comment(self):
        text = "function isRoot(p: string) {\n  return /^\\\\/*$/.test(p)\n}\n"
        self.assertEqual(long_blocks(text + "const x = 1\n" * 8, ".ts"), [])

    def test_backtick_inside_a_regex_does_not_open_a_template(self):
        text = "const tick = /`/.test(s)\n" + slash_lines(8)
        self.assertEqual(long_blocks(text, ".ts"), [Block(2, 9, 8)])

    def test_apostrophe_in_jsx_text_does_not_swallow_the_next_line(self):
        text = "const el = <p>Don't</p>\n" + slash_lines(8)
        self.assertEqual(long_blocks(text, ".tsx"), [Block(2, 9, 8)])

    def test_slashes_in_jsx_text_are_code(self):
        text = "const a = <p>https://x.io</p>\n" + slash_lines(8)
        self.assertEqual(long_blocks(text, ".tsx"), [Block(2, 9, 8)])
        self.assertEqual(comment_texts(text, ".tsx"), slash_lines(8).splitlines())

    def test_comment_opener_in_jsx_text_does_not_swallow_the_next_lines(self):
        text = "const a = <p>/* not a comment</p>\n" + "const x = 1\n" * 8
        self.assertEqual(long_blocks(text, ".tsx"), [])
        self.assertEqual(comment_texts(text, ".tsx"), [])

    def test_jsx_comment_after_an_apostrophe_in_jsx_text_is_a_comment(self):
        text = "const el = <p>Don't {/* note */}</p>\n"
        self.assertEqual(comment_texts(text, ".tsx"), ["/* note */"])

    def test_nested_elements_fragment_and_attribute_expression_lex_as_code(self):
        text = (
            "const el = (\n"
            "  <>\n"
            '    <Outer.Item data-x="a > b" onClick={() => go(/* why */ 1)}>\n'
            "      <p>See https://x.io. Don't</p>\n"
            "      <br />\n"
            "    </Outer.Item>\n"
            "  </>\n"
            ")\n"
        )
        self.assertEqual(comment_texts(text, ".tsx"), ["/* why */"])
        self.assertEqual(long_blocks(text + slash_lines(8), ".tsx"), [Block(9, 16, 8)])

    def test_comment_between_attributes_is_a_comment(self):
        text = (
            "const el = (\n  <button\n"
            + slash_lines(8)
            + '    type="button"\n  />\n)\n'
        )
        self.assertEqual(long_blocks(text, ".tsx"), [Block(3, 10, 8)])

    def test_generics_and_comparisons_do_not_open_jsx(self):
        text = "const id = <T,>(x: T) => x\nif (a < b) {\n}\n" + slash_lines(8)
        self.assertEqual(long_blocks(text, ".tsx"), [Block(4, 11, 8)])

    def test_a_generic_arrow_with_a_constraint_is_refused_rather_than_misread(self):
        text = "const id = <T extends unknown>(x: T) => x\n" + slash_lines(8)
        with self.assertRaises(guard.UnclosedJsx):
            long_blocks(text, ".tsx")

    def test_a_multi_line_attribute_string_does_not_hide_a_later_comment(self):
        text = 'const el = <p title="one\n two">Don\'t {/* note */}</p>\n'
        self.assertEqual(comment_texts(text, ".tsx"), ["/* note */"])
        self.assertEqual(long_blocks(text, ".tsx"), [])

    def test_a_ts_file_never_enters_jsx(self):
        text = "const a = <p>//x\n" + slash_lines(8)
        self.assertEqual(long_blocks(text, ".ts"), [Block(2, 9, 8)])


class Scope(unittest.TestCase):
    def test_source_files_are_in_scope(self):
        for path in (
            "backend/api/views.py",
            "frontend/src/App.tsx",
            "frontend/src/lib/x.ts",
        ):
            self.assertTrue(guard.in_scope(PurePosixPath(path)), path)

    def test_tests_generated_files_and_dependencies_are_excluded(self):
        for path in (
            "backend/api/tests/test_views.py",
            "backend/api/test_views.py",
            "backend/conftest.py",
            "backend/apps/receipts/migrations/0001_initial.py",
            "backend/.venv/lib/site.py",
            "frontend/src/lib/x.test.ts",
            "frontend/src/features/Editor.browser.test.tsx",
            "frontend/src/api/schema.d.ts",
            "frontend/src/i18n/messages/en.ts",
            "frontend/node_modules/x/index.ts",
            "frontend/src/index.css",
        ):
            self.assertFalse(guard.in_scope(PurePosixPath(path)), path)


class Check(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        (self.root / "backend").mkdir()
        (self.root / "frontend" / "src").mkdir(parents=True)

    def tearDown(self):
        self.tmp.cleanup()

    def write(self, rel: str, blocks: int) -> None:
        text = "\n".join(hash_lines(8) + "x = 1\n" for _ in range(blocks)) or "x = 1\n"
        (self.root / rel).write_text(text)

    def test_clean_tree_passes(self):
        self.write("backend/a.py", 0)
        self.write("frontend/src/b.ts", 0)
        self.assertEqual(guard.check(self.root), [])

    def test_long_block_is_a_finding_naming_the_file_and_its_lines(self):
        self.write("backend/a.py", 1)
        self.assertEqual(
            guard.check(self.root),
            ["backend/a.py: 1 long comment block(s)", "  lines 1-8 (8 comment lines)"],
        )

    def test_every_long_block_in_a_file_is_listed(self):
        self.write("backend/a.py", 2)
        findings = guard.check(self.root)
        self.assertEqual(findings[0], "backend/a.py: 2 long comment block(s)")
        self.assertEqual(len(findings), 3)

    def test_file_that_does_not_parse_is_a_finding_that_names_it(self):
        for text in ("def f(:\n", 'x = """\n'):
            (self.root / "backend" / "a.py").write_text(text)
            findings = guard.check(self.root)
            self.assertEqual(len(findings), 1, findings)
            self.assertTrue(
                findings[0].startswith("backend/a.py: does not parse as Python ("),
                findings[0],
            )

    def test_tsx_file_ending_inside_an_element_is_a_finding_that_names_it(self):
        path = self.root / "frontend" / "src" / "a.tsx"
        path.write_text("const id = <T extends unknown>(x: T) => x\n")
        message = "ends inside a JSX element, so the rest of it was not scanned"
        self.assertEqual(guard.check(self.root), [f"frontend/src/a.tsx: {message}"])


if __name__ == "__main__":
    unittest.main()


class GoBlocks(unittest.TestCase):
    def test_go_sources_and_not_go_tests_are_in_scope(self):
        self.assertTrue(guard.in_scope(PurePosixPath("internal/protocol/frame.go")))
        self.assertFalse(guard.in_scope(PurePosixPath("internal/protocol/frame_test.go")))

    def test_eight_slash_lines_in_go_fail(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "internal" / "a.go"
            path.parent.mkdir()
            path.write_text(slash_lines(8) + "package a\n", encoding="utf-8")
            self.assertEqual(len(guard.long_blocks(path)), 1)

    def test_go_raw_string_holding_slashes_is_code(self):
        text = "var s = `\n" + slash_lines(9) + "`\n"
        self.assertEqual(long_blocks(text, ".go"), [])
