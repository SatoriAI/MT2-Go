# Equipment: a weapon, body armour, a helmet and shoes

## What the player sees

The inventory window's upper half is the character: the places worn items go. A double-click on a weapon, armour, a helmet or shoes in the bag puts it on, into its own place; so does dragging it onto its own empty place. The character then holds the weapon and wears the armour, for themselves and everyone near, and on the character screen too. A double-click on something worn takes it off into the first room in the bag, and dragging it to empty cells puts it there.

Putting on a second weapon swaps them: the one held goes where the new one was. An item for another class stays where it was, and the chat says «Ta postać nie może tego nosić.»

A weapon makes every blow stronger. Armour, a helmet and shoes each take more off every bite, and the character window's DEF rises by their defence; its attack follows the weapon. Shoes also make the character faster, more with each grade, for themselves and as everyone near sees them run.

Only a weapon, body armour, a helmet and shoes can be worn so far: each class's first ones, the first shoes for all, and their grades ([upgrading.md](upgrading.md)).

## What the client does

- **Worn items are in the inventory window too:** cells 90 and up, by the client's wear places: the body at 90, the head at 91, the feet at 92 and the weapon at 94 (`GameType.h`, `ItemData.h`), with the window number `01`.
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
| ITEM_SET `14` | 42 | as on [items.md](items.md); worn items at cells 90, 91, 92 and 94 |
| CHARACTER_UPDATE `13` | 35 | vid u32, parts u16 ×4 (armour, weapon, head, hair), move speed u8, attack speed u8, state u8, affects u32 ×2, guild u32, alignment u16, player-kill mode u8, mount u32 |
| CHARACTER_INFO `88` | 54 | as on [entering-the-world.md](entering-the-world.md), with the armour and weapon parts |
| POINT_CHANGE `11` | 17 | DEF is point 20, the move speed 19 |

## What the server does

`internal/item` says what is worn and by whom; `internal/game/items.go` puts it on and takes it off; `internal/combat` counts it.

- **What can be worn:** each kind says where it goes, which classes may wear it, and a weapon's attack or armour's defence. The values come from the client's `item_proto`: `ITEM_WEAR`, the classes its `ANTI_` flags leave, `VALUE3` to `VALUE4` for a weapon and `VALUE1` for anything else, and the speed shoes add, its `APPLY_MOV_SPEED`.

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

  All of them are wearable from level 0, as are their grades, so the game checks no level yet; that comes with the first kind needing one. The Copper Bell, a shaman's weapon from level 10, was taken out of the table for that reason: only the starters are kept.
- **An item is in a cell or worn, never both,** and one item is worn in each place; the database keeps both rules. A swap moves two items in one transaction, the new one in before the old one out, so one to a place is checked at its end.
- **Putting on:**
  - A double-click on a wearable item in the bag, or a drag onto its own empty place.
  - What is worn there goes into the cells the new item leaves if it fits, or the first room in the bag. With no room, nothing changes.
  - A class that may not wear it is told why, as a chat line from the game (type 1, vid 0), and the item is shown where it was.
  - A drag onto the wrong place, or onto an occupied one, is refused the same way, without a chat line. The client sends a drag onto an occupied place as a double-click anyway.
- **Taking off:** a double-click on what is worn, into the first room in the bag, or a drag into cells where it fits. A full bag refuses it.
- **Each change** is written first, then shown:
  - the old places are emptied and the new ones filled, with ITEM_SET;
  - when what is worn changed, CHARACTER_UPDATE goes to the player and everyone who sees them, and the parts are kept for those who arrive later;
  - when the armour, helmet or shoes changed, the player is told their new DEF;
  - when the shoes changed, the player is told their move speed, and the update carries it to everyone who sees them.

  A change the database refuses is not made, and the client is shown the items where they were.
- **Entering:** the worn items are put on the character from the inventory, its CHARACTER_INFO carries the parts, and its POINTS carry DEF and the move speed, as its CHARACTER_ADD does. The character screen reads the armour worn with each character.
- **Fighting** ([combat.md](combat.md)):
  - **A blow** adds twice a roll of the weapon's attack. A level-1 sura with Miecz+0 hits a wild dog for 31 to 42 rather than 11 or 12, so a dog takes about four blows instead of twelve.
  - **A bite:** the armour's defence adds to the player's: a wild dog bites a level-1 sura in her armour for 24 to 28 rather than 27 to 31.
- **DEF** is the defence bites use: level + vitality + the armour's, helmet's and shoes' defence, and a defence buff's. It's sent on entering, and again when any of them changes.
- **The move speed** is everyone's 150, and what the shoes add. The server checks no move against it, so only the client and those watching need telling.

## How we know

- **The client's side:** its source and scripts named above. Every packet and cell was read there, and the swap and refusal rules are this server's own.
- **The kinds:** the client's `--dump_proto--/item_proto.txt`, as on [items.md](items.md).
- **Seen from a bot** against the running server: Szepta, a sura in her armour, put on the Krwawy Hełm, shown at 91, her DEF rising from 23 to 28, and shoes +6, shown at 92, her DEF then 29, and her speed told as 159. The armour trader showed the helmets and shoes on the third row, and shoes bought for 500 went into her bag.
- **The blow's weapon term:** this server's formula, which [combat.md](combat.md) explains; the original adds the weapon's attack too, scaled differently.

## What proves it

- `internal/item/item_test.go`: a worn item in no cell; who may wear what, by class; every worn kind with its classes and stats.
- `internal/combat/combat_test.go`: a sword adding twice a roll of its attack, 31 to 42 on a wild dog; armour taking bites down to 24 to 28.
- `internal/protocol/protocol_test.go`: CHARACTER_UPDATE and CHARACTER_INFO with armour and weapon, byte for byte; DEF at point 20.
- `internal/store/items_test.go`: a helmet and shoes kept worn; a swap kept whole; one item to a place, and an item in a cell or worn; the armour read with the character.
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
  - armour, a helmet and shoes all counting against a bite;
  - shoes +9 worn on entering setting her points' speed to 165.
- `internal/server/equipment_test.go` (real sockets and database):
  - a sword put on kept over Change character, and in CHARACTER_INFO on entering;
  - the armour on the character screen;
  - a neighbour seeing the sword put on.

## Not done yet

- The other places: wrists, neck, ears, shield, the unique places and arrows.
- More weapons and armour beyond the first, which come with drops.
- Magic attack, attack speed and the other bonuses the tooltip shows.
- Level limits, with the first kind that has one; the sex limits some kinds carry; costumes.
