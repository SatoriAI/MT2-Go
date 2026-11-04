# Experience events

## What the player sees

For a scheduled span of time, such as Christmas, every kill gives more experience, twice as much or whatever the event says. When an event begins, everyone in the world sees a notice scroll over the top of the screen, also kept in chat: «Wydarzenie „Boże Narodzenie” trwa do 27.12 00:00: doświadczenie ×2.» When it ends, they see «Wydarzenie „Boże Narodzenie” dobiegło końca.» A player who enters the world while an event runs sees the first notice as they arrive.

## What the client does

- **A chat line from vid 0** is from no character: a NOTICE (type 2) is shown as the scrolling tip at the top of the screen (`BINARY_SetTipMessage`) and appended to chat in the notice colour (`PythonNetworkStreamPhaseGame.cpp`, `RecvChatPacket`; `PythonChat.cpp`). A BIG_NOTICE (8) would show in the middle of the screen instead; it is not used.
- **The client knows nothing of events:** the experience it shows is the server's.

## The bytes

The notice is a CHAT (`04`) as on [chat.md](chat.md), with type `02`, vid `0`, kingdom `0`, and the text in cp1250. The «×», the Polish quotes and letters all exist in cp1250.

## What the server does

`internal/event` holds what an event is; `internal/store/events.go` keeps them; `internal/game/events.go` runs them.

- **An event** has a name of 1 to 64 characters, a multiplier above 1 and at most 10, kept in percent (200 doubles, to two decimals), a start, and an end, the first moment it no longer runs. One event runs at a time: the table refuses an event whose time overlaps another's, though one may begin the moment another ends.
- **The command line** schedules them, in the server's time zone, to the minute:

  ```bash
  mt2go event create "Boże Narodzenie" 2 2026-12-24T00:00 2026-12-27T00:00
  mt2go event list
  mt2go event delete 1
  ```

  A multiplier may be written `1.5` or `1,5`.
- **The server looks** for the running event as it starts and then every 10 seconds, since events are written by another process. An event starts or ends at most 10 seconds late; one created or deleted while it should run is switched on or off at the next look. A look that fails is logged, keeps what runs, and is tried again at the next.
- **Switching** tells every player in the world: the end of the event that stopped, then the start of the one that began. An event that is still the same one is not announced again.
- **The multiplier** applies to the monster's whole experience as it dies, before it is shared by damage and scaled by each player's level gap ([experience.md](experience.md)), so the rounding still goes to the killer. Only experience is multiplied; drops and gold can become further columns and multipliers once they exist.
- **The notices** are in Polish, the client's language, and name the end in the server's time zone.

## How we know

- **How the client shows a line from vid 0:** the client's source named above.
- **The rest is this server's own:** the original game's events come from its quest scripts and admin commands, which this server does not have.

## What proves it

- `internal/event/event_test.go`: a multiplier read as written, with a point or comma; one that is no bonus, too large or no number refused; written back with a comma; experience multiplied and rounded down; what may be stored.
- `internal/store/events_test.go`: an event running from its first moment up to its end; an overlapping one refused, one starting as another ends accepted, and the list in order; a deleted event no longer running.
- `internal/game/events_test.go`: the start and end told to every player, once; one event following another ending it first; a player entering during an event told; a dog's 15 doubled; the watch switching as the store says and riding out a failed look.
- `internal/server/events_test.go`: an event stored before the server starts told to Hendra on entering, in cp1250, with the end in the server's time zone.

## Not done yet

- Multipliers for drops and gold, with those mechanics.
- Scheduling from an admin panel or in-game command, rather than the command line.
- Events limited to a kingdom, a map or a level range.
