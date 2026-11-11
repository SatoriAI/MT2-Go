# Trading

## What the player sees

Clicking another player of the same kingdom and pressing «Handel» on their board opens a trade window on both screens at once. The board offers the button within 3000. Dragging an item or yang onto a player does the same and offers it at once.
- **The window:** each side has a grid of four by three cells, a yang field and an Accept button with a lamp.
- **Offering items:** an item dragged from the bag onto your grid shows on both sides.
- **Offering yang:** yang is set once. The other side reads a chat line naming the amount in full, «Hendra oferuje 1.000.000 yang.»

When both have accepted, the items and yang change hands at once and both windows close. Any change to either offer puts both lamps out, and an accept within two seconds of a change is refused («Oferta właśnie się zmieniła. Poczekaj chwilę.»).

A trade ends when either side presses the window's X («Handel został anulowany.» to the other), or walks more than 1000 from where it stood. It also ends when either side dies («Handel został przerwany.» to both) or leaves the game. While trading, the bag, the taskbar, shops and the blacksmith don't respond («Nie możesz tego zrobić podczas handlu.»), potions included.

**Refused, with a line:**
- a player of another kingdom («Nie możesz handlować z graczem z innego królestwa.»);
- one already trading («Ten gracz już handluje.»);
- one who blocks trades in the game options («Ten gracz nie przyjmuje propozycji handlu.»);
- starting a trade with a shop or the blacksmith open («Zamknij najpierw otwarte okno.»);
- trading with someone while you trade with someone else («Już handlujesz.»);
- an accept when what the other side offers would not fit the bag, counting what you give as gone («Nie masz miejsca w ekwipunku.»), or would take you past 2,000,000,000 yang («Miałbyś za dużo yang.»). The other side reads «Kira nie może przyjąć tej oferty.»

## What the client does

- **Starting:**
  - «Handel» sends START with the other's vid (`uitarget.py`).
  - Dropping an item on a player sends START, then ADD with the trade cell 0. Dropping yang sends START, then the yang (`game.py`, `__PutItem`, `__PutMoney`).
  - The client asks nothing of the other side. Its START opens the window, named after the vid it carries (`PythonNetworkStreamPhaseGame.cpp`, `RecvExchangePacket`).
- **Offering:**
  - A drop on the grid sends ADD with the item's place and the cell dropped on (`uiexchange.py`).
  - The client refuses worn items and items flagged not to be given. It refuses to edit the yang once it is above 0, and to put gear on during a trade.
  - It never sends DEL, the subheader to take an item back.
- **Accepting:** Accept disables the button until the client is told its accept was taken back.
- **Walking away:** the client cancels by itself past 1000 on either axis from where the window opened.
- **Messages:** the "already trading" and "too little yang" subheaders are only logged, so whatever the player should read comes as a chat line.

## The bytes

