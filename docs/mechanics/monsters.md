# Monsters and NPCs

## What the player sees

Each first village has its villagers: the weapon and armour dealers, the general store, the storekeeper, the old lady, the Teleporter ([maps.md](maps.md)), and fishermen along the water, each standing where the minimap marks them, with their name over their head. The fields around the village are full, as the original's: some 1,000 monsters a map over all of it, many standing alone and the rest mostly in twos and threes, wild dogs and wolves just past the gate, then alpha and blue wolves, grey wolves, boars, red boars and bears, and at the far edge grizzlies, black and brown bears and tigers. Every few seconds a monster strolls a few steps and stops, facing where it went. Walking away, they fade out like other players; walking back, they are there again. Monsters can be fought, and fight back when hit ([combat.md](combat.md)); NPCs cannot.

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
- **Where they stand** (`internal/world/spawns.go`): a table per first village. The NPCs are at the client's minimap marks for them (`locale/pl/map/metin2_map_*_point.txt`). The monster areas are this server's own places; how many of each kind stand how far out is the original's, as open-mt2's spawn data gives it (below), and the data itself stays out of this repository.
  - **As many as the original's, and as far out** (Dawid, finding the fields still wrong after playing, asked for the original's numbers): open-mt2's spawn data for the three first maps, read for numbers only, never its positions, holds some 930 to 945 monsters a map (Shinsoo 945, Chunjo 927, Jinno 937), each spawned alone, with no packs. A third to two fifths of them stand on their own, two thirds of the clumps; the rest stand within 12 m of one or more others, mostly in twos and threes, rarely up to 11, nearest neighbours some 10 m apart. They reach 680 to 850 m from the kingdom's start, across the whole map. The kinds are ten, the grey wolves among them:

    | Kind | Level | Shinsoo | Chunjo | Jinno | Out from the start, m (most of them, over the three maps) | Back after |
    |---|---|---|---|---|---|---|
    | Dziki Pies | 1 | 36 | 46 | 35 | 60 to 390 | 5 s |
    | Wilk | 3 | 143 | 160 | 147 | 110 to 400 | 15 s |
    | Alfa Wilk | 4 | 160 | 119 | 149 | 170 to 480 | 15 s |
    | Nieb. Wilk | 6 | 138 | 156 | 139 | 220 to 520 | 30 s |
    | Nieb. Alfa Wilk | 9 | 130 | 142 | 150 | 250 to 570 | 15 s in Shinsoo, 30 s |
    | Szary Wilk | 13 | 129 | 89 | 93 | 270 to 600 | 40 s |
    | Szary Alfa Wilk | 16 | 81 | 85 | 100 | 320 to 650 | 40 s |
    | Dzik | 7 | 32 | 38 | 32 | 310 to 640 | 60 s |
    | Czerw. Dzik | 10 | 39 | 39 | 40 | 350 to 670 | 60 s |
    | Niedźwiedź | 12 | 57 | 53 | 52 | 380 to 830 | 60 s |

    Our fields take each of the original's clumps, its kinds, how many and how far from the start, and place it afresh at the same distance in a random direction (a throwaway script, `m1.py` beside the client project's terrain tools), so the counts, the clumps and the gradient are the original's and the places are ours. Where the ground within 25 m of that distance was taken, the clump stands at the nearest free distance, at most 200 m off: 2 of Shinsoo's 687 clumps, ours below included, 125 of Chunjo's 661 and 80 of Jinno's 594, whose open ground is narrower, and Jinno lost 11 clumps for want of any. Each kind comes back after the original's time for it on its map; a clump of two kinds comes back whole on the slower's time.
  - **Our edge:** the original's first maps end at level 16, and its next maps, which we do not have, lead to Seungryong Valley at 20. So beyond four fifths of the original's monsters on each map, out to its farthest, stand 80 more of our own, alone: 30 Niedźw. Grizzly (15), 20 Czar. Niedźw. (19), 15 Brąz. Niedźw. (21) and 15 Tygrys (18), back after 60 s, at the veteran's call ahead of the valley.
  - **How many, per map:** each map in `internal/world/maps.go` has `Mobs`, which multiplies how many monsters each of its clumps and packs holds: 1 as placed, 2 twice as many, 0.5 half, rounded and never fewer than one. A monster alone, NPCs, stones and pack leaders stay one each, and everything keeps its place, so turning it up crowds the same ground rather than spreading wider. It is 1 for every map; raising it raises the world loop's load, so run `make bench` with it.
  - **Where:** clump centres 1,000 apart, 4,000 from a cursed pack, 2,500 from every NPC and 3,500 from every stone. Each clump puts its monsters at random points within 300 of its centre, and they stroll 600 more.
  - **Out of the towns and off the starts:** no clump's ground, its 300 and a stroll of 600, comes within 2,000 of a town, the safe ground the client's attribute map marks (`attr.atr`, cell bit 2, no PvP) and the box its village's NPCs stand in, or of the kingdom's start, which lies outside both: an early layout put wolves 6 m from Shinsoo's start, where a new character appears. Every stone stays 4,000 from the town.
  - **Terrain:** the server keeps no terrain, so a monster strolls anywhere within 600 of its spawn point; the script therefore chose only places where all of that is open ground: no water cell in the attribute map (`attr.atr`), at most 2 in 100 of its cells blocked, such as single trees, and no step steeper than 60 height units per 200 in the height map (`height.raw`, times its HeightScale of 0.5). Of the original's own spots, between half (Chunjo) and seven in ten (Shinsoo) pass this over the 900 a clump may reach, so ours stand on the gentler ground.
  - **The cursed packs:** from 30,000 out from the start stand the cursed kinds, which attack on sight (below): Przeklęty Alfa Wilk leading three Przeklęty Wilk, Przeklęty Nieb. Wilk and Przeklęty Dzik within 42,000; past it Przekl. Nieb. Alfa Wilk, Przekl. Szary Alfa Wilk and Przeklęty Czerw. Dzik each leading three of their kin, and packs of Przeklęty Niedźw. and Przeklęty Szary Wilk. Nine packs a map, each of fixed kinds and counts, back after 60 s; they stay where they stood before the fields followed the original's, as do our own Metin stones' kinds. Furthest out, 48,000 to 60,000 from the start, two more of three Biały Tygrys (115) each, the cursed tiger at level 24, close the gap to the valley, eleven in all; it attacks on sight, so it stands in packs of its own rather than with the tigers. The valley's families never hold a kind that attacks on sight.
