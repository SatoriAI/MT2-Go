# Chat, and leaving the world

## What the player sees

In the world, the player types a line and it appears in their chat window as "Name : text". Nobody else hears it yet: there is nobody else in the world. The game menu's **Change character**, **Logout** and **Exit** work, and the character comes back where it stood.

## What the client does

- **A typed line** is sent as `CHAT`. Before sending, the client strips the prefixes `#` (party), `%` (guild) and `!` (shout) into the chat type (`uichat.py`). Its input box takes up to 512 characters.
- **Commands** are lines starting with `/`, sent as ordinary chat. Many buttons send them without the player typing, such as `/pkmode` and `/setblockmode` (`uigameoption.py`), `/skillup` (`uicharacter.py`) and `/ride` (`game.py`).
- **The system menu** (`uisystem.py`) calls the client's own functions for change character, logout and exit. Those send `/phase_select`, `/logout` and `/quit` as chat. That isn't visible in the scripts; open-mt2 registers exactly these three, and the real client's behaviour settles it.
- **The text** is in the client's single-byte code page, cp1250 for Polish, not UTF-8.

## The bytes

### Packets with their own length

Most client packets have one size per header. Some vary, chat among them. Right after the header they carry a u16 **total size**, which counts the header, the size field and the body but not the trailing sequence byte. The server reads the size before it knows where the packet ends. It refuses a size below 3 or above 1024: the chat box allows 512, and the bound stops a lying length from making the server wait for a megabyte.

### CHAT — client, `03`, sized, sequenced

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `03` |
| 1 | u16 size | the total, as above |
| 3 | u8 type | `0` normal, `1` info, `3` party, `4` guild, `5` command, `6` shout (open-mt2's numbers) |
| 4 | text, then `00` | in the client's code page |

### CHAT — server, `04`, sized

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `04` |
| 1 | u16 size | the total |
| 3 | u8 type | |
| 4 | u32 vid | whose line it is; the client drops a line whose vid is a character it does not know |
| 8 | u8 kingdom | |
| 9 | text, then `00` | the whole line, already as "Name : text" |

### Phase close — server, `FD 00`

The phase packet with `0`: the connection is ending.

## What the server does

`internal/game/chat.go`, in the game phase only. Chat anywhere else closes the connection, like any packet out of its phase.

- **A line** goes back to its speaker as "Name : text", type normal, with their vid and kingdom. The bytes are kept as the client sent them, so Polish letters survive; open-mt2 decodes them as ASCII and garbles them. Party, guild and shout are answered as normal for now.
- **`/quit`:** the server replies with chat of type command and the text `quit`, which tells the client to close itself.
- **`/logout`:** phase close, then the connection ends.
- **`/phase_select`:** the character leaves the world (saved, and its claim released), then the server sends the kingdom, the character list and phase select, as at login. Start works again on the same connection.
- **Every other command** is logged and ignored.
- **When they act:** the original game counts down before leaving, so nobody escapes a fight. With no fighting yet, these act at once.
- **Saving:** in every case the position is saved, through the same path as a dropped connection.

## How we know

- **The client side:** the client's `uichat.py`, `uisystem.py`, `uigameoption.py` and `game.py`.
- **The layouts, type numbers and leave commands:** open-mt2. The three leave commands and what the client expects back for each are only in the client's compiled code; this page says what the real client did once that has been tried.

## What proves it

- `internal/protocol/protocol_test.go`:
  - a sized packet read by its size, split across reads, and refused with an impossible size;
  - chat in decoded with its bytes kept, and refused without its terminator;
  - chat out, byte for byte.
- `internal/server/server_test.go`:
  - a line coming back to its speaker;
  - Polish bytes unchanged;
  - an unknown command ignored;
  - `/quit` answered;
  - `/logout` closing and keeping the position;
  - `/phase_select` returning to the character screen with the list showing where the character stood, and Start working again;
  - chat before entering the world closing the connection.

## Not done yet

- Others hearing a line, and whispers.
- The leave countdown.
- A limit on how fast a player may talk.
- The GM and insult filters.
