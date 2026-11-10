# One world loop for what runs on a timer

Until monsters, everything the server did answered a packet, on that connection's goroutine. Monsters act on their own: they stroll, and later they will attack and come back after dying. Something has to make them act.

## A timer per monster

Each monster schedules its next act with `time.AfterFunc`. It needs no loop, but every act runs on a goroutine of its own, so two monsters' acts, and a monster's acts and its own state, need locks of their own, and a test has to wait for real time to pass or replace the timer.

## A ticking loop, chosen

`game.Service.Run` is one goroutine, started by `internal/server` beside the two ports and stopped with them. It puts the mobs in the world, then every 100 ms walks the monsters whose stroll is due. Only it touches the monsters' own state (where each stands, when it next strolls, the randomness), so that state needs no lock; what others see goes through the world record, under its lock, as a player's move does, and sending only queues (`connections.md`).

- **One goroutine** orders every monster's acts, and a test calls `tick(now)` with the time it chooses and a fixed seed, so no test waits for a stroll. (Checking that nothing arrives still waits 100 ms.) The end-to-end tests run the real loop: a monster in their view would first stroll 5 s after the server starts, and each test finishes in well under one.
- **100 ms** is far finer than anything a monster does yet: the shortest wait between strolls is 5 s. A stroll starts at most 100 ms late, which nobody sees.
- **The cost** is a scan of the monsters each tick: 45 µs with 16,000 of them (Measured, below), outside the world lock but for a short take of the monsters killed since the last tick. A server with many more would keep them ordered by when they next act.

## Mobs see no mobs

The world record links a player with anyone in view; two mobs never link. Mobs only need to be seen, not to see each other, so a field of monsters keeps no links between its members and tells nobody when one of them moves near another. A move is still checked against everyone on the record, players and mobs; grouping the record by map is the first fix, at the size the measurements below give. Aggressive monsters, later, look for players, which this keeps.

## Measured

`make bench` (`internal/game/world_bench_test.go`) measures the world record and loop with players and wild dogs spread at random over the three first maps, a counting sink standing in for each player's connection. It runs each benchmark six times; compare medians, since a single run can be far off (the first baseline, from one run, put a move among 1,000 players at 90 µs; six runs give 52). Medians on an Apple M3 Pro, Go 1.27.3, 2026-11-09, without the race detector, with the record kept by map and the fields full, some 1,250 monsters over the three first maps:

| One operation | 100 players, 1,250 monsters (today) | 1,000 and 12,500 | 10,000 and 125,000 |
|---|---|---|---|
| A player's move | 6.5 µs | 82 µs | 995 µs |
| A monster's stroll | 8.0 µs | 88 µs | 1.45 ms |
| A tick with nothing due | 3.4 µs | 34 µs | 391 µs |
| A player entering and leaving | 5.9 µs | 51 µs | 598 µs |

Before the fields were filled, with 160 monsters per 100 players, a move cost 1.0, 13 and 159 µs, a stroll 1.5, 15 and 238, an idle tick 0.47, 4.4 and 44, and entering 1.4, 9.1 and 105. Before the record was kept by map (2026-11-04, with combat in), a move cost 3.5, 52 and 537 µs, a stroll 3.4, 43 and 613, and entering 2.8, 33 and 369; the paragraphs below measure against those.

Combat added a monster's hit points and pickers to the record and bodies and returns to the loop. Moves, strolls and entering stayed within 4% of the figures before it; an idle tick, which now also looks for bodies to take away and monsters to bring back, costs a quarter to two fifths more. A first version, which read every monster's death under the world lock each tick, tripled the idle tick and held the lock 97 µs of each tick at 16,000 monsters; the record now keeps a short list of the monsters killed since the loop last asked, so the loop's work under the lock grows with deaths, not with monsters.

Experience changed none of these by more than 4%. A first version slowed the idle tick by two fifths at 16,000 monsters: adding a field to a monster's kind made the tick's copy of it, to ask whether it wanders, cross into another cache line; the loop now keeps that answer on the monster itself.

Experience events, which added the running event to the record and a look at it on entering, changed none of these by more than 2%.

The periodic save gave each world entry a mutex, ordering a player's saves. As a first version it grew the entry from 160 to 168 bytes, into the allocator's 176-byte class, which monsters use too; entries and monsters then shared spans, and an idle tick, which walks only monsters, was 17% slower at 16,000 of them. The entry kept the character's id twice, once beside its character; dropping the copy brought it back to 160 bytes, and every figure to within 3% of before. A field added to `present` should keep it at 160 bytes, or be measured. Monsters have since left the 176-byte class (they are 120 bytes, in the 128-byte class, since they share their kind), so a bigger entry would no longer share their spans, but every scan of the record would still walk more memory.

