# Skills: groups, teachers, points and using them

## What the player sees

A character's class has two skill groups, its paths. The sura's are **Weaponry** («Moc Broni») and **Black Magic** («Czarna Magia»), and the warrior's **Body** («Moc Ciała») and **Mental** («Moc Duchowa»). Each village has a teacher for each: «Trener Mocy Broni» and «Trener Mocy Czarn. Magii» a few steps east of the old lady, and «Trener Mocy Ciała» and «Trener Mocy Duchowej» a few steps west.

**Choosing a path.**
- **The offer.** From level 5, a character who clicks a teacher of their class is offered their path, for a sura at Weaponry's «Mogę nauczyć cię drogi Mocy Broni. Czy chcesz nią podążać?», with Tak and Nie. Yes asks whether they are sure, and a second yes takes it up.
- **The skill window** then shows the group's six skills, and the character has one skill point for every level from 5.
- **Changing path.** Below level 20, the other teacher offers a change of path. The character forgets what it learnt and gets every point back. From level 20 the path is kept.
- **Who is turned away.** A teacher tells anyone of another class, anyone below level 5, and anyone already on their path, why not.

**Raising a skill.** The + beside a skill raises it a level for a point, up to 20, where the + goes. Each level from 5 brings one more point.
- **What can be raised:** Weaponry's Finger Strike, Dragon Swirl, Enchanted Blade and Enchanted Armour, Body's Three-Way Cut (Trzystronne Cięcie), Sword Spin (Wir Miecza), Berserk and Sword Aura (Aura Miecza), and Mental's Spirit Strike (Duchowe Uderzenie), Bash (Walnięcie), Stump (Tąpnięcie), Strong Body (Silne Ciało) and Sword Strike (Uderzenie Miecza). The others show in the window, but raising one says «Tej umiejętności nie można jeszcze rozwijać.»
**Using a skill.** They are used from the skill window or the taskbar, as the client uses any skill.
- **Finger Strike:** on the monster she has picked, within a blow's reach, one strike of two and a half blows and more at the first level, four and a quarter at 20. It can't be used again for 10 seconds.
- **Dragon Swirl:** the sura spins her blade, and every monster around her within 500 is struck at once. Each blow is more than two of hers at the first level, three and a half at 20. Every monster struck turns on her. It costs mana and can't be used again for 15 seconds.
- **Enchanted Blade:** with a sword in hand, the blade glows, for her and everyone near, and her blows and swirls hit harder for three minutes, by what its tooltip says. Using it again turns it off for nothing. Dying ends it, and so do taking the sword out of hand and leaving the world.
- **Enchanted Armour:** a shimmer for her and everyone near, and her defence higher for 36 seconds at the first level, 90 at 20. It waits longer to be cast again than it lasts, so it drops for a few seconds between casts. It runs alongside the blade.
- **Three-Way Cut:** with a sword or a glaive in hand, three cuts on the monster he has picked and on up to four more within 200 of it, each a little more than a blow. 12 seconds before it can be used again.
- **Sword Spin:** with a sword or a glaive, every monster within 200 of him, twelve at most, struck once for three blows and more. 15 seconds before it can be used again.
- **Berserk:** his blows and his steps faster, for him and as everyone near sees him, for 64 seconds at the first level, 105 at 20. It waits a little longer to be cast again than it lasts.
- **Sword Aura:** with a sword or a glaive, his blows harder for 32 seconds at the first level, 55 at 20, by (100 + STR)·k, as its tooltip says. It ends when the blade leaves his hand, and runs alongside Berserk.
- **Spirit Strike:** with a sword or a glaive, a heavy blow on the monster picked and up to three beside it, two and a third blows and more. 15 seconds.
- **Bash:** with a sword or a glaive, a slam on up to eight in front of him within 200; nothing behind him is touched. 15 seconds.
- **Stump:** a stamp that strikes up to ten within 400 around him, and may stun each it leaves standing: a stunned monster stands still, sleeping on its feet, and bites nothing for two seconds. 25 seconds.
- **Strong Body:** his defence higher and his steps slower for 64 seconds at the first level, 105 at 20, its effect shown on him.
- **Sword Strike:** with a sword or a glaive, a blow from afar on the monster picked within 1200, or the nearest in front of him, and up to nine around it, each perhaps stunned. With nothing to land on, it is not used. 20 seconds.
- **Berserk's price:** while it runs, every bite takes (5 + 10k)% more of him, 5% at the first level, 10% at 20.

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
- **The glow:** an effect bit in CHARACTER_ADD and CHARACTER_UPDATE draws a buff's effect on the character, Enchanted Blade's glow or Enchanted Armour's shimmer. For the player's own character it also lights the skill's slot as on. Bit 19 is the blade's, 21 the armour's, 14 Berserk's, 15 Sword Aura's and 16 Strong Body's (`InstanceBase.h`).
- **A stun** is bit 5 of a monster's effects, AFFECT_STUN, which the client draws as the monster asleep on its feet (`InstanceBaseEffect.cpp`).
- **The warrior's skills** need a target for Three-Way Cut, and a sword or two-handed weapon for all but Berserk, the client's NEED_TARGET and WEAPON_LIMITATION (`skilldesc.txt`). The skill window lists each group's skills from its first place, so Three-Way Cut is a Body warrior's first as Finger Strike is a Weaponry sura's (`playersettingmodule.py`).

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
| CHARACTER_ADD `01`, CHARACTER_UPDATE `13` | 35 each | the effects, two u32 words: bit 19 of the first is Enchanted Blade, 21 Enchanted Armour, 14 Berserk, 15 Sword Aura, 16 Strong Body, and on a monster 5 its stun; and the move and attack speeds |
| POINT_CHANGE `11` | 17 | the attack speed is point 17, the move speed 19 |

