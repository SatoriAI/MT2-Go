# Upgrading

## What the player sees

Each village has a blacksmith, Kowal, by the weapon trader's building ([monsters.md](monsters.md)). Dragging a first weapon, armour, helmet, shoes or shield from the bag onto him opens the refine dialog. It shows what the item becomes, its cost in yang, its chance, and, from the step to +5, the material it takes.

- **Yes** upgrades it. The item is shown at its new grade, the yang goes down, and the chat says «Przedmiot został ulepszony.».
- **A failure** says «Ulepszanie nie powiodło się.». The yang and material are gone. From the step to +5 on, the item also drops a grade. It is never destroyed, and its bonuses stay with it at every grade ([bonuses.md](bonuses.md)).
- **Up to the step to +4,** the dialog asks nothing more. From the step to +5, it warns «Przedmiot może zostać zdegradowany w ten sposób.» first.
- **Refusals:**
  - anything else given to him, a potion, a +9 or nothing, is answered «Ten Przedmiot nie może zostać ulepszony.»;
  - without the yang, the chat says «Nie masz dość Yang na ulepszenie.», and without the material, «Brakuje ci materiałów do ulepszenia.».

| Upgrade to | Yang | Material | Chance | On a failure |
|---|---|---|---|---|
| +1 | 200 | – | 100% | the cost is lost |
| +2 | 400 | – | 90% | the cost is lost |
| +3 | 800 | – | 80% | the cost is lost |
| +4 | 1,500 | – | 70% | the cost is lost |
| +5 | 3,000 | 1 | 60% | the cost is lost, and a grade |
| +6 | 5,000 | 2 | 50% | the cost is lost, and a grade |
| +7 | 8,000 | 3 | 40% | the cost is lost, and a grade |
| +8 | 12,000 | 4 | 30% | the cost is lost, and a grade |
| +9 | 20,000 | 5 | 20% | the cost is lost, and a grade |

The yang is the table's for the first gear and gear below level 20. It is twice that from level 20, four times from 25 and eight times from 34, so a Miecz Półtoraręczny's step to +9 costs 160,000. The materials and chances stay the same for every kind.

**The material** is Szpon Wilka (30028) for a weapon, which wolves leave, and Ząb Dzika (30004) for armour, a helmet, shoes or a shield, which boars leave ([drops.md](drops.md)).

**Each grade is stronger,** as its tooltip shows:
- a weapon adds the same to both ends of its attack: 7 a grade for the sword and the fan, 4 for the dagger, and 8 for the bow and the glaive;
- armour adds 6 defence a grade, a helmet 4, and the shield 6, while staying 2 slower at every grade;
- shoes keep their 1 defence and move faster: 2 at +0, then 3, 4, 5, 6, 7, 9, 11, 13 and 15 at +9;
- the gear past the first adds what its item_proto rows say, which grows faster from level 25 ([equipment.md](equipment.md)).

## What the client does

- **Dragging an item onto an NPC** sends GIVE_ITEM with the NPC's vid, the item's place and its count, once the client has checked the NPC is an NPC and the item is not worn (`game.py`; `PythonNetworkStreamPhaseGame.cpp`).
- **REFINE_INFORMATION_NEW** opens the refine dialog. The client takes the cell, the kind it becomes, the cost, the chance and each material up to their count from the packet, and draws the rest from its own item data. It never uses the kind upgraded from (`PythonNetworkStreamPhaseGame.cpp`). The dialog checks neither yang nor material, so the server must.
- **The dialog's type** decides its question. At a chance of 100, or with type 5, yes is sent at once. Type 2 warns of a lower grade, and every other type of destruction (`uirefine.py`).
- **Yes** sends REFINE with the cell and the type it was opened with. Cancel, the close button and Escape send REFINE with 255 for both.
- **The outcome** has no packet of its own. The client is told the item's ITEM_SET, its yang's POINT_CHANGE and a chat line.

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| GIVE_ITEM `53`, client | 9, then the sequence byte | the NPC's vid u32, window u8, cell u16, count u8 |
| REFINE_INFORMATION_NEW `77` | 60 | type u8, cell u8, the kind upgraded u32, the kind it becomes u32, material count u8, cost u32, chance u32, then 5 materials of vnum u32 and count u32, those past the count empty |
| REFINE `60`, client | 3, then the sequence byte | cell u8, type u8; 255 and 255 when the dialog closes |
| ITEM_SET `14`, POINT_CHANGE `11`, CHAT `04` | 42, 17, by length | the item and each stack of material changed; yang, point 11; the outcome, type info |

The cell is one byte, which the bag's 90 cells fit.

## What the server does

`internal/upgrade` holds the table. `internal/item` holds the grades, `internal/mob` the blacksmith and the drops, and `internal/game/upgrade.go` carries out each step.

