# One send queue and one writer per connection

Until players could see each other, only a connection's own goroutine ever sent on it, and `Conn.Send` wrote straight to the socket. Once a move, a line or an arrival has to reach other players, other connections' goroutines send to it too. Three shapes were weighed.

## A lock around the socket

`Send` takes a lock and writes. It is the smallest change, but the sender waits on the receiver's socket: a client that stops reading holds up, for the whole write timeout, every player who moves near it. And a send made while the world's record is locked would hold that lock too, stalling everyone.

## One world goroutine

Every move, line and arrival goes to a single goroutine that owns the world and sends the updates. It orders everything for free. But it still cannot write to sockets itself without one slow client stalling the world, so it needs a queue per connection anyway. It also turns every session call into a message and a reply, for a world that is a map lookup today.

## A queue per connection, chosen

`Send` puts its packets, joined into one unit, on the connection's bounded queue, and the connection's writer goroutine writes them in order. Anyone can send, nobody waits for a socket, and a group like another player's ADD and INFO is never split. The world's record is a plain map under a lock, and packets about other players are sent under it, so everyone hears of things in the same order; that is safe because sending only queues.

- **A full queue** (256 sends) is taken to mean the client has stopped reading: the connection is closed, as a write timeout closed it before. The limit counts sends, not bytes, so whatever goes to one client in a burst is one send where it can be: someone entering a busy map is shown everyone at once. A burst of moves can still fill a reading client's queue when hundreds of players stand within its view range; the range bounds that by the crowd in view rather than on the whole map.
- **A hang-up** (Logout's phase close, then `ErrHangUp`) lets the writer send what is queued before the socket closes. **`Close`** (a takeover, shutdown) cuts at once, so a displaced connection's save is never held up by its client.
- **On the way out, the session closes before the queue drains.** Draining can take the whole write timeout when the client has stopped reading; closing the session first takes the player out of the world, saves them and lets go of their character meanwhile, so nobody sees a ghost for ten seconds and a new login is not refused. Nothing is lost: the session sends nothing to its own connection as it closes, and what it queued before is still written.

A world goroutine may still come, for things that run on a timer, such as monsters. It would send through the same queues.
