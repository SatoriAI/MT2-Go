# Entering the world

## What the player sees

On the character screen the player picks a character and presses **Start**. The character plays its selection motion, the loading screen fills its bar, and the character stands in its kingdom's first village and can walk around. The world is otherwise empty: no other players, no monsters, no items.

## What the client does

1. **Start.** The select screen plays the selection motion. Then the client opens a **new** game connection, does the handshake and logs in with the **same key** (`LOGIN2`), and on that connection asks for the character by its slot (`introselect.py`). Seen with the real client: the server logged a second "entered selection" a few seconds after the first, then the loading on the new connection. This is why a login key must survive its first use ([connection page](connection-and-login.md)).
2. **Loading.** At phase `LOADING` the client opens the loading screen, which always asks for the same slot again (`introloading.py`, unconditionally when it opens). It reports its build with `CLIENT_VERSION`, reads the main character's race, kingdom, skill group and position, loads the map those coordinates fall in, and steps through its loading work one frame at a time.
3. **Enter.** The last loading step sends `ENTERGAME`. At phase `GAME` the client opens the game window.

No map is named anywhere: positions are world coordinates, and the client's `atlasinfo.txt` gives each map's area.

## The bytes

### CHARACTER_SELECT — client, `06`, 2 bytes, sequenced

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `06` |
| 1 | u8 slot | `0`–`3` |

### CLIENT_VERSION — client, `F1`, 67 bytes, sequenced

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `F1` |
| 1 | char[33] name | the client executable's name |
| 34 | char[33] timestamp | its build time |

### ENTERGAME — client, `0A`, 1 byte, sequenced

The header alone.

### Phase — server, `FD`

`4` loading, `5` game, as on the [connection page](connection-and-login.md).

### MAIN_CHARACTER — server, `71`, 46 bytes

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `71` |
| 1 | u32 vid | the character's id in the world |
| 5 | u16 race | as in the character list |
| 7 | char[25] name | |
| 32 | i32 x, i32 y, i32 z | world coordinates; z is `0` and the client puts the character on the ground |
| 44 | u8 kingdom | `1`–`3` |
| 45 | u8 skill group | |

### POINTS — server, `10`, 1021 bytes

255 × u32, each value at its index. The indices sent so far:

| Index | Value | Index | Value |
|---|---|---|---|
| 1 | level | 9, 10 | stamina, max stamina |
| 3, 4 | exp, exp for the next level | 11 | gold |
| 5, 6 | hit points, max hit points | 12–15 | strength, vitality, dexterity, intelligence |
| 7, 8 | mana, max mana | 17, 19 | attack speed, move speed |
| 20 | DEF: level, vitality and armour ([equipment.md](equipment.md)) | | |
| 28 | skill points left, once a skill group is chosen ([skills.md](skills.md)) | | |

### SKILL_LEVEL — server, `4C`, 1531 bytes

255 skills × 6 bytes (master type u8, level u8, next reading time i32), each at the client's number for the skill: the levels of the skills learnt, all else `0` ([skills.md](skills.md)).

### TIME — server, `6A`, 5 bytes; CHANNEL — server, `79`, 2 bytes

A u32 time and a u8 channel. The time is the server clock the handshake already shares (milliseconds since the server started), as open-mt2 sends it. That is probably wrong: open-mt2's own description of the packet says Unix seconds, and the client's scripts compare the game's timestamp with Unix dates for affect and item expiry (`uiaffectshower.py`, `uidragonsoul.py`). Nothing uses it yet; before timed affects or items exist, it should become Unix seconds. The channel is `1`.

### CHARACTER_ADD — server, `01`, 35 bytes

| Offset | Field | |
|---|---|---|
| 1 | u32 vid | |
| 5 | f32 rotation | degrees |
| 9 | i32 x, y, z | |
| 21 | u8 type | `6` player |
| 22 | u16 race | |
| 24 | u8 move speed, u8 attack speed | |
| 26 | u8 state | `0` |
| 27 | u32 ×2 effects | `0` |

### CHARACTER_ADDITIONAL_INFO — server, `88`, 54 bytes

| Offset | Field | |
|---|---|---|
| 1 | u32 vid | |
| 5 | char[25] name | |
| 30 | u16 ×4 parts | body, weapon, head, hair |
| 38 | u8 kingdom | |
| 39 | u32 guild | `0` |
| 43 | u32 level | |
| 47 | u16 rank, u8 player-kill mode, u32 mount | `0` |

### CHARACTER_MOVE — client, `07`, 16 bytes, sequenced

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `07` |
| 1 | u8 type, u8 arg, u8 rotation | the kind of movement; the units are not known yet |
| 4 | i32 x, i32 y | where the character is going or stopped |
| 12 | u32 time | the client's clock |

The client sends it on its own a few seconds after entering, seen with the real client, and whenever the player moves.

## What the server does

`internal/game/enter.go`:

