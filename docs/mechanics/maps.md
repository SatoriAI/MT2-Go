# Maps and Seungryong Valley

## What the player sees

The world is four maps: each kingdom's first village (Shinsoo's Strefa Yongan, Chunjo's Strefa Joan, Jinno's Strefa Pyungmoo) and **Dolina Seungryong**, the valley all three kingdoms share. From level 20 three ways lead there, each to the open ground of its southern plateau:
- **the gate** at the far edge of each first village, the one with its arch and platform;
- **the Teleporter** on each village's market square, for 1,500 yang, whose offer says what lives there: «Dolina Seungryong: orki od 26 do 49 poziomu.»;
- **the teleport ring** ([teleporting.md](teleporting.md)).

Below level 20 each says «Dolina Seungryong jest dostępna od 20 poziomu.». The valley's southern gate takes a player home, a step inside their own kingdom's gate to it. Each village's other gate and the valley's other two say «Ta droga jest jeszcze zamknięta.».

The valley is orcs only:
- ordinary orcs at its edge;
- their scouts, warriors and sorcerers further in;
- the elite orcs beyond;
- the generals and black orcs at its heart, around Wódz Orków, the orc lord, alone in the middle.

Its Metin stones are stronger than the villages'. Dying there and restarting in town brings the player back at the valley's arrival, not in their village.

## What the client does

- **The map** is whichever map of its `atlasinfo.txt` holds the character's position in MAIN_CHARACTER: it loads that map's terrain, objects and minimap, and nothing names the map on the wire ([entering-the-world.md](entering-the-world.md)). One game server covers every map, so going to another map is a warp to coordinates on it, which the client follows by connecting again ([teleporting.md](teleporting.md)).
- **Walking:** the client stops a character only at blocked cells of the map's attribute map (`attr.atr`, bit 1). Water (bit 2) can be walked; in the valley the plateaus are joined by water channels.

## The bytes

None of their own: the map follows from the position in MAIN_CHARACTER and WARP.

## What the server does

- **The maps** are `internal/world/maps.go`, the client's `atlasinfo.txt` entries:

  | Map | Base | Size |
  |---|---|---|
  | metin2_map_a1 (Shinsoo) | 409,600, 896,000 | 4 × 5 cells of 25,600 |
  | metin2_map_b1 (Chunjo) | 0, 102,400 | 4 × 5 |
  | metin2_map_c1 (Jinno) | 921,600, 204,800 | 4 × 5 |
  | map_a2 (Dolina Seungryong) | 256,000, 665,600 | 6 × 6 |

  `MapAt` says which map a point lies on: it decides who sees whom, and a position on no map is never saved.
- **The valley's arrival** is (82,000, 137,500) on the map, (338,000, 803,100) in the world, on its bottom-middle plateau: within 30 m it is dry, unblocked and gentle. The client session chose it over the literal southern rim, which is rock, where the nearest flat ledge is clear only within 15 m.
  - The ring brings players there, and a restart in town on the valley puts them there.
  - A player who leaves while dead in the valley is saved there, as a restart in town would leave them.
  - A map a kingdom starts on has no arrival, and its town is the kingdom's start.
- **Only orcs**, in one family, the orcs, picked by weight as on the first maps ([monsters.md](monsters.md)). The gradient runs inward:
  - **Depth.** Each pack's `out` is its depth: how far in it stands from the ring through the arrival about the centre. That is the arrival's distance from the centre, 58,483, less the pack's own, and never below 0.
  - **The centre** is the open ground nearest the map's middle, (74,500, 79,500).
  - **Why the centre.** The debate measured from it rather than from the arrival, so that the kingdom entrances still to come, the three warp marks on the client's minimap, leave every pack where it is.

  | Kind | Level | Band (depth) | Most likely |
  |---|---|---|---|
  | Ork | 26 | 0 to 22,000 | 0 to 12,000 |
  | Ork Zwiadowca | 29 | 4,000 to 30,000 | 12,000 to 20,000 |
  | Ork Wojownik | 31 | 12,000 to 36,000 | 20,000 to 28,000 |
  | Ork Czarodziej | 33 | 18,000 to 42,000 | 26,000 to 34,000 |
  | Elit. Ork | 34 | 26,000 to 46,000 | 34,000 to 40,000 |
  | Elit. Ork Zwiadowca | 36 | 30,000 to 50,000 | 36,000 to 44,000 |
  | Elit. Ork Wojownik | 39 | 34,000 to 54,000 | 40,000 to 48,000 |
  | Elit. Ork Czarodziej | 40 | 36,000 to 56,000 | 42,000 to 50,000 |
  | Elit. Ork Generał | 44 | 44,000 to the centre | 50,000 on |
  | Czarny Ork | 46 | 48,000 to the centre | 54,000 on |
  | Czarny Ork Zwiadowca | 49 | 52,000 to the centre | 56,000 on |

