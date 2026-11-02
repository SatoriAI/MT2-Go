# Connection and login

## What the player sees

The login screen shows the channel as online. The player types a name and password. A new account is asked to choose a kingdom; an account that has one goes straight to the character screen, which shows up to four characters; an empty slot leads to [character creation](character-creation.md).

## What the client does

The client talks to two ports, set in its `serverinfo.py`: **auth** (11002) and **game** (13001). One process serves both here.

1. **Channel check.** The login screen opens a socket to the game port and keeps sending the one byte `CE` (no sequence byte). It reads replies a byte at a time, looking for `D2`.
2. **Auth connection.** On *Log in* the client connects to the auth port. The server speaks first: phase `HANDSHAKE`, then a handshake. Once the clocks agree the server sends phase `AUTH`, and the client sends `LOGIN3` with the name and password.
3. **The key.** On success the server returns a 32-bit login key. The client closes the auth connection and connects to the game port.
4. **Game connection.** Handshake again, then phase `LOGIN`; the client sends `LOGIN2` with the name and the key.
5. **Selection.** If the account has a kingdom, the server sends the kingdom and the character list, then phase `SELECT`. Without one it sends only `SELECT`, and the client opens the kingdom screen; the choice comes back as `EMPIRE`, and the client then moves to the character screen by itself.
6. At `SELECT` the client also opens a guild-mark connection to the game port and sends `MARK_LOGIN`. Marks are not served yet; the server closes that socket and the client carries on.

When a character later enters the world, the client opens **another** game connection and sends `LOGIN2` with the **same key**. This is why a key cannot be single-use.

## The bytes

### Phase — server, `FD`, 2 bytes

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `FD` |
| 1 | u8 phase | `1` handshake, `2` login, `3` select, `10` auth |

### Handshake — both directions, `FF`, 13 bytes, not sequenced

| Offset | Field |
|---|---|
| 0 | u8 header `FF` |
| 1 | u32 id |
| 5 | u32 time — milliseconds since the server started |
| 9 | i32 delta |

The client sets its clock to `time + delta` and echoes the same id with `time + 2·delta` and delta `0`. The server accepts when `0 ≤ now − (time + delta) ≤ 500` ms. Otherwise it sends a new handshake with the same id, `time = now` and `delta` = half the measured lag (falling back to half the time since its last send if that is negative), and the exchange repeats.

The id never contains the byte `D2`: the channel check reads the same socket byte by byte and would take it for the status reply. A real client settles in one or two rounds; after eight the server gives up and closes the connection.

### LOGIN3 — client, `6F`, 65 bytes, sequenced

| Offset | Field |
|---|---|
| 0 | u8 header `6F` |
| 1 | char[31] name |
| 32 | char[17] password |
| 49 | u32[4] client key — ignored, encryption is off |

### AUTH_SUCCESS — server, `96`, 6 bytes

| Offset | Field |
|---|---|
| 0 | u8 header `96` |
| 1 | u32 login key |
| 5 | u8 result — always `1`; `0` makes the client report a duplicate key |

### LOGIN_FAILURE — server, `07`, 10 bytes

`07`, then char[9] status. The client shows a message for each known status; this server uses `WRONGPWD` (unknown name or wrong password, deliberately the same answer) and `ALREADY` (the account is in the game). An unknown name is checked against a decoy password hash, so it takes as long to refuse as a wrong password and the response time does not reveal which accounts exist. After three refusals on one connection the server closes it. The client knows others, such as `BLOCK`, `FULL` and `SHUTDOWN`; an unknown string is shown as an unknown error.

### LOGIN2 — client, `6D`, 52 bytes, sequenced

| Offset | Field |
|---|---|
| 0 | u8 header `6D` |
| 1 | char[31] name |
| 32 | u32 login key |
| 36 | u32[4] client key — ignored |

### EMPIRE — both directions, `5A`, 2 bytes (sequenced from the client)

`5A`, then u8 kingdom: `1` Shinsoo (red), `2` Chunjo (yellow), `3` Jinno (blue). The server sends it **before** phase `SELECT`, because the client chooses between the kingdom screen and the character screen at the moment the phase arrives. The client sends it from two screens. The kingdom screen sends it for an account without a kingdom. Then, when the player clicks an empty slot while every slot is empty, the client shows the kingdom screen again before character creation, starting on a random kingdom, and sends whichever the player picks (`introselect.py` `CreateCharacter`, `introempire.py` `ReselectEmpireWindow`). So an unmodified client changes the kingdom of an account with no characters, and the server accepts that. Once the account has a character, only the same kingdom is accepted; another closes the connection and changes nothing. A repeat of the kingdom the connection already knows, for example from a double click during the screen's fade-out, is ignored without touching the database. Tried with the real client, the second packet arrived; closing on it is what this rule fixed.

### Character list (LOGIN_SUCCESS4) — server, `20`, 329 bytes

| Offset | Field |
|---|---|
| 0 | u8 header `20` |
| 1 | 4 × 63-byte character slot |
| 253 | u32[4] guild ids, then char[4][13] guild names — zero for now |
| 321 | u32 guild mark handle — zero |
| 325 | u32 random key — zero |

One slot, 63 bytes:

| Offset | Field | Offset | Field |
|---|---|---|---|
| 0 | u32 id (0 = empty slot) | 39 | u16 body part (armour shown) |
| 4 | char[25] name | 41 | u8 change-name flag |
| 29 | u8 race | 42 | u16 hair part |
| 30 | u8 level | 44 | u32 unused |
| 31 | u32 minutes played | 48 | i32 x |
| 35 | u8 ST, HT, DX, IQ (4 bytes) | 52 | i32 y |
| | | 56 | **4 bytes game IPv4**, in network order |
| | | 60 | **u16 game port** |
| | | 62 | u8 skill group |

