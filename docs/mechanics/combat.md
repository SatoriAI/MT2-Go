# Combat: hitting a monster until it dies

## What the player sees

Clicking a monster picks it: its name, level and a hit-point bar appear at the top of the screen. Attacking it swings at it, and each blow that lands shows its damage over the monster and lowers the bar. At zero the monster plays its death, lies a moment, and fades away; half a minute to a minute later, by how far out it stood, another of its kind stands somewhere in its pack's place ([monsters.md](monsters.md)). NPCs cannot be fought. A monster that is hit fights back: it turns on its attacker, follows them, and bites every two seconds or so, and the player's hit-point bar drops with each bite. Run far enough, or keep out of reach for ten seconds, and it gives up and walks home. A player whose hit points run out dies ([dying.md](dying.md)). Killing gives experience ([experience.md](experience.md)).

A level-1 sura without a weapon takes about 10 to 12 off a wild dog a blow, so a dog takes about twelve; with 10 more points of strength, about five. The damage is this server's own formula, not the original's: see below.

## What the client does

- **Picking:** choosing or clearing a target sends TARGET (`3D`) with the vid, 0 for none. The client draws the target window's bar only from the server's TARGET reply (`3F`), and never asks again: every change of hit points needs a new reply (`PythonNetworkStreamPhaseGame.cpp`, `RecvTargetPacket`; `uitarget.py`). It ignores a reply for a vid that is dead.
- **Swinging:** each swing of a combo sends a MOVE with func COMBO (3) and the motion in its arg, which the server already passes to the others, so they see the swing. The client works out for itself which characters the swing touched, plays their being-hit motion, and for each one sends ATTACK (`02`) once per hit (`ActorInstanceBattle.cpp`, `PythonPlayerEventHandler.cpp` `OnHit`). Before sending it, it makes the victim its target.
- **Damage:** DAMAGE_INFO (`87`) draws the number over the victim, but only over the player or the player's current target (`InstanceBaseEffect.cpp`, `ProcessDamage`); flags 1 normal, 2 poison, 4 dodge, 8 block, 16 penetrate, 32 critical.
- **Death:** DEAD (`0E`) plays the death motion once; the body stays until the character is deleted, which fades it out. A new character, the returning monster, is a fresh ADD with a new vid.
- **A monster's attack:** a MOVE (`03`) for the monster with func ATTACK (2) plays its normal attack where the move puts it, facing the move's rotation; the client never turns it towards anyone itself, and ignores the arg (`InstanceBase.cpp`, `RunNormalAttack`). Closer than 50 to the point it attacks at once; further, it walks there first.
- **A chase:** a visible monster walks a MOVE of func WAIT (0) at its own pace and stops at the point; the move's duration is ignored. A MOVE of func MOVE (1) would walk on past it, so it is not used.
- **Taking damage:** DAMAGE_INFO with the player's own vid draws the numbers over them; their hit-point bar follows POINT_CHANGE of hit points (5) for their own vid only (`PythonNetworkStreamPhaseGame.cpp`, `uitaskbar.py`). Nobody else sees another player's hit points.
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
  - **attack:** 2 × level + (4 × strength + 2 × the class's stat) ÷ 3: the client's attack value without its hit rate, and twice a roll of the attack of the weapon held ([equipment.md](equipment.md));
  - **defence:** the monster's level + HT + DEF from `mob_proto`, 10 for a wild dog;
  - **damage:** attack × 100 ÷ (100 + defence), a tenth more or less at random, rounded, never under 1. Defence takes a share of every blow, never all of it.

  The attacker gets the damage; everyone who has picked the monster gets its new hit points.
- **Bonuses:** what a player wears may raise their blows and skills, pierce or double a blow, and take off a bite ([bonuses.md](bonuses.md)).
- **Speed:** a blow sooner than 15000 ms divided by the player's attack speed (at least 80 ms; 100 ms at a new character's 150) after their last on the same victim is dropped without a word: open-mt2's rule, after the original's, against tools that send blows faster than the client swings. It is checked on both sides: the attacker's last victim, and the victim's last attacker, which catches a player switching between two victims to double up. A weapon in hand adds its own attack speed ([equipment.md](equipment.md)), so the Kosa Bojowa's 37 already reaches the 80 ms floor, and Berserk shortens its gap no further.
- **Death:** a blow that takes the last hit point sends the attacker its damage, everyone who picked the monster its 0%, and everyone who sees it its death; the picks end. The body cannot be hit, picked or moved.
- **A body** is shown lying, with the client's dead state in its ADD, to anyone who arrives while it lies there.
- **The world loop** owns every monster that can be killed, wandering or not, and takes the body away two seconds after the death, with DELETE to those who see it, as open-mt2 does, and after its spawn's time, 30, 45 or 60 seconds by its band, or twenty for a spawn that sets none, puts a new monster of the kind at a random point of its spawn's area, with a new vid and all its hit points.
- **Skills** (an ATTACK with a skill) are dropped: a skill's victims are this server's to pick when it is used, and a blow and a skill's strike go through the same `strike` ([skills.md](skills.md)). While Enchanted Blade runs, its bonus adds to every blow's attack. Blows and picks the client sent before it took in Change character are dropped, as moves and chat are.

