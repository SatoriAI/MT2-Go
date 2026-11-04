# Chat, and leaving the world

## What the player sees

In the world, the player types a line and it appears once in their chat window as "Name : text", and in a bubble above their character's head. Everyone near the speaker sees it too (see [other-players.md](other-players.md)); a line for one player is a whisper ([whispers.md](whispers.md)). The game menu's **Change character**, **Logout** and **Exit** work, and the character comes back where it stood.

## What the client does

- **A typed line** is sent as `CHAT`. Before sending, the client strips the prefixes `#` (party), `%` (guild) and `!` (shout) into the chat type (`uichat.py`). Its input box takes up to 512 characters.
- **Commands** are lines starting with `/`, sent as ordinary chat. Many buttons send them without the player typing, such as `/pkmode` and `/setblockmode` (`uigameoption.py`), `/skillup` (`uicharacter.py`) and `/ride` (`game.py`).
- **The system menu** (`uisystem.py`) calls the client's own functions for change character, logout and exit. With the real client, change character sent `/phase_select` and exit sent `/quit` as chat. With the real client, Logout ended the connection and the client showed the login screen. The server logged nothing, as it does when it hangs up on purpose. The command was not logged, so that it was `/logout`, as open-mt2 registers, is inferred.
- **The text** is in the client's single-byte code page, cp1250 for Polish, not UTF-8. With the real client, "żółta gęś" arrived as `bf f3 b3 74 61 20 67 ea 9c`. Under Wine this needs the prefix's code page set to Polish. With the Western one, the client turned ą into a and ż into z before sending.

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

- **A line** goes back to its speaker, and to everyone in their view, as "Name : text", type normal, with their vid and kingdom. The client doesn't draw its own line itself: with no reply, nothing appears. With the speaker's vid, the line appears once and in a bubble over the speaker. With vid 0, it appears only in the chat window. The bytes are kept as the client sent them, so Polish letters survive; open-mt2 decodes them as ASCII and garbles them. Party, guild and shout are answered as normal for now. An empty line gets no answer.
- **`/quit`:** the server replies with chat of type command and the text `quit`, which tells the client to close itself. With the real client, it closed.
- **`/logout`:** phase close, then the connection ends. With the real client, it went back to the login screen.
- **`/phase_select`:** the character leaves the world (saved, and its claim released), then the server sends phase select **alone**. The client, still in its game window, ignores a kingdom and list sent ahead of the phase: tried first, it stayed in the game and went on sending moves. With the phase alone it shows the character screen. When the player then presses Start, it reconnects and logs in with its key again, getting a fresh list. Moves, chat and whispers the client sent before it took in the phase are dropped on the old connection until the next Start, rather than ending it as out of their phase.
- **Every other command** is logged and ignored.
- **When they act:** the original game counts down before leaving, so nobody escapes a fight. With no fighting yet, these act at once.
- **Saving:** in every case the position is saved, through the same path as a dropped connection.

## How we know

- **The client side:** the client's `uichat.py`, `uisystem.py`, `uigameoption.py` and `game.py`.
- **The layouts, type numbers and leave commands:** open-mt2. What the client expects back for each leave command is only in the client's compiled code. The behaviour above was seen with the real client.

## What proves it

- `internal/protocol/protocol_test.go`:
  - a sized packet read by its size, split across reads, and refused with an impossible size;
  - chat in decoded with its bytes kept, and refused without its terminator;
  - chat out, byte for byte.
- `internal/server/server_test.go`:
  - a line coming back to its speaker, and an empty one getting no answer;
  - Polish bytes unchanged;
  - an unknown command ignored;
  - `/quit` answered;
  - `/logout` closing and keeping the position;
  - `/phase_select` answered with the phase alone, and Start on a new connection then loading the character where it was left;
  - moves, chat and whispers in flight after `/phase_select` ignored;
  - chat before entering the world held while incomplete, which shows it was framed, then closing the connection.
- `internal/game/session_test.go`: the error for a packet out of its phase shows at most 64 bytes of its body, and none of a login packet's.

## Not done yet

- The leave countdown.
- A limit on how fast a player may talk.
- The GM and insult filters.
