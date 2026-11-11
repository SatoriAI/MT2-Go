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
| 3 | u8 type | 0 a player's line, 2 a notice from the server ([events.md](events.md)), 5 a command to the client |
| 4 | u32 vid | whose line it is; the client drops a line whose vid is a character it does not know |
| 8 | u8 kingdom | |
| 9 | text, then `00` | the whole line, already as "Name : text" |

### Phase close — server, `FD 00`

The phase packet with `0`: the connection is ending.

## What the server does

`internal/game/chat.go`, in the game phase only. Chat anywhere else closes the connection, like any packet out of its phase.

- **A line** goes back to its speaker, and to everyone in their view, as "Name : text", type normal, with their vid and kingdom. The client doesn't draw its own line itself: with no reply, nothing appears. With the speaker's vid, the line appears once and in a bubble over the speaker. With vid 0, it appears only in the chat window. The bytes are kept as the client sent them, so Polish letters survive; open-mt2 decodes them as ASCII and garbles them. Party, guild and shout are answered as normal for now. An empty line gets no answer.
- **A line of 512 bytes or more** is not the client's, which refuses to send one (`PythonNetworkStreamPhaseGame.cpp`, `SendChatPacket`), and ends the connection, as an over-long whisper does. The bound keeps a modified client from pushing lines into others that the original can't produce; "Name : " plus 511 bytes always fits the 1024-byte buffer a client receives a line into (`RecvChatPacket`).
- **Talking too fast:** a player may say 10 lines, chat and whispers together, in any 5 seconds (`internal/game/flood.go`); a line over that is dropped without a reply and logged at debug level. The numbers are open-mt2's: no one types that fast, and the spam tools that send dozens of lines a second are stopped. Dropping rather than disconnecting doesn't punish a burst of real typing, and a spam tool gains nothing either way. A refused line isn't counted, so a flood doesn't keep its own block going. Commands aren't counted, so leaving always works; the one exception is `/setblockmode`, which writes to the database, so a spam tool could otherwise turn it into thousands of writes a second. A change dropped leaves the buttons as they were. The count belongs to the connection, so Change character followed by Start begins a fresh one; that takes a full reload of the world, which is no way to flood.
- **`/quit`:** the server replies with chat of type command and the text `quit`, which tells the client to close itself. With the real client, it closed.
- **`/logout`:** phase close, then the connection ends. With the real client, it went back to the login screen.
- **`/phase_select`:** the character leaves the world (saved, and its claim released), then the server sends phase select **first**, and the character list after it. The client, still in its game window, ignores a kingdom and list sent ahead of the phase: tried first, it stayed in the game and went on sending moves. Once in the select phase it takes a list as it does at login and replaces the one it kept (`PythonNetworkStreamPhaseSelect.cpp`, `__RecvLoginSuccessPacket4`); the character screen, opening after the fade, shows it (`introselect.py`, `Open`). A list that cannot be read ends the connection, logged, as it does at login. The list is read after the character is saved, so the screen shows its level and stats as they are now, not as they were at login. When the player then presses Start, it reconnects and logs in with its key again, getting a fresh list. Moves, chat and whispers the client sent before it took in the phase are dropped on the old connection until the next Start, rather than ending it as out of their phase.
- **`/bonusy`:** the bonuses of everything worn, summed and capped, as info lines named as the tooltip names each, or a line saying nothing worn has any ([bonuses.md](bonuses.md)).
- **`/setblockmode N`:** what the options window blocks. N is the client's whole mask after the button pressed: exchange 1, party 2, guild 4, whisper 8, friend 16, party request 32 (`Packet.h`, `BLOCK_*`). The server keeps it per character, by name, in `characters.blocks`, dropping bits the client doesn't name, and replies with a command chat `setblockmode N`. The client shows the buttons pressed only from that reply (`PythonNetworkStreamCommand.cpp`, `OnBlockMode`). The same reply goes out on entering the world, so the choice survives logging out, which the client alone doesn't keep. A mask that isn't a number is ignored. Whispers ([whispers.md](whispers.md)) and trades ([trading.md](trading.md)) are refused; the other blocks wait for their mechanics. open-mt2 keeps the mask in memory only and enforces none of it.
- **`/getitem VNUM [ilość]` and `/getyang ilość`,** for testing, only while `DEV_COMMANDS` is true; it is false unless set, and then these are ignored like any unknown command.
  - **`/getitem`** puts the kind into the bag where a pick-up would: onto a stack of it with room, or the first free cells. The count is 1 if not given, and a larger one than a stack holds gives a full stack. It answers with an info line for a kind the server doesn't know («Serwer nie zna przedmiotu N.»), a full bag, or a line it can't read («Użycie: /getitem VNUM [ilość]»).
  - **`/getyang`** adds the yang, which the client then announces itself. A sum that would take the character past the 2,000,000,000 it may hold is refused whole («Tyle się nie zmieści: postać może mieć najwyżej 2000000000 Yang.»), and a line it can't read is answered «Użycie: /getyang ilość».
  - Every item and every sum given is logged with the character's name.
  - There are no GM accounts yet. Dawid is the only tester, so the switch stands in for them, and must never be on where others play.
