# Metin stones

## What the player sees

**On the map:** seven Metin stones stand around each first village, further out the stronger they are:

| Stone | Level | How far out | Per village | Its waves |
|---|---|---|---|---|
| Metin Cierpienia (8001) | 5 | 25,000 to 35,000 | 3 | three wolves, two alpha wolves |
| Metin Walki (8002) | 10 | 35,000 to 45,000 | 2 | three blue wolves, two alpha blue wolves |
| Metin Bitwy (8003) | 15 | 45,000 to 55,000 | 2 | two red boars, two bears, a grizzly |

**Fighting one:**
- A stone never moves and never strikes back. A wave of its monsters already stands around it.
- Each fifth of its hit points the player knocks off calls another wave, which goes straight for them. A stone never has more than six of its monsters alive at once.
- As it wears down it smokes, thicker the closer it is to breaking.
- Left alone, it starts to heal ten to fifteen seconds after the last hit.

**Destroyed:**
- It breaks apart. Whoever did it the most damage is told «Zniszczyłeś kamień Metin.» and the drops are kept for them.
- Experience is shared out by damage, as for any monster.
- Ten minutes later the stone appears again, at one of its three spots, and everyone on the map sees «Pojawił się Metin Cierpienia.» across the top of the screen.

| Stone | Hit points | Yang | Potions | Claws or teeth | A piece of the first gear | Experience |
|---|---|---|---|---|---|---|
| Metin Cierpienia | 7,000 | 500 to 800 | 3 | 2 | 15 in 100 | 900 |
| Metin Walki | 11,500 | 900 to 1,400 | 4 | 3 | 25 in 100 | 2,200 |
| Metin Bitwy | 17,000 | 1,500 to 2,200 | 5 | 4 | 35 in 100, and one at +3 5 in 100 | 3,300 |

- **The potions** are all red or all blue.
- **The materials** are all Szpon Wilka or all Ząb Dzika, the upgrade materials ([upgrading.md](upgrading.md)).
- **The gear** is one of each class's first weapons and armours.
- **A piece of the rare gear,** 4 times in 100 from every stone: one +0 kind of the gear past the first from level 21 ([equipment.md](equipment.md)).
- **A skill book,** 3 times in 100 from every stone, is the book of one skill of any class; Metin Bitwy also leaves a soul stone once in 100 ([skills.md](skills.md)).

## What the client does

- **Type 2 is a stone** (`TYPE_STONE`, `ActorInstance.h`). A CHARACTER_ADD of type 2 builds it from its own mob data, without waiting for an info, as for a monster. The data names it, and marks 8001 to 8003 as type 2 (`PythonNetworkStreamPhaseGameActor.cpp`).
- **A stone is fought like a monster.** A player may attack it and see its hit points (`InstanceBase.cpp`, `IsAttackableInstance`, `CanViewTargetHP`). A blow on it plays the stone's own hit (`ActorInstanceBattle.cpp`). Its death plays its breaking (`ActorInstance.cpp`).
- **Its smoke comes from its affect bits,** read from CHARACTER_ADD and CHARACTER_UPDATE (`InstanceBaseEffect.cpp`, `__SetStoneSmokeFlagContainer`):
  - bit 7 draws the thickest smoke;
  - bits 4 to 6, a middle one;
  - bits 1 to 3, the thinnest;
  - none, no smoke.

  The client's names for the bits mark them at 85, 80, 60, 45, 40, 20 and 10 in a hundred (`InstanceBase.h`).
- **An UPDATE with no parts** changes nothing else on a stone: its armour and weapon are 0 already, and hair is only for players (`InstanceBase.cpp`, `ChangeArmor`, `ChangeHair`).

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| CHARACTER_ADD `01` | 35 | as for a monster, type 2, the affects with the smoke bit |
| CHARACTER_UPDATE `13` | 35 | the stone's vid, no parts, its speeds, the affects with the smoke bit |
| CHAT `04` | by length | type 2 (notice) for the stone appearing, type 1 (info) for the winner |

Damage, target, death and drops use the same packets as for monsters ([combat.md](combat.md), [drops.md](drops.md)).

## What the server does

`internal/mob` holds the stones and their waves, `internal/world` their spots, and `internal/game/monsters.go` and `combat.go` how they stand, call and fall.

- **Kind `stone`:** fought, never wandering, and never answering a blow, as `engage` skips it. Its level, HT and DEF are the client's. Its hit points, experience, yang and drops are this server's.
- **Spots:** each stone has three spots of its own and picks one at random each time it appears. They were chosen by the same throwaway terrain script as the monster areas, run over the client's height and attribute maps ([monsters.md](monsters.md)). Each spot has:
  - no blocked cell, and no step steeper than 60 per 200, within 1,500;
  - 3,500 to any monster area, 8,000 to any NPC, and 9,000 to any other spot.
