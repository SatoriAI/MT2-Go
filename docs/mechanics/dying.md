# Dying and restarting

## What the player sees

When a monster's bite takes the last of a player's hit points, the character falls, everyone near sees the body, and the restart dialog opens with two buttons. **Restart here** brings the character back where it fell with half its hit points and mana; **Restart in town** brings it back at the kingdom's start with all of them. Dying costs a little experience, never a level. The monster that killed the player loses interest and walks home. Back on their feet, a player's hit points and mana fill again over time ([combat.md](combat.md), healing); the dead do not heal.

## What the client does

- **DEAD (`0E`) for the player's own vid** calls the game window's `OnGameOver`, which closes the target window and opens the restart dialog, and lays the character down (`PythonNetworkStreamPhaseGame.cpp`, `game.py`). While dead, the client neither moves nor attacks (`ActorInstanceBattle.cpp`, `CanAct`).
- **The dialog's buttons** send `/restart_here` and `/restart_town` as chat (`uirestart.py`). It has no countdown, and Esc does not close it; the server closes it with the chat command `CloseRestartWindow` (`game.py`).
- **There is no revive packet.** A fresh CHARACTER_ADD for the player's own vid rebuilds the character alive and drops every other character the client knew (`NetworkActorManager.cpp`), so everything in view is sent again after it.

## The bytes

No new packets: DEAD, CHAT (type 5, a command, from the player's vid), CHARACTER_ADD and INFO, and POINT_CHANGE, as on [combat.md](combat.md), [chat.md](chat.md), [other-players.md](other-players.md) and [experience.md](experience.md).

## What the server does

`internal/game/dying.go`; the penalty is `account.LoseExpOnDeath`.

- **A player is dead** at no hit points. A dead player's moves are dropped, their blows and picks land nowhere, and monsters chasing them give up at their next step. Someone arriving sees the body lying, as for a monster.
- **Dying:** the player loses 2% of what their level needs, never below no experience, so a level is never lost: 6 at level 1, nothing at the top level. They get the experience lost as a POINT_CHANGE, then their death; everyone who sees them gets the death. Their pick ends.
- **Leaving while dead,** by logging out, Change character or a dropped connection, keeps the character at its kingdom's start, as a restart in town would put it, since it comes back with all its hit points.
- **Experience** from a monster that dies after the player did is not theirs: a dead player gets no share of a kill.
- **Restarting,** only while dead (otherwise the command is ignored): hit points and mana are set, half here (at least 1 hit point), all in town; the player gets `CloseRestartWindow`, their own character afresh, their hit points and mana, then everyone in view. Those who saw them get them afresh, or, after a restart in town, see them go. In town the character stands at its kingdom's start, which the session keeps as where it is, so leaving saves it there. The client is only told its character's new position, which works because every first village holds its kingdom's start: dying on another map will need a warp.

## How we know

- **The client's side:** its source and scripts named above.
- **The restart commands:** the client's `uirestart.py`; open-mt2 registers the same. **Reviving by a fresh CHARACTER_ADD:** the client has no other way, and open-mt2 sends its own add again.
- **This server's choices,** with Dawid: the 2% penalty, since open-mt2 has none and the original's rule is not recorded; half here and all in town.

## What proves it

- `internal/account/progress_test.go`: dying costing 2% of the level's need; never a level; nothing at the top.
- `internal/game/aggro_test.go`:
  - a bite that takes the last hit points: her hit points, the experience lost and her death to her, the death to an onlooker, and the dog giving up;
  - a dead player neither hitting nor picking;
  - restarting here with half, where she fell, with the packets in order and the dog sent again;
  - restarting in town with all, at the start, an onlooker seeing her go;
  - a restart while alive changing nothing;
  - a dead player getting no share of a kill.
- `internal/game/save_test.go`: a player who leaves dead kept at their kingdom's start.

## Not done yet

- Keeping hit points between logins; a character enters with all of them.
- Dying to other players, and duels.
- A pause before the buttons work, and a short protection after restarting here.
