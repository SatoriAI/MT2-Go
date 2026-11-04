# Whispers

## What the player sees

A whisper is a line for one player, by name, in a window of its own rather than the shared chat. It reaches them wherever they are in the game, on any map and at any distance. If nobody of that name is in the game, the sender's window says so. A player on the character screen counts as not in the game.

The original client keeps its whisper windows only until logout. The server keeps every whisper delivered, so a client can show a conversation after logging in again; the original client never asks, so it behaves as before.

## What the client does

- **Sending:** the whisper window sends `WHISPER` with the name and the line (`uiwhisper.py`). It refuses a line of 255 bytes or more. It shows the sender's own line itself as it sends it, as "Name : text", so the server sends nothing back for a line delivered.
- **Receiving:** a whisper of type line opens or updates the window for the player it names, as "Name : text". Type not-online shows "not logged on" with the name (`localeInfo.WHISPER_ERROR`). Type error shows the packet's text as it is (`PythonNetworkStreamPhaseGame.cpp`, `RecvWhisperPacket`).
- **The text** is in the client's code page, cp1250 for Polish, as in chat ([chat.md](chat.md)).

## The bytes

### WHISPER — client, `13`, sized, sequenced

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `13` |
| 1 | u16 size | the total, as for chat |
| 3 | char[25] name | the recipient, as typed; the client copies at most 24 bytes and leaves the 25th as its memory held, so the server reads 24 |
| 28 | text, then `00` | in the client's code page |

### WHISPER — server, `22`, sized

| Offset | Field | |
|---|---|---|
| 0 | u8 header | `22` |
| 1 | u16 size | the total |
| 3 | u8 type | `0` a line, `1` not online, `2` and `3` blocked, `4` error, `5` from a GM, `FF` system |
| 4 | char[25] name | the other player: who sent the line, or whom the sender tried to reach |
| 29 | text, then `00` | |

## What the server does

`internal/game/whisper.go`, in the game phase only; whispers in flight after Change character are dropped, as chat is.

- **Finding the recipient:** players in the world are kept by name, ignoring case as names are unique regardless of it (`internal/game/presence.go`). The lookup and the send happen under the world's lock, so a player leaving is either sent the line or not found.
- **A line delivered** goes to the recipient as type line with the sender's name, the bytes as the client sent them. Nothing goes back to the sender.
- **Nobody of that name in the game:** the sender gets type not-online with the name as they typed it, and nothing is kept.
- **An empty line** is ignored, and so is **a line to oneself**: the client has already shown it, and sending it back would show it twice. **A line over 254 bytes** is not the client's, and ends the connection.
- **Talking too fast:** whispers count against the same limit as chat, 10 lines in any 5 seconds ([chat.md](chat.md)). A whisper over it gets no reply at all, not even "not online", so a flood can't be turned into replies.
- **Keeping it:** a delivered whisper is stored in `whispers`, with the two characters' ids, the time, and the text decoded from cp1250 to UTF-8 (`internal/game/wire.go`), so the table reads plainly and a client of our own needs no knowledge of the code page. Bytes cp1250 leaves undefined become U+FFFD. The save has its own few seconds rather than the connection's, so a line delivered as the server stops is still kept. Whispers are kept with no limit for now.
- **A whisper that could not be stored** has already reached its recipient. The error is logged, the sender gets type error with "Not saved.", and the connection stays up.

## How we know

- **The layouts, the type numbers and what the client shows:** the client's own source, in `open-mt2/work/client-review/ClientVS22/source` (`Packet.h`, `PythonChat.h`, `PythonNetworkStreamPhaseGame.cpp`), and its `uiwhisper.py`, `game.py` and `localeInfo.py`.
- **No echo:** `uiwhisper.py` shows the sender's line as it sends it.
- **The client's own limit:** it formats a received line as "Name : text" into a buffer of 256 bytes, so the longest lines it sends, 254 bytes, are cut short when shown, possibly without their end; the original server passes them on the same way.
- **Seen with the real client,** logged in as Szepta against a scripted player, Botka:
  - Botka's whisper made a button with its name appear at the right of the screen (its icon did not draw, the label did); the window it opened showed the line with every Polish letter intact;
  - Szepta's reply reached Botka, and showed once in her window;
  - once Botka had logged out, a further line showed "Botka nie jest zalogowany." (not logged on);
  - the table held the two delivered whispers, in UTF-8, and nothing for the third.

## What proves it

- `internal/protocol/protocol_test.go`: whisper in, with its bytes kept, the name's 25th byte ignored, refused without its terminator or shorter than the name; whisper out for a line and for not online, byte for byte.
- `internal/server/whispers_test.go`:
  - a whisper reaching only its recipient, on another map, with nothing back to the sender;
  - the name matched regardless of case;
  - not online for a name nobody has, and for a player on the character screen;
  - two whispers stored between the right characters, a cp1250 line as UTF-8;
  - an empty whisper, and one to oneself whatever the case of the name, ignored;
  - a whisper that could not be stored delivered, the sender told, the connection kept;
  - 254 bytes passed on, 255 ending the connection.
- `internal/server/flood_test.go`: a whisper after ten lines of chat dropped without a reply.
- `internal/server/server_test.go`: whispers in flight after Change character ignored.

## Not done yet

- Reading a conversation back, for our own client: its packet is that client's to design.
- Blocking whispers (types 2 and 3), and GM whispers.
- A limit on how long whispers are kept.
- Deleting a character: its whispers reference it, so the deletion is refused until what happens to the other side's conversation is decided.
