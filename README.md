# MT2 Go

A Metin2 server written from scratch in Go: every mechanic documented and understood, compatible with the original game client. Unofficial hobby project.

[![Go](https://github.com/SatoriAI/MT2-Go/actions/workflows/go.yml/badge.svg)](https://github.com/SatoriAI/MT2-Go/actions/workflows/go.yml)
[![Repository](https://github.com/SatoriAI/MT2-Go/actions/workflows/repository.yml/badge.svg)](https://github.com/SatoriAI/MT2-Go/actions/workflows/repository.yml)

## Status

Early. The original TMP4 client (r40250) logs in, chooses a kingdom, creates a character and enters the world. There a player sees the others near them and chats with them, whispers to anyone in the game, with each delivered whisper kept in the database, and can block whispers. The first villages have their NPCs and wandering monsters; monsters can be fought and bite back, a player can die and restart, both heal over time, and hit points and mana are kept between logins. Killing monsters gives experience, levels and stat points to spend. Experience events, scheduled from the command line, multiply it for a span of time. Characters own items, given or taken on the command line, and see and move them in the inventory. Local use only: nothing here is hardened for a public server.

Each mechanic is written up in [`docs/mechanics/`](docs/mechanics/) — what the client does, the exact bytes, what the server does and how we know. That is the best place to start reading.

## Quick start

Requires Go 1.27 and Docker.

```bash
cp .env.example .env
make db                  # PostgreSQL on localhost:15432, plus the test database
make account NAME=admin  # asks for a password (at most 16 characters, the client's limit);
                         # the local convention is admin / admin
make run                 # auth on 127.0.0.1:11002, game on 127.0.0.1:13001
```

An experience event, such as double experience over Christmas, is scheduled with `go run ./cmd/mt2go event create "Boże Narodzenie" 2 2026-12-24T00:00 2026-12-27T00:00`; see [`docs/mechanics/events.md`](docs/mechanics/events.md).

Point the client's `serverinfo.py` at `127.0.0.1`, auth port `11002`, channel port `13001`, and log in with the account you created. **The client is not part of this repository** and is not provided here.

## Configuration

Every variable is in [`.env.example`](.env.example) with a line saying what it does. The two that matter first:

| Variable | Meaning |
| --- | --- |
| `DATABASE_URL` | PostgreSQL connection; the default matches `make db` |
| `GAME_PUBLIC_ADDRESS` | The address the client is sent to after login. Keep `127.0.0.1` locally; use the machine's LAN address to play from another computer |

## Working on it

`make check` is the definition of done — formatting, `go vet`, golangci-lint, the tests with the race detector, and the repository's own structure checks. Run `make hooks` once so it runs before every commit. The end-to-end tests need `make db`.

```text
cmd/mt2go/         the binary
internal/protocol/ packet layouts and framing — pure, no sockets
internal/network/  TCP connections
internal/auth/     the login port
internal/game/     the game port
internal/store/    PostgreSQL and migrations
docs/mechanics/    how each mechanic works
```

How to contribute — where code may come from, commits and pull requests — is in [`CONTRIBUTING.md`](CONTRIBUTING.md). Code conventions are in [`AGENTS.md`](AGENTS.md), which `CLAUDE.md` imports, and the reasoning behind them is in [`docs/decisions/`](docs/decisions/).

## Not affiliated

Metin2 is a trademark of Webzen; this project is not affiliated with or endorsed by Webzen or Gameforge. It contains no original server code. Protocol facts come from observing the client and from [open-mt2](https://github.com/willianmarquess/open-mt2), whose code is not copied here. Game assets belong to their owners and are not distributed.

## License

[MIT](LICENSE).
