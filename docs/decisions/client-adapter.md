# The original client is adapted to, not modelled

The server talks to the original TMP4 client today, and a client of our own is being built to replace it. The server's model therefore follows the game, not that client's protocol.

## What the old client imposes, and where it stops

The TMP4 client numbers what the game names: kingdoms are 1 to 3, skill groups 1 and 2, and class and sex share one "race" number from 0 to 7 in an order that is not arithmetic (the female ninja is 1, the male ninja 5). Storing those numbers would tie the database and every rule to one client's encoding, and to its quirks.

So the database and `internal/account` hold names (`'sura'`, `'female'`, `'jinno'`), and `internal/game/wire.go` converts between names and the client's numbers in both directions. It is the only place the numbers appear. `internal/protocol` holds the TMP4 packet layouts.

## When it goes

While both clients exist, the server speaks both: our client gets its own protocol, with separate class and sex and names rather than numbers, and needs no such table. When the original client is no longer supported, `wire.go` and the TMP4 layouts in `internal/protocol` are deleted, and nothing else should have to change.

## The lesson that came with it

The first version of `wire.go` computed the race as "class, plus four for female", taken from open-mt2. The client's own `playersettingmodule.py` shows otherwise, and the character list sent the wrong sex for ninjas and shamans until it was checked against the client. A table copied from the client, with a test per entry citing it, replaced the arithmetic.
