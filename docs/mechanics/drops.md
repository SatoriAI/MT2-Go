# Drops

## What the player sees

A monster may leave something when it dies: yang more often than not, a potion now and then, and from the stronger ones, rarely, a first weapon or armour, or from those of level 15 or more, more rarely still, a skill book ([skills.md](skills.md)). Each item lies on the grass beside the body with the name of whoever it's kept for over it in yellow, «Szepta's». That's the player who took the most of the monster's hit points, not whoever landed the last blow. For 20 seconds only they can pick it up; then the name goes and anyone may; after a minute it's gone.

Clicking an item walks the character to it and picks it up, and the pick-up key takes the nearest one. A potion joins the stack of its kind in the bag when the whole of it fits there; anything else takes the first free cells. With no room, the chat says «Nie masz miejsca w ekwipunku.» and the item stays where it lies. A potion picked up is drunk with a double-click ([potions.md](potions.md)).

A player can drop things too. Dragging an item out of the bag onto the ground, or yang from the money field, asks first; on yes, it lies beside them, with no name over it, for anyone to pick up at once, and is gone after a minute.

Yang lies on the ground as the client draws its kind 1, labelled with that kind's name, kept for its owner the same way. Picked up, it never needs room in the bag: it is added to the yang shown at the foot of the inventory window, and the chat says «Otrzymałeś 9 Yang.»

## What the client does

- **It lays an item on the terrain itself,** at the height it finds there, so the height sent is ignored (`PythonItem.cpp`). The item's look and name come from its own `item_proto`; nothing on the ground shows a count.
- **It shows the owner's name** over an item it has, and ignores a name for one it doesn't, so the name follows the item (`PythonItem.cpp`, `PythonTextTail.cpp`).
- **It refuses another player's item by itself:** it says it cannot pick it up and sends nothing (`PythonPlayer.cpp`). So when an item is freed, everyone near must be told, or their clients keep refusing.
- **Picking up:**
  - A click walks to the item, then sends PICKUP. The pick-up key takes the nearest item within 300 without walking (`PythonPlayerInput.cpp`).
  - The client sends at most one pick-up every half second.
- **Yang is an item of kind 1** on the ground (`VNUM_MONEY`, `PythonItem.h`): GROUND_ADD with vnum 1, labelled with that kind's name, with no amount. It is picked up with the same PICKUP. The yang the player has is point 11 (`POINT_GOLD`); a POINT_CHANGE for it with a positive amount makes the client print «Otrzymałeś N Yang.» itself, and a zero or negative one changes it silently (`PythonNetworkStreamPhaseGame.cpp`, `game.py`).
- **Dropping** (`game.py`, `RequestDropItem`): the client asks the player first. On yes it sends ITEM_DROP2, with the item's bag position and the count dragged, which may be part of a stack, or, for yang, the reserved window 0 at cell 0 with the amount. It moves nothing itself and waits to be told. It refuses some drops itself, with a popup and nothing sent: 1000 yang or more at once, and anything while the player sets up a private shop. The older ITEM_DROP its scripts never send.
- **Laying an item down twice** leaves a copy the client never removes, while taking away an item it doesn't have does nothing. So the server takes items away when a player leaves, and shows them again when they come back.

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| GROUND_ADD `1A` | 21 | x i32, y i32, height i32 (0), vid u32, vnum u32 |
| GROUND_DEL `1B` | 5 | vid u32 |
| OWNERSHIP `1F` | 30 | vid u32, name char[25]; empty for anyone |
| PICKUP `0F`, client | 5, then the sequence byte | vid u32 |
| POINT_CHANGE `11` | 17 | yang is point 11: the amount picked up, then the new balance |
| ITEM_DROP2 `14`, client | 9, then the sequence byte | position: window u8, cell u16; yang u32; count u8 |

Seen from the real client: a red potion laid at (977491, 272599) is `1a 53ea0e00 d7280400 00000000 a1bb0d00 79690000`, and a click on vid 900002 sends `0f a2bb0d00`.

## What the server does

`internal/mob` says what each monster may leave; `internal/game/drops.go` lays drops down, times them, and gives them up.

