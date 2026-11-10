# Items and the inventory

## What the player sees

Pressing I opens the inventory: two pages of 5 by 9 cells, tabs I and II. A character owns whatever was given to it on the command line, shown with the client's own icons, names and tooltips: a sword stands two cells tall, a glaive three, a stack of potions shows its count. Dragging an item to empty cells where it fits moves it there, and it is still there after logging out. Everything else the window offers does nothing yet, without the client being disconnected:
- splitting a stack or dropping one item onto another;
- using an item, dropping it on the ground, or putting it on the taskbar;
- giving it to an NPC, or trading it.

## What the client does

- **The client draws an item from its own data.** The server sends only the item's number (vnum) and count, and the client takes the icon, name, size and tooltip from its `item_proto` (`PythonNetworkStreamPhaseGameItem.cpp`, `RecvItemSetPacket`).
- **The inventory** is two pages of 45 cells, numbered 0 to 89 across a row of five then down; equipment cells follow from 90 (`GameType.h`). An item's cell is its top; a taller item covers the cells below it in its column.
- **Dragging** is a click to pick an item up and a click to put it down. The client sends a move with the item's cell, the cell it was put on, and the count, the whole stack unless the player split it (`uiinventory.py`).
- **The window sends nine other packets:**
  - a double-click (use);
  - an item put on the taskbar, taken off it, or two slots swapped;
  - a drop on the ground (`game.py`, after the "Czy chcesz upuścić…" question);
  - an item dropped onto another;
  - an item dragged onto an NPC, or onto a player, which starts a trade.

## The bytes

A position is the window (u8, the inventory is `01`) and the cell (u16). Every client packet is followed by its sequence byte.

| Packet | Bytes | Fields |
|---|---|---|
| ITEM_SET `14`, server | 42 | position, vnum u32, count u8, 3 sockets i32, 7 bonuses {type u8, value i16}; all zero for now. vnum 0 empties the cell |
| ITEM_MOVE `0D` | 8 | from, to, count u8 |
| ITEM_USE `0B` | 4 | position |
| ITEM_DROP `0C` | 8 | position, gold u32 |
| QUICKSLOT_ADD `10` | 4 | taskbar slot u8, type u8, position u8 |
| QUICKSLOT_DEL `11` | 2 | taskbar slot |
| QUICKSLOT_SWAP `12` | 3 | two taskbar slots |
| ITEM_DROP2 `14` | 9 | position, gold u32, count u8 |
| EXCHANGE `1B` | 10 | subheader, arg u32, arg u8, position |
| ITEM_USE_TO_ITEM `3C` | 7 | from, onto |
| GIVE_ITEM `53` | 9 | NPC vid u32, position, count u8 |

Seen from the real client: a sword dragged from cell 0 to 17 sends `0D 01 00 00 01 11 00 01`, and dropping a stack of 50 sends its count, `32`, in ITEM_DROP2.

## What the server does

`internal/item` holds the kinds and the inventory's rules; `internal/store/items.go` keeps what characters own; `internal/game/items.go` shows and moves it.

- **The kinds** are a table in `internal/item`, only those the game hands out: each class's first weapons, body armour and helmet, the first shoes and shield, and a curated few beyond them up to level 36, with their grades +1 to +9 ([equipment.md](equipment.md), [upgrading.md](upgrading.md)), red and blue potions, the two materials of upgrades, and the skill books of the game's 38 skills and the soul stone ([skills.md](skills.md)). Each has its vnum, the Polish client's name, its height in cells, and how many one item holds, 1 or 200; what is worn says more ([equipment.md](equipment.md)), and so do a shop's prices ([shops.md](shops.md)). A kind is added with a line there. A stack holds up to 200, as open-mt2's shops sell potions.
- **An item** is a row: its kind, its count, and the cell of its top. The database keeps two items from sharing a top cell, and the game keeps a tall item from covering another's cells. Every change is written as it happens, never in a periodic save, so a crash neither loses nor duplicates an item.
- **Where items come from:** monsters' drops ([drops.md](drops.md)), and the command line.
- **Giving and taking** are orders, left by the command line while there is no website:

  ```bash
  mt2go item give Szepta 10          # a sword
  mt2go item give Szepta 27001 50    # 50 red potions
  mt2go item list Szepta             # her items with their ids, and the orders waiting
  mt2go item take Szepta 7           # item 7
  ```

  - The server carries an order out the next time the character is selected, at login or Start after Change character. Until then the order waits and `item list` shows it.
  - **A give** goes into the first cell its item fits, page by page and row by row; one that fits nowhere waits for room, without holding up the others.
  - **A take** removes its item; one whose item is gone is done with nothing to take.
  - Orders are carried out oldest first, all in one transaction. A character is played on one connection at a time, so only one carries its orders out.
  - Only the server writes the items, so what a session holds is never stale.
  - `give` refuses a kind the game lacks and a count one item cannot hold, and `take` refuses an item the character doesn't own.
