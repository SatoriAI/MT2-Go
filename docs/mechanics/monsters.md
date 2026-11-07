# Monsters and NPCs

## What the player sees

Each first village has its villagers: the weapon and armour dealers, the general store, the storekeeper, the old lady, and fishermen along the water, each standing where the minimap marks them, with their name over their head. In the fields around the village are wild dogs, wolves, alpha wolves and boars. Every few seconds a monster strolls a few steps and stops, facing where it went. Walking away, they fade out like other players; walking back, they are there again. Monsters can be fought, and fight back when hit ([combat.md](combat.md)); NPCs cannot.

## What the client does

- **CHARACTER_ADD's type** decides how the client builds a character (`ActorInstance.h`): 0 a monster, 1 an NPC, 2 a stone, 6 a player.
- **A monster is built from its ADD alone.** The client names it from its own mob data (`mob_proto` in the locale) and shows no level over it (`PythonNetworkStreamPhaseGameActor.cpp`, `RecvCharacterAppendPacket`). An INFO for a monster's vid is dropped as one it was not waiting for.
- **An NPC is built like a player:** it waits for the INFO right after the ADD, and shows the name the INFO carries, not its own. Level 0 hides the "Lv" tag.
- **The race is the vnum,** which the client maps to a model through `npclist.txt`. A speed of 0 freezes the animation (`InstanceBaseMovement.cpp`), so mobs carry their real move and attack speed.
- **A MOVE of type 0** (the client's FUNC_WAIT) runs the character to the point and stands it there (`InstanceBase.cpp`). The rotation is in steps of 5 degrees, clockwise from the world's +y, as the client's `GetDegreeFromPosition` reckons it.

## The bytes

The packets are the ones on [other-players.md](other-players.md) and [entering-the-world.md](entering-the-world.md), with these values:

| Packet | Field | A monster | An NPC |
|---|---|---|---|
| ADD `01` | type | `00` | `01` |
| | race | vnum | vnum |
| | move, attack speed | the kind's | the kind's |
| INFO `88` | | not sent | name in cp1250, level 0, parts, kingdom and guild 0 |
| MOVE `03` | type, arg | `00`, `00` | never sent |
| | rotation | towards the point, in steps of 5° | |
| | time | the server clock | |
| | duration | 0 | |

## What the server does

- **What each kind is** (`internal/mob`): only the kinds the world spawns, with the client's `mob_proto` move and attack speeds. NPCs never move. The names are the Polish client's own, in UTF-8, turned into cp1250 for the INFO (`internal/game/wire.go`).
- **Where they stand** (`internal/world/spawns.go`): a table per first village. The NPCs are at the client's minimap marks for them (`locale/pl/map/metin2_map_*_point.txt`). The monster areas are this server's own: open-mt2's spawn files come from the original server's data, which this repository keeps out. Each village has five, 10000 to 30000 from its start and at least 7000 from its shops, and a second ring of four, 30000 to 55000 out and 9000 from anything else, the stronger the further: blue wolves; alpha blue wolves with red boars; bears with grizzlies; tigers. All were chosen by a throwaway script over the client's own terrain: no cell the attribute map (`attr.atr`) marks blocked or water within the area's spread plus a stroll, and no step steeper than 60 height units per 200 in the height map (`height.raw`). The first guess, placed by eye on the minimap, put one Jinno area on a cliff: 69% of it blocked. Each area puts its count of monsters at random points within its spread.
- **The world loop** (`internal/game/monsters.go`, `docs/decisions/world-loop.md`) starts with the server. It puts every mob in the world record, then every 100 ms walks each monster whose stroll is due: a random point within 600 of where it was spawned, every 5 to 25 seconds, open-mt2's numbers for an idle monster. A monster fighting a player does not stroll; it walks home when it gives up and strolls again from there ([combat.md](combat.md)).
- **Seeing:** mobs are on the same world record as players, under the same view rule and order (`internal/game/presence.go`). A player sees mobs within 8000 on the same map, and a mob walking into view appears before its move, already where it was heading, as a player does. Where a character stands and the way it last faced are kept, so someone meeting a monster after a stroll sees it facing the way it went. Mobs never see each other. A player entering sees the mobs in view in the same single send as the players.
- **The sura's teachers** stand a few steps east of the old lady in each village, at this server's own spots, as the minimap marks leave them out, not checked against the terrain; clicking one opens their dialog ([skills.md](skills.md)). The blacksmith, Kowal (20016), stands a few steps further east, on the same terms ([upgrading.md](upgrading.md)).
- **Names are players' only:** a whisper to an NPC's name finds nobody.

## How we know

- **The client's side:** its source (`ActorInstance.h`, `PythonNetworkStreamPhaseGameActor.cpp`, `InstanceBase.cpp`, `InstanceBaseMovement.cpp`, `GameUtil.cpp`) and data (`npclist.txt`, the minimap point files, `mob_proto`).
- **The values:** the client's `mob_proto`. Its dump in `--dump_proto--/mob_proto.txt` gives levels and speeds with Korean names. The Polish names were read once from the locale's encrypted `mob_proto`, by a throwaway decoder following `PythonNonPlayer.cpp` (an XTEA-encrypted, LZO-compressed table of 255-byte records). Every level it read matched the dump. The decoder is not in this repository.
- **The stroll's numbers and the packet values:** open-mt2, read for facts.

## What proves it

- `internal/mob/mob_test.go`: every kind the world spawns is known and has real speeds; only monsters wander.
- `internal/world/spawns_test.go`: every spawn area lies on its map; spawns are in world coordinates; a blacksmith and the second ring in each first village, the ring 30000 to 55000 from the start and 9000 from any other spawn.
- `internal/game/monsters_test.go`, with a fixed seed and the time given:
  - a monster standing until its stroll is due, then walking within 600 of home with type 0, facing where it goes;
  - resting between strolls;
  - strolling unseen when out of view;
  - an NPC never strolling;
  - a monster walking into view shown before its move;
  - a spawn putting its count within its spread;
  - an unknown kind refused;
  - the client's facing;
  - mobs not seeing each other;
  - a monster met after a stroll facing the way it went;
  - no stroll reaching off its spawn's map.
- `internal/server/monsters_test.go`:
  - a player entering seeing the NPC with its name and the monster by its add alone, and nothing out of view;
  - walking off swapping which mobs are shown;
  - a whisper to an NPC finding nobody.

What a monster leaves when it dies is on [drops.md](drops.md).

## Not done yet

- Aggressive monsters noticing players.
- Bosses and groups of monsters. Metin stones have their own page, [stones.md](stones.md).
- Warps, and the other maps.
- Shops and talking to NPCs.
- Terrain: the server has no heights or walls of its own. The areas are chosen so that no stroll can reach a blocked or water cell, but a monster does not avoid anything itself.
