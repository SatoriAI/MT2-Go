# Monsters and NPCs

## What the player sees

Each first village has its villagers: the weapon and armour dealers, the general store, the storekeeper, the old lady, and fishermen along the water, each standing where the minimap marks them, with their name over their head. The fields around the village are full: some 800 monsters a map, in packs of three to five that change each time they come back, wild dogs and wolves just past the gate, then alpha wolves and boars, blue wolves, red boars and bears, and further out grizzlies and tigers, each stretch holding two to four kinds. Every few seconds a monster strolls a few steps and stops, facing where it went. Walking away, they fade out like other players; walking back, they are there again. Monsters can be fought, and fight back when hit ([combat.md](combat.md)); NPCs cannot.

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
  - **The packs,** settled by the veteran and the designer, the veteran deciding, after Dawid found the fields thin and their kinds changing in steps: some 800 monsters a map, in about 200 packs. Each pack is of a **family**: dogs, wolves (wolves, alpha wolves, blue wolves, alpha blue wolves), boars (boars, red boars), bears (bears, grizzlies) or tigers. Each time a pack comes back, whole, it holds **3 to 5** monsters, each of a kind of its family picked by the kinds' weights where it stands, so the same place holds an alpha wolf with three wolves one time and five blue wolves the next. Families never mix: a dog never stands in a pack with a bear.
  - **The gradient:** distances are out from the village, the box its NPCs stand in and the safe ground beside it, not from the start, which lies by or in the town. Each kind is absent before its band, more and more likely up to its peak, most likely through its plateau, less and less to its end, so each stretch holds two kinds by the gate and three or four further out, the harder the further out:

    | Kind | Level | Band | Most likely |
    |---|---|---|---|
    | Dziki Pies | 1 | 1,500 to 12,000 | 1,500 to 6,000 |
    | Wilk | 3 | 2,000 to 20,000 | 6,000 to 12,000 |
    | Alfa Wilk | 4 | 6,000 to 25,000 | 12,000 to 18,000 |
    | Dzik | 7 | 7,000 to 25,000 | 12,000 to 20,000 |
    | Nieb. Wilk | 6 | 12,000 to 30,000 | 18,000 to 25,000 |
    | Nieb. Alfa Wilk | 9 | 17,000 to 35,000 | 23,000 to 30,000 |
    | Czerw. Dzik | 10 | 22,000 to 40,000 | 28,000 to 35,000 |
    | Niedźwiedź | 12 | 25,000 to 50,000 | 32,000 to 40,000 |
    | Niedźw. Grizzly | 15 | 33,000 to 55,000 | 42,000 to 50,000 |
    | Tygrys | 18 | 40,000 to 55,000 | 48,000 to 55,000 |

  - **How many where:** packs per 5,000 out from the village, about four monsters each: 5, 20, 28, 29, 26, 24, 21, 18, 14, 10 and 6 out to 55,000, so the fields are fullest from 10,000 to 25,000 and thin out beyond. Shinsoo holds 198 packs, Chunjo and Jinno 197 each; between 5,000 and 10,000 the towns and starts leave Shinsoo and Chunjo ground for 17 of the 20, Chunjo's map edge 9 of the 10 between 45,000 and 50,000, and Jinno's town and edge 4 of the 5 within 5,000 and 13 of the 16 past 45,000. Both debaters judged the original's fields to hold more, some 900 to 1,300 a map, but spread over many more kinds than these ten; the total rises when kinds are added.
  - **Coming back:** a pack comes back 30 s after its last monster died within 18,000 of the village, 45 s within 30,000, 60 s further out.
  - **How many, per map:** each map in `internal/world/maps.go` has `Mobs`, which multiplies how many monsters each of its packs holds: 1 as placed, 2 twice as many (a family's 6 to 10), 0.5 half, rounded and never fewer than one. NPCs, stones and pack leaders stay one each, and the packs keep their places, so turning it up crowds the same ground rather than spreading wider. It is 1 for all three first maps; raising it raises the world loop's load, so run `make bench` with it.
  - **Where:** pack centres 2,000 apart within 30,000 of the village, 2,500 where either lies further, 4,000 from a cursed pack, 2,500 from every NPC and 3,500 from every stone's spot. Each pack puts its monsters at random points within 800 of its centre, and they stroll 600 more.
  - **Out of the towns and off the starts:** no pack's ground, its 800 and a stroll of 600, comes within 2,000 of a town, the safe ground the client's attribute map marks (`attr.atr`, cell bit 2, no PvP) and the box its village's NPCs stand in, or of the kingdom's start, which lies outside both: the first layout of families put wolves 6 m from Shinsoo's start, where a new character appears. Every stone's spot stays 4,000 from the town.
  - **Terrain:** the places were chosen by a throwaway script over the client's own terrain: within the pack's 800 and a stroll of 600, no water cell in the attribute map (`attr.atr`), at most 2 in 100 of its cells blocked, such as single trees, and no step steeper than 60 height units per 200 in the height map (`height.raw`, times its HeightScale of 0.5). Each family's pack carries how far out it stands as the script measured it, which the world reads its kinds and time to come back from.
  - **The cursed packs:** from 30,000 out from the start stand the cursed kinds, which attack on sight (below): Przeklęty Alfa Wilk leading three Przeklęty Wilk, Przeklęty Nieb. Wilk and Przeklęty Dzik within 42,000; past it Przekl. Nieb. Alfa Wilk, Przekl. Szary Alfa Wilk and Przeklęty Czerw. Dzik each leading three of their kin, and packs of Przeklęty Niedźw. and Przeklęty Szary Wilk. Nine packs a map, each of fixed kinds and counts, back after 60 s. Biały Tygrys (115), the cursed tiger, is level 24, above every band, and is left out. A family never holds a kind that attacks on sight.
- **The world loop** (`internal/game/monsters.go`, `docs/decisions/world-loop.md`) starts with the server. It puts every mob in the world record, then every 100 ms walks each monster whose stroll is due: a random point within 600 of where it was spawned, every 5 to 25 seconds, open-mt2's numbers for an idle monster. A monster fighting a player does not stroll; it walks home when it gives up and strolls again from there ([combat.md](combat.md)).
- **Seeing:** mobs are on the same world record as players, under the same view rule and order (`internal/game/record.go`). A player sees mobs within 8000 on the same map, and a mob walking into view appears before its move, already where it was heading, as a player does. Where a character stands and the way it last faced are kept, so someone meeting a monster after a stroll sees it facing the way it went. Mobs never see each other. A player entering sees the mobs in view in the same single send as the players.
- **The class teachers** stand in a row before each village's temple, the two pavilions joined by a covered bridge (the client's `*-009-hotel`). The minimap marks leave them out, so the spots are this server's, placed at Dawid's call against the client's map objects: in line with the braziers before the temple's stone platform, 12 m out from its middle, with their backs to it, four each side of its middle path at an even 2.5 m apart, as the original stands them, the warrior's and ninja's on one side and the sura's and shaman's on the other, each at least 3 m clear of every map object. Clicking one opens their dialog ([skills.md](skills.md)).
- **The blacksmith, Kowal (20016),** stands near the general store's woman, Handlarka Różności, on the far side of the cart by her shop (the client's `b_general_obj_09`, the palanquin with the red canopy, in Shinsoo and Jinno, a carriage in Chunjo), 3.8 m past the cart, and in Chunjo, where the carriage would hide him from the market, 2.5 m west of that, facing the market as she does ([upgrading.md](upgrading.md)).
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
- `internal/world/spawns_test.go`: every spawn area lies on its map; spawns are in world coordinates; a blacksmith in each first village; each family's pack saying how far out it stands, as measured from the village, with a kind of its family there; the kinds at 3,000, 10,000, 26,000 and 45,000 out; a kind's weight rising, level and falling; each cursed pack in its band; every family's pack 3 to 5 and each cursed pack a leader with three of its kin or 3 to 5 of one kind; pack centres apart, and 2,500 from every NPC and 3,500 from every stone's spot; each village with at least 760 monsters on average, 60 within 10,000 of the village, and fewer each 5,000 further past 20,000; no pack's ground within 2,000 of a town or a start and no stone's spot within 4,000 of it; a pack carrying its time to come back and an NPC where it looks; a map's `Mobs` multiplying its own packs only, never below one, leaving leaders, NPCs and stones; every map setting `Mobs`.
- `internal/game/combat_test.go`: a monster coming back after its spawn's own time.
- `internal/game/packs_test.go`:
  - hit one of a pack and the pack coming, another pack beside it not; a leader coming with its kin; a packmate 3,001 off or fighting another staying out; a pack coming back only whole, its time after its last died;
  - an aggressive monster attacking within 2,000 with its pack, not at 2,001; a passive kind never looking out; the dead, a player 11 levels above, and one just risen left alone, one 10 above or risen 5 s ago attacked; the nearer of two attacked; entering and rising leaving a player unseen 5 s;
  - a cursed wolf spawned in the world attacking on the next tick; nothing aggressive within 10,000 of a village.
- `internal/game/families_test.go`: a family's pack holding 3 to 5 of its kinds by their weights, over 300 worlds every size and wolves about three times the alpha wolves; coming back rolled afresh; and refusing a kind that attacks on sight.
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
