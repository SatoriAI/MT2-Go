# MT2 Go — working conventions

A Metin2 server written from scratch in Go, compatible with the original TMP4 game client, built so that every mechanic is understood and written down. Go 1.27 + PostgreSQL 17.

**This file is the rules, for every agent and person working here; `CLAUDE.md` only imports it. The reasoning is in `docs/decisions/`**, indexed at the bottom. Read the one file covering the area you are working in; do not load them all. **Commits, pull requests and where code may come from are in [`CONTRIBUTING.md`](CONTRIBUTING.md)**, written for people and agents alike; follow it, and do not repeat it here.

## The bar

- **Small, single-purpose packages with obvious names.** If a file needs a paragraph to say what belongs in it, split it.
- **No speculative abstraction.** Build for today; promote a repeated idiom on its third occurrence.
- **No dead code and no commented-out code.** If it is unused, delete it.
- **Comments explain why, never what.** Case history belongs in `docs/decisions/`; a comment block of eight lines or more fails the gate.
- **Handle errors where they can be answered.** No defensive branches for states your own code makes impossible.
- **Tests read as documentation** — one behaviour per test, named for the behaviour (`TestAWrongPasswordIsRefusedWithTheStatusTheClientShows`).
- **When a rule's premise goes, the rule goes with it.**
- Consistency beats preference. Pick one way, do it everywhere, write it down here.

**Keep this file current.** When you establish, change or retire a convention, update it in the same commit.

## Gate

`make check` is the definition of done, and a pre-commit hook runs it: structure (including this file's 10,000-character budget), comment blocks, gofmt, `go vet`, golangci-lint, `go test -race`. No commit lands red; never `--no-verify`. CI invokes the same targets.

`make bench` measures the world record and loop, six runs each; run it before and after changing either, compare medians rather than single runs, and update the baseline in `docs/decisions/world-loop.md`. The gate compiles the benchmarks but does not run them.

The tests that need PostgreSQL fail when `TEST_DATABASE_URL` is unset, so a green gate always means they ran; copy `.env.example` to `.env` and start the database with `make db`. They wipe the test database, never the development one. `go test -short ./...` skips them for a quick loop, never as the gate.

PostgreSQL listens on host port 15432 so this project coexists with others running locally. The game ports, 11002 (auth) and 13001 (game), are the ones the client is configured for; open-mt2 uses the same, so run one server at a time.

## Mechanics are documented, not just implemented

Every game mechanic has a page in `docs/mechanics/`: what the client does, the exact bytes, what the server does, how we know (client script or observed traffic), and which test proves it. **A change to a mechanic updates its page in the same commit.** A page states facts in our own words; it never pastes code from another server or from the client.

**Sources.** Protocol facts come from [open-mt2](https://github.com/willianmarquess/open-mt2) and the client's scripts; no code is copied from either, and leaked original server code is never used. The full rule is in `CONTRIBUTING.md`.

## Go conventions

- **No global state and no dependency-injection framework.** Dependencies are struct fields set in `internal/server`, the one place that wires the parts.
- **Interfaces are declared where they are consumed** (`auth.Accounts` and `game.Accounts` are what each port needs from `store.Accounts`), and kept to the methods actually called.
- **`internal/protocol` is pure:** bytes in, values out, no sockets, no database, no clock. Every packet test compares exact bytes.
- **Header types are per direction** (`ClientHeader`, `ServerHeader`): numbers are reused across directions.
- **Game types and wire types stay apart.** `internal/account` holds the game's own types (`Empire`, `Character`, `Class`, `Sex`, `SkillGroup`); `internal/protocol` holds packet layouts. The game port converts between them in `internal/game/wire.go`, the one place the client's numbers for named things appear. It is an adapter for the original client and goes when our own client replaces it, so never let those numbers into the model; the store never imports `protocol`.
- **What the game names is stored by name**, as text with a CHECK listing the allowed values (`empire IN ('shinsoo', 'chunjo', 'jinno')`), so the tables read without a lookup. NULL means not chosen yet; the store reads it as `""`. Quantities stay numbers.
- **The schema guarantees what the wire needs.** A column sent to the client has a CHECK matching its field (`level` 1–255 for a u8, name length in bytes), so no stored row can fail to encode. Names are unique on `lower(name)`.
- **The environment is read only in `internal/config`** (forbidigo enforces it), and `.env.example` documents exactly the variables it reads (a config test enforces that).
- **Errors wrap with `%w` and are checked with `errors.Is`/`errors.As`.** A session returns an error to end its connection; it never panics.
- **`//nolint` names its linter and gives a reason**, enforced by nolintlint.

## Structure

```text
cmd/mt2go/          the binary: `serve`, and `account create` and `event` while there is no website
internal/config/    environment -> Config
internal/protocol/  packet layouts, framing, byte reading and writing
internal/network/   TCP: one goroutine per connection, cutting the stream into packets
internal/handshake/ the time sync every connection starts with
internal/auth/      the login port: password check, login key
internal/game/      the game port: login by key, kingdom, characters, entering the world
internal/account/   the game's types for accounts and characters, starting and derived values, login keys
internal/store/     PostgreSQL: pool, embedded migrations, queries
internal/world/     the game world: which maps exist and where, and where mobs are spawned
internal/mob/       what each kind of monster and NPC is
internal/combat/    how much a blow takes: attack, defence, damage
internal/event/     scheduled events: what one is, and its multiplier
internal/server/    wires everything and runs both ports; the end-to-end tests live here
docs/mechanics/     how each mechanic works
docs/decisions/     why the rules here are what they are
```

A connection's goroutine is the only one that touches its session, so session state needs no lock; any goroutine may `Send` on a connection, which queues (`docs/decisions/connections.md`). A connection that has not logged in 30 seconds after connecting is closed, and every write to a client gives up after 10; a session returns `network.ErrHangUp` to end its connection without logging a warning. State shared between connections (`account.Keys`) locks itself.

## Decisions worth not relitigating

- **One process, two ports** — auth and game share memory, so login keys need no Redis. See `docs/decisions/architecture.md`.
- **PostgreSQL with our own schema**, migrated by a small embedded runner rather than a tool. See `docs/decisions/architecture.md`.
- **Packet encryption stays off** — the server never sends `KEY_AGREEMENT`, and the client then talks plaintext. See `docs/mechanics/connection-and-login.md`.
- **The client's sequence byte is counted, not verified.** See `docs/mechanics/connection-and-login.md`.

## The decision archive

`docs/decisions/` holds the reasoning, the measurements and the rejected alternatives behind the rules above. **Read the one file covering your area; do not load the directory.** These are records, not rules: where one disagrees with this file, this file wins.

| Working on | Read |
|---|---|
| Process layout, database, migrations, dependencies | `architecture.md` |
| Sending to a connection from elsewhere | `connections.md` |
| Anything on a timer: the world loop | `world-loop.md` |
| Converting between names and the original client's numbers | `client-adapter.md` |
| The comment length guard | `comment-blocks.md` |
| The size of this file | `instructions-budget.md` |
| How a packet or mechanic works | the page in `docs/mechanics/` |
