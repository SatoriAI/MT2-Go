# Monsters and NPCs

## What the player sees

Each first village has its villagers: the weapon and armour dealers, the general store, the storekeeper, the old lady, and fishermen along the water, each standing where the minimap marks them, with their name over their head. The fields around the village are full: some 415 to 445 monsters a map, in packs of three to five, the first wild dogs a few thousand steps past the gate, then wolves and boars, alpha and blue wolves, and further out bears, grizzlies and tigers. Every few seconds a monster strolls a few steps and stops, facing where it went. Walking away, they fade out like other players; walking back, they are there again. Monsters can be fought, and fight back when hit ([combat.md](combat.md)); NPCs cannot.

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
- **Where they stand** (`internal/world/spawns.go`): a table per first village. The NPCs are at the client's minimap marks for them (`locale/pl/map/metin2_map_*_point.txt`). The monster areas are this server's own: open-mt2's spawn files come from the original server's data, which this repository keeps out.
  - **The packs,** settled by the veteran and the designer, the veteran deciding: some 415 to 445 monsters a map, against 54 before, in packs of 3 to 5, each one kind or a leader among its kin (an alpha wolf with three wolves, an alpha blue wolf with three blue wolves), in bands from the start that overlap a little:

    | From the start | Kinds | Packs | Back after |
    |---|---|---|---|
    | 2,500 to 15,000 | wild dogs | up to 20 | 30 s |
    | 9,000 to 18,000 | wolves, boars, an alpha wolf leading wolves | up to 30 | 30 s |
    | 18,000 to 30,000 | alpha wolves, blue wolves, an alpha blue wolf leading blue wolves | 30 | 45 s |
    | 30,000 to 42,000 | alpha blue wolves, red boars, bears | 13 | 60 s |
    | 42,000 to 55,000 | grizzlies, tigers, bears | 11 | 60 s |

  - **How many, per map:** each map in `internal/world/maps.go` has `Mobs`, which multiplies how many monsters each of its packs holds: 1 as they were placed, 2 twice as many, 0.5 half, rounded and never fewer than one. NPCs, stones and pack leaders stay one each, and the packs keep their places, so turning it up crowds the same ground rather than spreading wider. It is 1 for all three first maps; raising it raises the world loop's load, so run `make bench` with it.
  - **Where:** pack centres 2,000 apart within 18,000, 2,500 within 30,000 and 4,000 beyond, and 2,500 from every NPC and 3,500 from every stone's spot. Each pack puts its monsters at random points within 800 of its centre, and they stroll 600 more.
  - **Out of the towns:** no pack's ground, its 800 and a stroll of 600, comes within 2,000 of a town. A town is the safe ground the client's attribute map marks (`attr.atr`, cell bit 2, no PvP) together with the box its village's NPCs stand in. Every stone's spot stays 4,000 from the town.
    - **The bug:** the packs were first placed in bands from the start, and kept only 2,500 from each NPC. But the start is not the middle of a town: Shinsoo's square lies some 11,000 from it, and Jinno's NPCs spread over 13,000 beside it. So wolves stood among Shinsoo's shops, dogs in Jinno's streets, and 50 monster areas and stone spots in all stood in or beside a town.
    - **The fix:** the packs within 30,000 were placed afresh around the towns. That is why the dogs reach out to 15,000, and the packs near the start stand 2,000 apart rather than 2,500: the town takes much of the ground nearest the start.
  - **Terrain:** the places were chosen by a throwaway script over the client's own terrain: within the pack's 800 and a stroll of 600, no water cell in the attribute map (`attr.atr`), at most 2 in 100 of its cells blocked, such as single trees, and no step steeper than 60 height units per 200 in the height map (`height.raw`, times its HeightScale of 0.5). The very first areas, placed by eye on the minimap, put one of Jinno's on a cliff, 69% of it blocked.
  - **The cursed packs:** further out, from 30,000, stand the cursed kinds, which attack on sight (below): Przeklęty Alfa Wilk leading three Przeklęty Wilk, Przeklęty Nieb. Wilk and Przeklęty Dzik within 42,000; past it Przekl. Nieb. Alfa Wilk, Przekl. Szary Alfa Wilk and Przeklęty Czerw. Dzik each leading three of their kin, and packs of Przeklęty Niedźw. and Przeklęty Szary Wilk. Nine packs a map, placed by the same script among the others, 4,000 from every pack, back after 60 s. Biały Tygrys (115), the cursed tiger, is level 24, above every band, and is left out.
  - **Each village's start** sits by its town, so the nearest pack stands 3,100 out in Shinsoo, 4,500 in Chunjo and 7,900 in Jinno, whose town wraps its start on two sides. Within 15,000 of the start stand 129, 70 and 78 monsters; in all a map holds 444, 420 and 418. The wolves' and boars' band holds 24, 20 and 17 packs of the 30 it had, as the dogs took part of its ground.