- **Every other command** is logged and ignored.
- **When they act:** the original game counts down before leaving, so nobody escapes a fight. With no fighting yet, these act at once.
- **Saving:** in every case the position is saved, through the same path as a dropped connection.

## How we know

- **The client side:** the client's `uichat.py`, `uisystem.py`, `uigameoption.py` and `game.py`.
- **The layouts, type numbers and leave commands:** open-mt2.
- **The flood limit:** open-mt2's numbers and its reasoning for dropping silently. The original instead scores banned words per address, which needs a word list. Checked against the running server with two scripted players: of 20 lines sent at once, the other player got the first 10, a whisper right after them got nothing back and was not stored, and a line six seconds later went through. What the client expects back for each leave command is only in the client's compiled code. The behaviour above was seen with the real client.

## What proves it

- `internal/protocol/protocol_test.go`:
  - a sized packet read by its size, split across reads, and refused with an impossible size;
  - chat in decoded with its bytes kept, and refused without its terminator;
  - chat out, byte for byte.
- `internal/server/server_test.go`:
  - a line coming back to its speaker, and an empty one getting no answer;
  - 511 bytes passed on, 512 ending the connection;
  - Polish bytes unchanged;
  - an unknown command ignored;
  - `/quit` answered;
  - `/logout` closing and keeping the position;
  - `/phase_select` answered with the phase and then the list, and Start on a new connection then loading the character where it was left;
  - moves, chat and whispers in flight after `/phase_select` ignored;
  - chat before entering the world held while incomplete, which shows it was framed, then closing the connection;
  - an eleventh line within five seconds reaching nobody, with the speaker kept in the game, and a command still working after it.
- `internal/server/blocks_test.go`: `/setblockmode` answered and kept for the next entry, where it still refuses whispers; unknown bits dropped and a non-number ignored; a change over the flood limit dropped and not kept.
- `internal/game/wire_test.go`: each block's bit is the client's and leads back, and no blocks is an empty list, not NULL.
- `internal/game/flood_test.go`: ten lines passing and the eleventh not, the window rolling at exactly five seconds, and refused lines not counted.
- `internal/game/devcommands_test.go`:
  - `/getitem` putting the kind in the bag, kept and shown;
  - a count joining a stack, and stopping at a full one;
  - an unknown kind, an unreadable line and a full bag answered, with nothing given;
  - `/getyang` adding to the yang, up to exactly the most a character may hold, and refusing a sum past it;
  - both ignored with the switch off, and the switch passed on from what the game port is made with.
- `internal/config/config_test.go`: `DEV_COMMANDS` off unless true, and a value neither true nor false refused.
- `internal/game/session_test.go`: the error for a packet out of its phase shows at most 64 bytes of its body, and none of a login packet's.

## Not done yet

- The leave countdown.
- GM accounts, which would let `/getitem` and `/getyang` be given to a few players rather than to all of them by a switch.
- A server-side insult filter. The client already refuses lines with banned words before sending, in chat (`uichat.py`) and whispers (`uiwhisper.py`); the original's own filter needs a banned-word list we don't have.
