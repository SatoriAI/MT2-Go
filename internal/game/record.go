package game

import (
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/event"
	"github.com/SatoriAI/MT2-Go/internal/item"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// viewRange is how far a player sees other characters: open-mt2's choice, since the
// client takes any. It draws characters up to 20000 away, so all of these are drawn.
const viewRange = 8000

// record is the world record. It holds who stands in the world, players and mobs, and
// who sees whom, a player and anything else on the same map within viewRange of each
// other. Mobs see no mobs, so the many standing in the fields cost nothing between themselves.
// Packets go out under the lock, so every player is told of the same things in the same
// order: a character always appears before its moves. Sending only queues, so the lock is
// never held across a slow client. A send that fails is left: that connection closes itself.
type record struct {
	mu    sync.Mutex
	state state // reached only through lock
}

// state is what the record's lock guards.
type state struct {
	all      map[uint32]*present               // by vid
	onMap    map[string]map[uint32]*present    // the same, by the map each stands on and vid
	byName   map[string]*present               // players, by lower-cased name: unique regardless of case
	news     news                              // what happened since the world loop last asked
	event    *event.Event                      // the event running; nil for none
	ground   map[uint32]*groundItem            // items lying in the world, by vid
	groundOn map[string]map[uint32]*groundItem // the same, by the map each lies on and vid
	trades   map[uint32]*trade                 // the trades going on, under each side's vid
	settled  map[uint32]settlement             // what trades others kept changed for players, for each to take
	spent    map[uint32][]int64                // items whose time ran out, by their player's vid, for each to take
}

// newRecord is an empty world record.
func newRecord() record {
	return record{state: state{
		all: map[uint32]*present{}, onMap: map[string]map[uint32]*present{}, byName: map[string]*present{},
		ground: map[uint32]*groundItem{}, groundOn: map[string]map[uint32]*groundItem{},
		trades: map[uint32]*trade{}, settled: map[uint32]settlement{}, spent: map[uint32][]int64{},
	}}
}

// locked is the world record's state while its lock is held. What may be done only under
// the lock is a method of locked, so the compiler sees the lock taken first; a method
// that uses nothing of it but its receiver, such as prey, takes one as that proof.
type locked struct{ *state }

// lock takes the record's lock, and hands back its state held and the way to let it go.
func (w *record) lock() (locked, func()) {
	w.mu.Lock()
	return locked{&w.state}, w.mu.Unlock
}

// sender is where a player's packets go: their connection, which only queues them.
type sender interface {
	Send(packets ...[]byte) error
}

// present is one character in the world, as the players who see it see it.
type present struct {
	out    sender     // the player's connection; nil for a mob
	saving sync.Mutex // held while the player is saved, so their saves land in order
	world  string     // the map's name; "" for a position on no known map, which those share
	add    protocol.CharacterAdd
	info   protocol.CharacterInfo
	blocks account.Blocks // what they refuse from others, as they last chose
	target *present       // the monster the player has picked, while they see it
	// char is the player's character, stored under ID; the vid lasts one login. While the
	// player is in the world, its level, experience and stats change only under the
	// record's lock: others' kills raise them.
	char   *account.Character
	fight  *fight // a monster's hit points; nil for those that cannot be fought
	tether tether // where a player's character can be, by the server's reckoning
	// sees is who this player has been shown, by vid. Seeing is mutual, so both sides
	// change together, and leaving tells exactly these.
	sees map[uint32]*present
	// timed is what a player wears for a limited time, counted while they are in the world.
	timed *wearing
	// hand is when the third hand a player wears ends, zero for none; lasting is what they
	// hold that lasts in real time, which the world loop ends on time while watched.
	hand    time.Time
	lasting []item.Item
	watched bool
	// credited is set once their third hand's yang is kept, for their connection to read
	// its yang again; it is written off the world's lock, so it is atomic.
	credited atomic.Bool
	// creditTurn is held by the one credit of their yang being kept, so credits take turns.
	creditTurn sync.Mutex
}

// appearance is what shows the character to another client. The client builds a player
// or an NPC only when its info follows its add, so the two go out as one send; it builds
// a monster from the add alone, naming it from its own mob data, and drops an info for one.
func (p *present) appearance() [][]byte {
	add := p.add
	add.Dead = p.dead() // a body is shown lying until it is taken away
	switch add.Type {
	case protocol.EntityMonster:
		return [][]byte{protocol.EncodeCharacterAdd(add, mobAffects(p))}
	case protocol.EntityStone:
		return [][]byte{protocol.EncodeCharacterAdd(add, p.fight.smoke)}
	}
	return [][]byte{protocol.EncodeCharacterAdd(add, affectsOf(p.char)), protocol.EncodeCharacterInfo(p.info)}
}

func (p *present) isPlayer() bool { return p.out != nil }

// send queues packets for the player; a mob has nobody to tell.
func (p *present) send(packets ...[]byte) {
	if p.isPlayer() && len(packets) > 0 {
		_ = p.out.Send(packets...)
	}
}

// inView is whether p and o see each other: a player, and on the same map within range.
func (p *present) inView(o *present) bool {
	return (p.isPlayer() || o.isPlayer()) && p.world == o.world && within(p, o, viewRange)
}

func (l locked) link(a, b *present) {
	a.sees[b.add.VID] = b
	b.sees[a.add.VID] = a
}

// unlink parts a and b, and a player stops targeting what they no longer see.
func (l locked) unlink(a, b *present) {
	delete(a.sees, b.add.VID)
	delete(b.sees, a.add.VID)
	l.untarget(a, b)
	l.untarget(b, a)
}

// enter puts p in the world, and shows p and the others in its view to each other. The
// newcomer gets everyone in one send, so a crowd does not fill their queue.
func (w *record) enter(p *present) {
	l, unlock := w.lock()
	defer unlock()
	l.admit(p)
	if l.event != nil && p.isPlayer() {
		p.send(eventNotice(*l.event))
	}
}

// admit puts p in the world: everyone in view and p are shown to each other.
func (l locked) admit(p *present) {
	p.world = ""
	if m, ok := world.MapAt(p.add.X, p.add.Y); ok {
		p.world = m.Name
	}
	p.sees = map[uint32]*present{}
	var others [][]byte
	for _, other := range l.onMap[p.world] {
		if p.inView(other) {
			l.link(p, other)
			other.send(p.appearance()...)
			others = append(others, other.appearance()...)
		}
	}
	if p.isPlayer() {
		others = append(others, l.groundInView(p, false)...)
	}
	p.send(others...)
	l.all[p.add.VID] = p
	if l.onMap[p.world] == nil {
		l.onMap[p.world] = map[uint32]*present{}
	}
	l.onMap[p.world][p.add.VID] = p
	if p.isPlayer() {
		l.byName[strings.ToLower(p.info.Name)] = p
	}
}

// move keeps where the character stands, brings those who came into view and drops those
// who left it, on both sides, then shows the move to every player in view: someone just
// met gets the character before its move. A moving player gets all its changes in one
// send.
func (w *record) move(vid uint32, m protocol.Move) {
	l, unlock := w.lock()
	defer unlock()
	l.shift(l.all[vid], m)
}

// shift is move, under the lock.
func (l locked) shift(p *present, m protocol.Move) {
	vid := p.add.VID
	if p.dead() {
		return // a body lies where it fell
	}
	fromX, fromY := p.add.X, p.add.Y
	// Kept as the client draws it, so whoever meets the character later sees it where it
	// went and facing the way it last moved: the move counts in steps of 5 degrees.
	p.add.X, p.add.Y, p.add.Rotation = m.X, m.Y, float32(m.Rotation)*5
	var mine [][]byte
	if p.isPlayer() {
		mine = l.groundSeen(p, fromX, fromY)
	}
	if p.isPlayer() && len(l.trades) > 0 {
		l.walkedAway(p)
	}
	for _, other := range l.onMap[p.world] { // only those on its map can come into or leave its view
		if other == p {
			continue
		}
		_, seen := p.sees[other.add.VID]
		switch near := p.inView(other); {
		case near && !seen:
			l.link(p, other)
			other.send(p.appearance()...)
			mine = append(mine, other.appearance()...)
		case !near && seen:
			l.unlink(p, other)
			other.send(protocol.EncodeCharacterDelete(vid))
			mine = append(mine, protocol.EncodeCharacterDelete(other.add.VID))
		}
	}
	p.send(mine...)
	l.toViewers(p, protocol.EncodeMove(vid, m))
}

// say passes a packet from the player on to everyone who sees them.
func (w *record) say(vid uint32, packet []byte) {
	l, unlock := w.lock()
	defer unlock()
	l.toViewers(l.all[vid], packet)
}

// leave takes the player out of the world, and out of the view of everyone who sees them.
func (w *record) leave(vid uint32) *present {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[vid]
	l.remove(p)
	return p
}

// remove takes p out of the world, telling those who saw it.
func (l locked) remove(p *present) {
	vid := p.add.VID
	delete(l.all, vid)
	delete(l.onMap[p.world], vid)
	if p.isPlayer() {
		delete(l.byName, strings.ToLower(p.info.Name)) // a mob's name may be a player's too
		// The connection stays open through Change character and a restart: the client must
		// not keep these, nor be shown them twice on coming back.
		p.send(l.groundInView(p, true)...)
	}
	l.toViewers(p, protocol.EncodeCharacterDelete(vid))
	for _, other := range p.sees {
		l.unlink(p, other)
	}
}

// sendOff takes the monster out of the world, unless it has died there. It reports whether
// it was taken.
func (w *record) sendOff(p *present) bool {
	l, unlock := w.lock()
	defer unlock()
	if p.dead() {
		return false
	}
	l.remove(p)
	return true
}

// tellMap shows a notice to every player on the map p stands on.
func (w *record) tellMap(p *present, line string) {
	packet := notice(line) // outside the lock, to keep it short
	l, unlock := w.lock()
	defer unlock()
	for _, other := range l.byName {
		if other.world == p.world {
			other.send(packet)
		}
	}
}

// players returns everyone playing in the world now.
func (w *record) players() []*present {
	l, unlock := w.lock()
	defer unlock()
	all := make([]*present, 0, len(l.byName))
	for _, p := range l.byName {
		all = append(all, p)
	}
	return all
}

// snapshot copies what is saved of a player, if they are still in the world and alive:
// their progress and hit points, which change only under the lock, and where the world
// record has them. The rest of their character is their connection's, so it is not read
// here.
func (w *record) snapshot(p *present) (account.Character, bool) {
	l, unlock := w.lock()
	defer unlock()
	if l.all[p.add.VID] != p || p.dead() {
		return account.Character{}, false
	}
	c := p.char
	return account.Character{ID: c.ID, Name: c.Name, Level: c.Level, Exp: c.Exp, StatPoints: c.StatPoints,
		Stats: c.Stats, HP: c.HP, SP: c.SP, X: p.add.X, Y: p.add.Y}, true
}

// whisperOutcome is what became of a whisper.
type whisperOutcome int

const (
	whisperDelivered whisperOutcome = iota
	whisperNotFound                 // nobody of that name is in the world
	whisperRefused                  // they block whispers
)

// whisper sends the player named a packet, wherever they are in the world, unless they
// block whispers, and returns their character's id with what became of it.
func (w *record) whisper(name string, packet []byte) (uint32, whisperOutcome) {
	l, unlock := w.lock()
	defer unlock()
	p, ok := l.byName[strings.ToLower(name)]
	switch {
	case !ok:
		return 0, whisperNotFound
	case p.blocks.Has(account.BlockWhisper):
		return p.char.ID, whisperRefused
	}
	p.send(packet)
	return p.char.ID, whisperDelivered
}

// setBlocks records what the player now refuses, for whispers sent to them.
func (w *record) setBlocks(vid uint32, blocks account.Blocks) {
	l, unlock := w.lock()
	defer unlock()
	l.all[vid].blocks = blocks
}

func (l locked) toViewers(p *present, packet []byte) {
	for _, other := range p.sees {
		other.send(packet)
	}
}

// showMob shows those who see the monster its effects now.
func (l locked) showMob(v *present) {
	l.toViewers(v, protocol.EncodeCharacterUpdate(v.info, v.add, mobAffects(v)))
}

// A tether is where the server reckons a player's character can be, and since when: it
// follows their moves half as fast again as the client runs (4.5 units a second for each
// point of move speed), banking at most tetherCredit unspent, and a move may land within
// tetherLength of it, the 1000 a run reports ahead either way and 500 for lateness
// (docs/mechanics/other-players.md). A zero time is where the server put the character.
type tether struct {
	x, y int32
	at   time.Time
}

const (
	tetherLength = 2500
	tetherCredit = 2 * time.Second
)

// walk takes the player's move if it lands within their tether, and shows it to the
// others; one further is not taken: the character is put where the tether has gone to, for
// the others and, with SYNC_POSITION, for the player's own client. It returns where the
// character is now. The dead stay where they fell.
func (w *record) walk(vid uint32, m protocol.Move, now time.Time) (int32, int32) {
	l, unlock := w.lock()
	defer unlock()
	p := l.all[vid]
	if p.dead() {
		return p.add.X, p.add.Y
	}
	p.tether = p.tether.towards(m.X, m.Y, 1.5*4.5*float64(p.add.MoveSpeed), now)
	t := p.tether
	if apart(m.X, m.Y, t.x, t.y) <= tetherLength*tetherLength {
		l.shift(p, m)
		return m.X, m.Y
	}
	l.shift(p, protocol.Move{Type: protocol.MoveWait, Rotation: m.Rotation, X: t.x, Y: t.y, Time: m.Time})
	p.send(protocol.EncodeSyncPosition(vid, t.x, t.y))
	return t.x, t.y
}

// towards is the tether followed towards x, y at rate units a second for the time it has
// banked since it last moved, up to tetherCredit. Reaching the point spends only the time
// that took, so the moves of a burst after a stall share it.
func (t tether) towards(x, y int32, rate float64, now time.Time) tether {
	if t.at.IsZero() {
		return tether{x: t.x, y: t.y, at: now}
	}
	banked := min(now.Sub(t.at), tetherCredit)
	dx, dy := float64(x)-float64(t.x), float64(y)-float64(t.y)
	far := math.Hypot(dx, dy)
	if needed := time.Duration(far / rate * float64(time.Second)); needed <= banked {
		return tether{x: x, y: y, at: now.Add(needed - banked)}
	}
	k := rate * banked.Seconds() / far
	return tether{x: t.x + int32(dx*k), y: t.y + int32(dy*k), at: now}
}
