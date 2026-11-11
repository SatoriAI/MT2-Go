# Teleporting

## What the player sees

A double-click on Pierścień Teleportacji (70058) opens a dialog: «Dokąd chcesz się przenieść?», with the player's own village, Strefa Yongan, Strefa Joan or Strefa Pyungmoo by kingdom, and «Anuluj». Picking the village shows the loading screen, and the character stands at the kingdom's start, where a new character first appears and a restart in town puts them. The others nearby see the character vanish. The ring stays in the bag.

It is refused with a line in the chat:
- «Masz za niski poziom, aby użyć tego przedmiotu.» below level 30;
- «Nie możesz tego zrobić podczas handlu.» while trading, as every use of an item is ([trading.md](trading.md));
- «Nie możesz teraz użyć tego przedmiotu.» while dead, at a shop or at the blacksmith, and when the answer comes during a trade.

No shop sells the ring and nothing drops it; it is given on the command line (`mt2go item`) or with `/getitem 70058` on a development server.

## What the client does

- **The dialog** is the SCRIPT window NPCs use. The client sends SCRIPT_ANSWER with the button picked, from 0, and the last button for Escape ([skills.md](skills.md)).
- **A warp** (`PythonNetworkStreamPhaseGame.cpp`, `RecvWarpPacket`): the client remembers the character slot it plays, drops the game connection and connects to the address and port the packet names. It goes through the handshake and logs in with the same name and login key as before (`PythonNetworkStreamPhaseLogin.cpp`, its direct-enter mode), skips the selection screen, and sends SELECT_CHARACTER for that slot from the loading screen. The packet's x and y are not used for where the character stands: that comes, as on any login, from MAIN_CHARACTER.
- **The address** is read in network byte order, as the character list's is (`NetStream.cpp`, `Connect`).

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| ITEM_USE `0B`, client | 4, then the sequence byte | the item's position ([items.md](items.md)) |
| SCRIPT `2D` | its own length | skin `01`, the text: `Dokąd chcesz się przenieść?[ENTER][QUESTION 1;Strefa Yongan\|2;Anuluj]` |
| SCRIPT_ANSWER `1D`, client | 2, then the sequence byte | the button, from 0 |
| WARP `41` | 15 | x i32, y i32, the game server's IPv4 in network order (4 bytes), port u16 |

## What the server does

`internal/game/teleport.go`:

- **Asking:** a use of the ring checks the player may go, then asks. It keeps which ring it asked about until the answer, as the soul stone's question does.
- **The answer** is checked again, since a trade may open or the player die while the dialog is up. It first takes in what a trade the other side kept has moved, as item packets do, so a ring traded away with the question open goes nowhere. The first button, if the player still carries the ring, warps them to `Empire.StartPosition`; any other does nothing.
- **The warp,** in order:
  1. the player is taken out of the world record, so everyone who saw them is told CHARACTER_DELETE;
  2. their position is set to the destination and the character saved there, as leaving the world saves it;
  3. the connection lets go of the character, so the client's next connection takes it without waiting;
  4. WARP goes to the client with the game port's public address.
  The old connection then sits in the select phase until the client closes it. Moves and other game packets still in flight from the client are dropped, as after Change character. The login key outlives the connection ([connection-and-login.md](connection-and-login.md)), so the new one logs in with it.
- **Not modelled:**
  - **The 50 charges** of item_proto's VALUE0, and the Polish tooltip's «50 razy». Our items keep no count of their own besides a stack's, so the ring is used without wearing out. With one destination, the player's own village, that gives nothing a restart in town does not. The charges come with the first way for a player to get the ring.
  - **VALUE1, 1800.** Its meaning is not known. The veteran recalls no cooldown on the ring, so none is modelled.
  - **Fighting** does not stop a warp. The veteran recalls players escaping fights with it.
- **The rules** were settled in a debate between a Metin2 veteran and a game designer (2026-11-11), with the veteran deciding ties: unlimited for now, level 30 enforced, no combat lock, a dev command as the only source.

## How we know

- **The item:** the client's `--dump_proto--/item_proto.txt` (ITEM_QUEST, not stackable, LIMIT LEVEL 30, VALUE0 50), and the Polish `itemdesc.txt`. The village names are `locale_game.txt`'s MAP_A1, MAP_B1 and MAP_C1.
- **The warp:** the client's source named above, and open-mt2's teleport ring, which warps with the same 15-byte packet after taking the player out of view.
- **Seen from a bot** against the dev server (2026-11-11). Ringo, a Jinno character set to level 30 in the database, stood at (967400, 278200), 4000 east of the start:
  - the ring asked with «Strefa Pyungmoo» and «Anuluj»;
  - the first button brought WARP to (963400, 278200) at 127.0.0.1:13001;
  - the bot closed the old connection as the client does, and a new one with the same key loaded her there.
- **Seen in MT2-Godot** (its e8df425, against the same server): a level-91 Jinno character got the question, and the first button sent WARP. The client reconnected by key and slot, skipped the select screen, loaded, and entered at Jinno's start (963400, 278200).

## What proves it

- `internal/protocol/protocol_test.go`: WARP byte for byte.
- `internal/game/teleport_test.go`:
  - the ring asking, with the kingdom's village, and staying in the bag;
  - the village picked: the warp to the start, the others seeing the player go, the position saved there, the character let go, and the ring kept;
  - a move after the warp dropped;
  - cancel going nowhere;
  - refusals below level 30 and at a shop;
  - a player who died with the question open not taken;
  - a ring traded away with the question open taking nobody.
- `internal/server/teleport_test.go`: a level-30 player given the ring on the command line, taken over real connections from the question to WARP, and a new connection with the same key, while the old one is still open, loading her at the start.
