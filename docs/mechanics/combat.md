# Combat: hitting a monster until it dies

## What the player sees

Clicking a monster picks it: its name, level and a hit-point bar appear at the top of the screen. Attacking it swings at it, and each blow that lands shows its damage over the monster and lowers the bar. At zero the monster plays its death, lies a moment, and fades away; twenty seconds later another of its kind stands somewhere in its field. NPCs cannot be fought. Monsters do not fight back yet, and killing gives no experience yet.

A level-1 character without a weapon barely scratches a wild dog: its blows graze for 1 to 5, so a dog takes about forty. That is the formula, not a shortcut; weapons will change it once characters carry them.

## What the client does

- **Picking:** choosing or clearing a target sends TARGET (`3D`) with the vid, 0 for none. The client draws the target window's bar only from the server's TARGET reply (`3F`), and never asks again: every change of hit points needs a new reply (`PythonNetworkStreamPhaseGame.cpp`, `RecvTargetPacket`; `uitarget.py`). It ignores a reply for a vid that is dead.
- **Swinging:** each swing of a combo sends a MOVE with func COMBO (3) and the motion in its arg, which the server already passes to the others, so they see the swing. The client works out for itself which characters the swing touched, plays their being-hit motion, and for each one sends ATTACK (`02`) once per hit (`ActorInstanceBattle.cpp`, `PythonPlayerEventHandler.cpp` `OnHit`). Before sending it, it makes the victim its target.
- **Damage:** DAMAGE_INFO (`87`) draws the number over the victim, but only over the player or the player's current target (`InstanceBaseEffect.cpp`, `ProcessDamage`); flags 1 normal, 2 poison, 4 dodge, 8 block, 16 penetrate, 32 critical.
- **Death:** DEAD (`0E`) plays the death motion once; the body stays until the character is deleted, which fades it out. A new character, the returning monster, is a fresh ADD with a new vid.
- **Attack value:** the character window computes the player's attack itself from level, strength, dexterity and the class's stat (`PythonPlayer.cpp`, `__UpdateBattleStatus`): 2 × level + (4 × strength + 2 × the class's stat) / 3 × the hit rate, where the hit rate is (rating + 210) / 300 and the rating is min(90, (4 × dexterity + 2 × level) / 6). The class's stat is strength for warriors and suras, dexterity for ninjas, intelligence for shamans.

## The bytes

### ATTACK — client, `02`, 8 bytes, sequenced

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `02` |
| 1 | u8 skill | 0 for a plain or combo blow |
| 2 | u32 victim | vid |
| 6 | 2 bytes | a check of the client's files; not read |

### TARGET — client, `3D`, 5 bytes, sequenced: u32 vid, 0 for none

### TARGET — server, `3F`, 6 bytes

| Offset | Field |
|---|---|
| 0 | u8 header `3F` |
| 1 | u32 vid |
| 5 | u8 hit points left, as a percentage, rounded |

### DAMAGE_INFO — server, `87`, 10 bytes

| Offset | Field |
|---|---|
| 0 | u8 header `87` |
| 1 | u32 victim |
| 5 | u8 flags; 1, a plain blow, is the only one sent yet |
| 6 | i32 damage |

### DEAD — server, `0E`, 5 bytes: u32 vid

## What the server does

`internal/game/combat.go`, with the formulas in `internal/combat`.

- **Hit points** are the world record's, under its lock: each monster starts with its `mob_proto` hit points, and a blow can never take more than it has left.
- **Picking** a living monster the player sees answers with its hit points and records the pick; picking anything else, 0, a player or an NPC, ends the pick. A pick ends too when the player walks out of sight of it, picks another, or it dies. The server sends nothing when a pick ends this way, and the client keeps its target window open until the player closes it or picks again; open-mt2 does the same.
- **A blow** lands only on a living monster the player sees, within 300 of the player's last known position (open-mt2's reach; the edge counts). Its damage is the attack against the monster less the monster's defence:
  - **attack:** the client's attack value above, with the monster's evasion taken off the hit rate: (2 × its rating + 5) / (its rating + 95) × 0.3, rounded down;
  - **defence:** the monster's level + HT + DEF from `mob_proto`;
  - **a blow its defence stops** still grazes, for 1 to 5 at random.

  The attacker gets the damage; everyone who has picked the monster gets its new hit points.
- **Speed:** a blow sooner than 15000 ms divided by the player's attack speed (at least 80 ms; 100 ms at a new character's 150) after their last on the same victim is dropped without a word: open-mt2's rule, after the original's, against tools that send blows faster than the client swings. It is checked on both sides: the attacker's last victim, and the victim's last attacker, which catches a player switching between two victims to double up.
- **Death:** a blow that takes the last hit point sends the attacker its damage, everyone who picked the monster its 0%, and everyone who sees it its death; the picks end. The body cannot be hit, picked or moved.
- **A body** is shown lying, with the client's dead state in its ADD, to anyone who arrives while it lies there.
- **The world loop** owns every monster that can be killed, wandering or not, and takes the body away two seconds after the death, with DELETE to those who see it, as open-mt2 does, and twenty seconds after the death puts a new monster of the kind at a random point of its spawn's area, with a new vid and all its hit points: this server's pause, chosen for the village fields.
- **Skills** (an ATTACK with a skill) are dropped. Blows and picks the client sent before it took in Change character are dropped, as moves and chat are.

## How we know

- **The packets and what the client does with them:** the client's source, `Packet.h`, `PythonNetworkStreamPhaseGame.cpp`, `InstanceBaseEffect.cpp`, `ActorInstanceBattle.cpp`, `PythonPlayerEventHandler.cpp`, `PythonPlayer.cpp`, and `uitarget.py`.
- **The attack value:** the client's own, as above. **Evasion, a monster's defence, the graze, the reach and the speed rule:** open-mt2 (`PlayerBattleStrategy.ts`, `MobPoints.ts`, `PlayerBattleAgainstMobStrategy.ts`, `Player.ts`), read for facts. The monsters' values: the client's `mob_proto` dump.
- **The body's two seconds:** open-mt2. **The twenty-second return:** this server's.

## What proves it

- `internal/combat/combat_test.go`: a wild dog's defence; a level-1 sura's attack on it, worked through; each class's own stat; a blow above the defence taking the difference; a stopped blow grazing for each of 1 to 5; the rating's cap.
- `internal/protocol/protocol_test.go`: ATTACK and TARGET in; TARGET, DAMAGE_INFO and DEAD out, byte for byte.
- `internal/game/combat_test.go`:
  - picking a monster showing its hit points;
  - a blow taking its damage, the attacker told it and its bar;
  - a blow out of reach landing nowhere, and one at the edge landing;
  - an NPC neither fought nor picked;
  - a kill telling the attacker, the onlookers, and ending the picks;
  - a body neither hit, picked nor moved;
  - the body taken away after two seconds and a new monster returning after twenty, with a new vid and full hit points;
  - walking out of sight, and picking another, ending a pick;
  - taking a boar called Dzik away keeping a player called dzik findable;
  - the gap between blows following attack speed, and switching between two victims not getting round it;
  - a body shown lying to someone arriving.
- `internal/server/combat_test.go`: a bare-handed sura picking and grazing a dog over real sockets, its bar dropping; a blow too soon dropped and one after the gap landing; blows and picks in flight after Change character dropped.

## Not done yet

- Monsters fighting back, and aggressive monsters.
- Experience, levels and drops.
- Skills, bows, critical and penetrating blows, and every bonus that comes with items.
- Monsters regaining hit points out of a fight.
- Players fighting players.
- Who dealt how much, which experience will need.
