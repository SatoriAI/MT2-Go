# The third hand

## What the player sees

Trzecia Ręka (71010) is worn in a unique place, 97 or 98, as the thief's gloves are, and beside them. While it is worn and working, the affect bar shows its icon, and the yang of every monster whose drops are the wearer's goes straight into their yang: it never lies on the ground, and the client shows the amount as for yang picked up. Monsters falling together in one moment, as a bravery cape's pull does, are told as one sum.

It lasts thirty days of real time from the moment it is made, worn or not, logged in or not. Its tooltip shows the time left, «Pozostały Czas : 30Dni» when new. At its end it is gone, worn or in the bag, its icon with it, and the chat says «Czas działania przedmiotu Trzecia Ręka minął.»

- **Only the wearer's yang:** that of monsters whose drops are kept for them, the player who took most of their hit points. Yang kept for another, yang free for anyone after its owner's time, and yang players drop are clicked as always.
- **Not from elsewhere:** a wearer who has left the monster's map, by the teleport ring, gets its yang on the ground by the body, theirs, as any owner does.
- **Never past the cap:** yang the store will not keep, past what a character may hold or when the store fails, falls at the wearer's feet, theirs alone, if they are still in the world; else it is lost, and logged as a warning.
- **Items** are never picked up, only yang.
- **Bound:** it is never dropped, traded or sold ([items.md](items.md)). It comes only from `/getitem 71010` on a development server, and the command line, for now.

## What the client does

- **The tooltip:** for a unique item whose VALUE2 is 1, the client reads its last socket as when it ends, in unix seconds, and shows what is left of that against its own clock (`uitooltip.py`, `AppendMallItemLastTime`). The server never sends it again to count down.
- **The icon:** affect type 503, NEW_AFFECT_AUTOLOOT, which the affect bar draws with its own tooltip (`uiaffectshower.py`).
- **The yang:** a POINT_CHANGE of point 11, which the client tells as yang received, as it does for yang clicked up ([drops.md](drops.md)).

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| ITEM_SET `14` | 42 | the hand where it is, sockets `00000000 00000000` and its end in unix seconds as i32; at its end, vnum 0 there |
| AFFECT_ADD `7E` / AFFECT_REMOVE `7F` | 23 / 6 | type 503, for ever, as the aids are ([skills.md](skills.md)) |
| POINT_CHANGE `11` | 17 | yang, point 11: the amount picked up and the new balance |

## What the server does

`internal/item` has the kind, which lasts thirty days (item_proto's VALUE0, 2,592,000 seconds), and its end is set as it is made: by the server's clock when it is picked up or given by `/getitem`, by the store's when an order from the command line makes it. `internal/store` keeps it in `items.ends_at`. `internal/game/hand.go` does the rest.

- **Who collects:** as a monster dies, under the world's lock, its drops' owner is its collector if they wear a third hand not yet at its end and stand on its map. The world loop then rolls its yang as always, the thief's gloves doubling none of it, and adds a collector's to what they picked up this tick instead of laying it down.
- **Keeping it:** after the tick, the loop starts one write a collector, off its own goroutine, since a write may take seconds, and waits for those under way as it stops. One player's writes take turns, so the balances they are told only rise. Each tells the player their new balance, and marks that their connection must read its yang again, which it does, and shows, before its next packet that touches what it holds or `/getyang`, so the yang it holds is always the store's.
- **The end:** the world record holds what each player holds that lasts in real time, where it is, as their connection tells it on entering and after each move of an item. The loop watches the players holding any, and at an item's end tells the client it is gone, takes the icon away for a worn hand, and leaves the item for the connection to forget, as a spent pair of gloves is. One ended while its owner was away is forgotten as they are next loaded.

The rules were settled on 2026-11-12 by a Metin2 veteran and a game designer, with the veteran deciding ties. Dawid chose the thirty calendar days and the development command as its only source.
- **Agreed by both:** only the wearer's yang, yang past the cap falling for them, the icon, the thirty days running from the making, and the end with a line.
- **The veteran's call:** picked up at the death, wherever the wearer stands on its map. The designer wanted the pickup range of a click, and a sweep while walking past.
- **The designer's idea, uncontested:** telling a tick's yang as one sum.

## How we know

- **The item:** the client's `--dump_proto--/item_proto.txt`: ITEM_UNIQUE, VALUE0 2592000, VALUE2 1, not stackable, ANTI_DROP, ANTI_SELL, ANTI_GIVE and ANTI_MYSHOP; the Polish `itemdesc.txt`, «Ręka, która automatycznie podnosi Yang».
- **The tooltip:** `uitooltip.py`, which reads the last socket of a unique item with VALUE2 1 as an end against `app.GetGlobalTimeStamp`.
- **The icon:** the client's source, `InstanceBase.h`, NEW_AFFECT_AUTOLOOT 503.
- **Seen in MT2-Godot:** its session's tooltip «Pozostały Czas : 30Dni» from a probe, before the server had the item.

## What proves it

- `internal/game/hand_test.go`:
  - a dog's yang going straight to the wearer, told and kept, and read again by the connection;
  - three dogs' yang told as one sum;
  - nothing picked up from the bag, for another owner, or from another map;
  - yang the store refuses falling at the wearer's feet;
  - the icon on putting it on and off;
  - its end in the last socket;
  - its end on time: told gone, its icon gone, no yang after, then forgotten;
  - one past its end forgotten on loading;
  - one given by `/getitem` ending thirty days on, and watched.
- `internal/store/items_test.go`: one given by an order ending in thirty days, a sword never.