## What the server does

`internal/skill` says what each skill is; `internal/game/skills.go` runs the teachers and the + button; `internal/store/skills.go` keeps it.

- **The skills the game has** are a table, each by name, with its class and group: `finger_strike`, `dragon_swirl`, `enchanted_blade` and `enchanted_armour`, the sura's Weaponry, and `three_way_cut`, `sword_spin`, `berserk` and `sword_aura`, the warrior's Body. The client's numbers for them, 61, 62, 63 and 65, and 1, 2, 3 and 4, and their places in the skill window, by group, are in `internal/game/wire.go`.
- **The teachers** are NPC kinds that teach a group (`mob.Proto.Teaches`): 20304, 20324 and 20344 teach Weaponry, 20305, 20325 and 20345 Black Magic, 20300, 20320 and 20340 Body, and 20301, 20321 and 20341 Mental, one of each per village (`internal/world/spawns.go`).
  - Their spots are this server's own, in a row through the old lady, the warrior's west of her. open-mt2's come from the original server's data, which this repository keeps out ([monsters.md](monsters.md)). Unlike the monster areas, they have not been checked against the client's terrain; the Jinno pair was seen standing by a bot.
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
- **On the taskbar,** a skill learnt is kept by name, and its key sends USE_SKILL like the skill window ([taskbar.md](taskbar.md)).
- **Using a skill,** on USE_SKILL:
  - **Checks:** the skill's cooldown is the character's, kept by the server rather than the connection, so changing character or logging in again does not end it. The rest is checked under the world's lock: the player alive, the skill learnt, the sword Enchanted Blade needs in hand, the monster Finger Strike strikes, and its mana. The cooldown is only noted once the skill is used. A refusal is dropped without a word, since the client checked the same first.
  - **The cooldown the client counts:** it works a skill's cooldown out at the character's casting speed, point 21, and doubles it at 0, so POINTS sends 100, at which it waits what its table says, as this server does.
  - **Mana** is spent and told. Costs follow the client's own formulas at the skill's power `k`, its table by level (0.05 at 1, 0.40 at 19, 0.50 at 20), so its check and ours agree:
    - Finger Strike: 30 + 140k, and 10 s to cool down.
    - Dragon Swirl: 50 + 150k, and 15 s.
    - Enchanted Blade: 20 + 240k, a toggle with none.
    - Enchanted Armour: 70 + 170k, and 33 + 140k s.
    - Three-Way Cut: 40 + 100k, and 12 s.
    - Sword Spin: 50 + 130k, and 15 s.
    - Berserk: 50 + 140k, and 63 + 90k s.
    - Sword Aura: 100 + 200k, and 33 + 50k s.
  - **Finger Strike:** the target the client names in USE_SKILL must be a living monster the player sees within 300, a blow's reach; otherwise nothing is spent. It is struck once for (250 + 350k)% of a normal blow, the veteran's number, through the same `strike`. The skilltable's four is victims per motion, so it is not four strikes; it does not pierce yet.
  - **Dragon Swirl:** every living monster the player sees within 500 is struck, with no cap. Each blow is the player's normal blow, then (200 + 300k)% of it. A struck monster turns on them, counts the damage towards its drops and experience, and dies for everyone if it is killed, as a blow does: both go through one `strike`.
    - The damage lands as the skill is used; the client's motion swings a little later.
    - The client's own hit reports are dropped.
  - **Enchanted Blade:** it runs for three minutes, a choice of this server's (the client's own lasts 50 + 100k seconds), and adds 7 + (5·INT + 13)·k to every blow and swirl, rounded down: the client's tooltip, so what it shows is what is given. It needs a one-handed sword in hand, as the client's own check does, and taking the sword off ends it.
    - **Showing it:** its effect bit is worked out from the buffs a character has running whenever it is shown, so the record's entries keep their size. It is shown in CHARACTER_UPDATE to the player and everyone who sees them as it starts and ends, in every later CHARACTER_ADD, and in the update a change of armour sends.
    - **Its end** is handed to the world loop with its news, and queued in order of when it ends. The loop ends it then, if it is still the same run.
    - **Using it again** ends it, for nothing, as the client turns it off without checking mana. Death ends it. It is not stored, so leaving the world ends it too.
  - **Enchanted Armour:** the client's skilltable throughout. It runs 30 + 120k seconds and adds (INT + 30)·k to the armour set against every bite, rounded down. It is not a toggle: its cooldown always outlasts it, so using it again while it runs is refused. Its effect bit is 21, shown as the blade's is, and the player is told their defence, point 20, as it starts and ends, and when a point of intelligence or a level of the skill raises it.
  - **Three-Way Cut:** the target must be a living monster within a blow's reach, and a sword or two-handed weapon in hand. It and the living monsters the player sees within 200 of it, the nearest first, five in all at most as the client's skilltable has it, are each struck three times for (110 + 10k)% of a normal blow, the hits on one stopping when it dies. The percent follows the client's tooltip, 1.1 + 0.1k of the attack, where its skilltable has 1.1 + 0.5k; the STR term is left out, as the sura's skills leave theirs. The skilltable marks no splash for it, so the 200 around the target, from its splash range and five victims per motion, is this server's reading.
  - **Sword Spin:** as Dragon Swirl, but within 200 of the player, twelve at most, the nearest first, for (300 + 80k)% of a blow, with a sword or two-handed weapon. Twelve is the client's number of victims; Dragon Swirl, whose skilltable says the same, has no cap yet.
  - **Berserk:** the client's skilltable: it runs 60 + 90k seconds and adds 50k to the attack speed and 20k to the move speed, rounded down. As it starts and ends, the speeds go into the player's record, so the update everyone is sent and every later CHARACTER_ADD carry them, and the player is told both, points 17 and 19. The time a player's blows must leave between them on one victim follows the speed. The damage it costs is under Mental's notes below.
  - **Sword Aura:** the client's skilltable for its mana, cooldown and run of 30 + 50k seconds; it adds (100 + STR)·k to every blow and skill strike, its tooltip, where the skilltable adds 3 for each level of the character too. A change of weapon that leaves no sword or two-handed weapon in hand ends it, as for the blade.
  - **Mental,** settled by the veteran and the designer, the veteran deciding where they differed: the client's skilltable for mana, cooldowns, victims and splash, the tooltips' attack terms for the damage, as for Body.
    - Spirit Strike: the target and up to three within 100 of it, (230 + 400k)%.
    - Bash: up to eight within 200 in front of the player, by the way they last faced, (230 + 300k)%: half of the world, the side their facing points to.
    - Stump: up to ten within 400 around the player, (200 + 200k)%, stunning.
    - Strong Body: its tooltip's 7 + (40 + 0.2·STR + 0.4·CON)·k to the defence, where the skilltable's (200 + ...)·k would leave the first maps' monsters nothing to bite, and its skilltable's 1 + 9k off the move speed, for 60 + 90k seconds.
    - Sword Strike: the monster picked if it lives within 1200, else the nearest living one in front of the player within 1200; it and up to nine within 200 of it, (200 + 100k)%, stunning. With neither, it is not used and costs nothing. Its knockback is left out, as it would move monsters the server owns.
  - **Stuns:** each monster a stunning skill leaves alive is stunned at the client's chance, (100 + 1000k/6)/10 percent, rolled for each. A stun holds it two seconds from then, its fight's next step falling due as it ends: a second one resets the time, never adding to it, and there is no time after one in which it cannot be stunned again. Metin stones are never stunned. A stunned monster in a fight takes its steps but neither walks nor bites; hits do not wake it. Its stun is shown to those who see it, in CHARACTER_UPDATE as it starts and ends and in every later CHARACTER_ADD; its fight ending wakes it, so it goes home awake, and its death too, so no body lies stunned.
  - **Berserk's price:** the veteran's (5 + 10k)% more of every bite while it runs, which the client's description names but gives no number for.
  - **Bonuses come from the buffs running:** each buff raises or lowers the attack, the defence, the speeds or the damage bites take, by its own formula of the character's STR, CON or INT, and what the running ones add to the attack and defence is worked out whenever a blow or bite needs it, so they stack. The speeds are kept on the player's record, and worked out again whenever a buff starts or ends, a running one is raised, or the player dresses. Taking up another group ends every buff running. A buff runs until the world loop ends it.