**Race** is class and sex in one number, and not "female plus four": `0` warrior male, `1` ninja female, `2` sura male, `3` shaman female, `4` warrior female, `5` ninja male, `6` sura female, `7` shaman male (the client's `RACE_*` constants in `playersettingmodule.py`). An earlier version of this page, following open-mt2, had the ninja and shaman sexes the wrong way round. The **skill group** is `1` or `2`, the first or second path of the class (warrior: body, mental; ninja: dagger, archery; sura: weaponry, black magic; shaman: dragon, healing), and `0` before one is chosen. The database stores kingdom, class, sex and skill group by name (`'chunjo'`, `'sura'`, `'female'`, `'black_magic'`) and `internal/game/wire.go` turns them into these numbers. The **body part** is the shape the select screen draws, from the class's `.msm` file (`sura_m.msm` and the others): `0` and `1` are the two looks of the create screen, the same model in two colours, and higher numbers are armour. Until characters wear armour it is the character's look; then it will be worked out from the equipment.

The address and port are where the client connects when this character starts. They must be reachable from the client: never `0.0.0.0` (hence `GAME_PUBLIC_ADDRESS`). The address is written as four raw bytes; open-mt2 builds it as a signed integer and fails for any address whose last byte is 128 or more.

### Channel status — client `CE` (1 byte, not sequenced), server `D2`

The reply is `D2`, then i32 **number of entries**, then per entry u16 port and u8 status (`1` normal; `3` full stops the client connecting), then a u8 success flag the client discards. One channel on 13001 is `D2 01000000 C932 01 01`.

### Mark login — client, `64`, 9 bytes, not sequenced

`64`, u32 handle, u32 random key. Answered by closing the socket.

## Framing what the client sends

The server never sends a length: the client knows each header's size. The client does not send one either, for these packets: the server keeps its own table of size per header (`internal/protocol/frame.go`). A header missing from the table cannot be delimited, so the connection is closed and the header logged. A few packets vary in length, chat first among them; they carry a u16 total size right after the header ([chat.md](chat.md) has the details).

**The sequence byte.** The client turns sequencing on for its auth and game connections, and from then appends one byte to every packet except the handshake reply, the channel check and the mark login. The byte comes from a fixed 32,768-entry table in the client, starting `AF CA 8A CF`, restarting at each new connection — so `LOGIN3` and the first `LOGIN2` carry `AF` and the `EMPIRE` after it `CA`. This server counts the byte and does not check it: verifying would mean carrying the client's table, and it protects nothing a plaintext connection does not already give away. Counting one on a packet that has none, or the reverse, stalls the connection.

**Encryption** is off. The client is built with an improved packet cipher, but it only switches it on after the server sends a key agreement; this server never does, and the client talks plaintext.

## What the server does

| Step | Code |
|---|---|
| Accept, one goroutine per connection, cut packets | `internal/network/server.go`, `internal/protocol/frame.go` |
| Handshake | `internal/handshake/handshake.go` |
| Password check (bcrypt), login key | `internal/auth/session.go`, `internal/account/keys.go` |
| Login by key, kingdom, character list, channel status | `internal/game/session.go` |
| Names to the client's numbers (kingdom, race, skill group) | `internal/game/wire.go` |
| Accounts and characters | `internal/store/accounts.go`, `internal/store/migrations/001_accounts.sql` |

A key is redeemable while its account holds a game connection and for 60 seconds after the last one closes, with the account name in any case. An account with an open game connection cannot log in again (`ALREADY`). A new login replaces the account's earlier key, so two clients that log in one after the other cannot both reach the game port. Account names are unique regardless of case.

**Connections that do not log in.** A connection still open 30 seconds after it was accepted is closed, however much it sent, so sockets that never log in cannot use up the server's file descriptors. Counting from the accept rather than from the last packet matters: the channel-status ping is answered before login, and a client sending it every few seconds would otherwise stay forever. The deadline is lifted once a game connection has redeemed its key, because the player may stay on the selection screens as long as they like. Every send gives up after 10 seconds, so a client that stops reading cannot hold the connection's goroutine. If accepting a connection fails, for example because the process is out of file descriptors, the port waits briefly and tries again.

## How we know

The flow, layouts and the sequence-byte exemptions come from the client's own scripts and headers (`intrologin.py`, `introselect.py`, `introempire.py`, `networkmodule.py`, the packet header list) and from open-mt2, a server that works with this client. The two-logins-per-key behaviour is open-mt2's issue #104.

## What proves it

- `internal/protocol/protocol_test.go` — every packet above against its exact bytes, every field of a character slot, framing with and without the sequence byte, an incomplete packet, an address ending in 200.
- `internal/handshake/handshake_test.go` — in-time echo, late echo and the corrected delta, a foreign id, a client that never syncs, no `D2` in ids.
- `internal/account/keys_test.go` — the key works for a second connection, expires after the grace period, never while held; a new key revokes the old one; no key while the account is in the game.
- `internal/game/wire_test.go` — all eight race numbers both ways, skill group numbers, kingdom numbers both ways.
- `internal/network/server_test.go` — a failed accept does not stop the port, the login deadline (busy or silent) and lifting it, a send to a client that stops reading giving up, shutdown and a closed listener closing open connections.
- `internal/server/server_test.go` — the whole path over real sockets and a real database: new account to the kingdom screen, the kingdom stored and a stored character listed byte for byte on the next connection, a double-clicked kingdom keeping the connection, a new account changing its kingdom, a kingdom that cannot change once the account has a character, a skill group of another class refused by the database, `ALREADY`, a name in another case, a key presented for another name, a packet out of its phase, wrong passwords and unknown names, the channel check.
