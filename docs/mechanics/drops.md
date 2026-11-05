# Drops

## What the player sees

A monster may leave something when it dies: a potion now and then, and from the stronger ones, rarely, a first weapon or armour. Each item lies on the grass beside the body with the name of whoever it's kept for over it in yellow, «Szepta's». That's the player who took the most of the monster's hit points, not whoever landed the last blow. For 20 seconds only they can pick it up; then the name goes and anyone may; after a minute it's gone.

Clicking an item walks the character to it and picks it up, and the pick-up key takes the nearest one. A potion joins the stack of its kind in the bag when the whole of it fits there; anything else takes the first free cells. With no room, the chat says «Nie masz miejsca w ekwipunku.» and the item stays where it lies.

## What the client does

- **It lays an item on the terrain itself,** at the height it finds there, so the height sent is ignored (`PythonItem.cpp`). The item's look and name come from its own `item_proto`; nothing on the ground shows a count.
- **It shows the owner's name** over an item it has, and ignores a name for one it doesn't, so the name follows the item (`PythonItem.cpp`, `PythonTextTail.cpp`).
- **It refuses another player's item by itself:** it says it cannot pick it up and sends nothing (`PythonPlayer.cpp`). So when an item is freed, everyone near must be told, or their clients keep refusing.
- **Picking up:**
  - A click walks to the item, then sends PICKUP. The pick-up key takes the nearest item within 300 without walking (`PythonPlayerInput.cpp`).
  - The client sends at most one pick-up every half second.
- **Laying an item down twice** leaves a copy the client never removes, while taking away an item it doesn't have does nothing. So the server takes items away when a player leaves, and shows them again when they come back.

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| GROUND_ADD `1A` | 21 | x i32, y i32, height i32 (0), vid u32, vnum u32 |
| GROUND_DEL `1B` | 5 | vid u32 |
| OWNERSHIP `1F` | 30 | vid u32, name char[25]; empty for anyone |
| PICKUP `0F`, client | 5, then the sequence byte | vid u32 |

Seen from the real client: a red potion laid at (977491, 272599) is `1a 53ea0e00 d7280400 00000000 a1bb0d00 79690000`, and a click on vid 900002 sends `0f a2bb0d00`.

## What the server does

`internal/mob` says what each monster may leave; `internal/game/drops.go` lays drops down, times them, and gives them up.

- **What drops:** each monster kind has a list of drops, each an item kind, a count, and a chance in 10,000, rolled on its own when it dies. There are no rank tables or level gaps; a gate test checks every drop is an item the game has, in a count one item holds.

  | Monster | Drops |
  |---|---|
  | Dziki Pies (101), Wilk (102) | red potion, 8 in 100; blue potion, 8 in 100 |
  | Alfa Wilk (103), Dzik (108) | the same, and each class's first weapon and armour, 2 in 1,000 each: about 2 in 100 for any one |

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
- **Nothing on the ground is stored:** a crash loses at most a minute of loot.

## How we know

- **The client's side:** its source named above, and a throwaway test in the real client.
  - **Confirmed live:** two potions laid down on entering lay on the grass with «Kira's» and «Szepta's» over them. Once the first was freed, a click walked Szepta to it and sent PICKUP, and taking it away removed it.
  - **From the source only:** that the client refuses another player's item before it is freed.
- **The rules** are this server's own: open-mt2 gives a drop to the killer and keeps it 15 s of 30, and the original weighs drops by monster rank and level gap.

## What proves it

- `internal/mob/mob_test.go`: every drop an item the game has, in a count it holds.
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
- `internal/server/items_test.go`: a pick-up of nothing ignored, the connection kept.
- `internal/game/world_bench_test.go`: a player's move with drops lying about.

## Not done yet

- Yang, which comes with shops.
- Events raising drop chances.
- Parties sharing loot.
- Players dropping their own items.
- Drops with a range of counts.
- Splitting a stack across cells.
- Ground items kept by map, if there are ever thousands.
