# Item bonuses

## What the player sees

Gear can carry bonuses, drawn in its tooltip below its stats, each in the client's own words:

| Bonus | Tooltip | What it does |
|---|---|---|
| Średnie Obrażenia | «Średnie Obrażenia: %d%%» | a blow's damage, by its percent; below zero, less |
| Obrażenie Umiejętności | «Obrażenie Umiejętności: %d%%» | a skill's damage, by its percent; below zero, less |
| Szansa na cios krytyczny | «Szansa na cios krytyczny +%d%%» | that chance of a blow striking twice as hard, on top of a buff's |
| Szansa na przeszywające Uderzenie | «Szansa na przeszywające Uderzenie: %d%%» | that chance of a blow going through the monster's defence |
| Silny przeciwko Zwierzętom | «Silny przeciwko Zwierzętom +%d%%» | damage on animals, every monster the world has so far, by its percent |
| Szybkość ataku | «Szybkość ataku +%d%%» | attack speed, beside the weapon's own |
| Maks. PŻ | «Maks. PŻ +%d» | that many more hit points at most |
| Odporność na Obrażenia | «Odporność na Obrażenia: %d%%» | a monster's bite, less by its percent |

- **Where bonuses come from:**
  - Gear a **Metin stone** leaves carries one bonus half the time, two 35 times in 100 and three 15, each of a different kind its place allows.
  - A **level-30 or level-36 weapon**, from anything that leaves it, carries **the pair** 15 times in 100: Średnie Obrażenia or Obrażenie Umiejętności at a strong value, and the other always taken off. It's a weapon for hitting or for skills, never both.
  - Nothing else carries bonuses, and nothing sold in a shop does.
- **Grades:** each bonus rolls one of five values, the top one rare (40, 30, 18, 9 and 3 times in 100). Gear below level 25 never rolls the top one.

| Place | Bonuses (values, grade 1 to 5) |
|---|---|
| Weapon | Silny przeciwko Zwierzętom 2/4/6/8/10 · krytyczny 1/2/3/5/8 · przeszywające 1/2/3/5/8 |
| Armour | Maks. PŻ 50/100/180/280/400 · Odporność 1/2/3/4/5 (two kinds, so at most two bonuses) |
| Helmet | Szybkość ataku 1/2/3/4/5 · Odporność · Silny przeciwko Zwierzętom |
| Shield | Maks. PŻ · Odporność · Silny przeciwko Zwierzętom |
| Shoes | Szybkość ataku · Maks. PŻ · krytyczny |
| The pair | Średnie 10/15/20/25/30 with Umiejętności −5/8/10/12/15, or Umiejętności 5/8/10/12/15 with Średnie −5/8/10/12/15 |

- **What counts:** only what is worn. Each kind is summed across everything worn, then capped:
  - Średnie and Umiejętności 30;
  - krytyczny and przeszywające 20;
  - Zwierzęta 25;
  - Odporność 15;
  - Szybkość ataku 20.

  Hit points have no cap.
- **Bonuses stay with their item:** through upgrades at the blacksmith (an upgrade keeps the item, and a failed one never destroys it), trades, being dropped and picked up again, and logging out.
- **/bonusy** lists what everything worn adds, named as the tooltip names each, or says nothing worn has any.
- **A remarkable roll:** once it lies on the ground, when a monster leaves gear with three bonuses besides a pair, or any bonus at its top grade, everyone on the map is told: «Hendra zdobywa Żelazny Hełm z niezwykłymi bonusami!».

## What the client does

- **The tooltip** draws each of an item's seven bonus slots that holds a type, with the line its type names and the value; a negative value is drawn as one (`uitooltip.py`, `locale_game.txt`'s TOOLTIP_APPLY_ and TOOLTIP_*_BONUS lines).
- **A critical or piercing blow:** DAMAGE's flags only colour the number. The flash on the attacker is a SPECIAL_EFFECT of its own, 5 for a critical blow and 6 for a piercing one, drawn on the vid it names, as a potion's is ([potions.md](potions.md)). MT2-Godot read the numbers from the client's effect table and draws both.

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| ITEM_SET `14` | 42 | after the count, 3 sockets of i32 (0), then 7 bonus slots of type u8 and value i16 (read signed); an empty slot is 0 and 0 |
| EXCHANGE `2A` | 47 | an item offered carries its 7 slots after its 3 sockets, as ITEM_SET does |
| DAMAGE `87` | 10 | the flags: 1 a plain blow, 16 a piercing one, 32 a critical one, 48 both |
| SPECIAL_EFFECT `72` | 6 | after a critical or piercing blow's DAMAGE, the effect u8 (5 critical, 6 piercing, both on a blow that is both) and the attacker's vid u32 |
| POINT_CHANGE `11` | 17 | max HP is point 6 and HP point 5, told when what is worn changes the maximum |

The client's numbers for the bonuses, its APPLY_ types (open-mt2's `ApplyTypeEnum`):