- **Held under the world's lock:** a character's group and skills are on its record, which the world's lock guards, since its level changes there. They are read with the character on entering, and changed only through the world record.

## How we know

- **The client's side:** its source and scripts named above.
- **The teachers' names:** the Polish client's `mob_proto`.
- **Seen from a bot** against the running server: Szepta, a level-6 sura, clicked the Jinno Weaponry teacher, said yes twice, and was told group 1, the levels and 2 points, in that order; she raised Dragon Swirl and Enchanted Blade to 1 each, was told the group's other skills cannot be raised yet, and entered again on Weaponry with no points left. Later, with Dragon Swirl and Enchanted Blade at 1, she swirled the dog beside her: 57 mana spent, the dog struck for 96 where her blows take about 40, and it turned on her and bit. A second swirl within its 15 seconds was refused. The blade cost 32 mana and put bit 19 (0x80000) in her CHARACTER_UPDATE; using it again cleared it for nothing. Later Wojak, a level-10 warrior, took up Body at the Jinno teacher, told group 1 and 6 points, and raised the four skills to 1 each. With a sword in hand by the wild dogs: Sword Aura cost 110 mana and showed bit 15; Berserk cost 57 and showed bit 14, with his attack speed told as 152 and move speed 151, up from 150; Sword Spin cost 56 and killed a dog for its 126 hit points; Three-Way Cut cost 45 and cut its target for 107, then 19 as it died. The dialog window, the skill window and its + buttons have not been seen in the real client.
- **The rules:**
  - The rules are this server's own, settled in a debate between a Metin2 veteran and a genre designer:
    - points from level 5;
    - a free change of path below 20;
    - no skill books, and no rank past 20 yet.
  - open-mt2 gives points the same way, and the original sells the change of path.

