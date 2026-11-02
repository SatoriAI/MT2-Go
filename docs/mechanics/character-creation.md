# Character creation

## What the player sees

On the character screen, the player clicks an empty slot. If every slot is empty, the client first shows the kingdom screen again, so a new account can still change its mind. Then the create screen: the player picks a sex, a class, one of the class's two looks and a name, and confirms. After a short animation the character stands in its slot on the character screen. A taken name shows "name taken"; anything else refused shows a general failure, and the player can try again.

## What the client does

1. **Kingdom again.** Clicking an empty slot while every slot is empty opens the kingdom screen a second time (`introselect.py` `CreateCharacter`, `introempire.py` `ReselectEmpireWindow`), starting on a random kingdom, and sends `EMPIRE` with the choice. See [connection-and-login.md](connection-and-login.md) for that packet and the rule.
2. **Create.** On confirm the client waits 1.5 seconds for the animation, then sends `CREATE_CHARACTER` (`introcreate.py`).
3. **Reply.** On success it returns to the character screen, with the new character in its slot, filled from this reply (seen with the real client). On failure it shows the message for the reason and stays on the create screen.

The create screen limits the name to 12 characters. It also refuses, before sending, a name containing "GM" or a word from its insult list; the server does not repeat those two checks yet.

## The bytes

### CREATE_CHARACTER — client, `04`, 34 bytes, sequenced

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `04` |
| 1 | u8 slot | `0`–`3` |
| 2 | char[25] name | |
| 27 | u16 race | class and sex in one number; the table is on the connection page |
| 29 | u8 shape | the look: `0` or `1` |
| 30 | u8 ×4 stat points | CON, INT, STR, DEX on top of the class's start; always `0` from this client |

### Character created — server, `08`, 65 bytes

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `08` |
| 1 | u8 slot | |
| 2 | 63 bytes | the same record as one slot of the character list |

### Creation failed — server, `09`, 2 bytes

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `09` |
| 1 | u8 reason | `1` name taken; any other value is shown as a general failure. This server sends `0` |

## What the server does

`internal/game/session.go` handles `CREATE_CHARACTER` on the character screen (phase `SELECT`):

- **Kingdom first.** An account without a kingdom cannot create a character; the real client never asks, so the connection is closed.
- **The rules,** each refused with reason `0`:
  - the race is one of the eight;
  - the slot is `0`–`3`;
  - the shape is `0` or `1`;
  - the name is 2–12 ASCII letters or digits (`account.ValidCharacterName`). The 12 is the create screen's limit; the minimum of 2 and the character set are this server's own rule, since the client refuses only an empty name. The database checks the same pattern.
- **The new character** is level 1, with its class's starting stats and its kingdom's start position (`internal/account/start.go`). The stat points the client sends are ignored, so a modified client cannot buy stats.

  | Class | Vitality (CON) | Intelligence (INT) | Strength (STR) | Dexterity (DEX) |
  |---|---|---|---|---|
  | Warrior | 4 | 3 | 6 | 3 |
  | Ninja | 3 | 3 | 4 | 6 |
  | Sura | 3 | 5 | 5 | 3 |
  | Shaman | 4 | 6 | 3 | 3 |

  | Kingdom | x | y |
  |---|---|---|
  | Shinsoo | 475000 | 966100 |
  | Chunjo | 60000 | 156000 |
  | Jinno | 963400 | 278200 |

- **Storing** happens in one transaction holding the account row's lock (`store.Accounts.CreateCharacter`):
  - It inserts only if the kingdom is still the one the start position came from.
  - A name taken in any case is refused with reason `1`.
  - A taken slot is refused with reason `0`, so the screen shows a general failure. open-mt2 sends `1` there, which shows "name taken"; we chose not to say something untrue.
  - A kingdom changed meanwhile by another connection of the same account closes this connection: its screens are out of date, and logging in again shows the account as it is.
  - `ChooseEmpire` takes the same lock, so a kingdom change and a creation cannot interleave. Each one sees the other's result.
- **The reply** is `08` with the new character's record, its look sent as the body part. The map is not stored: positions are world coordinates, and the client's `atlasinfo.txt` gives each map's area.

## How we know

- **Layouts and failure reasons:** open-mt2's create packets, and the client's `introcreate.py` (what it sends, and which reason shows "name taken").
- **Starting stats:** the client's `START_STAT` table in `introcreate.py`, which is what the create screen shows. open-mt2 uses much larger values that disagree with the client.
- **Start positions:** open-mt2's `empire.json`. Each lies inside its kingdom's first village in the client's `atlasinfo.txt` (`metin2_map_a1`, `b1`, `c1`).
- **The 12-character limit:** the create window's input limit in the client's UI script.

## What proves it

- `internal/protocol/protocol_test.go`:
  - `CREATE_CHARACTER` decoded field by field;
  - the created reply as the list's record;
  - the failure reply.
- `internal/server/server_test.go`:
  - a character created and listed byte for byte on the next login, with its look stored;
  - a name taken in another case;
  - each rule refused with the general failure;
  - a taken slot;
  - the kingdom fixed once a character exists;
  - creation without a kingdom;
  - a kingdom changed by another connection of the same account, which closes the connection and stores nothing.
- `internal/store/store_test.go`: a kingdom change waiting on a creation is refused once it runs, and a creation waiting on a kingdom change is refused too.
- `internal/account/start_test.go`: every class's starting stats and every kingdom's start position, against the sources above.

## Not done yet

- Deleting a character. The original asks for a 7-character code (`05`); that policy is too weak and is deliberately not adopted, and deletion will get a rule of its own, in our own client.
- The "GM" and insult name checks.
- Hit points, mana and the other derived values.
