# Other players

## What the player sees

A player sees the other players near them: on the same map and within 8000 units, about the distance the camera shows. A player entering a village sees whoever already stands near them, where they stand now, and they see the newcomer appear. When someone walks, those near watch them walk; walking away, a character vanishes for the others and they for it, and walking back, both reappear. A line said in chat appears for everyone near the speaker, in the chat window and in a bubble over the speaker. When a player leaves, by Logout, Change character, Exit or a dropped connection, they vanish for the others.

The three starting villages are separate maps, so a player in Jinno's village never hears of one in Shinsoo's, whatever the distance.

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
| 20 | u32 duration | in milliseconds; sent as 0 (open-mt2 works it out from the walking animation) |

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

`internal/game/presence.go` keeps who is in the world, by vid, with the map each stands on (from `internal/world`), where they are, and whom each has been shown. Two players see each other when they are on the same map and at most 8000 apart, the edge included; seeing is always mutual, so both sides change together. Players on no known map share one map with each other; only bad data puts a character there.

The range is open-mt2's choice; the client takes any, and draws characters up to 20000 away, so everyone in range is drawn. Who sees whom changes only when someone enters, moves or leaves, as in open-mt2.

- **Entering:** after the player's own packets, the server sends the player the ADD and INFO of everyone in range, all in one send, and sends theirs to each of them, then records the player. Only then is the session in the game phase, so a connection that closed while entering never leaves a world it never joined.
- **Moving:** the position is kept, so a later arrival sees the character where it stands. Then, against everyone else on the map: those who came into range and the mover are shown to each other, and those who left it and the mover get each other's DELETE; the mover gets all of that in one send. Last, the move goes to everyone in range, with the fields as the mover's client sent them, so someone just met has the character before its move. It is not sent back to the mover.
- **Chat:** a line goes to the speaker and to everyone in range, with the speaker's vid, so the bubble shows. Commands stay the speaker's.
- **Leaving:** every way out of the world goes through `leaveWorld`, which takes the player out and sends DELETE to everyone who had them in view: the record of whom each has been shown, rather than the range at that moment, so nobody is left holding a character that is gone. A character taken over by a new login leaves before the new connection's claim is granted, so the old vid disappears before the new one appears.
- **ON_CLICK and TARGET** are accepted and ignored in the world.

**Order.** Every packet about other players is sent while the record is locked, so everyone is told of the same things in the same order, and nobody receives a move for a character before the character. Sending only queues: each connection has its own queue and writer (see `docs/decisions/connections.md`), so a slow client never holds the lock.

## How we know

- **The ordering rules, the layouts, and what the client ignores:** the client's own source, in `open-mt2/work/client-review/ClientVS22/source`: `Packet.h`, `NetworkActorManager.cpp`, `PythonNetworkStreamPhaseGameActor.cpp`, `InstanceBase.cpp`.
- **The fields passed through, sending moves to others only, the range and the order on a move:** open-mt2.
- **Seen with the real client,** with two windows side by side: Hendra and Kira saw each other in Jinno's village, each watched the other walk, a line appeared in both windows, and Logout made the character vanish from the other. The move's duration of 0 caused no visible problem: the other character walked as it should.
- **The range, seen with the real client** against a scripted second player walking east and back: the scripted player vanished from the real client's window once about 8000 away and reappeared on coming back, and the script received DELETE past the range, then ADD and INFO before the next MOVE on returning, at 7983 apart.

## What proves it

- `internal/protocol/protocol_test.go`: MOVE and DELETE, byte for byte.
- `internal/server/others_test.go`:
  - two players on a map seeing each other, each as their own client was shown them;
  - a move reaching the other player with its time, and not the mover;
  - a later arrival seeing the other where they moved to;
  - a line reaching both;
  - Logout, Change character and a dropped connection each sending DELETE;
  - a player on another map seeing nothing;
  - ON_CLICK and TARGET ignored;
  - walking out of range hiding both from each other, with no move after;
  - walking back showing both again, and the onlooker the move after the character;
  - the edge of the range, 8000, in view, and the range measured in a straight line;
  - players entering out of range meeting by walking;
  - a line out of range not heard;
  - leaving telling those who saw the player, and not a third player on the map out of range.
- `internal/game/presence_test.go`: a player whose connection closed while entering leaving without disturbing the others; someone entering a map of 20,000 players getting them all without filling their queue.
- `internal/network/server_test.go`: sends from several goroutines arriving whole and in order; what was sent before a hang-up reaching the client; a client that stops reading being disconnected; a sender to a client far behind being refused at once rather than made to wait.

## Not done yet

- A spatial grid, so a move is checked against those nearby rather than everyone on the map; a measurement should ask for it first.
- A margin against flickering, for two players walking along the edge of each other's range.
- Changing map. A player's map is taken on entering only, which holds while the only way to another map is entering it; a warp must update it, or the player stays visible on the old map.
- CHARACTER_UPDATE (`13`), which open-mt2 sends after another player's INFO, for equipment and guild.
- SYNC_POSITION (`08`), which the client sends once it hits things.
- The move's duration. With 0, a walk looked right with the real client; what the client uses it for is not confirmed in its source here.
- Checking that a move is possible, rather than trusting the client.