- **Appearing** (`place`):
  1. Monsters it called before and still alive are taken away. One a player has just killed is left to die as any monster does.
  2. The notice goes to every player on its map.
  3. Its first wave is called with no one to chase.
- **Waves** (`wear`, `callWave`):
  - A blow or skill that leaves it alive calls one wave for each of 80, 60, 40 and 20 in a hundred it has passed and not called before. So a heal never brings a wave back.
  - Each wave is the stone's list. Only as many of it come as keep the stone's living monsters at six, picked from the list at random when not all can.
  - They stand 500 to 800 from it, at random angles, and chase the player as though each had been hit by them.
  - They are ordinary monsters with their own drops. Once one dies or is sent away, the loop forgets it.
- **Healing:** 1 in a hundred every five seconds, from when it was first hurt. A turn that falls within ten seconds of its last hit is skipped, so the first heal comes ten to fifteen seconds after it.
- **Smoke:** recomputed on each hit and each heal. Those who see it are sent an UPDATE when it changes. Every later ADD carries it:
  - bit 1 from 85 in a hundred down, as the target window rounds it;
  - bit 4 from 45;
  - bit 7 from 10.
- **Destroyed:**
  - The drop owner is told, the drops are rolled and laid, and experience is shared, as for any monster ([drops.md](drops.md), [experience.md](experience.md)).
  - The potions, the materials and the gear are each one kind, picked evenly from its list.
  - It comes back after ten minutes, where a field monster takes 30 to 60 seconds.

## How we know

- **The client:** its source, as named above, and its mob data:
  - `--dump_proto--/mob_proto.txt` for each stone's rank (KING), type, level, HT, DEF and speeds;
  - the Polish locale's `mob_proto` for the names, read with MT2-Godot's decoder.
- **The design:** a debate between a Metin2 veteran and a game designer, with the veteran deciding where they differed, as Dawid asked.
  - **Both agreed on:**
    - the three stones and leaving 8004 for the second maps;
    - random spots;
    - hit points below the client's, which assume parties;
    - healing only once unhit;
    - a wave at each fifth;
    - drops to whoever did the most damage.
  - **The veteran's call:** the counts, the hit points, the drops and the ten minutes.
  - **The designer's warning:** guaranteed materials may come too fast for the upgrade table. They would have given one at 30 to 60 in 100. That is the first number to change if materials pile up.
- **Seen from a bot,** as Szepta (level 8) against the running server, at a Metin Cierpienia in Jinno:
  - it was shown as type 2 at one of its spots;
  - 148 blows of 41 to 53 brought it down, while she killed the five wolves about it and each wave as it came;
  - its smoke went to bit 1 at 85 in a hundred, bit 4 at 45 and bit 7 at 10;
  - destroyed, it told her «Zniszczyłeś kamień Metin.», gave her 864 experience (its 900 less her level's gap), and left her 3 blue potions, 2 claws and 562 yang.
- **Not yet seen in the real client:** the smoke, the breaking and the notice.

## What proves it

- `internal/mob/mob_test.go`:
  - each stone's waves are monsters no stronger than it;
  - only stones call waves and come back late;
  - each stone's drops are as in the table above, with a skill book 3 times in 100 and a piece of the rare gear 4 times in 100;
  - the +3 gear is the first gear's +3.
- `internal/world/spawns_test.go`: three, two and two stones per village, each with three spots; each spot in its band, on its map, and clear of everything else.
- `internal/game/monsters_test.go`: a stone's waves and their strolls stay on its map.
- `internal/game/stones_test.go`:
  - **Appearing:**
    - a stone shown as type 2 at its spot, with its first wave 500 to 800 about it;
    - back after ten minutes, at its spot, announced to a player out of view on the map, with its old wave sent away and a new one called;
    - a monster sent away then never strolling, and one just killed then left to die.
  - **Fighting:**
    - it never fights back;
    - a fifth lost calls a whole wave chasing the player;
    - at most six alive, a part wave picking its kinds at random;
    - each wave called once, even after a heal;
    - no healing until ten seconds unhit, then 1 in a hundred;
    - smoke shown to a viewer and to a newcomer.
  - **Destroyed:**
    - the winner told and given 3 potions of one kind, 2 materials of one kind and 500 to 800 yang;
    - a called monster never coming back.
- `internal/server/monsters_test.go` (real sockets): a player entering beside a stone sees it as type 2 with five of its wave.

## Not done yet

- Metin Chciwości (8004) and the stones of the second maps.
- Party loot, the poison and slowing of later stones, and the original's other effects on a wave.
