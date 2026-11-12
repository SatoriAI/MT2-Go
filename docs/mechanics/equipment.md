# Equipment: a weapon, body armour, a helmet, shoes and a shield

## What the player sees

The inventory window's upper half is the character: the places worn items go. A double-click on a weapon, armour, a helmet, shoes or a shield in the bag puts it on, into its own place; so does dragging it onto its own empty place. The character then holds the weapon and wears the armour, for themselves and everyone near, and on the character screen too. A double-click on something worn takes it off into the first room in the bag, and dragging it to empty cells puts it there.

Putting on a second weapon swaps them: the one held goes where the new one was. An item for another class stays where it was, and the chat says «Ta postać nie może tego nosić.»; one above the character's level does too, with «Masz za niski poziom, aby to założyć.»

A weapon makes every blow stronger. Armour, a helmet, shoes and a shield each take more off every bite, and the character window's DEF rises by their defence; its attack follows the weapon. Shoes also make the character faster, more with each grade, for themselves and as everyone near sees them run. A shield makes them slower at every grade: the first 2, the Pięciokątna Tarcza 6. It is not drawn on the character, as in the original.

Only a weapon, body armour, a helmet, shoes, a shield and arrows can be worn so far, arrows a whole stack at a time, for the bow to shoot ([bow.md](bow.md)). The kinds are each class's first ones, the first shoes and shield for all, and a curated few beyond them up to level 36, with their grades ([upgrading.md](upgrading.md)):
- **Weapons:** each line at levels 10, 20, 25, 30 and 36, the bells at 32 in place of 25, which has none. Level 30 is the client's own tier of its best-known weapons, the Miecz Pełni Księżyca among them, added at Dawid's call as the ones players chase most.
- **Armour:** each class's at levels 9, 18 and 34.
- **Others:** the helmets and a shield of level 21, and shoes of levels 17 and 34.

The weapon and armour traders sell those below level 21 ([shops.md](shops.md)). The rest only drops, from the cursed monsters and the Metin stones ([drops.md](drops.md)).

## What the client does

- **Worn items are in the inventory window too:** cells 90 and up, by the client's wear places: the body at 90, the head at 91, the feet at 92, the weapon at 94, the two unique places at 97 and 98, the arrows at 99 and the shield at 100 (`GameType.h`, `ItemData.h`, WEAR_UNIQUE1 7, WEAR_UNIQUE2 8, WEAR_ARROW 9, WEAR_SHIELD 10), with the window number `01`. The inventory window has a slot for each (`inventorywindow.py`).
- **What it sends** (`uiinventory.py`):
  - **A double-click** sends ITEM_USE with the item's position, whether it's in the bag or worn.
  - **A drag onto an empty place** sends ITEM_MOVE to its place; onto an occupied place, ITEM_USE of the dragged item, leaving the place to the server.
  - **A drag from a worn place** to the bag sends ITEM_MOVE from it.
- **The client checks no class or level and moves nothing itself:** it waits for the server's ITEM_SETs. A refusal it is not told about shows nothing.
- **How a character is dressed:** by its parts, armour, weapon, head and hair, each an item's vnum.
  - The weapon's model comes from its item data.
  - The armour's vnum becomes a body shape through the item's VALUE3, and 0 or 1 is the class's own look (`InstanceBase.cpp`).
  - CHARACTER_INFO gives the parts when a character arrives, and CHARACTER_UPDATE re-dresses one already shown (`NetworkActorManager.cpp`).
  - For the player's own character, the update also sets the weapon behind the window's attack (`PythonNetworkStreamPhaseGameActor.cpp`).
- **The character screen** draws the body part of each slot through the same vnum-to-shape path (`introselect.py`).

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| ITEM_USE `0B`, client | 4 | position: window u8, cell u16 |
| ITEM_MOVE `0D`, client | 8 | from, to, count u8 |
| ITEM_SET `14` | 42 | as on [items.md](items.md); worn items at cells 90, 91, 92, 94, 97, 98, 99 and 100 |
| CHARACTER_UPDATE `13` | 35 | vid u32, parts u16 ×4 (armour, weapon, head, hair), move speed u8, attack speed u8, state u8, affects u32 ×2, guild u32, alignment u16, player-kill mode u8, mount u32 |
| CHARACTER_INFO `88` | 54 | as on [entering-the-world.md](entering-the-world.md), with the armour and weapon parts |
| POINT_CHANGE `11` | 17 | DEF is point 20, the move speed 19 |

