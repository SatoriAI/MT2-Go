# The comment length guard

`scripts/check_comment_blocks.py` fails the gate when a source file holds a comment block of eight lines or more. It comes from the project blueprint, which took it from Slip.

## Why it exists

A comment should carry a decision and the one fact that justifies it — two to four lines. Comments that grow past that start narrating: incidents, measurements, alternatives tried. That is case history, and in a codebase without a guard it accumulates one reasonable paragraph at a time (Slip measured 291 such blocks before the guard existed). Case history belongs in `docs/decisions/`, where it is read when it is wanted instead of on every visit to the file.

A block is a run of lines holding only a comment; a blank line does not end it, a line of code does. Seven lines pass, eight fail.

## Go support

The blueprint scans Python and TypeScript only. This copy adds `.go` under `cmd/` and `internal/`, scanned by the TypeScript scanner: Go's `//` and `/* */` comments and its quoted strings behave the same way for this purpose, and backquoted raw strings are treated as code, which a test pins. `_test.go` files are exempt, as TypeScript test files are.

The script stays a copy of the blueprint's, Python and TypeScript scanning included: Go reuses the TypeScript scanner, the script's own tests are written against Python and TypeScript fixtures, and directories that do not exist here are skipped. Keeping it whole lets it be updated from the blueprint as one file.
