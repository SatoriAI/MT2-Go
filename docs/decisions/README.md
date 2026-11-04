# Decision archive

Why the rules in `AGENTS.md` are what they are: the reasoning, the measurements, the alternatives that were tried and rejected, and the bugs that taught each lesson.

**These are records, not rules.** `AGENTS.md` is the authority — where a file here disagrees with it, the file is out of date. Read the one that covers the area you are working in; do not load the directory.

| File | Covers |
|---|---|
| `architecture.md` | One process with two ports, PostgreSQL, the migration runner, the dependency budget |
| `instructions-budget.md` | Why the rules live in `AGENTS.md`, why it has a character budget of 10,000, and where it is checked |
| `client-adapter.md` | Why the server models the game rather than the original client's protocol, and when `internal/game/wire.go` goes |
| `comment-blocks.md` | The comment length guard, and how Go was added to it |
| `connections.md` | Why each connection has a send queue and a writer, so other connections can send to it |
| `world-loop.md` | Why monsters are driven by one ticking loop, and why mobs see no mobs |

How a game mechanic works is not a decision and does not live here: see `docs/mechanics/`.

## Adding to the archive

Create a file when a rule needs more than a clause of justification: the rule stays in `AGENTS.md`, the justification comes here. When a decision is superseded, move the old reasoning here rather than leaving it in place marked retired.