- **What drops:** each monster kind has a list of drops, each an item kind, a count, and a chance in 10,000, rolled on its own when it dies. There are no rank tables or level gaps; a gate test checks every drop is an item the game has, in a count one item holds.

  | Monster | Drops |
  |---|---|
  | Dziki Pies (101) | red potion, 8 in 100; blue potion, 8 in 100 |
  | Wilk (102) | the same, and a Szpon Wilka (30028), 3 in 100 |
  | Alfa Wilk (103) | the potions; each class's first weapon and armour, 2 in 1,000 each, about 2 in 100 for any one; and a Szpon Wilka, 3 in 100 |
  | Dzik (108) | the potions and the first gear, as Alfa Wilk, and a Ząb Dzika (30004), 3 in 100 |
  | Nieb. Wilk (104) | as Wilk |
  | Nieb. Alfa Wilk (105) | as Alfa Wilk |
  | Czerw. Dzik (109) | as Dzik |
  | Niedźwiedź (110), Niedźw. Grizzly (111), Tygrys (114) | the potions and the first gear |

  The claw and the tooth are the materials of the later upgrades ([upgrading.md](upgrading.md)).

  Metin stones leave more, each a set count of one kind picked from a list ([stones.md](stones.md)), and 3 times in 100 a skill book; Metin Bitwy also a soul stone once in 100. Only the stones leave the helpers of a read, Zwój Egzorcyzmu and Rada Pustelnika ([skills.md](skills.md)). Monsters of level 15 or more leave a skill book 3 times in 1,000, picked evenly from every skill's. The rare gear, the gear past the first from level 21, which no trader sells, comes only from the cursed monsters, once in 200, and the stones, 4 times in 100, one +0 kind picked evenly ([equipment.md](equipment.md)). Gear from a stone carries bonuses, and a level-30 weapon from anything sometimes the pair of average and skill damage ([bonuses.md](bonuses.md)); a piece picked up keeps them, and so does one a player drops.

- **The thief's gloves** double the chance of each item a monster leaves, up to certain, when the player its drops are kept for wears a pair whose time has not run out as it dies ([thief-gloves.md](thief-gloves.md)). It is the owner who counts, the top damager, not whoever struck last, so a pair cannot be lent to a kill. Yang is not doubled, nor the stones' chance of the gloves themselves.
- **Yang:** each monster kind has a chance in 10,000 of leaving an amount from its least to its most, rolled once as it dies, after its items, and multiplied by a running event, rounded down ([events.md](events.md)).

  | Monster | Yang |
  |---|---|
  | Dziki Pies (101) | 5 to 12, 6 in 10 |
  | Wilk (102) | 8 to 18, 6 in 10 |
  | Alfa Wilk (103) | 10 to 25, 6 in 10 |
  | Dzik (108) | 20 to 40, 6 in 10 |
  | Nieb. Wilk (104) | 15 to 30, 6 in 10 |
  | Nieb. Alfa Wilk (105) | 20 to 40, 6 in 10 |
  | Czerw. Dzik (109) | 30 to 55, 6 in 10 |
  | Niedźwiedź (110) | 25 to 45, 6 in 10 |
  | Niedźw. Grizzly (111) | 40 to 70, 6 in 10 |
  | Tygrys (114) | 35 to 60, 6 in 10 |

  The amounts are this server's, below the client's mob_proto gold and rising with it: the tiger leaves less than the grizzly there too.

  A small red potion costs 32 in a shop, about six dogs.
- **Who it is kept for:**
  - The player who took the most of the monster's hit points, among those still in the world and alive, as experience counts its sharers ([experience.md](experience.md)); the killer on a tie. The drop goes whole to that one player, where experience is split, so a last blow takes nothing from whoever fought.
  - The owner is held by character id, so they keep it over a relog. The name is only the label.
- **Falling:** the world loop rolls the drops when it takes the death, at most a tick after it. Each item that falls:
  - gets a vid from the same counter as characters, so a vid always means one thing;
  - lies within 100 of the body;
  - is shown, with its owner's name, to the players in view of it.
- **Lying:**
  - **Kept apart from characters:** items on the ground are held in their own table under the world's lock, so the world record's entries keep their size ([decisions/world-loop.md](../decisions/world-loop.md)).
  - **No per-player memory:** they never move, so who sees one follows from where the players stand. A player is shown each item that comes into view as they enter or move, and told of each that leaves it as they move or leave.
  - **Timers:** every drop lives the same times, so the loop keeps two queues in the order drops fell, and looks only at their heads. At 20 s it frees an item: everyone near is told its name is gone. At 60 s it clears it away.
- **Picking up:**
  - A player may take an item lying within 300 of them on their map, if they are alive and it is theirs or free, and their bag has room for it. One test, under the world's lock, decides all of that and takes it off the ground, so of two players reaching for one, the first has it and the second finds nothing.
  - Room is a stack of its kind with space for the whole of it, or the first cells it fits.
  - The item is then written, as a new item or the stack's new count, and the bag shows it.
  - An item that cannot be written goes back on the ground, with its owner if their time isn't up, unless its minute is over, judged when the write gives up, which may be seconds after the click. A write that timed out after the database took it would leave the item both kept and on the ground; that's a known limit for now.
