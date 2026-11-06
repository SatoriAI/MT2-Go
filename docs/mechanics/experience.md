# Experience and levels

## What the player sees

Killing a monster fills the experience bar, the four orbs at the bottom of the screen, by that monster's experience, more or less depending on how far its level is from the player's. Filling the bar raises the level: the level-up effect plays, the number over the character's head goes up for everyone near, health and mana grow and refill, and three stat points appear in the character window, where its + buttons put them into strength, vitality, dexterity or intelligence. Level, experience, unspent points and stats are kept between logins.

Monsters far below the player give almost nothing, and each level needs much more than the last, so a dog worth 15 is worth killing only for a few levels.

## What the client does

- **Points:** the client keeps a character's values in an array of 255 points, which POINTS (`10`) sends whole on entering and POINT_CHANGE (`11`) changes one at a time: level 1, experience 3, the next level's need 4, hit points 5 and their maximum 6, mana 7 and its maximum 8, strength 12, vitality 13, dexterity 14, intelligence 15, unspent stat points 26 (`Packet.h`).
- **The bar** is drawn from experience and the next level's need, in quarters (`uitaskbar.py`); the client has no table of its own.
- **A POINT_CHANGE** for the player's own vid sets the point; for the level of anyone's vid, it plays the level-up effect over them, and for another player's vid it updates the level over their head (`PythonNetworkStreamPhaseGame.cpp`, `PythonCharacterManager.cpp`).
- **The + buttons** show while unspent points are above 0, and send `/stat st`, `/stat ht`, `/stat dx` or `/stat iq` as chat (`uicharacter.py`).

## The bytes

### POINT_CHANGE — server, `11`, 17 bytes

| Offset | Field | |
|---|---|---|
| 0 | the header, a 4-byte int in the client's struct | `11 00 00 00` |
| 4 | u32 vid | whose point |
| 8 | u8 point | its index, as above |
| 9 | i32 amount | how much it changed |
| 13 | i32 value | what it is now |

Unspent stat points sit at index 26 of POINTS; experience at 3, as before.

## What the server does

`internal/account/progress.go` holds the rules; `internal/game/experience.go` applies them.

- **The needs:** each level needs the original game's experience to reach the next: 300 from 1, 800 from 2, 33,000 from 10, 472,000 from 20, 2,122,000 from 30, on to 2,029,047,116 from 98. Level 99 is the top and gains nothing more.
- **The level gap:** a player gets a share of a monster's experience by the monster's level less theirs: 1% at 15 or more below, then 5, 10, 20, 30, 50, 70, 80, 85, 90, 92, 94, 96 and 98% at 2 below, 100% at 1 below and level with them, then 105% rising by 5 to 170% at 14 above and 180% at 15 or more.
- **Events** multiply a monster's experience before it is shared ([events.md](events.md)).
- **Sharing:** when a monster dies, each player who hurt it and is still in the world, alive, gets the share of its experience that matches the share of its hit points they took, rounded down, then scaled by their own level gap. What rounding leaves goes to the killer; the share of a player who left is lost. A share of nothing is not sent.
- **Levelling:** experience past a level's need carries into the next, and one kill can raise several levels. Each level brings 3 stat points, and from level 5 one skill point ([skills.md](skills.md)), told once a skill group is chosen. The player is told their experience, new level, next need, stat points, and their new maximum hit points and mana, refilled; everyone who sees them, their new level, which later arrivals see too.
- **Dying** costs 2% of the level's need, never a level ([dying.md](dying.md)).
- **Spending:** `/stat st|ht|dx|iq` moves one point into the stat, up to 90 a stat, and tells the player the stat, their points left, and for vitality or intelligence their new maximum hit points or mana. With no point to spend, at the cap, or for any other name, nothing happens. Attack follows the new stats on the next blow.
- **While in the world** a player's level, experience and stats live in the world record and change only under its lock, since others' kills raise them; the blow's attack is read there too.
- **Kept** with the position when the character leaves the world, by any way out: level, experience, unspent stat points and the four stats (`characters.exp`, `stat_points`), with the hit points and mana ([entering-the-world.md](entering-the-world.md)). They are also saved every minute while the character stays in the world (`internal/game/save.go`, as [entering-the-world.md](entering-the-world.md) describes), so a crash loses at most a minute of play.

## How we know

- **The packets and the client's side:** the client's source and scripts named above.
- **The needs, the level-gap shares, 3 points a level and the top level:** the original game's tables as open-mt2's data records them (`exp.ts`, `expDeltaLevel.ts`, `PlayerPoints.ts`), read for facts. open-mt2 reads one past the end of its gap table for a monster 16 or more levels above, giving nothing; this server counts any gap past 15 as 15. It also shares experience evenly among attackers with a bonus meant for the one who dealt most, which a bug never moves; this server shares by damage, as Dawid chose.
- **The cap of 90 a stat:** the original's; open-mt2's data says 150, its tests 90.
- **This server's choices:** stat points come with each level, where the original gives one at each quarter of the bar; a level-up refills hit points and mana.

## What proves it

- `internal/account/progress_test.go`: the needs; the level gap, both ends and past them; experience short of a need, filling it with the rest carried, several levels at once, the top level; a stat point up to the cap, and none to spend.
- `internal/protocol/protocol_test.go`: POINT_CHANGE byte for byte; stat points at their index in POINTS.
- `internal/game/experience_test.go`: a lone kill's whole experience; a monster far below giving nothing worth sending; one above giving more; a level-up's packets to the player and the onlooker, and the level later arrivals see; sharing by damage with the rounding to the killer; a player who left getting nothing; spending a point, and vitality's maximum; no point or an unknown stat changing nothing.
- `internal/server/experience_test.go`: Hendra killing a dog over real sockets and its 15 kept; the + button spending a point, kept; without points doing nothing; entering again showing the experience and points kept; the character screen after Change character showing a point spent in the world.

## Not done yet

- Skill points, parties sharing experience, and items or premium bonuses.
- Taking stat points back (`/stat-`).