- **Entering** shows each item, last of what entering sends, after the player's own character and their block settings; worn items show in their places ([equipment.md](equipment.md)).
- **A move** is made when all of the item fits at the new cell, on one page and over no other item. The item may overlap its own old cells. The game writes the new cell, then tells the client the old cell is empty and the new one holds the item. Refused, and answered with the item where it was:
  - a split;
  - a drop onto another item;
  - a cell it would run off the page from;
  - another window;
  - a move the database did not take.
- **The other nine packets** are taken and logged at debug level, so the client is never disconnected for them.
- **Removing a kind** that someone owns, or is to be given, stops the server from starting, naming the vnums, rather than taking the items away unseen.

## How we know

- **The client's side:** its source (`Packet.h`, `GameType.h`, `PythonNetworkStreamPhaseGameItem.cpp`) and scripts (`uiinventory.py`, `game.py`). Each packet's size and the moves were checked against the real client: an ITEM_SET sent on entering showed its icon, two pages, a cleared cell, and the bytes of a drag, a double-click, a drag to the taskbar and a drop.
- **The kinds:**
  - **Sizes and stacking:** the client's `--dump_proto--/item_proto.txt` (`SIZE`, `ITEM_STACKABLE`).
  - **Polish names:** read once from the locale's encrypted `item_proto`, by a throwaway decoder following `ItemManager.cpp` (XTEA with the item key, then LZO, 156-byte records). Its sizes matched the dump. The decoder is not in this repository.
- **The stack of 200:** open-mt2's shop data.

## What proves it

- `internal/item/item_test.go`:
  - every kind within what the client holds;
  - an item fitting by height on one page, and around another item's cells;
  - an item not in its own way;
  - the first free cell, and none in a full inventory;
  - a give of a kind and a count one item holds.
- `internal/protocol/protocol_test.go`: ITEM_SET byte for byte; each of the ten packets cut at its size; a move read as the real client sent it.
- `internal/store/items_test.go`:
  - gives placed in order;
  - a give that fits nowhere waiting while the next is done;
  - takes, including of an item already gone;
  - orders refused for nobody or for an item not theirs;
  - the database refusing a bad count, cell or a shared cell;
  - unknown kinds found;
  - a move kept.
- `internal/game/items_test.go`: a move made, kept and told; a split, a drop onto another item, a move off the page and to another window refused with the item shown back; a move the database refuses not made; a drag from an empty cell not answered.
- `internal/server/items_test.go` (real sockets and database):
  - items given shown on entering;
  - a dragged sword still where it was put after Change character;
  - the nine other packets not disconnecting;
  - the server refusing to start with an item of an unknown kind.

## Not done yet

- Yang.
- Using items other than potions ([potions.md](potions.md)). Dropping them is on [drops.md](drops.md), and the taskbar on [taskbar.md](taskbar.md).
- Splitting, merging and swapping stacks.
- The storekeeper's safebox. Shops are on [shops.md](shops.md), trading on [trading.md](trading.md).
- Delivering an order while the character is in the world, and in-game commands for game masters.