| Bonus | Type |
|---|---|
| Maks. PŻ | 1 |
| Szybkość ataku | 7 |
| krytyczny | 15 |
| przeszywające | 16 |
| Silny przeciwko Zwierzętom | 18 |
| Obrażenie Umiejętności | 71 |
| Średnie Obrażenia | 72 |
| Odporność na Obrażenia | 74 |

## What the server does

- **The model:**
  - `internal/account` holds the kinds, by name, and a character's `Totals`.
  - `internal/item` holds an item's `Bonuses`, the values, the places, and `Roll`.
  - `internal/game` gives them as gear falls (`rollBonuses`), carries them through pickups and drops, and counts them in a fight.
- **Storage:** `item_bonuses`, a row for each by its item and slot, its kind checked by name and its value never 0, goes with its item (migration 029). An item is read with its bonuses in slot order, and a new one is kept with them in one transaction.
- **Rolling:** where a monster's drops are made, each piece of gear rolls with the world loop's generator. Each roll is logged, with the item, the monster and the bonuses, so the odds can be weighed against what players found.
- **What is worn:**
  - Wearing gear, at entering and at every change, sums its bonuses into the character's totals and caps them.
  - A change of the maximum cuts the hit points to it, and the player is told both.
- **In a fight:**
  - **A blow, a swing or a plain arrow:**
    1. a piercing roll first, taking it at no defence;
    2. then the damage, raised by Średnie and, on an animal, by Zwierzęta;
    3. then a critical roll from buffs and bonuses alike, doubling it;
    4. each bonus a percent of the rolled damage, taken in one sum, never taking a blow that hurts to nothing;
    5. a critical or piercing blow drawn with its effect on the attacker, for them and everyone who sees them.
  - **A skill, physical or magic:** each victim's damage raised by Umiejętności and, on an animal, by Zwierzęta. It never pierces and is never critical.
  - **A bite:** after buffs, less by Odporność.
  - A roll is only made when its chance is above 0, so a player with no bonuses rolls exactly as before.
- **Race:** a monster's race is `mob.Proto.Animal`, from mob_proto's RACE_FLAG. Every monster the world has is ANIMAL, and stones are of none.

## How we know

- **The tooltips and their words:** the client's `locale_game.txt` and `uitooltip.py`.
- **The slots and the damage flags:** open-mt2's `ApplyTypeEnum` and `DamageFlagEnum`, read for facts.
- **The rules, kinds, values, odds and caps:** the veteran and the designer debated them, the veteran deciding where they differed, as Dawid asked. The two disagreed on:
  - which drops roll bonuses (any monster's or only a stone's): a stone's only, which the veteran accepted;
  - where the pair comes: only on level-30 and level-36 weapons, the veteran's;
  - resistance: the designer's, which the veteran accepted.
- **Every monster being an animal:** mob_proto's RACE_FLAG for each of the 19.

## What proves it

- `internal/item/bonus_test.go`:
  - a stone's helmet carrying one to three bonuses at their odds, each its place's and of its own kind;
  - armour carrying at most two;
  - the top grade only from level 25;
  - nothing but a stone giving ordinary bonuses, and nothing unworn any;
  - the pair 15 times in 100, always with its weak side, and on no other weapon;
  - what is remarkable;
  - totals summed then capped.
- `internal/store/items_test.go`: an item's bonuses kept in order and read back, through an upgrade and a trade, gone with the item, and an unknown kind refused with nothing kept.
- `internal/protocol/protocol_test.go`: ITEM_SET's and EXCHANGE's slots byte for byte, a negative value included.
- `internal/game/bonuses_test.go`:
  - average damage, strength against animals on an animal only, both, and a negative average, each exact against the same dice without;
  - a piercing blow through the defence with flag 16, a critical one doubled with flag 32, and both;
  - a critical and a piercing blow drawn with their effects on the attacker, for her and a player who sees her, and a plain one with neither;
  - a bonus never making a harmless blow hurt nor a hurting one harmless, average and animals in one sum;
  - an upgrade and a trade keeping an item's bonuses;
  - skill damage on a skill and average damage not;
  - resistance off a bite;
  - totals of what is worn, capped, with the bag not counted, and the hit points' maximum rising and falling, told;
  - gear picked up and dropped keeping its bonuses;
  - a stone's gear rolling and a monster's not, and a remarkable one told on the map after it lies;
  - /bonusy's lines.
- `internal/mob/mob_test.go`: every monster an animal, nothing else.
- `internal/server/items_test.go` (real sockets and database): armour with a bonus shown with it in its slot on entering, and worn telling her max HP.

## Not done yet

- **Zaczarowanie Przedmiotu and Wzmocnienie Przedmiotu,** which reroll an item's bonuses or add one, from the stones.
- **The other races' bonuses,** with monsters of those races.
- **The sixth and seventh slots;** jewellery; the bonuses that matter only against players.
