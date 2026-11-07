# Shops

## What the player sees

Clicking a shopkeeper in a village walks the character to them and opens their shop: a window of five by eight cells beside the inventory, each item with its price in its tooltip.
- **The weapon trader** (Handlarz Bronią) sells each class's first weapon.
- **The armour trader** (Handlarz Zbrojami) sells each class's first armour and helmet, and the first shoes.
- **The general store** (Handlarka Różności) sells the red and blue potions, singly and twenty to a pack.

A right-click on an item, or a click with the buy cursor, buys it into the bag. Potions join a stack of their kind with room for them. The yang at the foot of the inventory goes down by the price.

To sell, the player drags an item from the bag onto the shop, or clicks it with the sell cursor. The shop pays what the item's tooltip says it is worth, and the chat says «Otrzymałeś N Yang.» A first weapon is worth nothing, so selling one just gets rid of it.

A buy the player can't pay for, has no room for, or that the shop can't do is refused with a window saying why:
- «Niewystarczająca ilość Yang»;
- «Nie ma miejsca w ekwipunku.»;
- «Ten Przedmiot nie może zostać tu użyty.»

Walking away closes the shop.

## What the client does

- **A click on an NPC** walks to within 500 of them, then sends ON_CLICK, at most once a second (`PythonPlayerInput.cpp`, `PythonPlayer.cpp`).
- **The shop window:**
  - It shows what SHOP START lists, each entry in the cell its place in the list names. A tall item is drawn over the cells below it (`PythonShop.cpp`, `uishop.py`, `ui.py`).
  - The price shown for an entry is the server's, for the whole entry. A count of one is not drawn.
- **What it sends:**
  - **A buy** sends BUY with the shop cell. Dragging an item into the bag buys too, without naming a cell, so the server always picks where it goes (`uiinventory.py`).
  - **A sale** sends SELL2 with the inventory cell and the count. The sell cursor refuses a worn item, but a drag onto the shop does not (`uishop.py`).
  - **Closing:** the window closes past 1000 from where it opened, on either axis, or when the player closes it, and the client then sends END.
- **The sell price is the client's own,** from its item_proto, shown in the tooltip and the sale's question: the item's sell value times the count, divided by 5 (`PythonPlayerModule.cpp`).
- **The client checks no yang before buying.** It shows each refusal by its subheader (`game.py`, `OnShopError`), and has no case for OK or SOLD_OUT, so the server never sends those. Success is shown by ITEM_SET and the yang's POINT_CHANGE.

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| ON_CLICK `1A`, client | 5, then the sequence byte | vid u32 |
| SHOP `26`, START | 1728 | size u16, subheader `00`, the owner's vid u32, then 40 entries of 43 bytes: vnum u32, price u32 for the whole entry, count u8, display position u8 (ignored for an NPC), 3 sockets i32 and 7 bonuses {u8, i16}, all 0 |
| SHOP `26`, refusal | 4 | size u16 (4), subheader: `05` not enough yang, `07` bag full, `08` not possible |
| SHOP `32`, client | by subheader, then the sequence byte | END `00` (2 bytes); BUY `01`, a count u8 always 1, the shop cell u8 (4); SELL `02`, a cell (3, never sent by the scripts); SELL2 `03`, the inventory cell u8, the count u8 (4) |
| ITEM_SET `14`, POINT_CHANGE `11` | 42, 17 | the item bought or what is left of the one sold; yang is point 11 |

The client's SHOP is the one packet framed by its subheader. A subheader the server doesn't know ends the connection, as an unknown header does, since nothing after it can be delimited.

## What the server does

`internal/mob` says what each shopkeeper sells, `internal/item` what each kind costs, and `internal/game/shops.go` carries out each step.

- **Prices** are on the item kinds:

  | Kind | Price | Sells for |
  |---|---|---|
  | Czerwona Mikstura(M) (27001) | 32 | 6 |
  | Czerwona Mikstura(Ś) (27002) | 96 | 18 |
  | Niebieska Mikstura(M) (27004) | 96 | 24 |
  | the first weapons (10, 1000, 2000, 3000, 7000) | 100 | 0 |
  | the first armours (11200, 11400, 11600, 11800) | 3000 | 600 |
  | the first helmets (12200, 12340, 12480, 12620) and shoes (15000) | 500 | 100 |
  | their grades +1 to +9 ([upgrading.md](upgrading.md)) | not sold | a weapon 20 at +1 to 150 at +9; armour 1000 at +1 to 16000 at +9; a helmet or shoes 160 at +1 to 4000 at +9 |
  | Szpon Wilka (30028), Ząb Dzika (30004) | not sold | 100 |

  - **The price** is the client's item_proto shop price, but 100 for each first weapon, which it prices at nothing or 100.
  - **What a shop pays** is the client's sell value divided by 5, exactly what the tooltip shows. A gate test checks it is never more than the price, so buying and selling back never makes yang.