- **The world loop** (`internal/game/monsters.go`, `docs/decisions/world-loop.md`) starts with the server. It puts every mob in the world record, then every 100 ms walks each monster whose stroll is due: a random point within 600 of where it was spawned, every 5 to 25 seconds, open-mt2's numbers for an idle monster. A monster fighting a player does not stroll; it walks home when it gives up and strolls again from there ([combat.md](combat.md)).
- **Seeing:** mobs are on the same world record as players, under the same view rule and order (`internal/game/record.go`). A player sees mobs within 8000 on the same map, and a mob walking into view appears before its move, already where it was heading, as a player does. Where a character stands and the way it last faced are kept, so someone meeting a monster after a stroll sees it facing the way it went. Mobs never see each other. A player entering sees the mobs in view in the same single send as the players.
- **The class teachers** stand in a row before each village's temple, the two pavilions joined by a covered bridge (the client's `*-009-hotel`). The minimap marks leave them out, so the spots are this server's, placed at Dawid's call against the client's map objects: in line with the braziers before the temple's stone platform, 12 m out from its middle, with their backs to it, four each side of its middle path at an even 2.5 m apart, as the original stands them, the warrior's and ninja's on one side and the sura's and shaman's on the other, each at least 3 m clear of every map object. Clicking one opens their dialog ([skills.md](skills.md)).
- **The blacksmith, Kowal (20016),** stands near the general store's woman, Handlarka Różności, on the far side of the cart by her shop (the client's `b_general_obj_09`, the palanquin with the red canopy, in Shinsoo and Jinno, a carriage in Chunjo), 3.8 m past the cart, and in Chunjo, where the carriage would hide him from the market, 2.5 m west of that, facing the market as she does ([upgrading.md](upgrading.md)).
- **Where an NPC looks** is its spawn's, sent as ADD's rotation: the teachers, Kowal and Handlarka Różności face the open ground, and the other NPCs rotation 0. A monster enters at a random one of eight directions.
- **Names are players' only:** a whisper to an NPC's name finds nobody.
- **Packs fight together** (`internal/game/aggro.go`): the monsters of the spawns sharing a centre are a pack, a leader among them, and so is a clump of the fields. When one starts a fight, hit or on sight, every packmate alive within 3,000 of it and fighting no one turns on the same player. A pack comes back together: none of it returns while any of it lives, and all of it returns its slowest kind's time after its last member died.
- **Attacking on sight:** a kind the client's `mob_proto` flags AGGR attacks, every tick, the nearest player it sees within 2,000, its AGGRESSIVE_SIGHT, with its pack. It ignores the dead, a player more than 10 levels above it, and anyone for 5 seconds after they enter the world or rise. None of the first maps' field kinds, dogs to tigers, carries the flag, so the fields stay as safe as the original's; only the cursed kinds attack. The loop keeps the aggressive monsters on a list of their own, so an idle tick looks only at those. The client's AGGRESSIVE_HP_PCT is not used: what it did is not settled.

