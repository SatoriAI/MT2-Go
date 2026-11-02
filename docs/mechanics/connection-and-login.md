# Connection and login

## What the player sees

The login screen shows the channel as online. The player types a name and password. A new account is asked to choose a kingdom; an account that has one goes straight to the character screen, which shows up to four characters (or the creation screen when there are none).

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

`5A`, then u8 kingdom: `1` Shinsoo (red), `2` Chunjo (yellow), `3` Jinno (blue). The server sends it **before** phase `SELECT`, because the client chooses between the kingdom screen and the character screen at the moment the phase arrives. The client sends it once, for an account without a kingdom; a second one, which only a modified client would send, closes the connection and changes nothing.

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
| 29 | u8 job | 42 | u16 hair part |
| 30 | u8 level | 44 | u32 unused |
| 31 | u32 minutes played | 48 | i32 x |
| 35 | u8 ST, HT, DX, IQ (4 bytes) | 52 | i32 y |
| | | 56 | **4 bytes game IPv4**, in network order |
| | | 60 | **u16 game port** |
| | | 62 | u8 skill group |

**Job** is class and sex in one number: `0`–`3` are the male warrior, ninja, sura and shaman, `4`–`7` the female ones. The database keeps class and sex apart and the server combines them when it sends the list. The **body part** is the armour the character wears; it is `0`, the bare body, until characters have equipment, and it will be worked out from the equipment rather than stored.

The address and port are where the client connects when this character starts. They must be reachable from the client: never `0.0.0.0` (hence `GAME_PUBLIC_ADDRESS`). The address is written as four raw bytes; open-mt2 builds it as a signed integer and fails for any address whose last byte is 128 or more.

### Channel status — client `CE` (1 byte, not sequenced), server `D2`

The reply is `D2`, then i32 **number of entries**, then per entry u16 port and u8 status (`1` normal; `3` full stops the client connecting), then a u8 success flag the client discards. One channel on 13001 is `D2 01000000 C932 01 01`.

### Mark login — client, `64`, 9 bytes, not sequenced

`64`, u32 handle, u32 random key. Answered by closing the socket.

## Framing what the client sends

The server never sends a length: the client knows each header's size. The client does not send one either, for these packets: the server keeps its own table of size per header (`internal/protocol/frame.go`). A header missing from the table cannot be delimited, so the connection is closed and the header logged.

**The sequence byte.** The client turns sequencing on for its auth and game connections, and from then appends one byte to every packet except the handshake reply, the channel check and the mark login. The byte comes from a fixed 32,768-entry table in the client, starting `AF CA 8A CF`, restarting at each new connection — so `LOGIN3` and the first `LOGIN2` carry `AF` and the `EMPIRE` after it `CA`. This server counts the byte and does not check it: verifying would mean carrying the client's table, and it protects nothing a plaintext connection does not already give away. Counting one on a packet that has none, or the reverse, stalls the connection.

**Encryption** is off. The client is built with an improved packet cipher, but it only switches it on after the server sends a key agreement; this server never does, and the client talks plaintext.

## What the server does

| Step | Code |
|---|---|
| Accept, one goroutine per connection, cut packets | `internal/network/server.go`, `internal/protocol/frame.go` |
| Handshake | `internal/handshake/handshake.go` |
| Password check (bcrypt), login key | `internal/auth/session.go`, `internal/account/keys.go` |
| Login by key, kingdom, character list, channel status | `internal/game/session.go` |
| Accounts and characters | `internal/store/accounts.go`, `internal/store/migrations/001_accounts.sql` |

A key is redeemable while its account holds a game connection and for 60 seconds after the last one closes, with the account name in any case. An account with an open game connection cannot log in again (`ALREADY`). A new login replaces the account's earlier key, so two clients that log in one after the other cannot both reach the game port. Account names are unique regardless of case.

**Idle connections.** A connection that sends nothing for 30 seconds is closed, so idle sockets cannot use up the server's file descriptors. The limit is lifted once a game connection has redeemed its key, because the player may stay on the selection screens as long as they like. If accepting a connection fails, for example because the process is out of file descriptors, the port waits briefly and tries again.

## How we know

The flow, layouts and the sequence-byte exemptions come from the client's own scripts and headers (`intrologin.py`, `introselect.py`, `introempire.py`, `networkmodule.py`, the packet header list) and from open-mt2, a server that works with this client. The two-logins-per-key behaviour is open-mt2's issue #104.

## What proves it

- `internal/protocol/protocol_test.go` — every packet above against its exact bytes, every field of a character slot, framing with and without the sequence byte, an incomplete packet, an address ending in 200.
- `internal/handshake/handshake_test.go` — in-time echo, late echo and the corrected delta, a foreign id, a client that never syncs, no `D2` in ids.
- `internal/account/keys_test.go` — the key works for a second connection, expires after the grace period, never while held; a new key revokes the old one; no key while the account is in the game.
- `internal/network/server_test.go` — a failed accept does not stop the port, the idle timeout and lifting it, shutdown closing open connections.
- `internal/server/server_test.go` — the whole path over real sockets and a real database: new account to the kingdom screen, the kingdom stored and a stored character listed byte for byte on the next connection, a kingdom that cannot be changed, `ALREADY`, a name in another case, a key presented for another name, a packet out of its phase, wrong passwords and unknown names, the channel check.
