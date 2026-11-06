# Taskbar

## What the player sees

The bar at the foot of the screen has eight slots, for keys 1 to 4 and F1 to F4, and the client pages it four times over. Dragging potions from the bag onto a slot puts them there, with their count. Pressing its key drinks one, as a double-click in the bag does ([potions.md](potions.md)). A skill learnt can go on the bar the same way, dragged from the skill window, and its key uses it ([skills.md](skills.md)).
- **Dragging a slot onto another** swaps them, or moves it to an empty one. Dragging it off the bar empties it.
- **The slot holds the potions themselves, not their cell.** Moving the stack in the bag takes the slot with it. Drinking or selling the last one empties it.
- **A skill's slot goes with the skill:** changing skill path, which forgets every skill, empties its slots.
- **The bar is kept between logins.**

## What the client does

- **It keeps 36 slots** (`QUICKSLOT_MAX_NUM`): four pages of eight that it shows, and four more (`Packet.h`, `uitaskbar.py`). On each page, keys 1 to 4 are its first four slots and F1 to F4 its last four (`game.py`).
- **Each slot holds a type and a position.** Type 1 is an inventory cell. Type 2 is a skill, by its place in the skill window: the sura's Weaponry skills are 1 to 6 in order, so Finger Strike is 1, Dragon Swirl 2, Enchanted Blade 3 and Enchanted Armour 5 (`playersettingmodule.py`). A skill dragged from the window carries its place with the rank taken off, modulo 20, as the window draws each rank 20 places on (`uicharacter.py`, `SelectSkill`, `__RealSkillSlotToSourceSlot`). Emotions are 3.
- **Pressing a skill's key** uses the skill at that place, sending USE_SKILL as the skill window does (`RequestUseLocalQuickSlot` in `PythonPlayer.cpp`, then `ClickSkillSlot` in `PythonPlayerSkill.cpp`).
- **It changes its bar only when the server says so.** A drag sends QUICKSLOT_ADD, DEL or SWAP, and the slot changes when the same packet comes back (`PythonNetworkStreamPhaseGameItem.cpp`). It takes ADD while loading too.
- **What it lets onto the bar:** from the bag, only usable and quest items (`CanAddToQuickSlotItem`, `ITEM_TYPE_USE` and `ITEM_TYPE_QUEST`), of which this server has only the potions; emotions it offers without that check (`uitaskbar.py`).
- **Pressing a slot's key** sends an ordinary ITEM_USE for its cell (`RequestUseLocalQuickSlot`).

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| QUICKSLOT_ADD `10`, client | 4, then the sequence byte | slot u8, type u8, position u8 |
| QUICKSLOT_DEL `11`, client | 2, then the sequence byte | slot u8 |
| QUICKSLOT_SWAP `12`, client | 3, then the sequence byte | slot u8, other slot u8 |
| QUICKSLOT_ADD `1C` | 4 | slot, type (1, an inventory cell), the cell |
| QUICKSLOT_DEL `1D` | 2 | slot |
| QUICKSLOT_SWAP `1E` | 3 | slot, other slot |

## What the server does

`internal/game/taskbar.go` answers the client, and `internal/store/taskbar.go` keeps the slots.
- **A slot holds an item, by its id, or a skill, by name,** in the `taskbar` table, never both: the database checks it. On the wire an item is its cell now, a skill its place in the skill window (`internal/game/wire.go`).
- **A skill's slot refers to the skill learnt,** so the database lets the slot go when the skill is forgotten, on a change of skill group; the client is then told the slot is empty, after the group, its levels, the points and the teacher's last word. The client forgets the skills on a new group but not the bar's slots, which would otherwise show the new group's skill in their place.
  - When the item goes, the database lets the slot go with it.
  - The connection keeps each slot's item and the cell the client was last told, which only its goroutine touches, as with the inventory.
- **Filling:** a slot below 36 is filled with the item at a bag cell, if its kind is drunk. It replaces whatever the slot held, is kept, and is told back. Or it is filled with a skill the player has learnt, by its place in the skill window. An emotion, a skill not learnt or the game has not got, any other item or an empty cell is refused without a word.
- **Emptying and swapping** are kept, then told back. A swap is one transaction, and either slot may be empty.
- **Following the items:** after anything that changes the bag — a move, a potion drunk, a sale, a drop — each slot whose item is gone is emptied, and each whose item moved is told its new cell. A buy or a pick-up only raises a stack where it stands or adds a new item, so neither can move a slot's item. Anything later that moves or takes an item out of the bag, such as splitting a stack or trading, must follow it too.
- **Entering:** the slots are read with the inventory, and each is told after it.
- **A write that fails** changes nothing and tells the client nothing, so the bar stays as it was.

## How we know

- **The client's side:** its source and scripts named above.
- **Seen from a bot** against the running server:
  - Szepta's red potions went to slot 0 and her blue ones to slot 1, each told back;
  - the two were swapped;
  - moving the red stack to cell 20 told her its slot at cell 20;
  - after a relog, both slots came with entering.
- **Not yet seen in the real client:** the bar itself, and a key drinking from it.

## What proves it

- `internal/protocol/protocol_test.go`: the three packets read, and the three written byte for byte.
- `internal/store/taskbar_test.go`:
  - a slot set, replaced and cleared;
  - two swapped, and one moved onto an empty slot;
  - a slot gone with its item;
  - each character's slots their own.
- `internal/game/taskbar_test.go`:
  - potions dragged on filling a slot;
  - a sword, an empty cell, a skill and slot 36 refused;
  - slots emptied and swapped;
  - a slot following its potions as they move, and not told again when they stay;
  - emptied when the last is drunk or the stack is sold;
  - a failed write changing nothing.
- `internal/store/taskbar_test.go`: a skill's slot kept and swapped; gone when choosing a skill group forgets the skill; a slot of a skill not learnt, of nothing, or of both refused.
- `internal/game/taskbar_test.go`: a skill learnt filling a slot and one not learnt refused; a skill's slot left as the bag changes; a change of group emptying the skills' slots and keeping items'; a skill's slot shown at its place.
- `internal/server/skills_test.go`: Dragon Swirl on the first slot kept over Change character.
- `internal/server/taskbar_test.go` (real sockets and database): potions on the first slot, key 1, kept over Change character, told on entering after the inventory, and drunk by a use of their cell, as the key sends it.

## Not done yet

- Emotions on the bar.
- Tying a slot's item to its character in the database, for when items change hands: today a slot is only ever filled from its own character's bag.
