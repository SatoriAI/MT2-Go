# Architecture

## One process, two ports

The client was built for a split deployment: it logs in on an auth server, receives a key, and presents that key to a game server on another address. The original server and open-mt2 both run them as separate processes, and have to share the key through something both can reach (open-mt2 uses Redis).

Here both ports listen in one process. The key store is a map behind a mutex (`account.Keys`), so there is no cache service to run, no serialisation, and no network hop between issuing a key and redeeming it. The client cannot tell the difference: it only sees two addresses.

Rejected for now: two binaries. The split earns its cost when one auth server fronts several game channels on different machines. That is not a goal yet; if it becomes one, `account.Keys` is the single seam to replace.

## The key outlives its first connection

The client presents its login key twice: once to reach character selection, and again on a fresh game connection when the chosen character enters the world. A key consumed on first use therefore disconnects every player at the moment of entering the world — open-mt2 shipped that bug (its issue #104). Keys here are redeemable while the account holds a game connection and for 60 seconds after its last one closes.

The key is 32 bits because the client's packet says so. It comes from `crypto/rand`; what stays open is that 2³² is small enough to guess by brute force against a public game port. Rate-limiting refused keys is the answer, and belongs with the first public deployment.

## PostgreSQL, our own schema

open-mt2 and the original game use MySQL with a schema shaped by twenty years of the original's decisions. Starting with our own schema means every column exists because a mechanic in `docs/mechanics/` needs it. PostgreSQL also matches the rest of the owner's projects.

## The migration runner is forty lines

`internal/store/store.go` applies the embedded `migrations/*.sql` files in name order, each in a transaction with its record in `schema_migrations`, under a PostgreSQL advisory lock held for the whole run, so servers starting at once take turns, from creating `schema_migrations` onwards. A migration tool (goose, golang-migrate) would add down-migrations, a CLI and a dependency for features this project does not use; the runner is short enough to read in a minute, which is the project's point.

## Dependency budget

Three modules outside the standard library: `pgx` (PostgreSQL), `golang.org/x/crypto` (bcrypt), `golang.org/x/sync` (errgroup, to run both ports and stop them together). golangci-lint is a `tool` in `go.mod`, so its version is pinned with the code and nothing is installed globally. A new dependency needs a reason a reader would accept, written here.
