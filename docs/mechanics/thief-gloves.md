# The thief's gloves

## What the player sees

Rękawica Złodzieja (70043) is worn in one of the inventory window's two unique places, beside the shoes. A double-click puts it in the first, a drag in either. Its tooltip says how much time it has left: «Pozostały Czas : 2Godzin» when new, then for example «1Godzin 59Minut». The time counts down only while the gloves are worn and the character is in the world: taking them off, changing character or logging out stops it, and putting them on again goes on from where it stopped. When the time runs out, the gloves leave their place and the chat says «Czas działania przedmiotu Rękawica Złodzieja minął.»

A second pair is refused while one is worn, with «Nie możesz założyć dwóch takich samych przedmiotów.», since both would only spend their time.

What the gloves do, doubling the chance of what their wearer's kills drop, comes in the next change, and so does a way to find them. Until then they are given on the command line (`mt2go item`) or with `/getitem 70043` on a development server.

## What the client does

- **The unique places** are cells 97 and 98 of the inventory window, WEAR_UNIQUE1 and WEAR_UNIQUE2 (`inventorywindow.py`). Moves to and from them are ITEM_USE and ITEM_MOVE, as for any worn item ([equipment.md](equipment.md)).
- **The time left:** for a unique item without LIMIT_REAL_TIME, the tooltip reads the item's last socket as minutes and shows them as hours and minutes (`uitooltip.py`, `AppendUniqueItemLastTime`). The client counts nothing itself; the server sends the item again as each minute goes.
- **CONFIRM_WHEN_USE:** the client asks before the first use on its own; the server is not involved.

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| ITEM_SET `14` | 42 | the gloves at cell 97 or 98, sockets `00000000 00000000` and the minutes left as i32, rounded up; at their end, vnum 0 there |
| CHAT `04` | its own length | the info line at their end |

## What the server does

`internal/item` has the kind and its two hours (item_proto's VALUE0, 120 minutes); `internal/game/timed.go` counts them; `internal/store` keeps how long each pair has been worn, to the second, in `items.worn_for`.

- **Counting:** the world record holds what each player wears for a limited time, with how long it had been worn and since when, as they put it on or enter the world wearing it. Taking it off, leaving the world, and the periodic save keep how long it has been worn by then, so a crash loses at most one save's round.
- **The minutes:** the world loop looks at the players wearing such an item each tick, and sends the item again when its minutes left, rounded up, drop by one.
- **The end:** at 0 the loop tells the client the place is empty and why, and leaves the item for the player's connection to forget, here and in the store, before its next packet that touches what it holds. Until then a save keeps the pair as worn its whole time, so a pair whose player left first is forgotten as the character is next loaded.
- **In order:** the loop is handed each run of a pair as it is put on, not the player, so a pair put on and taken off between two ticks counts nothing, and one worn again ends once. Taking a pair off writes its time while holding the player's save, so a periodic save that read it before cannot land after.
- **Not stacking:** a second item of the same unique kind is refused, in either place, so two pairs never run at once.
- **Not yet:** the ANTI_ flags. Dropping a pair and picking it up gives it a new two hours, and it can be traded or sold. These wait for the change that enforces the flags, which comes before the gloves can be found.

The rules were settled on 2026-11-11 by a Metin2 veteran and a game designer, with the veteran deciding ties. They agreed on every point here:
- 70043 alone, not the item shop's timed copies (72004–72006) or the 30-minute scroll (71016);
- 120 minutes of wearing;
- no stacking;
- a plain line at the end.

## How we know

- **The item:** the client's `--dump_proto--/item_proto.txt`: ITEM_UNIQUE, VALUE0 120, not stackable, ANTI_DROP, ANTI_SELL, ANTI_GIVE and ANTI_MYSHOP. Its ITEM_WEAR reads WEAR_SHIELD, the dump's name for the unique bit, as it calls the shield's WEAR_UNIQUE. The Polish `itemdesc.txt` says it doubles the chance of an item dropping.
- **The tooltip:** `uitooltip.py`, which reads the last of the three sockets as minutes for a unique item.
- **open-mt2** wears 70043 in a unique place and doubles drops while it is worn.

## What proves it

- `internal/game/timed_test.go`:
  - the gloves put on in 97 with 120 minutes shown, or dragged onto 98;
  - a second pair refused in either place;
  - 119 shown once a minute is over, not before;
  - gone at their 120th minute, then forgotten here and in the store;
  - taken off after 30 minutes, with 90 shown and 30 kept;
  - counted only while worn, and from entering the world wearing them;
  - kept on leaving and in the periodic save;
  - a spent pair kept whole on leaving, and forgotten on loading;
  - on and off within a tick counting nothing, worn again ending once, and worn while the loop ticks on its own goroutine without a race.
- `internal/store/items_test.go`: the gloves kept in the second unique place with their time worn.
- `internal/protocol/protocol_test.go`: ITEM_SET with its sockets.
