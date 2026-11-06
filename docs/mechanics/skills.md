# Skills: groups, teachers, points and using them

## What the player sees

A character's class has two skill groups, its paths. The sura's are **Weaponry** («Moc Broni») and **Black Magic** («Czarna Magia»). Each village has a teacher for each: «Trener Mocy Broni» and «Trener Mocy Czarn. Magii», a few steps east of the old lady.

**Choosing a path.**
- **The offer.** From level 5, a sura who clicks a teacher is offered their path: «Mogę nauczyć cię drogi Mocy Broni. Czy chcesz nią podążać?», with Tak and Nie. Yes asks whether they are sure, and a second yes takes it up.
- **The skill window** then shows the group's six skills, and the character has one skill point for every level from 5.
- **Changing path.** Below level 20, the other teacher offers a change of path. The character forgets what it learnt and gets every point back. From level 20 the path is kept.
- **Who is turned away.** A teacher tells anyone of another class, anyone below level 5, and anyone already on their path, why not.

**Raising a skill.** The + beside a skill raises it a level for a point, up to 20, where the + goes. Each level from 5 brings one more point.
- **What can be raised:** only Dragon Swirl and Enchanted Blade so far. The group's four other skills show in the window, but raising one says «Tej umiejętności nie można jeszcze rozwijać.»
**Using a skill.** Dragon Swirl and Enchanted Blade are used from the skill window, as the client uses any skill.
- **Dragon Swirl:** the sura spins her blade, and every monster around her within 500 is struck at once. Each blow is more than two of hers at the first level, three and a half at 20. Every monster struck turns on her. It costs mana and can't be used again for 15 seconds.
- **Enchanted Blade:** with a sword in hand, the blade glows, for her and everyone near, and her blows and swirls hit harder for three minutes, by what its tooltip says. Using it again turns it off for nothing. Dying ends it, and so do taking the sword out of hand and leaving the world.

## What the client does

- **The group:**
  - The client learns the group as MAIN_CHARACTER's skill group on entering, and as CHANGE_SKILL_GROUP afterwards (`PythonNetworkStreamPhaseGame.cpp`). It has no packet to choose one itself.
  - On a new group it forgets every skill's level and registers the group's six skills in the window: the sura's 61–66, or 76–81 for Black Magic (`playersettingmodule.py`). So the levels must follow the group.
  - With no group, the window offers both paths' names, and no + buttons.
- **Levels:**
  - SKILL_LEVEL_NEW gives every skill's level at the client's number for it.
  - 0 is unlearnt: the window shows it, but the skill can't be used.
  - The client works out a skill's mana, cooldown and power from its own skill table at that level.
- **The + button:**
  - It shows while POINT_SKILL (28) is above 0, a group is chosen, and the skill is below 20 (`uicharacter.py`). This server sends every skill's master type as 0, so a skill at 20 shows as 20; the client would show M1 only for master type 1.
  - It sends the chat command `/skillup <skill number>`.
- **A dialog:**
  - SCRIPT opens a window of its text, written in the client's markup: `[ENTER]` for a new line, `[QUESTION 1;Tak|2;Nie]` for buttons, `[DONE]` for an OK (`PythonEventManager.cpp`, `uiquest.py`).
  - A button sends SCRIPT_ANSWER with its index from 0, and the client closes the window itself. The escape key sends the last button, so Nie comes last. The dialog's own Continue and Cancel send 254. OK sends nothing.

- **Using a skill** (`PythonPlayerSkill.cpp`):
  - The client checks its own rules first: the skill learnt, the weapon, mana against its own cost, the cooldown.
  - It then plays the motion and sends MOVE with `FUNC_SKILL` and the motion, which others replay, and USE_SKILL with the skill's number.
  - During the motion it reports its own hits as ATTACK with the skill's number, which this server drops.
- **The glow:** an effect bit in CHARACTER_ADD and CHARACTER_UPDATE draws Enchanted Blade's glow on the character. For the player's own character it also lights the skill's slot as on. Bit 19 is the blade's (`InstanceBase.h`).

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| SCRIPT `2D` | 6 + text | size u16 (all of it), skin u8 (1, a window), the text's length u16 (ignored), then the text in cp1250 |
| SCRIPT_ANSWER `1D`, client | 2, then the sequence byte | the button's index u8 |
| CHANGE_SKILL_GROUP `70` | 2 | group u8: 1 or 2, 0 for none |
| SKILL_LEVEL_NEW `4C` | 1531 | 255 skills × 6: master type u8 (0), level u8, a reading time u32 (0), each at the client's number for the skill |
| POINT_CHANGE `11` | 17 | skill points are point 28 |
| USE_SKILL `34`, client | 9, then the sequence byte | the skill's number u32, its target's vid u32 (ignored; a toggle turned off may carry other numbers there) |
| POINTS `10` | 1021 | casting speed at point 21: 100 |
| CHARACTER_ADD `01`, CHARACTER_UPDATE `13` | 35 each | the effects, two u32 words: bit 19 of the first is Enchanted Blade |