Monsters fighting back added a fight to the loop: blows on living monsters are kept by the world record, as deaths are, and taken with them under one lock a tick, and the loop steps only the monsters fighting, each step one call under the world lock. An idle tick does no more than before. A first version slowed the idle tick by 14% at 16,000 monsters: the bite's values grew the monster's kind, which each monster held a copy of, from 56 to 64 bytes, and the monster from 168 to 184, past the allocator's 176-byte class. Monsters now share their spawn's kind through a pointer and are 120 bytes; the idle tick is 4% faster than before the fight, and moves, strolls and entering, run alternately with main's, stay within 3%. The first runs of those, taken while the game client was running, differed by up to 10%; close the client before benchmarking. The table as it stood then still held within 4%, the idle tick that much below it.

Healing added two jobs to the loop. Hurt monsters are kept on a list, as fighting ones are, so an idle tick still walks none of them. Players are healed every three seconds in one round under the world lock, which `BenchmarkPlayerRegeneration` measures with every player hurt: 5.5 µs at 100 players, 73 µs at 1,000, 0.73 ms at 10,000, 0.02% of each three seconds even at the largest size.

Equipment added nothing to the record's entries: what a player wears is kept on their character, as their hit points are, and a blow looks its weapon up by vnum. Measured against main, alternately and with the client closed, every benchmark stayed within 1% except the 100-player regeneration and entering, 2 to 7% faster, which is noise in the other direction; the geometric mean moved 1%.

Drops lie on the ground in a table of their own under the world's lock, not in the record's entries, and nobody keeps what each player has been shown: items never move, so a player's move looks at each item on the ground, at its old and new position, to show those coming into view and take away those leaving it. With one item lying for every ten monsters, a generous guess at a minute of kills, `BenchmarkPlayerMoveAmongDrops` puts a move 3 to 7% above the same move with nothing on the ground (3.75 µs against 3.52 at today's size, 558 µs against 532 at a hundred times it). Each drop's timers are two queues in the order drops fell, so an idle tick with nothing on the ground does nothing more; the existing benchmarks stayed within 3% of main. Ground items are kept by map since 2026-11-09, as the record is, so a move looks only at those on its own map: `BenchmarkPlayerMoveAmongDrops` came out 13 to 16% cheaper at every size against main, run alternately, the other benchmarks within noise.

Yang lies on the ground as a drop does, with its amount in the ground item, and is rolled in the same loop step as a monster's items; a player's yang is the connection's own, beside the inventory, not the record's. Measured against main, alternately and with the client closed, every benchmark stayed within 2%, a move among drops included.

What players drop is laid down by their connection under the world's lock, and its clearing is handed to the loop with the news it already takes once a tick, so an idle tick takes no more locks. Measured against main, three rounds alternately with the client closed, the moves, strolls and idle tick stayed within 3%; a first round on a busy machine had shown up to 9% either way on code the change does not touch.

Skills add no field to the record's entries: a buff's end is handed to the loop with its news, into a queue in order, and a character's effects are worked out from its running buffs when it is shown, rather than kept on its entry, which would have grown every monster's too. The character gained a map of its buffs, beside its skills. Measured against main, three rounds alternately with the client closed, every benchmark stayed within 3%; a regeneration round first read up to 7% slower, and three more rounds of it alone read 0 to 3%.

A move costs about 15 ns per character on its own map, as the scan predicts; before the record was kept by map it was about 20 ns per character in the world. The world lock is held for every move, so the share of each second it is held is the moves per second times their cost:

- **Today:** 100 player moves and 83 strolls a second, the full fields' 1,250 monsters each strolling every 15 s, hold the lock for 1.3 ms of each second: 0.13%. Before the fields were filled it was 0.012%.
- **Ten times today,** players and monsters both: 15% of each second.
- **A hundred times:** about twenty seconds of every second; the lock would never be free.

The cost grows with the square of the crowd: more players move, and each move scans more. Holding the lock a quarter of each second is where waiting to move would start to show. The benchmarks grow the monsters with the players, which puts it at about 1,300 players; but the fields' monsters stay at 1,250 whoever plays, and with them fixed it comes at about 6,500 players, each move scanning a third of the players and a map's 420 monsters. Before the fields were filled, by the same reckoning, it came at some 6,000 to 7,000. Idle ticks cost nothing worth counting at any size.

