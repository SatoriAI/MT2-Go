# The bow: shooting a monster

## What the player sees

A ninja holding a bow, with arrows in the arrow place of the equipment window, shoots at the monster she clicks from up to about 2400 away. The client walks her closer first if the monster is further. Each arrow that lands shows its damage over the monster, like a blow, and her arrows count down by one. When the last one is gone, the place empties and the client says «Potrzebuję Strzały» instead of shooting. Others see the arrow fly. A bow never strikes in melee. The weapon trader sells a hundred arrows, under the dagger, for 20 yang.

## What the client does

- **The target:**
  - Picking a monster with a bow in hand and starting to attack sets the arrow's target.
  - As the shot's motion begins, the client sends FLY_TARGETING (`33`) with the target's vid and point (`PythonPlayerEventHandler.cpp`, `ActorInstanceMotion.cpp`).
  - Then it sends a MOVE with func COMBO for others to see the shot.
- **The shot:**
  - At the motion's FLY event, 0.85 s into its 1.5 s at attack speed 100 (`pc/assassin/bow/attack.msa`), the client lets the arrow go and sends SHOOT (`36`) with 0, or the skill's number for a skill's arrow (`ActorInstanceMotionEvent.cpp`).
  - It never sends ATTACK for an arrow: its hit callbacks only draw the hit (`PythonPlayerEventHandler.cpp`, `FlyingInstance.cpp`). So the server decides the hit.
- **A skill's further targets** go as ADD_FLY_TARGETING (`35`), in the same bytes (`PythonPlayerSkill.cpp`).
- **Arrows:**
  - The client refuses to shoot unless the arrow place, 99 on the wire, holds arrows.
  - It never counts them down itself (`PythonPlayerSkill.cpp`, `__HasEnoughArrow`), so the server does.
- **Reach:** the client shoots from 2500 less 100, plus POINT_BOW_DISTANCE (34), which this server never raises (`InstanceBase.cpp`, `InstanceBaseBattle.cpp`).
- **Others:**
  - Another client draws the arrow only from FLY_TARGETING (`47`) with the shooter's vid, sent before the shot's MOVE reaches it (`PythonNetworkStreamPhaseGame.cpp`).
  - With target 0, it flies to the point instead.
  - The server passes each of them on to everyone who sees the shooter.

## The bytes

### FLY_TARGETING — client, `33`, 13 bytes, sequenced; ADD_FLY_TARGETING `35`, the same

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `33` or `35` |
| 1 | u32 target | vid, 0 for a point on the ground |
| 5 | i32 x | the point, in the world's coordinates |
| 9 | i32 y | |

### SHOOT — client, `36`, 2 bytes, sequenced: u8 the skill's number, 0 for a plain shot

### FLY_TARGETING — server, `47`, 17 bytes; ADD_FLY_TARGETING `45`, the same

| Offset | Field |
|---|---|
| 0 | u8 header `47` or `45` |
| 1 | u32 shooter's vid |
| 5 | u32 target, 0 for a point |
| 9 | i32 x |
| 13 | i32 y |

## What the server does

- **Aiming** (`internal/game/bow.go`):
  - FLY_TARGETING keeps the target for the next shot and is passed on to everyone who sees the shooter, while the shooter lives and holds a bow.
  - ADD_FLY_TARGETING is passed on the same way.
- **A shot lands** as a blow, through the same damage, stray, defence and critical chance as a swing (`land`), if all of these hold:
  - a target was named since the last shot;
  - the shot is a plain one;
  - arrows are worn and a bow is in hand;
  - the shooter is alive;
  - the monster is alive, in sight and within 2800 (the bow's 2500 and 300 for late positions, the debaters' reach);
  - 1.25 s × 100 ÷ attack speed have passed since the last shot that landed (the bow's motion less a sixth, the debaters' limit).

  It uses one arrow up, kept one fewer and shown so, or forgotten with its last, which undresses the character of arrows. A shot that cannot land is dropped, and uses no arrow. A shot that landed whose arrow cannot be kept in the database still lands: the client is shown the arrows it had, and the error is logged.
- **Arrows in the attack:**
  - Drewniana Strzała (8000) adds its VALUE3, 3, to the bow's least attack and its VALUE4, 0, to its most, so Łuk+0's 7–29 becomes 10–29 (the veteran's reading).
  - Arrows add nothing to any other weapon.
- **No melee:** a swing reported with a bow in hand lands nothing; the client never sends one.
- **The price:** Drewniana Strzała is flagged COUNT_PER_1GOLD, and its item_proto price, 5, is how many a yang buys, so a hundred cost 20, never under 1 (`item.Proto.PriceOf`).
- **Skills that shoot** land each arrow the same way, from the volley their USE_SKILL leaves ([skills.md](skills.md)).
- **Not done yet:** arrows past the first.

## How we know

- **The packets, their order and the arrow place:** the client's source, `Packet.h`, `PythonPlayerEventHandler.cpp`, `ActorInstanceMotion.cpp`, `ActorInstanceMotionEvent.cpp`, `PythonPlayerSkill.cpp`, `PythonNetworkStreamPhaseGame.cpp`, `InstanceBase.cpp`, `GameType.h`, and the bow's `attack.msa`.
- **Drewniana Strzała's values, flags and price:** the client's `item_proto` dump; **its name:** the Polish locale's `item_proto`, read by MT2-Godot.
- **Its message:** `game.py` and `locale_game.txt`.
- **The reach margin, the shot limit, arrows used per shot and the shop's hundred:** the debaters', the veteran deciding ties (`mt2go-design-decisions`).

## What proves it

- `internal/protocol/protocol_test.go`: FLY_TARGETING, ADD_FLY_TARGETING and SHOOT framed and read; FLY_TARGETING out byte for byte.
- `internal/item/item_test.go`: arrows the ninja's, worn in their place, stacking to 200, adding 3; the bow and its grades shooting; a kind sold by the gold costing its count over five.
- `internal/game/bow_test.go`:
  - a shot landing and using an arrow;
  - a shot dropped, with no arrow used, when it is too far, has no target, comes too soon, or is shot without arrows, without a bow, by a skill, or by the dead;
  - a target shot once;
  - shots following the attack speed;
  - the last arrow emptying its place;
  - the aim shown to others byte for byte, but not by the dead or without a bow;
  - a bow not striking in melee;
  - arrows adding to the bow's attack alone;
  - arrows put on in their place;
  - the weapon trader's hundred arrows for 20.
- `internal/server/combat_test.go`: over real sockets and the database, a ninja with a bow and a hundred arrows shooting a dog 1500 away, the arrow landing, 99 arrows kept, and the connection framed on.