## What the server does

`internal/skill` says what each skill is; `internal/game/skills.go` runs the teachers and the + button; `internal/store/skills.go` keeps it.

- **The skills the game has** are a table, each by name, with its class and group: `dragon_swirl` and `enchanted_blade`, the sura's Weaponry. The client's numbers for them, 62 and 63, are in `internal/game/wire.go`.
- **The teachers** are NPC kinds that teach a group (`mob.Proto.Teaches`): 20304, 20324 and 20344 teach Weaponry, and 20305, 20325 and 20345 Black Magic, one of each per village (`internal/world/spawns.go`).
  - Their spots are this server's own, beside the old lady. open-mt2's come from the original server's data, which this repository keeps out ([monsters.md](monsters.md)). Unlike the monster areas, they have not been checked against the client's terrain; the Jinno pair was seen standing by a bot.
  - A click reaches them from 1000, as it does a shopkeeper.
- **Talking:**
  - The connection keeps which teacher asked, and the next answer answers it. Yes to the offer asks whether the player is sure; yes to that takes up the group. Anything else, or an answer to nothing asked, ends the talk. Leaving the world forgets it, so another character of the account cannot answer it.
  - Each yes is taken only while the teacher is still in reach and the player still may, by the same rules as the click: an offer kept open while they levelled to 20 is told «Twoja droga jest już wybrana.», and one answered from afar is dropped.
  - Taking up a group is written first: the group is set and every skill forgotten, in one transaction. Then the client is told, in this order: the group, the levels (all 0), the points, and the teacher's last word.
  - A write that fails takes nothing up.
- **Points** are not stored. They are one for every level from 5, less a point for every skill level learnt, so nothing can drift. The skill window is shown them once a group is chosen. A level up then tells the player their new points.
- **Raising,** on `/skillup <number>`:
  - The skill is checked under the world's lock: of the player's class and group, below 20, and a point free.
  - It is then written one level higher, and the player is shown the levels and the points left.
  - A number the game has no skill for is told «Tej umiejętności nie można jeszcze rozwijać.»
- **Using a skill,** on USE_SKILL:
  - **Checks:** the skill's cooldown is the character's, kept by the server rather than the connection, so changing character or logging in again does not end it. The rest is checked under the world's lock: the player alive, the skill learnt, the sword Enchanted Blade needs in hand, and its mana. A refusal is dropped without a word, since the client checked the same first.
  - **The cooldown the client counts:** it works a skill's cooldown out at the character's casting speed, point 21, and doubles it at 0, so POINTS sends 100, at which it waits what its table says, as this server does.
  - **Mana** is spent and told. Costs follow the client's own formulas at the skill's power `k`, its table by level (0.05 at 1, 0.40 at 19, 0.50 at 20), so its check and ours agree:
    - Dragon Swirl: 50 + 150k.
    - Enchanted Blade: 20 + 240k.
  - **Dragon Swirl:** every living monster the player sees within 500 is struck, with no cap. Each blow is the player's normal blow, then (200 + 300k)% of it. A struck monster turns on them, counts the damage towards its drops and experience, and dies for everyone if it is killed, as a blow does: both go through one `strike`.
    - The damage lands as the skill is used; the client's motion swings a little later.
    - The client's own hit reports are dropped.
  - **Enchanted Blade:** it runs for three minutes, a choice of this server's (the client's own lasts 50 + 100k seconds), and adds 7 + (5·INT + 13)·k to every blow and swirl, rounded down: the client's tooltip, so what it shows is what is given. It needs a one-handed sword in hand, as the client's own check does, and taking the sword off ends it.
    - **Showing it:** its effect bit is worked out from the buffs a character has running whenever it is shown, so the record's entries keep their size. It is shown in CHARACTER_UPDATE to the player and everyone who sees them as it starts and ends, in every later CHARACTER_ADD, and in the update a change of armour sends.
    - **Its end** is handed to the world loop with its news, and queued in order of when it ends. The loop ends it then, if it is still the same run.
    - **Using it again** ends it, for nothing, as the client turns it off without checking mana. Death ends it. It is not stored, so leaving the world ends it too.