- **How many where:** 184 family packs.
  - **Per 5,000 of depth** the script asked for 30, 28, 26, 22, 18, 17, 15, 14, 12, 10, 8 and 6.
  - **Where it fell short:** the ground gave all but 25,000 to 30,000 (12 of 17), 30,000 to 35,000 (4 of 15) and the last 5,000 (none). There, a ring of cliffs and water about the middle leaves little open ground, and only 13 cells in 100 of the map are open enough for a pack.
  - **Leaders:** eight more packs of a leader with three of its kin:
    - Ork Wojownik with Orks, 20,000 to 35,000 deep (four);
    - Elit. Ork Generał with Elit. Ork Wojownik, 40,000 to 50,000 (three);
    - Czarny Ork with Elit. Orks, past 50,000 (one).
  - **The total:** with the orc lord, 193 packs, some 760 orcs. The debate weighed 150 to 250, and the veteran settled on about 200.
  - **By the arrival** a pack holds 3 to 4, not 3 to 5, within 12,000 of it, a kindness to a level-20 player.
  - **Coming back:** as on the first maps, 30, 45 or 60 s by depth.
  - **None of these orcs attacks on sight;** mob_proto flags only the lord.
- **The orc lord, Wódz Orków (691),** level 50, attacks on sight. It stands alone at the centre: the debate's escort of elites would have shared its pack, and so its time to come back.
  - **Coming back:** 20 to 30 minutes after it dies, at a point within those ten minutes that the nanoseconds of its death pick. A roll kept on the monster would have grown every monster, which the loop walks ten times a second.
  - **Waking:** when it appears, everyone on the valley is told «Wódz Orków przebudził się!», as a stone's appearance is told.
- **Where:**
  - **Spacing:** pack centres stand 2,000 apart less than 30,000 deep and 2,500 deeper, and 3,500 from every stone's spot.
  - **The arrival:** no pack's ground, its 800 and a stroll of 600, comes within 2,500 of it.
  - **Terrain:** each place passed the same terrain check as the first maps' fields: no water, at most 2 in 100 cells blocked, no step steeper than 60.
  - **The script:** `MT2-Godot/work/tools/mt2go-terrain/valley.py` placed them, over the client's extracted `map_a2` files.
- **The stones** ([stones.md](stones.md)): nine, each with three spots of its own, 9,000 between spots and 6,000 from the arrival:
  - three Metin Czerni (level 25) in the outer 20,000;
  - three Metin Ciemności (30) 15,000 to 35,000 deep;
  - two Metin Zazdrości (35) 30,000 to 48,000 deep;
  - one Metin Duszy (40) past 45,000.

  Each calls the orcs about it, at most four levels above itself, where the first maps' stones call none above their own, and comes back after ten minutes.
- **What they leave** ([drops.md](drops.md)):
  - **Strength against orcs,** a bonus valley gear may carry, strikes them harder ([bonuses.md](bonuses.md)).
  - **An orc** leaves the middle red potion or the small blue one, each 8 in 100, and yang rising with its level. It also leaves a skill book 3 in 1,000 and a piece of the gear of levels 21 to 36 once in 100: the gear that on the first maps only the cursed monsters and the stones leave.
  - **The lord** leaves 3,000 to 5,000 yang, that gear 3 times in 10, a book once in 10, a soul stone and Zwój Błogosławieństwa each 3 times in 10, and Zaczarowanie Przedmiotu 2 times in 10.
  - **The stones** leave a soul stone 2 to 5 times in 100 and Zwój Błogosławieństwa 1 to 4 times in 100, the deeper the more ([stones.md](stones.md), [upgrading.md](upgrading.md)).

## How we know

- **The maps:**
  - the client's `atlasinfo.txt` (`map_a2 256000 665600 6 6`);
  - the valley's `setting.txt`, whose base agrees;
  - the Polish `locale_game.txt` for its name;
  - its three warp marks in `locale/pl/map/map_a2_point.txt`.