### Monsters fighting back

`internal/game/aggro.go`, with the bite in `internal/combat`.

- **Provoked:** every blow that lands on a living monster is kept for the world loop, which takes them with the deaths, under one lock a tick, and has a monster that is not already fighting take that player as its target. The loop steps only fighting monsters, so idle ticks cost no more.
- **A step,** every half second, under the world record's lock:
  - **in reach,** the attack range from `mob_proto` (175 for the four field monsters) and 15% more, with its bite due: everyone who sees the monster gets its attack, facing the player, and the player gets the damage and their hit points left;
  - **out of reach:** it walks towards the player, to nine tenths of its range from them, at most 400 a second at move speed 100 (this server's estimate of the client's walk), with a MOVE of func WAIT.
- **Bites** come every two seconds at attack speed 100, 2,000 × 100 ÷ attack speed in ms and never under one second: two and a half for the boar. The first comes as soon as the player is in reach.
- **A bite** is twice the monster's level, twice its strength (`mob_proto` ST), and a roll of its damage dice (`DAMAGE_MIN` to `DAMAGE_MAX`), close to the original's monster attack, of which the player's defence, level + vitality + the defence of the armour worn ([equipment.md](equipment.md)), takes its share as for a blow: attack × 100 ÷ (100 + defence), rounded, never under 1. A wild dog bites a level-1 sura for 27 to 31 of her 950 hit points, the boar for 69 to 76.
- **Giving up:** the monster stops when it died, when its target has left the world or is dead, when it has neither bitten nor been hit by its target for ten seconds, or when it is 5,000 from where it last bit (or was first hit). It walks straight home and strolls again from there, once the client can have walked it home at 400 a second. The world record has it home at once, so until the client's walk ends, a blow on it lands only within reach of its home. Going home does not heal it. While it fights, blows from other players are not answered.
- **While fighting** a monster does not stroll. A monster killed mid-fight stops.
- **Hit points** are the character's, kept under the world record's lock as experience is; they are saved with the character's progress and it enters the world with them ([entering-the-world.md](entering-the-world.md)).

### Healing

`internal/game/regen.go`, run by the world loop.

