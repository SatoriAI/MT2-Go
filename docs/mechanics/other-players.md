# Other players

## What the player sees

A player sees the other players near them: on the same map and within 8000 units, about the distance the camera shows. Monsters and NPCs follow the same rule ([monsters.md](monsters.md)). A player entering a village sees whoever already stands near them, where they stand now, and they see the newcomer appear. When someone walks, those near watch them walk; walking away, a character vanishes for the others and they for it, and walking back, both reappear. A line said in chat appears for everyone near the speaker, in the chat window and in a bubble over the speaker. When a player leaves, by Logout, Change character, Exit or a dropped connection, they vanish for the others.

The three starting villages are separate maps, so a player in Jinno's village never hears of one in Shinsoo's, whatever the distance.

## What the client does

- **Its own character** is added first, as on entering the world. An ADD for the player's own vid makes the client drop every other character it holds (`NetworkActorManager.cpp`, `__RemoveDynamicActors`), so the others must come after it.
- **Another player** is built from two packets: an ADD, which the client keeps in a single slot, then the INFO for the same vid, which creates the character (`PythonNetworkStreamPhaseGameActor.cpp`). An INFO for another vid is dropped. The pair must therefore arrive back to back.
- **An ADD for a vid it already shows** respawns that character. Each observer is told of a character once.
- **A MOVE** for another character makes the client walk it to the point. A MOVE for the player's own vid is ignored (`InstanceBase.cpp`, `PushTCPState`), and one for an unknown vid too.
- **The player's own MOVE** (`PythonNetworkStreamPhaseGame.cpp`, `ActorInstanceEvent.cpp`, `PythonPlayerEventHandler.cpp`):
  - The client sends where its character stands as a run starts, as it stops, attacks or uses a skill, and every 100 ms while it drifts.
  - While it runs it sends, at most every 300 ms, the run's end instead, at most 1000 ahead of where it stands.
  - Its run covers 300 in 0.667 s at move speed 100, its run motion's, and faster in step with the speed (`InstanceBaseMovement.cpp`).
- **SYNC_POSITION** (`05`) for the player's own vid puts their character where it names: the client blends it there, and plays a fall past 150, as after a knockback. Nothing else moves the player's own character (`PythonNetworkStreamPhaseGameActor.cpp`, `ActorInstanceSync.cpp`).
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

### SYNC_POSITION — server, `05`, 15 bytes for one character

| Offset | Field |
|---|---|
| 0 | u8 header `05` |
| 1 | u16 size, 15 |
| 3 | u32 vid |
| 7 | i32 x |
| 11 | i32 y |

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

`internal/game/presence.go` keeps who is in the world, by vid and by the map each stands on (from `internal/world`), so a move looks only at those on its map, where they are, and whom each has been shown. Two players see each other when they are on the same map and at most 8000 apart, the edge included; seeing is always mutual, so both sides change together. Players on no known map share one map with each other; only bad data puts a character there.

The range is open-mt2's choice; the client takes any, and draws characters up to 20000 away, so everyone in range is drawn. Who sees whom changes only when someone enters, moves or leaves, as in open-mt2.

- **Entering:** after the player's own packets, the server sends the player the ADD and INFO of everyone in range, all in one send, and sends theirs to each of them, then records the player. Only then is the session in the game phase, so a connection that closed while entering never leaves a world it never joined.
- **How far a move may go:** a player's character is held on a tether, where the server reckons it can be.
  - **Where the tether stands:** where the character entered, rose after dying, here or in town, or was dashed to.
  - **How it follows:** with each move it follows at most half as fast again as the character runs, 4.5 a second for each point of move speed, so 1012 a second at 150. It banks at most 2 seconds unspent, so a client standing still earns no teleport, and a move it reaches spends only the time that took, so moves held up and arriving together share the time.
  - **How far a move may land:** at most 2500 from it. A running client reports its walk's end up to 1000 ahead, so a tether following those reports can be 1000 ahead, and a turn back lands 1000 the other way; 500 is for moves arriving late.
  - **A move further off** is not taken: the character is stopped where the tether has got to, for the others, and put there with SYNC_POSITION for the player's own client.
  - **Why:** a client cannot teleport, nor chain moves past the tether, and every reach check, of blows, pickups and NPCs, reads a position at most 2500 from where the character can have walked to. The tether is this server's own; open-mt2 caps each move at 2500 from the last, which moves can be chained past.
