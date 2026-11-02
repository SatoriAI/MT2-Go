# Decision archive

Why the rules in `CLAUDE.md` are what they are: the reasoning, the measurements, the alternatives that were tried and rejected, and the bugs that taught each lesson.

**These are records, not rules.** `CLAUDE.md` is the authority — where a file here disagrees with it, the file is out of date. Read the one that covers the area you are working in; do not load the directory.

| File | Covers |
|---|---|
| `architecture.md` | One process with two ports, PostgreSQL, the migration runner, the dependency budget |
| `claude-md-budget.md` | Why `CLAUDE.md` has a character budget, why 10,000, and where it is checked |
| `comment-blocks.md` | The comment length guard, and how Go was added to it |

How a game mechanic works is not a decision and does not live here: see `docs/mechanics/`.

## Adding to the archive

Create a file when a rule needs more than a clause of justification: the rule stays in `CLAUDE.md`, the justification comes here. When a decision is superseded, move the old reasoning here rather than leaving it in place marked retired.

A lesson that is not yet a rule belongs in the guidelines inbox your global agent instructions point at, not here.