- **Start (`SELECT`).** `06` loads the character in that slot. An empty slot closes the connection, since the client only offers slots it shows a character in. The character gets a vid, unique in this server process. Then the server sends phase `LOADING`, `MAIN_CHARACTER`, `POINTS` and `SKILL_LEVEL`.
- **One connection per character** (`internal/game/claims.go`). Start claims the character for its connection. If another connection holds it, which the client's key reuse allows, that connection is closed. Start then waits for it to save and reads the character afterwards, so the newcomer loads where the other left off. The wait is bounded, at eleven seconds: long enough for a periodic save of the character already under way and the save on leaving. It ends at shutdown. A newcomer that gives up closes its connection, but its claim lets go only after the connection it waited on has, so a third connection never reads the character before the slow one has saved. One login per account stays the rule at the auth port (`ALREADY`).
- **Loading.** A second `06` for the same slot is ignored; any other slot closes the connection. `CLIENT_VERSION` is logged and otherwise ignored. `ENTERGAME` moves to `GAME` and sends the time, the channel, and the player's own character: `CHARACTER_ADD`, then `CHARACTER_ADDITIONAL_INFO`, then what the player blocks, as the command chat `setblockmode N` ([chat.md](chat.md)). The parts are the armour and weapon worn, or the character's look when no armour is ([equipment.md](equipment.md)).
- **In the world.** `CHARACTER_MOVE` is accepted, and the session keeps the reported position. The others in range see the move. Chat, ON_CLICK and TARGET are handled too (see [chat.md](chat.md) and [other-players.md](other-players.md)). Any other packet closes the connection for now, and the log names its header.
- **Where a character appears.** The first time, at its kingdom's start, written into its row at creation. Afterwards, where it stood when its connection closed: logout, a dropped connection and a server shutdown all go through the session's close, which saves the position. A position is saved only if it lies on a map the server knows (`internal/world`); otherwise the last good one stays and the server logs a warning. The map check uses each map's area from the client's `atlasinfo.txt`, in cells of 25600; without warps, the known maps are the three first villages. While the character stays in the world, the server also saves it every minute (`internal/game/save.go`): its position as the world record has it, as the save on leaving does too, its progress ([experience.md](experience.md)), and its hit points and mana. A hard crash loses at most that minute. A position on no known map is logged once, on leaving, not every minute. The saves run one player after another, two statements each; at a few thousand players a round could outlast the minute, and would then become one batched statement. The periodic save copies what it keeps under the world's lock and writes after letting go; a save of the character under way when it leaves lands before the one on leaving, so the last write is always the newest. A new login waits up to eleven seconds for the old connection to let go, long enough for both saves.
- **Derived values** (`internal/account/points.go`) are worked out, not stored:
  - **Max hit points:** 700 + 50 per vitality + 100 per level.
  - **Max mana:** 500 for warrior and ninja, 700 for sura and shaman, + 20 per intelligence + 50 per level.
  - **Stamina, speeds and exp:** stamina 1000, attack and move speed 150. The next level needs 300 exp at level 1; other levels send 0 until they matter.
  - **Current values:** hit points and mana are what the character was last saved with, cut to its maximum now, which a later change to the formula or, one day, taking off an item can lower. A character never saved in play has all of them. They are saved with its progress, on leaving and every minute, except while it lies dead ([dying.md](dying.md)).

  A new sura has 950 hit points and 850 mana.
- **Yang** is stored, not derived: what the character has, kept in its row as it changes ([drops.md](drops.md)), and sent in POINTS at index 11.

## How we know

- **Map areas:** the client's `atlasinfo.txt` (name, base x and y, width and height in cells); open-mt2 uses the same 25600-unit cell.
- **The flow and layouts:** open-mt2, whose flow brings this client into the world, and the client's `introselect.py`, `introloading.py` and `networkmodule.py`.
- **The derived values:** open-mt2's formula and `jobs.json` constants, applied to our starting stats, which come from the client. No source closer to the original game has been checked; this is the first thing to revisit when one is.

## What proves it

- `internal/protocol/protocol_test.go`: every new packet, byte for byte.
- `internal/account/points_test.go`: each class's level-1 hit points and mana, and growth per level; entering with all of them when never saved, with those saved, and cut to the maximum.
- `internal/server/server_test.go`:
  - Start then enter, with every packet compared;
  - the same slot asked for again while loading;
  - the client's version while loading;
  - a move in the world;
  - another slot while loading;
  - an empty slot;
  - entering before Start;
  - a move while loading, entering twice, and the version report outside loading;
  - a character starting where it stood when it left;
  - a position on no known map not saved;
  - a character started on a second connection, which closes the first and loads where it left off;
  - a server shutdown saving where the player stood.
- `internal/game/claims_test.go`: a takeover stopping the holder and waiting for it, and a claim given up letting go only after the holder has.
- `internal/store/store_test.go`: hit points and mana kept with the progress, and none for a character never saved.
- `internal/server/experience_test.go` and `combat_test.go`: entering again with the hit points and mana kept, and the hit points a bite left kept over Change character.
- `internal/game/save_test.go`: a player in the world saved as they are now, hit points and mana included, with the position the world record has; a player who left not saved again; a save under way as the player leaves landing before the save on leaving; leaving saving where the world record has the player; the saver saving every player until stopped, and stopping mid-round ending the round; the periodic save reading nothing the connection writes unlocked, under the race detector.
- `internal/world/maps_test.go`: map corners, and every kingdom's start lying on a known map.

## Not done yet

- Movement checked against the map's terrain rather than only its area.
- Monsters and NPCs.
- Quickslots; items and equipment are on [items.md](items.md) and [equipment.md](equipment.md).
- Quests.
- Play time.
- Warps between maps.