## What proves it

- `internal/skill/skill_test.go`: every skill of a group of its class; points earned from level 5, less those spent; the client's power table; mana, cooldowns and runs equal to the client's formulas at every level, the warrior's included; the armour ending before it can be cast again; the blade's bonus as its tooltip shows it, and the armour's as its skilltable; Sword Aura's and Berserk's as the client's; which weapons each skill takes.
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
  - Finger Strike striking the dog she picked, and not the one beside it, for its share;
  - Finger Strike not used on nothing, on a dog too far, on a dead one or on a player;
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
  - Enchanted Armour: its effect, its mana and the defence told, a bite met with more armour, and refused while it runs; ended by the world loop at 36 seconds with the defence told again; the defence told when intelligence raises it.
  - The blade and the armour running together, each bit shown and each bonus added.
- `internal/game/mental_test.go`:
  - Spirit Strike on its target and three beside it;
  - Bash on eight in front of her and none behind;
  - a stunned dog neither walking nor biting for two seconds, its stun shown and ended on time; a second stun resetting the time; a stunned dog going home awake, and one killed lying unstunned;
  - Stump stunning some of ten at level 20, and never a stone in fifty;
  - Strong Body's defence, slowdown and effect;
  - Sword Strike landing on the dog picked 1100 off, else the nearest in front, and not used facing nothing;
  - Berserk making a bite 5% stronger;
  - strength raised under Strong Body telling her defence.
- `internal/game/warrior_test.go`:
  - Three-Way Cut cutting five around the dog picked, three times each, and its mana told; refused with nothing picked or a fan in hand; used with a glaive;
  - Sword Spin striking the three dogs within 200 and not the one beyond, and twelve of fourteen, the nearest;
  - Berserk telling both speeds and its effect, timing her blows faster, and ending with both back; raised as it runs, faster at once; ended by taking up another group;
  - Sword Aura raising the attack by 6, and ending when the sword is taken off;
  - each group's own skill window places;
  - a warrior taking up Body at its teacher.
- `internal/world/spawns_test.go`, `internal/mob/mob_test.go`: the four teachers in each first village, each teaching a group there is.
- `internal/server/skills_test.go` (real sockets and database): a level-6 sura taking up Weaponry, raising Dragon Swirl twice, and entering again with it at 2 and no points left; an answer still in flight after Change character ending nothing; an offer left open over Change character forgotten; a Weaponry sura's Dragon Swirl spending its mana and striking both dogs beside her.

## Not done yet

- Fear and Dispel, which only matter against players; Enchanted Armour's reflect; Finger Strike's pierce. Black Magic, the ninja's and shaman's groups and their teachers.
- Body's Charge and Sword Strike's knockback, which need the server to move characters; slows and poisons.
- Damage at the motion's hit, and no blows during a skill's motion.
- The mana Enchanted Blade drains while it runs in the original, and keeping it over a relog.
- Ranks past 20: Grand and Perfect.
- Support and horse skills.
- A reset item.