- **The world loop** (`internal/game/monsters.go`, `docs/decisions/world-loop.md`) starts with the server. It puts every mob in the world record, then every 100 ms walks each monster whose stroll is due: a random point within 600 of where it was spawned, every 5 to 25 seconds, open-mt2's numbers for an idle monster. A monster fighting a player does not stroll; it walks home when it gives up and strolls again from there ([combat.md](combat.md)).
- **Seeing:** mobs are on the same world record as players, under the same view rule and order (`internal/game/record.go`). A player sees mobs within 8000 on the same map, and a mob walking into view appears before its move, already where it was heading, as a player does. Where a character stands and the way it last faced are kept, so someone meeting a monster after a stroll sees it facing the way it went. Mobs never see each other. A player entering sees the mobs in view in the same single send as the players.
- **The class teachers** stand in a row before each village's temple, the two pavilions joined by a covered bridge (the client's `*-009-hotel`). The minimap marks leave them out, so the spots are this server's, placed at Dawid's call against the client's map objects: in line with the braziers before the temple's stone platform, 12 m out from its middle, with their backs to it, four each side of its middle path at an even 2.5 m apart, as the original stands them, the warrior's and ninja's on one side and the sura's and shaman's on the other, each at least 3 m clear of every map object. Clicking one opens their dialog ([skills.md](skills.md)).
- **The blacksmith, Kowal (20016),** stands by the general store's woman, Handlarka Różności, on the far side of the cart by her shop (the client's `b_general_obj_09`, the palanquin with the red canopy, in Shinsoo and Jinno, a carriage in Chunjo), 3.8 m past the cart, and in Chunjo, where the carriage would hide him from the market, 2.5 m to its west, facing the market as she does ([upgrading.md](upgrading.md)).
- **Where an NPC looks** is its spawn's, sent as ADD's rotation: the teachers, Kowal and Handlarka Różności face the open ground, and the other NPCs rotation 0. A monster enters at a random one of eight directions.
- **Names are players' only:** a whisper to an NPC's name finds nobody.
- **Packs fight together** (`internal/game/aggro.go`): the monsters of the spawns sharing a centre are a pack, a leader among them. When one starts a fight, hit or on sight, every packmate alive within 3,000 of it and fighting no one turns on the same player. A pack comes back together: none of it returns while any of it lives, and all of it returns its band's time after its last member died.
- **Attacking on sight:** a kind the client's `mob_proto` flags AGGR attacks, every tick, the nearest player it sees within 2,000, its AGGRESSIVE_SIGHT, with its pack. It ignores the dead, a player more than 10 levels above it, and anyone for 5 seconds after they enter the world or rise. None of the first maps' own kinds, dogs to tigers, carries the flag, so the fields within 30,000 stay as safe as the original's; only the cursed kinds attack. The loop keeps the aggressive monsters on a list of their own, so an idle tick looks only at those. The client's AGGRESSIVE_HP_PCT is not used: what it did is not settled.

## How we know

- **The client's side:** its source (`ActorInstance.h`, `PythonNetworkStreamPhaseGameActor.cpp`, `InstanceBase.cpp`, `InstanceBaseMovement.cpp`, `GameUtil.cpp`) and data (`npclist.txt`, the minimap point files, `mob_proto`).
- **The values:** the client's `mob_proto`. Its dump in `--dump_proto--/mob_proto.txt` gives levels and speeds with Korean names. The Polish names were read once from the locale's encrypted `mob_proto`, by a throwaway decoder following `PythonNonPlayer.cpp` (an XTEA-encrypted, LZO-compressed table of 255-byte records). Every level it read matched the dump. The decoder is not in this repository.
- **The stroll's numbers and the packet values:** open-mt2, read for facts.

## What proves it

- `internal/mob/mob_test.go`: every kind the world spawns is known and has real speeds; only monsters wander.
- `internal/world/spawns_test.go`: every spawn area lies on its map; spawns are in world coordinates; a blacksmith in each first village; every monster in its band and back after the time its distance gives; every pack 3 to 5 of one kind or a leader with three of its kin; pack centres apart, and 2,500 from every NPC and 3,500 from every stone's spot; each village with at least 380 monsters, at least 70 within 15,000 of the start, the nearest pack within 4,500, or 8,000 for Jinno; no pack's ground within 2,000 of a town and no stone's spot within 4,000 of it; a spawn carrying its time to come back and where its NPC looks; a map's `Mobs` multiplying its own packs only, never below one, leaving leaders, NPCs and stones; every map setting `Mobs`.
- `internal/game/combat_test.go`: a monster coming back after its spawn's own time.
- `internal/game/packs_test.go`:
  - hit one of a pack and the pack coming, another pack beside it not; a leader coming with its kin; a packmate 3,001 off or fighting another staying out; a pack coming back only whole, its time after its last died;
  - an aggressive monster attacking within 2,000 with its pack, not at 2,001; a passive kind never looking out; the dead, a player 11 levels above, and one just risen left alone, one 10 above or risen 5 s ago attacked; the nearer of two attacked; entering and rising leaving a player unseen 5 s;
  - a cursed wolf spawned in the world attacking on the next tick; nothing aggressive within 10,000 of a village.
- `internal/game/monsters_test.go`: an NPC standing facing where its spawn says.
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

- A cap on how many packs may chase one player; the debaters left it out.
- Bosses. Metin stones have their own page, [stones.md](stones.md).
- Warps, and the other maps.
- Shops and talking to NPCs.
- Terrain: the server has no heights or walls of its own. The areas are chosen so that no stroll can reach a blocked or water cell, but a monster does not avoid anything itself.