## What the server does

`internal/item` says what is worn and by whom; `internal/game/items.go` puts it on and takes it off; `internal/combat` counts it.

- **What can be worn:** each kind says where it goes, which classes may wear it, and a weapon's attack or armour's defence. The values come from the client's `item_proto`: `ITEM_WEAR` (for the shield, whose `ITEM_WEAR` reads `WEAR_UNIQUE`, its SUB_TYPE `ARMOR_SHIELD`, which the client wears at WEAR_SHIELD), the classes its `ANTI_` flags leave, `VALUE3` to `VALUE4` for a weapon and `VALUE1` for anything else, and the speed shoes add or a shield takes off, its `APPLY_MOV_SPEED`.

  | Kind | Place | Classes | Attack | Defence |
  |---|---|---|---|---|
  | Miecz+0 (10) | weapon | warrior, ninja, sura | 13–15 | |
  | Sztylet+0 (1000) | weapon | ninja | 8–11 | |
  | Łuk+0 (2000) | weapon | ninja | 7–29 | |
  | Glewia+0 (3000) | weapon | warrior | 14–22 | |
  | Wachlarz+0 (7000) | weapon | shaman | 11–15 | |
  | Mnisia Zbr. Płyt.+0 (11200) | body | warrior | | 12 |
  | Błękitne Ubranie+0 (11400) | body | ninja | | 12 |
  | Żałobna Zbr. Płyt.+0 (11600) | body | sura | | 12 |
  | Błękitna Szata+0 (11800) | body | shaman | | 12 |
  | Tradycyjny Hełm+0 (12200) | head | warrior | | 5 |
  | Skórzana Maska+0 (12340) | head | ninja | | 5 |
  | Krwawy Hełm+0 (12480) | head | sura | | 5 |
  | Czapka Mnicha+0 (12620) | head | shaman | | 5 |
  | Skórzane Buty+0 (15000) | feet | all | | 1, and 2 faster |
  | Bojowa Tarcza+0 (13000) | shield | all | | 3, and 2 slower |
  | Drewniana Strzała (8000) | arrows | ninja | adds 3 to a bow's least | |

  The starters are wearable from level 0, as are their grades.
- **The gear past the first** (`internal/item/gear.go`), 43 kinds at +0 chosen by the veteran and the designer, is transcribed from item_proto by a throwaway script with every graded kind's grades. Each kind takes:
  - its level from `LIMIT_LEVEL`;
  - its classes, place, attack, magic attack and defence as the starters do;
  - its prices and sell prices by grade;
  - the speed its shoes or shield give by grade.

  | Line | Level 10 | 20 | 25 | 30 | 36 |
  |---|---|---|---|---|---|
  | Swords (warrior, ninja, sura) | Sejmitar (30) | Szeroki Miecz (50) | Srebrny Miecz (60) | Miecz Pełni Księżyca (290) | Miecz Półtoraręczny (80) |
  | Two-handed (warrior) | Gilotynowe Ostrze (3020) | Gizarma (3040) | Kosa Bojowa (3050) | Ostrze Z Czerw. Stali (3210) | Halabarda (3070) |
  | Daggers (ninja) | Sztylet Kobry (1010) | Sztylet Nożycowy (1020) | Krótki Nóż (4020) | Kozik Czar. Liś. (1170) | Ukąszenie Kota (1040) |
  | Bows (ninja) | Kompozytowy Łuk (2020) | Długi Łuk Jeźdźcy (2040) | Bojowy Łuk Jeźdźcy (2050) | Łuk Z Rogu Jelenia (2150) | Łuk Czarnych Ruin (2070) |
  | Fans (shaman) | Wachlarz Czarn. Tygr. (7020) | Pawi Wachlarz (7040) | Wodny Wachlarz (7050) | Wachlarz Jes. Wiatru (7160) | Oceaniczny Wachlarz (7070) |
  | Bells (shaman) | Miedziany Dzwon (5000) | Złoty Dzwon (5020) | Jadeitowy Dzwon (5030), level 32 | Antyczny Dzwon (5110) | Dzwon Fontanny (5040) |

