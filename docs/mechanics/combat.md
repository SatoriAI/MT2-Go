# Combat: hitting a monster until it dies

## What the player sees

Clicking a monster picks it: its name, level and a hit-point bar appear at the top of the screen. Attacking it swings at it, and each blow that lands shows its damage over the monster and lowers the bar. At zero the monster plays its death, lies a moment, and fades away; twenty seconds later another of its kind stands somewhere in its field. NPCs cannot be fought. Monsters do not fight back yet, and killing gives no experience yet.

A level-1 sura without a weapon takes about 10 to 12 off a wild dog a blow, so a dog takes about twelve; with 10 more points of strength, about five. The damage is this server's own formula, not the original's: see below.

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
- **A blow** lands only on a living monster the player sees, within 300 of the player's last known position (open-mt2's reach; the edge counts). Its damage is this server's own formula (`internal/combat`):
  - **attack:** 2 × level + (4 × strength + 2 × the class's stat) ÷ 3: the client's attack value without its hit rate. A weapon will add twice its attack value;
  - **defence:** the monster's level + HT + DEF from `mob_proto`, 10 for a wild dog;
  - **damage:** attack × 100 ÷ (100 + defence), a tenth more or less at random, rounded, never under 1. Defence takes a share of every blow, never all of it.

  The attacker gets the damage; everyone who has picked the monster gets its new hit points.
- **Speed:** a blow sooner than 15000 ms divided by the player's attack speed (at least 80 ms; 100 ms at a new character's 150) after their last on the same victim is dropped without a word: open-mt2's rule, after the original's, against tools that send blows faster than the client swings. It is checked on both sides: the attacker's last victim, and the victim's last attacker, which catches a player switching between two victims to double up.
- **Death:** a blow that takes the last hit point sends the attacker its damage, everyone who picked the monster its 0%, and everyone who sees it its death; the picks end. The body cannot be hit, picked or moved.
- **A body** is shown lying, with the client's dead state in its ADD, to anyone who arrives while it lies there.
- **The world loop** owns every monster that can be killed, wandering or not, and takes the body away two seconds after the death, with DELETE to those who see it, as open-mt2 does, and twenty seconds after the death puts a new monster of the kind at a random point of its spawn's area, with a new vid and all its hit points: this server's pause, chosen for the village fields.
- **Skills** (an ATTACK with a skill) are dropped. Blows and picks the client sent before it took in Change character are dropped, as moves and chat are.

## Why not the original's formula

The original takes the defence off the attack and scales the stat part by a hit rate of about two thirds, less the victim's dodging: a level-1 sura's attack of 8 against a wild dog's defence of 10 leaves nothing, and every blow grazes for 1 to 5, about 42 blows a dog; ten points of strength jump that to 12. Subtraction also lets high defence wall off a weaker character entirely. This server takes a share instead, so every blow and every point of strength counts and armour matters at every level, and it drops the hit rate. Characters hit harder than in the original: at low levels the attack is about half again as high, and against a wild dog a blow takes about three times as much (2.6 times with strength 15), and far more against armoured monsters; monsters will use the same formula against players, and their health and the experience tables can be tuned once it has been played.

**The TMP4 client's character window** still shows its own attack value, computed by the original formula, so it differs from the damage dealt. Our own client can show this server's.

| Character | Formula | Attack | On a wild dog | Blows to kill |
|---|---|---|---|---|
| Level-1 sura | this server's | 12 | 10–12 | about 12 |
| Same, strength 15 | this server's | 32 | 26–32 | about 5 |
| Level-1 sura | the original | 8 | 1–5 | about 42 |
| Same, strength 15 | the original | 21 | 11 | 12 |

## How we know

- **The packets and what the client does with them:** the client's source, `Packet.h`, `PythonNetworkStreamPhaseGame.cpp`, `InstanceBaseEffect.cpp`, `ActorInstanceBattle.cpp`, `PythonPlayerEventHandler.cpp`, `PythonPlayer.cpp`, and `uitarget.py`.
- **The attack's stat part and a monster's defence:** the client's character window and open-mt2's `MobPoints.ts`. **The reach and the speed rule:** open-mt2 (`PlayerBattleAgainstMobStrategy.ts`, `Player.ts`), read for facts. The monsters' values: the client's `mob_proto` dump.
- **The body's two seconds:** open-mt2. **The twenty-second return:** this server's.

## What proves it

- `internal/combat/combat_test.go`: a wild dog's defence; a level-1 sura's attack; each class's own stat; defence taking a share; no defence stopping a blow whole; a blow straying a tenth either way across that range; a blow taking at least 1.
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
- `internal/server/combat_test.go`: a bare-handed sura picking and hitting a dog for 10 to 12 over real sockets, its bar dropping; a blow too soon dropped and one after the gap landing; blows and picks in flight after Change character dropped.

## Not done yet

- Monsters fighting back, and aggressive monsters.
- Experience, levels and drops.
- Skills, bows, critical and penetrating blows, and every bonus that comes with items.
- Monsters regaining hit points out of a fight.
- Players fighting players.
- Who dealt how much, which experience will need.