- **Held under the world's lock:** a character's group and skills are on its record, which the world's lock guards, since its level changes there. They are read with the character on entering, and changed only through the world record.

## How we know

- **The client's side:** its source and scripts named above.
- **The teachers' names:** the Polish client's `mob_proto`.
- **Seen from a bot** against the running server: Szepta, a level-6 sura, clicked the Jinno Weaponry teacher, said yes twice, and was told group 1, the levels and 2 points, in that order; she raised Dragon Swirl and Enchanted Blade to 1 each, was told the group's other skills cannot be raised yet, and entered again on Weaponry with no points left. Later, with Dragon Swirl and Enchanted Blade at 1, she swirled the dog beside her: 57 mana spent, the dog struck for 96 where her blows take about 40, and it turned on her and bit. A second swirl within its 15 seconds was refused. The blade cost 32 mana and put bit 19 (0x80000) in her CHARACTER_UPDATE; using it again cleared it for nothing. The dialog window, the skill window and its + buttons have not been seen in the real client.
- **The rules:**
  - The rules are this server's own, settled in a debate between a Metin2 veteran and a genre designer:
    - points from level 5;
    - a free change of path below 20;
    - no skill books, and no rank past 20 yet.
  - open-mt2 gives points the same way, and the original sells the change of path.

## What proves it

- `internal/skill/skill_test.go`: every skill of a group of its class; points earned from level 5, less those spent; the client's power table; mana costs equal to the client's formulas at every level; the blade's bonus as its tooltip shows it.
- `internal/combat/combat_test.go`: a buff's bonus adding to the attack.
- `internal/protocol/protocol_test.go`:
  - SKILL_LEVEL_NEW at a skill's number;
  - skill points at index 28;
  - SCRIPT byte for byte;
  - CHANGE_SKILL_GROUP;
  - SCRIPT_ANSWER framed and read;
  - USE_SKILL framed and read;
  - the effects in CHARACTER_ADD and CHARACTER_UPDATE.
- `internal/store/skills_test.go`: a skill raised and kept, and choosing a group forgetting the skills; the database refusing an unknown skill, a level past 20 and another class's group.
- `internal/game/skills_test.go`:
  - a sura taking up a teacher's group, in the client's order;
  - the offer declined, closed or answered stale; offered from level 5 exactly, and a change up to 19; kept open past 20 and refused then; answered out of reach and dropped;
  - a teacher telling the wrong class, the too young, one on their path, and one of the other path at 20, why not;
  - a change below 20 giving every point back;
  - a group not kept not taken up;
  - a skill raised;
  - one not raised with no point, at 20, of the other group, with no group, or not kept, and one the game has not got told so;
  - a level up telling the points of one with a group, and not of one without;
  - points shown once a group is chosen.
- `internal/game/skilluse_test.go`:
  - Dragon Swirl striking both dogs within 500 and not the one beyond, each turning on her, and its mana told;
  - a swirl's blow stronger than a normal blow;
  - a swirl killing what it strikes to death;
  - its 15 seconds waited out, and the dead not struck again;
  - a skill not used while cooling down, without its mana, unlearnt, by the dead, without the sword it needs, or by a number with no skill;
  - a cooldown outlasting the connection;
  - the casting speed the client is told.
  - Enchanted Blade:
    - glowing for her and a neighbour, and adding to her blows;
    - turned off free by using it again, with no mana left;
    - ended by taking the sword off;
    - ended by the world loop at three minutes, adding nothing at its end;
    - a later run outlasting the first's end;
    - ended by death;
    - its glow shown to a newcomer, and kept when she dresses.
- `internal/server/skills_test.go` (real sockets and database): a level-6 sura taking up Weaponry, raising Dragon Swirl twice, and entering again with it at 2 and no points left; an answer still in flight after Change character ending nothing; an offer left open over Change character forgotten; a Weaponry sura's Dragon Swirl spending its mana and striking both dogs beside her.

## Not done yet

- The sura's other skills, Black Magic, the other classes and their teachers.
- Damage at the motion's hit, and no blows during a skill's motion.
- The mana Enchanted Blade drains while it runs in the original, and keeping it over a relog.
- Skills on the taskbar.
- Ranks past 20: Grand and Perfect.
- Support and horse skills.
- A reset item.