| Packet | Bytes | Fields |
|---|---|---|
| EXCHANGE `1B`, client | 10, then the sequence byte | subheader u8, arg u32, arg u8, item position {window u8, cell u16} |
| EXCHANGE `2A`, server | 47 | subheader u8, mine u8 (1 for the side's own offer or accept), arg u32, position {window u8, cell u16}, arg u32, 3 sockets i32 and 7 bonuses {u8, i16}, all 0 |

**Client subheaders:**
- START `00`: arg is the other's vid.
- ADD `01`: the position is the item, the u8 is the trade cell dropped on.
- YANG `03`: arg is the amount.
- ACCEPT `04`.
- CANCEL `05`.

**Server subheaders:**
- START `00`: arg is the other's vid.
- ADD `01`: arg is the kind, the position's cell the trade cell, the second arg the count.
- YANG `03`: arg is the amount.
- ACCEPT `04`: arg 1 given, 0 taken back.
- END `05`.
- LESS_YANG `07`.

The client reads only the cell of the server's position, so the window byte is 0.

## What the server does

`internal/game/trades.go` keeps the trades in the world record. `internal/game/trade.go` carries out each packet, and `internal/store/trade.go` keeps a trade done.

- **Starting:** START opens a trade when the other player is alive and the starter is too. They must be of the same kingdom, on the same map within 3000, neither trading, and the other not blocking trades.
  - A player out of reach, dead or unknown is refused silently, as the client hides the button then.
  - START to the one already traded with changes nothing, so a second item dropped on them is only offered.
- **An item offered** must be one the player carries, not worn, not bound ([items.md](items.md)), which is told «Nie możesz handlować tym przedmiotem.» as the client says it, and not offered already. It goes to the trade cell it was dropped on if it fits there: an item stands in its column from its top cell down, as in the bag. Otherwise it goes to the first cell it fits, or is refused with «Nie ma już miejsca w oknie handlu.» A stack goes whole.
- **Yang** is offered once, at most what the player holds. More is told LESS_YANG with «Nie masz tyle yang.»
- **Any change** to either offer takes back both accepts, each side told.
- **An accept:**
  - It counts 2 s after the last change.
  - It plans where each item the other side offers would go in the player's bag: the first cells each fits, with what the player gives counted as gone. Items go to free cells and never join stacks.
  - It checks the player's yang, less what they give and with what they get, stays within the cap.
  - It notes which of the player's taskbar slots hold what they give.
  - A refused accept is taken back on the player's screen with ACCEPT 0, so the button returns.
- **Inventories frozen:** from START to END, the packets that read or change what a player holds are refused with a line: the bag's, the taskbar's, shops', clicks on NPCs and the blacksmith's. Only those that close a shop or the refine dialog pass. So what each side accepted for stays what it holds.
- **Keeping it:** the second accept keeps the trade, in one transaction:
  - Each item given changes owner and moves to the cell planned for it. The bag's cells are checked unique only at the end of the transaction, so an item can move into a cell the other side's item is leaving.
  - Each side's yang becomes what it had, less what it gave, plus what it got. The database refuses a balance below 0 or past the cap.
  - The taskbar slots holding items given are dropped.
  - The trade is logged in `trades` (who, the yang each gave) and `traded_items` (each item, its kind, count and giver). The log names characters and items by id without a reference, so it outlives a character deleted.

  An item a side no longer owns refuses all of it.
- **After:** each side is told each item it gave gone from its cell, each it got in its new cell, its yang, its taskbar slots emptied, and END.
  - The side that accepted last updates what it holds at once.
  - The other side's connection takes the change, left for it in the world record, before its next packet that touches what it holds. A connection's bag and yang belong to its goroutine alone ([drops.md](drops.md)).
  - The change is applied to what the side holds then, not a copy taken as it accepted: the items given go, those got come, the yang moves by the difference, and the slots empty. Shooting is not frozen, as arrows are worn and cannot be traded, so arrows shot meanwhile stay shot.
- **A trade that cannot be kept** changes nothing. Both are told END and «Handel nie powiódł się.», and each connection reads again what it holds from the database, as a write that timed out may have been kept after all. A read that fails ends the connection. The client's screen is not redrawn, so after such a timeout it shows what the player held until they enter again.
- **A trade being kept** ignores cancels, walking away and death. A side leaving the game waits until it is kept, so a new login reads what it holds after it.

## How we know

- **The client's side:** its source, `Packet.h` (`TPacketCGExchange`, `TPacketGCExchange`, `EXCHANGE_SUBHEADER_*`), `PythonNetworkStreamPhaseGame.cpp` and `PythonExchange.h` (12 cells), and its scripts `uiexchange.py`, `uiscript/exchangedialog.py` (4 by 3), `uitarget.py` and `game.py`. The window's lines are from the client's Polish `locale_game.txt`.
- **The rules not in the client** — no prompt, one kingdom, the two seconds, the frozen bag and potions refused, accepts refused for room, the trade log — were settled in a debate between a veteran of the original game and a designer. Where they differed, the veteran's memory of the original decided.
- **Seen from two bots** against the running server: Iskra, with 5 red potions, met Cien, with 3 blue ones, in Jinno.
  - Iskra's START opened the window on both sides, each naming the other.
  - Cien's drag of an item in her bag was refused with «Nie możesz tego zrobić podczas handlu.».
  - Each offer was shown on both sides, at the cells dropped on.
  - Iskra accepted, then Cien, which kept the trade: Iskra's potions left cell 0 and Cien's came there; Cien's left cell 1 and Iskra's came there, beside her dagger. Both windows closed.
  - The database then held each stack with its new owner, and one trade logged with both items.
- **Seen in MT2-Godot,** whose trade window follows the original's: Cienista traded her dagger to Testka in a full trade, both accepting.

## What proves it

- `internal/protocol/protocol_test.go`: the client's EXCHANGE framed and read; the server's 47 bytes, byte for byte.
- `internal/store/trade_test.go` (real database):
  - a trade kept whole: items both ways, one into a cell the other side gives up, the yang, the taskbar slot dropped, the log;
  - an item not owned, or yang past what a side has, changing nothing.
- `internal/game/trade_test.go`:
  - the window opening on both sides;
  - refusals: another kingdom, already trading, blocking trades, out of reach, dead, oneself;
  - items offered at their cell or the first free, a worn one and one offered twice not;
  - yang offered once and no more than held, told in full;
  - an accept too soon; a change taking back both accepts;
  - an accept refused for room, for two items with room for one, or for yang, and allowed by the room and the yang what is given makes;
  - two tall items never overlapping in the window;
  - a trade kept whole; what each side is told; the side keeping it holding the change at once, the other from its next packet, applied to what it then holds;
  - a trade the store refuses changing nothing, both reading again what they hold;
  - a drag refused on either side while trading and while the trade is being kept, and a shop's closing not;
  - ending by cancel, walking away, death and leaving; a walk of 1000 keeping it;
  - a side leaving while it is kept waiting for it.
- `internal/server/trade_test.go` (real sockets and database): Hendra trading two potions to Kira, both accepting, the potions Kira's and the trade logged.

## Not done yet

- Taking an item back out of an offer, which the client cannot ask for.
- Items flagged not to be traded: no kind is yet.
- Reading a trade's log from the command line.
