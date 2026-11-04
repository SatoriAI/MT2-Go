# One world loop for what runs on a timer

Until monsters, everything the server did answered a packet, on that connection's goroutine. Monsters act on their own: they stroll, and later they will attack and come back after dying. Something has to make them act.

## A timer per monster

Each monster schedules its next act with `time.AfterFunc`. It needs no loop, but every act runs on a goroutine of its own, so two monsters' acts, and a monster's acts and its own state, need locks of their own, and a test has to wait for real time to pass or replace the timer.

## A ticking loop, chosen

`game.Service.Run` is one goroutine, started by `internal/server` beside the two ports and stopped with them. It puts the mobs in the world, then every 100 ms walks the monsters whose stroll is due. Only it touches the monsters' own state (where each stands, when it next strolls, the randomness), so that state needs no lock; what others see goes through the world record, under its lock, as a player's move does, and sending only queues (`connections.md`).

- **One goroutine** orders every monster's acts, and a test calls `tick(now)` with the time it chooses and a fixed seed, so no test waits for a stroll. (Checking that nothing arrives still waits 100 ms.) The end-to-end tests run the real loop: a monster in their view would first stroll 5 s after the server starts, and each test finishes in well under one.
- **100 ms** is far finer than anything a monster does yet: the shortest wait between strolls is 5 s. A stroll starts at most 100 ms late, which nobody sees.
- **The cost** is a scan of the monsters each tick: 36 µs with 16,000 of them (Measured, below). A server with many more would keep them ordered by when they next act.

## Mobs see no mobs

The world record links a player with anyone in view; two mobs never link. Mobs only need to be seen, not to see each other, so a field of monsters keeps no links between its members and tells nobody when one of them moves near another. A move is still checked against everyone on the record, players and mobs; grouping the record by map is the first fix, at the size the measurements below give. Aggressive monsters, later, look for players, which this keeps.

## Measured

`make bench` (`internal/game/world_bench_test.go`) measures the world record and loop with players and wild dogs spread at random over the three first maps, a counting sink standing in for each player's connection. It runs each benchmark six times; compare medians, since a single run can be far off (the first baseline, from one run, put a move among 1,000 players at 90 µs; six runs give 52). Medians on an Apple M3 Pro, Go 1.27.3, 2026-11-04, without the race detector:

| One operation | 100 players, 160 monsters (today) | 1,000 and 1,600 | 10,000 and 16,000 |
|---|---|---|---|
| A player's move | 3.5 µs | 52 µs | 525 µs |
| A monster's stroll | 3.3 µs | 41 µs | 614 µs |
| A tick with nothing due | 0.3 µs | 3.1 µs | 36 µs |
| A player entering and leaving | 2.8 µs | 33 µs | 365 µs |

A move costs about 20 ns per character in the world, as the scan predicts. The world lock is held for every move, so the share of each second it is held is the moves per second times their cost:

- **Today:** 100 player moves and 11 strolls a second hold the lock for 0.4 ms of each second: 0.04%.
- **Ten times today:** 5.6% of each second.
- **A hundred times:** about six seconds of every second; the lock would never be free.

The cost grows with the square of the crowd: more players move, and each move scans more. Holding the lock a quarter of each second is where waiting to move would start to show, and that comes at about 3,500 players with today's monsters, or about 2,000 with a monster and a half per player. Idle ticks cost nothing worth counting at any size.

**What the estimate assumes,** each of which moves the limit:
- **A walking player moves about once a second,** and a monster strolls every 15 s on average. Twice the move rate brings the limit down by about a third.
- **A quarter of each second** is where waiting would show; it is a judgement, not a measurement.
- **Players are spread evenly over the maps.** Real players crowd the village centres, where each sees many more others, so a real move links and sends more under the lock than these do, and the real limit is lower than the estimate.
- **No contention:** the benchmarks run one move at a time; real goroutines waiting on the lock are not measured. A load test with real connections is the step that would.

**The first fix, when it is needed:** keep the record by map, so a move scans its own map only. With the three villages that divides every cost above by three, and every map added divides it further. A grid within each map comes next, if one map ever holds more than about a thousand characters; that also answers the crowding above. Monsters nobody sees could stand still, as open-mt2's do, which takes their strolls off the lock altogether. Run `make bench` before and after any of these.