- **The wares** are on the shopkeeper's kind, each in a cell written out:

  | Shopkeeper | Sells |
  |---|---|
  | 9001 | the five first weapons, along the top row |
  | 9002 | the four first armours along the top row, and the four helmets and the shoes on the third, below them |
  | 9003 | each potion singly on the top row, and twenty to a pack on the next |

  The rows below a tall item are left empty. A gate test checks that every ware fits the window, covers no other, and is priced within the client's u32.
- **Opening:** a click opens the shop of a character whose kind has wares, if the player is alive and it stands on their map within 1000 of them. That's the client's 500, plus room for moves not yet arrived. The connection keeps which shop is open until END, Change character or leaving.
- **Buying** a cell's ware, in order:
  1. a cell with nothing is refused;
  2. a price above the player's yang is refused;
  3. no room in the bag is refused. The room is a stack of the kind with space for all of it, or the first cells it fits, as picking up uses.
  4. Otherwise one transaction takes the yang and keeps the item, as a new one or the stack's new count, and the player is shown it and their yang.
- **Selling** from a bag cell, any kind to any shopkeeper:
  1. a cell with no item, a worn item (which has no cell), and a count of 0 or more than the stack holds are refused;
  2. a sale that would take the player past what they may hold is refused.
  3. Otherwise one transaction pays the yang and keeps what is left, the stack at its lower count or the item forgotten, and the player is shown it and their yang.
- **No lock for either:** a character's yang and inventory belong to its connection's goroutine ([drops.md](drops.md)), so buying and selling take no lock. Only the click looks at the world, under its lock.
- **A write that fails** changes nothing and is logged. The client moved nothing itself, so nothing needs undoing on screen.
- **Dropped:** a buy or sale with no shop open, and SELL, which the scripts never send.
- **The client's END** may arrive after Change character, as the window closes, and is dropped like the other packets still in flight.

## How we know

- **The client's side:** its source and scripts named above. The item_proto prices are from the client's `--dump_proto--/item_proto.txt`.
- **Seen from a bot** against the running server:
  - Szepta, with 56 yang, walked to the Jinno general store and clicked it. The shop opened with its six wares at 32, 96, 96, 640, 1920 and 1920.
  - A red potion joined her stack for 32, and selling one back paid 6.
  - A pack she couldn't afford was refused with `05`, and her worn sword's cell with `08`.
  - A buy after END did nothing.
- **Not yet seen in the real client:** the window itself, its icons, tooltips and popups.

## What proves it

- `internal/item/item_test.go`: every kind's prices, and none paying more than it costs.
- `internal/mob/mob_test.go`: every ware fitting the window without covering another, and only the three traders selling.
- `internal/protocol/protocol_test.go`:
  - START byte for byte at its cells;
  - a refusal;
  - a SHOP from the client framed by its subheader, with an unknown subheader refused;
  - a click's vid.
- `internal/store/shop_test.go`: a buy kept whole, as a new item or onto a stack; one the yang can't cover, or whose item can't be kept, changing nothing; a sale kept whole, part of a stack or all of it.
- `internal/game/shops_test.go`:
  - a click opening the general store with its wares;
  - nothing opening from too far, to the dead, on a monster, or on the storekeeper;
  - buying into a new cell, and a pack onto a stack;
  - not enough yang, no room and an empty cell each refused with their reason;
  - nothing bought or sold with no shop open, or after closing it;
  - sales of part of a stack, all of it, and a weapon for nothing;
  - a worn item, by the cell the client names or the cell 0 a worn item is kept at, too many, none, or an empty cell refused;
  - a sale past the cap refused;
  - a failed write changing nothing.
- `internal/server/shops_test.go` (real sockets and database): a potion bought and sold back, the END the client sends while changing character ending nothing, and 74 of 100 yang kept with no potion.

## Not done yet

- Stock that runs out, shop tabs and other currencies.
- More than one entry per click, and checking the distance again on every buy: an open shop stays open until the client closes it or the player leaves.
- The storekeeper's safebox (9005), trading between players and private shops.
- Dropping yang from the inventory.