- **Players** regain, every three seconds while alive and out of a fight, 15 hit points and 5% of their maximum (62 a round for a level-1 sura's 950), and 5% of their mana, up to their maximums, and are told each that changed. A player is in a fight until five seconds after their last blow that landed or bite they took, so their first round comes five to eight seconds after it; a restart ends the fight. Healing in a fight, as open-mt2 does, outpaced a wild dog's bites in the first live test, so nobody could die to one; Dawid chose to heal out of a fight only. The first round comes three seconds after the loop starts. The dead do not heal. A round holds the world lock over every player: 0.7 ms at 10,000.
- **Monsters** heal 7% of their hit points every six seconds (`mob_proto` `REGEN_PERCENT` and `REGEN_CYCLE`, the same for the four field kinds), at least 1, from their first blow on, fighting or not, as open-mt2's do, rather than only out of a fight, as this page once planned; everyone who picked one sees its bar fill. One that dies leaves the list; the monster that comes back in its place is whole. The loop keeps a list of the hurt and walks only those, so idle ticks cost no more; a monster leaves the list when whole or dead.

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
- **The body's two seconds:** open-mt2. **The returns:** this server's, the band times the veteran's and the designer's.
- **A monster's attack and chase as the client shows them:** `InstanceBase.cpp` (`PushTCPState`, the FUNC_ATTACK and FUNC_WAIT branches), `NetworkActorManager.cpp`. **The fight's numbers,** a hit monster targeting its attacker, its reach and 15%, its bite every 2,000 × 100 ÷ attack speed ms, its step each half second towards nine tenths of its range, and giving up after ten seconds or 5,000: open-mt2's `Behavior.ts` and `Monster.ts`, read for facts. **The bite:** the original's monster attack (open-mt2's `MobPoints.ts`, `MonsterBattle.ts`) of twice level and strength and the dice, without its hit rate, under this server's mitigation, as Dawid chose. **The walk of 400 a second:** this server's estimate.

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
  - the body taken away after two seconds and a new monster returning after twenty, with a new vid and full hit points, or after its spawn's own time;
  - walking out of sight, and picking another, ending a pick;
  - taking a boar called Dzik away keeping a player called dzik findable;
  - the gap between blows following attack speed, and switching between two victims not getting round it;
  - a body shown lying to someone arriving.
- `internal/combat/combat_test.go`, bites: a wild dog's on a level-1 sura, 27 to 31, every value seen; a boar's harder; defence taking its share.
- `internal/game/aggro_test.go`, with a fixed seed and the time given:
  - a hit monster biting back at once in reach, and the sura losing as much;
  - bites every two seconds, not between;
  - a monster out of reach walking to nine tenths of its range, then biting;
  - a chase step no longer than the monster walks;
  - giving up after ten seconds without a bite, or 5,000 from where it last bit, and walking home, strolling only once the client can have walked it there;
  - its target's blows keeping it fighting, others' not;
  - a monster killed after the loop last looked for deaths not biting from its body;
  - giving up on a target who left;
  - a monster killed mid-fight, or fighting, not strolling;
  - a bite that kills, a dead player who can neither hit nor pick, and restarts (on [dying.md](dying.md)).
- `internal/game/regen_test.go`: a player regaining 62 hit points and 42 mana three seconds on, not before; not in a fight, only five seconds after it; her blow and the dog's bite each marking the fight; healing stopping at the maximum, telling nothing once whole; the dead not healing; a hurt dog healing 8 a round until whole, its picker seeing the bar fill, and leaving the list; a dead or whole monster not healed; a blow putting a monster on the list once.
- `internal/server/combat_test.go`: a bare-handed sura picking and hitting a dog for 10 to 12 over real sockets, its bar dropping; a blow too soon dropped and one after the gap landing; blows and picks in flight after Change character dropped; the dog she hits biting back with its attack, 27 to 31, and her hit points.

## Not done yet

- Aggressive monsters and packs fighting together are on [monsters.md](monsters.md).
- Penetrating blows, and every bonus that comes with items. Shooting with a bow is on [bow.md](bow.md); skills, their stuns, poisons, slows and burns, magic damage, critical blows, and the bites Berserk makes harder are on [skills.md](skills.md).
- Players fighting players.