**What the estimate assumes,** each of which moves the limit:
- **A walking player moves about once a second,** and a monster strolls every 15 s on average. Twice the move rate brings the limit down by about a third.
- **A quarter of each second** is where waiting would show; it is a judgement, not a measurement.
- **Players are spread evenly over the maps.** Real players crowd the village centres, where each sees many more others, so a real move links and sends more under the lock than these do, and the real limit is lower than the estimate.
- **No contention:** the benchmarks run one move at a time; real goroutines waiting on the lock are not measured. A load test with real connections is the step that would.

Aggression and packs added a lookout to the loop and a pack to each monster. The loop keeps the aggressive monsters on a list of their own, and the lookout takes the world lock once a tick to look over them, not at all where there are none. Each monster gained a pointer to its pack, 104 bytes to 112, the same allocator class. A first version kept a player's few seconds unseen on the world record's entry, which took it from 192 bytes to 216, into the 224-byte class: every move, stroll and arrival was 4 to 9% slower. It lives on the character now, as `LastFought` does. A second gave the tick a case of its own for a pack's return, which every living monster passed through: the idle tick was 8% slower. One case for anything whose body is gone answers both now. Measured against main, two rounds alternately, every benchmark came out within 2%, or faster, but a stroll among 100 players, 4% slower; three more rounds of the idle tick alone came out within 0.5%.

**Kept by map since 2026-11-09:** a move, a stroll and an arrival scan the record of their own map only, which divided their cost by about three with the three villages, half to three quarters less at every size in the table above, as estimated; the idle tick and regeneration, which scan no one, stayed within 1% of main in the same alternating rounds. Every map added divides it further. Each entry's map is taken on entering, so a warp, when there is one, must move it between maps. A grid within each map comes next, if one map ever holds more than about a thousand characters; that also answers the crowding above. Monsters nobody sees could stand still, as open-mt2's do, which takes their strolls off the lock altogether. Run `make bench` before and after any of these.

Finger Strike and Enchanted Armour add nothing to the loop but a bite's look at the defender's buffs. Measured against main, two rounds alternately with the client closed, every benchmark stayed within 2% but the idle tick, 7 to 10% slower, at 0.26 ns a monster, in three more rounds of it alone. The profile puts the time in `tick`'s own loop over the monsters, and that loop compiles to the same instructions on both builds, only at another address: it is where the code is laid out, not what it does, and will move again with the next change elsewhere.

Metin stones made a monster share its spawn through a pointer, as it shares its kind, because a stone's spawn carries its spots. That took the monster from 120 bytes to 104, though it gained a mark for what a stone called. A monster's fight, a separate allocation that only blows, heals and deaths touch, grew from 104 bytes to 144: the time of its last hit, from which a stone waits to heal, and a stone's waves and smoke. What a stone called is kept in a map beside the loop, not on every monster. That list, and the clean-up of called monsters once they are gone, cost a tick nothing until a stone calls or a called monster dies. Measured against main, two rounds alternately with the client closed, every benchmark was 0 to 3% faster, the 10,000-player entering 1% slower.

Trading keeps its trades in the world record, by each side's vid, outside the record's entries. A player's move checks how far the player walked from where their trade began, but only while some trade is going on. Measured against main on 2026-11-10, alternately, every benchmark but four moves stayed within 2% or came out cheaper. Those four came out 2 to 12% above: a move at 100 and 1,000 players (+4%), and a move among drops at 1,000 (+2.3%) and at 10,000 (+12%). Run again alone, alternately, six times each, all six move benchmarks came within 2% of main.

A tidying pass folded the loop's three lists of timed effects into one generic list and the record's eight lists of news into one struct, changing no behaviour. The first cut gave a monster a stone dismissed a check of its own in the tick's stroll case, which every living monster passed through: alternately against main on 2026-11-10, eight rounds of the idle tick alone came out 8 to 9% slower at 1,250 and 12,500 monsters. One case now catches every monster whose body is gone, alive or not, so a living one passes two checks rather than three: eight more rounds came out 10% faster than main at both sizes, and level at 125,000. In six full rounds alternately, every other benchmark stayed level with main within its spread; a move among drops at 10,000 players read 15% faster, at an 18% spread.

The world record became `record`, its guarded fields a `state` reached only through `lock()`, and what may be done under its lock the methods of `locked`, so a call without the lock fails to compile. `lock()` inlines at every call, and the unlock it hands back stays on the stack. Measured against main on 2026-11-10, six rounds alternately, every benchmark came within 1.5%, none of it significant.

A monster's four ailments became a set of bits and the data each needs, an end time for a stun, slow or poison and a dose for a poison or fire, which took its fight from 280 bytes to 272, the same 288-byte class. A first shape gave every ailment the same entry, and took it to 344. Measured against main on 2026-11-10, six rounds alternately, every benchmark came out level but the idle tick at 125,000 monsters, 2% faster.