- **Moving:** the position and facing are kept, so a later arrival sees the character where it stands, facing the way it last moved. Then, against everyone else on the map: those who came into range and the mover are shown to each other, and those who left it and the mover get each other's DELETE; the mover gets all of that in one send. Last, the move goes to everyone in range, with the fields as the mover's client sent them, so someone just met has the character before its move. It is not sent back to the mover.
- **Chat:** a line goes to the speaker and to everyone in range, with the speaker's vid, so the bubble shows. Commands stay the speaker's.
- **Leaving:** every way out of the world goes through `leaveWorld`, which takes the player out and sends DELETE to everyone who had them in view: the record of whom each has been shown, rather than the range at that moment, so nobody is left holding a character that is gone. A character taken over by a new login leaves before the new connection's claim is granted, so the old vid disappears before the new one appears.
- **Effects:** a character's ADD carries the effects of the buffs it has running, such as Enchanted Blade's glow, and an UPDATE tells everyone who sees it when one starts or ends ([skills.md](skills.md)). A skill's motion is a MOVE like any other, passed on as the client sent it.
- **ON_CLICK and TARGET** are accepted and ignored in the world, except a click on an NPC who sells or teaches.

**Order.** Every packet about other players is sent while the record is locked, so everyone is told of the same things in the same order, and nobody receives a move for a character before the character. Sending only queues: each connection has its own queue and writer (see `docs/decisions/connections.md`), so a slow client never holds the lock.

## How we know

- **The ordering rules, the layouts, and what the client ignores:** the client's own source, in `open-mt2/work/client-review/ClientVS22/source`: `Packet.h`, `NetworkActorManager.cpp`, `PythonNetworkStreamPhaseGameActor.cpp`, `InstanceBase.cpp`.
- **The fields passed through, sending moves to others only, the range and the order on a move:** open-mt2.
- **Seen with the real client,** with two windows side by side: Hendra and Kira saw each other in Jinno's village, each watched the other walk, a line appeared in both windows, and Logout made the character vanish from the other. The move's duration of 0 caused no visible problem: the other character walked as it should.
- **The range, seen with the real client** against a scripted second player walking east and back: the scripted player vanished from the real client's window once about 8000 away and reappeared on coming back, and the script received DELETE past the range, then ADD and INFO before the next MOVE on returning, at 7983 apart.

## What proves it

- `internal/protocol/protocol_test.go`: MOVE, DELETE and SYNC_POSITION, byte for byte.
- `internal/server/others_test.go`:
  - two players on a map seeing each other, each as their own client was shown them;
  - a move reaching the other player with its time, and not the mover;
  - a later arrival seeing the other where they moved to;
  - a line reaching both;
  - Logout, Change character and a dropped connection each sending DELETE;
  - a player on another map seeing nothing;
  - ON_CLICK and TARGET ignored;
  - walking out of range hiding both from each other, with no move after, each walk short enough to be taken, from where the test stood them;
  - walking back showing both again, and the onlooker the move after the character;
  - the edge of the range, 8000, in view, and the range measured in a straight line;
  - players entering out of range meeting by walking;
  - a line out of range not heard;
  - leaving telling those who saw the player, and not a third player on the map out of range.
- `internal/game/tether_test.go`:
  - a move within 2500 taken;
  - one further putting her back, by SYNC_POSITION and a stop for Kira;
  - the tether following at her speed;
  - moves not chained past it;
  - a run turning back after eight seconds never put back;
  - standing still banking no teleport, and a burst of moves sharing the time they took;
  - rising where she fell starting the tether afresh;
  - the tether following a restart in town and a dash;
  - the dead not walking.
- `internal/game/presence_test.go`: a player whose connection closed while entering leaving without disturbing the others; someone entering a map of 20,000 players getting them all without filling their queue.
- `internal/network/server_test.go`: sends from several goroutines arriving whole and in order; what was sent before a hang-up reaching the client; a client that stops reading being disconnected; a sender to a client far behind being refused at once rather than made to wait.

## Not done yet

- A spatial grid within a map, so a move is checked against those nearby rather than everyone on the map; the record is kept by map already, and a measurement should ask for the grid first.
- A margin against flickering, for two players walking along the edge of each other's range.
- Changing map. A player's map is taken on entering only, which holds while the only way to another map is entering it; a warp must update it, or the player stays visible on the old map.
- CHARACTER_UPDATE (`13`) for the guild; equipment uses it ([equipment.md](equipment.md)).
- SYNC_POSITION (`08`), which the client sends once it hits things.
- The move's duration. With 0, a walk looked right with the real client; what the client uses it for is not confirmed in its source here.
- Checking that a move is possible, rather than trusting the client.