- **A weapon's attack speed:** every weapon adds the attack speed item_proto gives it (its ADDON APPLY_ATT_SPEED), the same at every grade: from 15, the level-30 weapons and some others, to 37, the Kosa Bojowa; the first sword 22. Held, it joins everyone's 150 and what buffs add, never past 255, and so shortens the gap between blows the server allows and quickens the client's swing. Putting a weapon on or taking it off tells the player their new attack speed, POINT_CHANGE point 17, and everyone near sees it in CHARACTER_UPDATE. The veteran and the designer both asked for it before the bonus system, as the tooltip shows it.

  The armour of levels 9, 18 and 34 for each class: 11210, 11410, 11610 and 11810, defence 21; 11220 to 11820, 29; 11240 to 11840, 47. The helmets of level 21 (12220, 12360, 12500, 12640), defence 9; the Pięciokątna Tarcza (13020), 4 and 6 slower; and the shoes Drewniane Buty (15040, level 17) and Złote Buty (15100, level 34).
- **Level limits:** an item is put on only from its level, read under the world's lock, as kills raise it. Gear already worn stays on.
- **Left out of these kinds,** as the debaters chose: armour's slowness (2 to 7), and the shoes' resists.
- **An item is in a cell or worn, never both,** and one item is worn in each place; the database keeps both rules. A swap moves two items in one transaction, the new one in before the old one out, so one to a place is checked at its end.
- **Putting on:**
  - A double-click on a wearable item in the bag, or a drag onto its own empty place.
  - A unique item double-clicked goes in the first unique place that is free, 97 then 98, and only with both taken in place of what is in 97. The client sends no place with a double-click, so the server chooses; open-mt2 always takes 97, which would take off the gloves to put on the third hand.
  - What is worn there goes into the cells the new item leaves if it fits, or the first room in the bag. With no room, nothing changes.
  - A class that may not wear it is told why, as a chat line from the game (type 1, vid 0), and the item is shown where it was.
  - A drag onto the wrong place, or onto an occupied one, is refused the same way, without a chat line. The client sends a drag onto an occupied place as a double-click anyway.
- **Taking off:** a double-click on what is worn, into the first room in the bag, or a drag into cells where it fits. A full bag refuses it.
- **Each change** is written first, then shown:
  - the old places are emptied and the new ones filled, with ITEM_SET;
  - when what is worn changed, CHARACTER_UPDATE goes to the player and everyone who sees them, and the parts are kept for those who arrive later;
  - when the armour, helmet, shoes or shield changed, the player is told their new DEF;
  - when their move speed changed, the player is told it, and the update carries it to everyone who sees them.

  A change the database refuses is not made, and the client is shown the items where they were.
- **Entering:** the worn items are put on the character from the inventory, its CHARACTER_INFO carries the parts, and its POINTS carry DEF and the move speed, as its CHARACTER_ADD does. The character screen reads the armour worn with each character.
- **Fighting** ([combat.md](combat.md)):
  - **A blow** adds twice a roll of the weapon's attack. A level-1 sura with Miecz+0 hits a wild dog for 31 to 42 rather than 11 or 12, so a dog takes about four blows instead of twelve.
  - **A bite:** the armour's defence adds to the player's: a wild dog bites a level-1 sura in her armour for 24 to 28 rather than 27 to 31.
- **DEF** is the defence bites use: level + vitality + the armour's, helmet's, shoes' and shield's defence, and a defence buff's. It's sent on entering, and again when any of them changes.
- **The move speed** is everyone's 150, what the shoes add, and the shield's 2 off. The server checks no move against it, so only the client and those watching need telling.

## How we know

