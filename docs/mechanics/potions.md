# Potions

## What the player sees

A double-click on a red potion in the bag, or its key on the taskbar ([taskbar.md](taskbar.md)), heals the character, and one on a blue potion restores mana. A red swirl, or a blue one, rises around the character for them and everyone near, with the potion sound for the one who drank. The stack shows one fewer, and the last one empties its cell. It works in the middle of a fight, which is what potions are for.

A potion that would do nothing isn't used up: a red one at full hit points, a blue one at full mana, or any potion once the character is dead.

## What the client does

- **A double-click** in the bag sends ITEM_USE with the cell (`uiinventory.py`). The client checks nothing about a potion and has no wait between uses, so the server decides everything.
- **The swirl and the sound** come from SPECIAL_EFFECT. The client draws the effect on the vid it names, and plays the potion sound only when that is the player's own character (`PythonNetworkStreamPhaseGameItem.cpp`). The click itself makes no sound for a potion (`PythonItem.cpp`).
- **What a potion restores** is in the client's `item_proto`: VALUE0 is the hit points and VALUE1 the mana.

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| ITEM_USE `0B`, client | 4 | position: window u8, cell u16 |
| ITEM_SET `14` | 42 | as on [items.md](items.md): the stack's new count, or vnum 0 for an emptied cell |
| POINT_CHANGE `11` | 17 | HP is point 5, mana point 7: the amount gained and the new value |
| SPECIAL_EFFECT `72` | 6 | effect u8 (1 red potion, 2 blue potion; 5 and 6 a blow's, [bonuses.md](bonuses.md)), vid u32 |

## What the server does

`internal/item` says what each kind restores; `internal/game/potions.go` drinks it.

- **The potions:**

  | Kind | Hit points | Mana |
  |---|---|---|
  | Czerwona Mikstura(M) (27001) | 300 | |
  | Czerwona Mikstura(Ś) (27002) | 800 | |
  | Niebieska Mikstura(M) (27004) | | 100 |

  A kind that restores either is drunk; there's no item type beyond that. A level-1 sura has about 1,050 hit points, so a small red potion is about a third of them.
- **Drinking,** on a double-click on one in the bag:
  1. **Whether it would help** is judged under the world's lock. The player must be alive and short of something the potion restores. Otherwise nothing happens and nothing is sent.
  2. **The stack is written first:** one fewer, or the item forgotten with its last potion. If the write fails, nothing is healed, and the client is shown the stack as it was.
  3. **The bag is updated,** and the client is shown the new count or the emptied cell.
  4. **The player is healed** under the lock, up to their maximum. They are told their new hit points or mana, and they and everyone who sees them get the effect. Hit points are saved as always: by the periodic snapshot and on leaving.

  The write takes a moment between steps 1 and 4. A player who dies in it loses that potion and stays dead: a potion never raises the dead. One whom regaining over time fills in it loses the potion too, and is shown no effect.
- **No waiting:** potions work in a fight, where regaining hit points over time stops, and there's no cooldown beyond how fast a player clicks.

## How we know

- **The client's side:** its source and scripts named above. The potions' values come from the client's `--dump_proto--/item_proto.txt`.
- **Seen from a bot** against the running server: Szepta at 469 hit points drank a red potion and went to 769, with the effect on her and her stack one fewer. A blue one at full mana did nothing. After two dogs, a red potion took her from 785 to her maximum of 1,050, a third at full did nothing, and 51 of 53 were kept.

## What proves it

- `internal/item/item_test.go`: the potions restore what `item_proto` says, and nothing else restores anything.
- `internal/protocol/protocol_test.go`: SPECIAL_EFFECT byte for byte.
- `internal/store/items_test.go`: an item removed is forgotten, and the others are kept.
- `internal/game/potions_test.go`:
  - a red potion healing 300, one fewer kept and shown, the effect seen by the player and a neighbour;
  - healing no further than the maximum;
  - a blue potion restoring mana, with its own effect;
  - the last potion emptying its cell;
  - a potion drunk on a closed connection ending the session and healing nothing;
  - a potion not used up at full hit points, at full mana, or by the dead;
  - a failed write healing nothing;
  - drinking in a fight;
  - no healing for one who died meanwhile, and no effect for one filled meanwhile.
- `internal/server/potions_test.go` (real sockets and database): a hurt player drinking one of two potions, and entering again after Change character with the hit points and the one potion kept.

## Not done yet

- Potions that heal over time, automatic potions, and other usable items.
