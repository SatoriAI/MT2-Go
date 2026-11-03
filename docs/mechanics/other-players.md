# Other players

## What the player sees

Everyone on the same map sees everyone else there. A player entering a village sees whoever already stands in it, where they stand now, and they see the newcomer appear. When someone walks, the others watch them walk. A line said in chat appears for everyone on the map, in the chat window and in a bubble over the speaker. When a player leaves, by Logout, Change character, Exit or a dropped connection, they vanish for the others.

There is no view range yet: a map is the range. The three starting villages are separate maps, so a player in Jinno's village never hears of one in Shinsoo's.

## What the client does

- **Its own character** is added first, as on entering the world. An ADD for the player's own vid makes the client drop every other character it holds (`NetworkActorManager.cpp`, `__RemoveDynamicActors`), so the others must come after it.
- **Another player** is built from two packets: an ADD, which the client keeps in a single slot, then the INFO for the same vid, which creates the character (`PythonNetworkStreamPhaseGameActor.cpp`). An INFO for another vid is dropped. The pair must therefore arrive back to back.
- **An ADD for a vid it already shows** respawns that character. Each observer is told of a character once.
- **A MOVE** for another character makes the client walk it to the point. A MOVE for the player's own vid is ignored (`InstanceBase.cpp`, `PushTCPState`), and one for an unknown vid too.
- **A DELETE** fades the character out. An unknown vid is ignored.
- **Clicking a character** sends ON_CLICK, and picking or clearing a target sends TARGET, so these arrive once others are around.

## The bytes

### MOVE — server, `03`, 24 bytes

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `03` |
| 1 | u8 type | as in the client's move: walking, stopping, attacking |
| 2 | u8 arg | as in the client's move |
| 3 | u8 rotation | the direction faced, in steps of 5 degrees |
| 4 | u32 vid | who moves |
| 8 | i32 x | |
| 12 | i32 y | |
| 16 | u32 time | the mover's clock, as their client sent it |
| 20 | u32 duration | in milliseconds; sent as 0 |

The time is on the clock every connection's handshake sets from the same server clock, so it means the same thing to every client. The client runs the move once its own server clock reaches that time.

### DELETE — server, `02`, 5 bytes

| Offset | Field |
|---|---|
| 0 | u8 header `02` |
| 1 | u32 vid |

### ON_CLICK — client, `1A`, and TARGET — client, `3D`; 5 bytes each, sequenced

| Offset | Field | |
|---|---|---|
| 0 | u8 header | |
| 1 | u32 vid | the character clicked or targeted; 0 clears a target |

Another player's ADD (`01`) and INFO (`88`) are the ones on [entering-the-world.md](entering-the-world.md), filled in for that player.

## What the server does

`internal/game/presence.go` keeps who is in the world, by vid, with the map each stands on (from `internal/world`) and where they are.

- **Entering:** after the player's own packets, the server sends the player each other's ADD and INFO, as one send, and sends theirs to each of the others, then records the player.
- **Moving:** the position is kept, so a later arrival sees the character where it stands. The move goes to everyone else on the map, with the fields as the mover's client sent them. It is not sent back to the mover.
- **Chat:** a line goes to the speaker and to everyone else on the map, with the speaker's vid, so the bubble shows. Commands stay the speaker's.
- **Leaving:** every way out of the world goes through `leaveWorld`, which takes the player out and sends the others DELETE. A character taken over by a new login leaves before the new connection's claim is granted, so the old vid disappears before the new one appears.
- **ON_CLICK and TARGET** are accepted and ignored in the world.

**Order.** Every packet about other players is sent while the record is locked, so everyone is told of the same things in the same order, and nobody receives a move for a character before the character. Sending only queues: each connection has its own queue and writer (see `docs/decisions/connections.md`), so a slow client never holds the lock.

## How we know

- **The ordering rules, the layouts, and what the client ignores:** the client's own source, in `open-mt2/work/client-review/ClientVS22/source`: `Packet.h`, `NetworkActorManager.cpp`, `PythonNetworkStreamPhaseGameActor.cpp`, `InstanceBase.cpp`.
- **The fields passed through, and sending moves to others only:** open-mt2.
- **Seen with the real client,** with two windows side by side: Hendra and Kira saw each other in Jinno's village, each watched the other walk, a line appeared in both windows, and Logout made the character vanish from the other. The move's duration of 0 caused no visible problem.

## What proves it

- `internal/protocol/protocol_test.go`: MOVE and DELETE, byte for byte.
- `internal/server/others_test.go`:
  - two players on a map seeing each other, each as their own client was shown them;
  - a move reaching the other player with its time, and not the mover;
  - a later arrival seeing the other where they moved to;
  - a line reaching both;
  - Logout, Change character and a dropped connection each sending DELETE;
  - a player on another map seeing nothing;
  - ON_CLICK and TARGET ignored.
- `internal/network/server_test.go`: sends from several goroutines arriving whole and in order; what was sent before a hang-up reaching the client; a client that stops reading being disconnected.

## Not done yet

- A view range, rather than the whole map.
- CHARACTER_UPDATE (`13`), which open-mt2 sends after another player's INFO, for equipment and guild.
- SYNC_POSITION (`08`), which the client sends once it hits things.
- The move's duration, which only matters for characters the client holds but has not drawn.
- Checking that a move is possible, rather than trusting the client.