- **Yang picked up** follows the same rules of owner, reach and time, but needs no room: it is added to the character's yang. A character holds at most 2,000,000,000, a round cap under what the client's signed 32-bit points hold, which the database checks too: yang that would take them past it is left lying, and they are told nothing. Otherwise it is written and the player is told the amount and their new balance. Yang that cannot be kept goes back on the ground as an item does. Only the connection's own goroutine reads or changes a character's yang, as with its inventory, so no lock is needed: it is read with the inventory on entering and written as it changes.
- **Dropping,** on the connection's goroutine:
  - an item in a bag cell may be dropped, all of its stack or part, by a living player; a worn item, an empty cell, none or more than the stack holds is refused without a word, and the client still shows it where it was;
  - yang may be dropped from 1 to what the player has;
  - it is written first, the item forgotten or its stack lowered, or the yang spent, and the player told; the taskbar follows ([taskbar.md](taskbar.md)). A write that fails changes nothing and lays nothing down; once written, it is laid down even if telling the player failed, so nothing written is lost;
  - then it is laid within 100 of the player with no owner, so anyone may pick it up at once, as a player hands something to a friend beside them, and shown to everyone who sees it. The world loop takes what players dropped with the rest of its news, one lock a tick, and clears each away a minute after it was dropped. A player's drop is timed from when their connection laid it, which can fall after the time of the tick it reaches the loop in, so each is put into the queue in order of when it is due, as a monster's drop is.
- **Nothing on the ground is stored:** a crash loses at most a minute of loot, and what players dropped in that minute.

## How we know

- **The client's side:** its source named above, and a throwaway test in the real client.
  - **Confirmed live:** two potions laid down on entering lay on the grass with «Kira's» and «Szepta's» over them. Once the first was freed, a click walked Szepta to it and sent PICKUP, and taking it away removed it.
  - **From the source only:** that the client refuses another player's item before it is freed.
- **Yang, by bot as Szepta** against the running server: three of six kills left yang, shown as kind 1 with her name, and picking each up told her +7, +6 and +5, a balance of 18, which she entered with again. During an event of yang ×2 the notice named only yang, and two piles brought 22 and 16. Yang's look and label, and the client's line «Otrzymałeś N Yang.», come from its source and have not been seen in the real client.
- **Dropping, by bots:** Szepta dropped one of her 52 red potions and 5 of her 30 yang; each lay beside her with no name, and Botka beside her picked up both at once, the potion into its bag and the yang to a balance of 5. The client's question before a drop, and the items on the grass, have not been seen in the real client.
- **The rules** are this server's own: open-mt2 gives a drop to the killer and keeps it 15 s of 30, and the original weighs drops by monster rank and level gap.

## What proves it

- `internal/mob/mob_test.go`: every drop an item the game has, in a count it holds; the wolves and boars, blue and red ones too, leaving the materials of upgrades; skill books from monsters of level 15 or more; the rare gear every +0 kind of level 21 or more, from the cursed monsters only, once in 200.
- `internal/item/item_test.go`: an item picked up joining its stack or taking a free cell.
- `internal/protocol/protocol_test.go`: GROUND_ADD, GROUND_DEL and OWNERSHIP byte for byte; PICKUP cut and read as the real client sent it.
- `internal/store/items_test.go`: a new item and a stack's count kept.
- `internal/game/drops_test.go`:
  - the top damage dealer owning a drop; the killer on a tie, or when the other has died or left;
  - a death leaving what its drops roll, shown to a player beside it and not to one far off;
  - an item coming into and going out of view as a player moves; entering showing it, and leaving taking it away;
  - an item freed at 20 s and cleared at 60 s;
  - only its owner picking it up until it is freed, anyone after; nobody too far, dead, or twice;
  - no room leaving it on the ground;
  - a potion joining a stack, or taking a new cell when the stack is full;
  - no room told; an item that cannot be kept put back, freed if its owner's time ran out meanwhile, and not at all once its minute is over;
  - an item picked up before its time, not cleared twice.
- `internal/mob/mob_test.go`: every monster's yang a chance of a range, and only monsters leaving any.
- `internal/store/items_test.go`: yang added and the balance returned, and more than a character may hold refused.
- `internal/game/yang_test.go`: yang falling near the body, shown as kind 1 with its owner; rolled within its range, and none without a chance; multiplied by an event; picked up into her yang with her bag full; left lying past what she may hold; put back when it cannot be kept.
- `internal/server/experience_test.go`: entering with the yang kept.
- `internal/game/playerdrop_test.go`: a stack dropped beside her for anyone, shown to a neighbour, its taskbar slot emptied; part of a stack leaving the rest; a worn item, an empty cell, none, too many, and a dead player refused; a write that fails laying nothing; yang dropped spent and lying as yang, and more than she has or none refused; cleared a minute after, and a player's drop and a monster's in one tick each cleared when due; picked up by another at once.
- `internal/server/playerdrop_test.go` (real sockets and database): a potion Hendra dropped picked up by Kira, each keeping one.
- `internal/server/items_test.go`: a pick-up of nothing ignored, the connection kept.
- `internal/game/world_bench_test.go`: a player's move with drops lying about.

## Not done yet

- Events raising drop chances.
- Parties sharing loot.
- Drops with a range of counts.
- Splitting a stack across cells.
- A limit on how often or how little a player drops, for a public server: a modified client could fill the ground with single potions or 1 yang at a time.