- **The client's side:** its source and scripts named above. Every packet and cell was read there, and the swap and refusal rules are this server's own.
- **The kinds:** the client's `--dump_proto--/item_proto.txt`, as on [items.md](items.md).
- **Seen from a bot** against the running server: Szepta, a sura in her armour, put on the Krwawy Hełm, shown at 91, her DEF rising from 23 to 28, and shoes +6, shown at 92, her DEF then 29, and her speed told as 159. The armour trader showed the helmets and shoes on the third row, and shoes bought for 500 went into her bag. Later she bought the Bojowa Tarcza for 400 from the fourth row. Put on, it was shown at 100, her DEF went from 29 to 32 and her speed from 159 to 157, and taking it off put both back. Given to the blacksmith, it was offered +1 for 200 at 100 in a hundred. Later Iskra, a level-20 shaman, opened Jinno's weapon trader: the window held the first weapons, the arrows and each line's weapons of levels 10 and 20 at their cells, for 600 and 3,000. Her Wodny Wachlarz, of level 25, was refused with «Masz za niski poziom, aby to założyć.» and stayed in its cell; her Pawi Wachlarz, of level 20, was put on, the first fan going to its cell.
- **Which kinds:** settled by the veteran and the designer, the veteran deciding: each line at levels 10, 20, 25 and 36, armour at 9, 18 and 34, sold below 21 and dropped above, the level-30 weapons added later in the same way, upgrades costing more by tier, jewellery later.
- **The blow's weapon term:** this server's formula, which [combat.md](combat.md) explains; the original adds the weapon's attack too, scaled differently.

## What proves it

- `internal/item/item_test.go`: a worn item in no cell; who may wear what, by class; gear worn only from its level; every worn kind with its classes and stats; the 49 kinds past the first, from levels 9 to 36, with their grades, levels, sell prices, speeds and weapon kinds as item_proto has them, at +5 and +9, the Miecz Pełni Księżyca and the Kozik Czar. Liś. among them. Every weapon adding its attack speed, the same at every grade, and nothing else worn adding any.
- `internal/game/equipment_test.go`: also a weapon put on telling its attack speed, 150 and its 22, and taken off telling 150 again, both in CHARACTER_UPDATE too; gear above the character's level refused with a line and kept in its cell, then put on at its level.
- `internal/combat/combat_test.go`: a sword adding twice a roll of its attack, 31 to 42 on a wild dog; armour taking bites down to 24 to 28.
- `internal/protocol/protocol_test.go`: CHARACTER_UPDATE and CHARACTER_INFO with armour and weapon, byte for byte; DEF at point 20.
- `internal/store/items_test.go`: a helmet, shoes and a shield kept worn; a swap kept whole; one item to a place, and an item in a cell or worn; the armour read with the character.
- `internal/game/equipment_test.go`:
  - a weapon put on by a double-click, shown worn, and seen by the player and a neighbour;
  - armour put on by a drag, with DEF raised to 16;
  - a swap;
  - taking off by a double-click and by a drag;
  - a fan refused for a sura, with the chat line;
  - a sword dragged onto the body's place refused, and nothing taken off into a full bag;
  - the sword and armour, once put on, counting in a fight;
  - a swap emptying both places, then filling both;
  - a helmet put on, at the head's place, with DEF raised by 5;
  - shoes +6 making her 9 faster, told to her and shown to a neighbour, and back at 150 when taken off;
  - a shield put on at 100, with DEF raised by 3 and her 2 slower, told to her and shown to a neighbour, and back at 150 when taken off;
  - armour, a helmet, shoes and a shield all counting against a bite;
  - shoes +9 worn on entering setting her points' speed to 165, and with a shield 163.
- `internal/server/equipment_test.go` (real sockets and database):
  - a sword put on kept over Change character, and in CHARACTER_INFO on entering;
  - the armour on the character screen;
  - a neighbour seeing the sword put on.

## Not done yet

- The other places: wrists, neck and ears. The unique places hold only the thief's gloves ([thief-gloves.md](thief-gloves.md)) and the third hand ([third-hand.md](third-hand.md)).
- Gear past level 36, and jewellery: bracelets, necklaces and earrings.
- Attack speed, armour's slowness and the other bonuses the tooltip shows.
- The sex limits some kinds carry; costumes.
