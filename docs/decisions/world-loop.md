# One world loop for what runs on a timer

Until monsters, everything the server did answered a packet, on that connection's goroutine. Monsters act on their own: they stroll, and later they will attack and come back after dying. Something has to make them act.

## A timer per monster

Each monster schedules its next act with `time.AfterFunc`. It needs no loop, but every act runs on a goroutine of its own, so two monsters' acts, and a monster's acts and its own state, need locks of their own, and a test has to wait for real time to pass or replace the timer.

## A ticking loop, chosen

`game.Service.Run` is one goroutine, started by `internal/server` beside the two ports and stopped with them. It puts the mobs in the world, then every 100 ms walks the monsters whose stroll is due. Only it touches the monsters' own state (where each stands, when it next strolls, the randomness), so that state needs no lock; what others see goes through the world record, under its lock, as a player's move does, and sending only queues (`connections.md`).

- **One goroutine** orders every monster's acts, and a test calls `tick(now)` with the time it chooses and a fixed seed, so no test sleeps.
- **100 ms** is far finer than anything a monster does yet: the shortest wait between strolls is 5 s. A stroll starts at most 100 ms late, which nobody sees.
- **The cost** is a scan of the monsters each tick. A few hundred monsters is nothing; a server with many thousands would keep them ordered by when they next act.

## Mobs see no mobs

The world record links a player with anyone in view; two mobs never link. Mobs only need to be seen, not to see each other, so a field of monsters keeps no links between its members and tells nobody when one of them moves near another. A move is still checked against everyone on the record, players and mobs; grouping the record by map is the first fix if that scan ever shows in a measurement. Aggressive monsters, later, look for players, which this keeps.
