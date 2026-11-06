# The original client is adapted to, not modelled

The server began by talking to the original TMP4 client, and its own client, MT2-Godot, now speaks the same protocol. The server's model follows the game, not that client's protocol.

## What the old client imposes, and where it stops

The TMP4 client numbers what the game names: kingdoms are 1 to 3, skill groups 1 and 2, and class and sex share one "race" number from 0 to 7 in an order that is not arithmetic (the female ninja is 1, the male ninja 5). Storing those numbers would tie the database and every rule to one client's encoding, and to its quirks.

So the database and `internal/account` hold names (`'sura'`, `'female'`, `'jinno'`), and `internal/game/wire.go` converts between names and the client's numbers in both directions. It is the only place the numbers appear. A character's look (`shape`, 0 or 1) is kept as a number because it is one: which of the class's two looks, with no name in the game to give it. `internal/protocol` holds the TMP4 packet layouts.

## When it goes

On 2026-11-06 we decided that the original client is no longer a goal: the protocol is the contract between this server and MT2-Godot, and either side may add or change a packet, landing in both at once with each side's test vector. The TMP4 layouts stay where they serve, and the original client still connects today, but nothing here waits on it any more.

`wire.go` goes when the protocol stops carrying the original's numbers: once a packet names class, sex or kingdom rather than numbering them, its conversion goes with it, and nothing else should have to change but `cmd/mt2go`, which refuses event names the old client cannot show (`ClientCanShow`) and can drop that check once no client it serves needs it.

## The lesson that came with it

The first version of `wire.go` computed the race as "class, plus four for female", taken from open-mt2. The client's own `playersettingmodule.py` shows otherwise, and the character list sent the wrong sex for ninjas and shamans until it was checked against the client. A table copied from the client, with a test per entry citing it, replaced the arithmetic.