- **The grades** are made, +1 to +9 at the next vnums as the client numbers them, from each graded kind's rows in item_proto, transcribed (`internal/item/gear.go`): the first gear and the gear past it ([equipment.md](equipment.md)). Each takes the client's own values:
  - its name, its +0's with its grade;
  - its item_proto VALUE5 added to both ends of a weapon's attack and of its magic attack, or twice to armour's defence, as the tooltip shows. It is a step a grade for the first gear, and grows faster for the gear from level 25: a sword of 25 adds 6, 12, 18, 24, 32, 41, 51, 62 and 74;
  - its sell price, divided by 5 as for every kind ([shops.md](shops.md));
  - for shoes and a shield, their speed.

  No shop sells them.
- **Giving:** an item given to an NPC within reach (as a click's, 1000) whose kind upgrades, from a bag cell, of a kind graded below +9, opens the dialog.
  - It uses type 5 up to the step to +4 and type 2 from the step to +5, so the client's question matches what can happen: never its warning of destruction.
  - The connection keeps which item was given, and to whom, until the dialog answers, Change character or leaving. So a yes that arrives after Change character, or after the player is back, upgrades nothing.
  - Anything else given to the blacksmith is told no, in a dialog of his. Given to anyone else, nothing happens. Any give closes a dialog open before it.
- **Yes,** in order:
  1. it is taken only for the item given, still at that cell, with the blacksmith still in reach;
  2. without the yang, or the material in the bag, it is refused and told;
  3. otherwise the connection's generator rolls 0 to 99 against the chance.
  4. One transaction then takes the yang, keeps the item's new kind and keeps what is left of each stack of material. The materials come from the first stacks in the bag, a stack used up forgotten.
  5. The player is shown the item, each stack, their yang and the outcome.
  - A failure keeps the kind below the step to +5, and from it on lowers it a grade.
- **No lock:** the yang and the bag belong to the connection's goroutine. Only the reach looks at the world, under its lock.
- **A write that fails** changes nothing and is logged.
- **255** closes the dialog and does nothing. Any answer closes it, so a second yes needs the item given again.

## How we know

- **The client's side:** the client's source and the scripts named above. The dialog script `uirefine.py` comes from another build of the client, as the TMP4 client's root has none unpacked; its types decide only the question asked.
- **The values:** the grades' values and sell prices are from the client's `--dump_proto--/item_proto.txt`, and the names from its Polish locale item_proto, read as in [items.md](items.md). The chat lines for the outcome and for an item that cannot be upgraded are the client's own, from `locale_game.txt`.
- **The table** is the veteran's, from the design debate: an item is never destroyed, so no scrolls are needed.
- **Seen from a bot** against the running server, as Szepta at the Jinno blacksmith with 10,000 yang:
  - a sword +0 given opened the dialog for +1 at 200 and 100%, and yes made it +1;
  - a sword +4 failed its step to +5 and was kept at +3, one of her two claws and 3,000 yang spent;
  - the +1 became +2 for 400 with «Przedmiot został ulepszony.», and the +3 failed its 70% to +4 with «Ulepszanie nie powiodło się.», kept at +3;
  - a potion was told «Ten Przedmiot nie może zostać ulepszony.», and a dialog closed with 255, then a yes sent after it, did nothing.
- **Not yet seen in the real client:** the dialog, its icons and its question.

## What proves it

- `internal/upgrade/upgrade_test.go`: a sword from +0 to +9 by the table; armour, a helmet, shoes and the shield taking a tooth; a failure costing a grade only from the step to +5; only gear below +9 upgraded; gear past the first costing the table's yang twice from level 20, four times from 25 and eight times from 34, its chance and material the table's.
- `internal/item/item_test.go`: the grades' names, attack and defence as the client's; their sell prices; every worn kind graded.
- `internal/mob/mob_test.go`: wolves and boars leaving the materials, 3 in 100; only the blacksmith upgrading.
- `internal/protocol/protocol_test.go`: GIVE_ITEM and REFINE read and framed; REFINE_INFORMATION_NEW byte for byte.
- `internal/store/upgrade_test.go` (PostgreSQL): an upgrade kept whole, a stack used up forgotten; one the yang cannot cover changing nothing.
- `internal/game/upgrade_test.go`:
  - **The dialog:**
    - a sword +0 opening it with no question;
    - the step to +5 listing a claw and warning of a lower grade;
    - a potion, a +9, a worn sword and an empty cell told no;
    - only the blacksmith taking an item, not a shopkeeper.
  - **The outcome:**
    - a success raising the grade, told and kept;
    - the step to +3 succeeding on a roll of 79 and failing on 80, keeping the grade;
    - a failure on the step to +5 spending the claw and costing a grade;
    - claws taken from the first stacks, the next left alone.
  - **Refusals:**
    - no yang, or no claw, refused and told;
    - closing, no dialog open, another cell, another item, walking off, or a yes after another give doing nothing;
    - a write that fails changing nothing.
- `internal/world/spawns_test.go`: a blacksmith in each first village.
- `internal/server/upgrade_test.go` (real sockets and database): Hendra upgrading a sword +0 at the blacksmith, kept with her yang; a dialog left open over Change character upgrading nothing, with a yes in flight or sent after.

## Not done yet

- Upgrading the gear past the first maps, and jewellery.
- Scrolls, magic stones and the blacksmiths of other rates.
- The effect drawn on a success or failure (SPECIAL_EFFECT `72`, 11 and 12), which the original may have sent.