## How we know

- **The client's side:** its source (`ActorInstance.h`, `PythonNetworkStreamPhaseGameActor.cpp`, `InstanceBase.cpp`, `InstanceBaseMovement.cpp`, `GameUtil.cpp`) and data (`npclist.txt`, the minimap point files, `mob_proto`).
- **The values:** the client's `mob_proto`. Its dump in `--dump_proto--/mob_proto.txt` gives levels and speeds with Korean names. The Polish names were read once from the locale's encrypted `mob_proto`, by a throwaway decoder following `PythonNonPlayer.cpp` (an XTEA-encrypted, LZO-compressed table of 255-byte records). Every level it read matched the dump. The decoder is not in this repository.
- **The stroll's numbers and the packet values:** open-mt2, read for facts.

## What proves it

- `internal/mob/mob_test.go`: every kind the world spawns is known and has real speeds; only monsters wander.
- `internal/world/spawns_test.go`: every spawn area lies on its map; spawns are in world coordinates; a blacksmith in each first village; every first village's monster of a field or a cursed kind; 900 to 950 of the original's kinds a map, each at least 25, and 75 to 80 of the edge's; at least half the clumps one monster, none over 11 or of more than two kinds; the fields reaching 650 m, the first wolves nearer than the blue and the blue than the grey, and the edge's beyond three quarters of the original's kinds; each field kind back after the original's time; each cursed pack in its band; every valley pack 3 to 5 and each cursed pack a leader with three of its kin or 3 to 5 of one kind; clump centres 1,000 apart, 4,000 from a cursed pack, 2,500 from every NPC and 3,500 from every stone; no clump's ground within 2,000 of a town or a start and no stone within 4,000 of it; a valley pack carrying its time to come back and an NPC where it looks; a map's `Mobs` multiplying its own clumps and packs only, never below one, leaving a monster alone, leaders, NPCs and stones; every map setting `Mobs`.
- `internal/game/combat_test.go`: a monster coming back after its spawn's own time.
- `internal/game/packs_test.go`:
  - hit one of a pack and the pack coming, another pack beside it not; a leader coming with its kin; a packmate 3,001 off or fighting another staying out; a pack coming back only whole, its time after its last died, and a clump of dogs and a wolf on the wolf's 15 s;
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
- The original's mini-bosses on the first maps: 15 single ones (151 to 155, a tenth of an hour each), its groups of 317, 318 and 139 to 142, and a boss, 191, on Shinsoo's and Jinno's. Our cursed packs stand in for them.
- Warps, and the other maps.
- Shops and talking to NPCs.
- Terrain: the server has no heights or walls of its own. The areas are chosen so that no stroll can reach a blocked or water cell, but a monster does not avoid anything itself.