- **The client's maps:** the client has no data for the other kingdoms' valleys (`map_b2`, `map_c2`), only their atlas entries, so this one is shared.
- **The orcs:** stats from the client's `--dump_proto--/mob_proto.txt`, and names from its Polish `mob_proto`. The yang, the stones' hit points, experience and drops, and the loot are this server's.
- **The design:** the debate of the veteran, the designer and the youth-games expert, with the veteran deciding ties: level 20, orcs only, the gradient from the centre, no camp, rising at the arrival, the lord's announcement and its 20 to 30 minutes.

### The ways in

- **The gates** are the client's WARP marks on each map's minimap (`locale/pl/map/*_point.txt`), with a gate object on each but Shinsoo's second, which is bare ground.
  - **Passing:** whoever walks within 6 m of a mark, the platform's inner half-width, goes through, once until they walk out of it. There is no question, as in the original: the gates stand at the map's edges, walked to on purpose, and the level is the guard.
  - **Refused** while dead, trading, or at a shop or the blacksmith: «Nie możesz teraz przejść przez bramę.».
  - **Which leads where:** each village's first mark is named for its kingdom's second map (자양현, 복정현, 박라현), and the valley's three marks carry the same three names, so in the original the valley lay beyond the second maps. Until they exist, that first gate, which is also the one further from the village, leads to the valley. The other, named for the old man (노인), stays closed, and so do the valley's two that face the other second maps.
  - **Landing:** 20 m inside a gate, on ground the client session checked open. Players coming from a first village land at the valley's one arrival, which keeps its gradient; they reach home a step inside their own gate.
  - **Code:** `internal/world/portals.go` places them, and `internal/game/gates.go` takes a move through them.
- **The Teleporter (9012),** the client's «Teleporter», stands on each village's market square, at the middle of its shops and blacksmith, as no minimap mark places him.
  - **The price** is 1,500 yang, the veteran's: a service, not a taxi.
  - **The answer** is checked as the soul stone's is: the NPC still in reach, the player free («Nie mogę cię teraz zabrać.»), of the level, and with the yang («Nie masz dość Yang na podróż.»), spent and kept before the warp; the client reads the yang left as it enters the valley.
- **The debate (2026-11-15),** the veteran deciding: no teleporter in the valley, its gate home standing by the arrival; one arrival for all; no foreign villages.

## What proves it

- `internal/world/valley_test.go`:
  - the valley on the map list, with its arrival on it, and no arrival on a kingdom's map;
  - nothing within reach of the arrival;
  - every pack as deep as it says, holding orcs there;
  - the orcs' kinds at five depths;
  - about 200 packs, eight leaders and one lord at the centre;
  - each leader with three of its kin in its band;
  - the stones, their counts, spots and bands;
  - the packs apart.
- `internal/world/spawns_test.go` and `valley_test.go`: a pack by the arrival holding 3 to 4, and 6 to 8 at twice the valley's `Mobs`.
- `internal/mob/mob_test.go`: every monster but the orcs an animal; the valley stones' waves up to four levels above them, their drops and helpers; the rare gear from orcs and the lord, and the lord's book.
- `internal/game/teleport_test.go`: the ring offering the valley, warping to its arrival at level 20 and refusing it at 19.
- `internal/game/dying_test.go`: a fall in the valley rising at its arrival, and at home at the start; restart in town in the valley rising there.
- `internal/game/save_test.go`: one who leaves dead in the valley kept at its arrival.
- `internal/world/portals_test.go`: each map's gates; a gate taking those within 6 m; home inside the kingdom's gate to the valley; a teleporter in each village.
- `internal/game/gates_test.go`: the gate to the valley taking her there at level 20; refusing her at 19 once per approach; a closed gate saying so; the valley's gate taking her home; no passing while trading; the teleporter taking her for its price, and saying why not below the level, without the yang or trading, and nothing on no or once she walked away.
- `internal/game/boss_test.go`: the lord back 20 to 30 minutes after it dies, and its waking told to its map.

## Not yet

- **The way in:** the three kingdom entrances at the minimap's warp marks, and portals.
- **Monsters:** the human monsters (Black Wind, thieves, barbarians), and the other orc lords.
- **Loot:** jewellery, which the next change brings to the valley's drops, and orc teeth.
