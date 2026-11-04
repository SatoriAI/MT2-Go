package game

import (
	"strings"
	"sync"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// viewRange is how far a player sees other characters: open-mt2's choice, since the
// client takes any. It draws characters up to 20000 away, so all of these are drawn.
const viewRange = 8000

// presence is who stands in the world, players and mobs, and who sees whom: a player and
// anything else on the same map within viewRange of each other. Mobs see no mobs, so the
// many standing in the fields cost nothing between themselves. Packets go out under the
// lock, so every player is told of the same things in the same order: a character always
// appears before its moves. Sending only queues, so the lock is never held across a slow
// client. A send that fails is left: that connection closes itself.
type presence struct {
	mu     sync.Mutex
	all    map[uint32]*present // by vid
	byName map[string]*present // players, by lower-cased name: unique regardless of case
	fallen []death             // monsters killed since the world loop last asked
}

// sender is where a player's packets go: their connection, which only queues them.
type sender interface {
	Send(packets ...[]byte) error
}

// present is one character in the world, as the players who see it see it.
type present struct {
	out       sender // the player's connection; nil for a mob
	character uint32 // the stored character's id; the vid lasts one login
	world     string // the map's name; "" for a position on no known map, which those share
	add       protocol.CharacterAdd
	info      protocol.CharacterInfo
	blocks    account.Blocks // what they refuse from others, as they last chose
	target    *present       // the monster the player has picked, while they see it
	// char is the player's character. While the player is in the world, its level,
	// experience and stats change only under the record's lock: others' kills raise them.
	char  *account.Character
	fight *fight // a monster's hit points; nil for those that cannot be fought
	// sees is who this player has been shown, by vid. Seeing is mutual, so both sides
	// change together, and leaving tells exactly these.
	sees map[uint32]*present
}

// appearance is what shows the character to another client. The client builds a player
// or an NPC only when its info follows its add, so the two go out as one send; it builds
// a monster from the add alone, naming it from its own mob data, and drops an info for one.
func (p *present) appearance() [][]byte {
	add := p.add
	add.Dead = p.dead() // a body is shown lying until it is taken away
	if add.Type == protocol.EntityMonster {
		return [][]byte{protocol.EncodeCharacterAdd(add)}
	}
	return [][]byte{protocol.EncodeCharacterAdd(add), protocol.EncodeCharacterInfo(p.info)}
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
	dx, dy := int64(p.add.X)-int64(o.add.X), int64(p.add.Y)-int64(o.add.Y)
	return (p.isPlayer() || o.isPlayer()) && p.world == o.world && dx*dx+dy*dy <= viewRange*viewRange
}

func link(a, b *present) {
	a.sees[b.add.VID] = b
	b.sees[a.add.VID] = a
}

// unlink parts a and b, and a player stops targeting what they no longer see.
func unlink(a, b *present) {
	delete(a.sees, b.add.VID)
	delete(b.sees, a.add.VID)
	untarget(a, b)
	untarget(b, a)
}

// enter puts p in the world, and shows p and the others in its view to each other. The
// newcomer gets everyone in one send, so a crowd does not fill their queue.
func (w *presence) enter(p *present) {
	if m, ok := world.MapAt(p.add.X, p.add.Y); ok {
		p.world = m.Name
	}
	p.sees = map[uint32]*present{}
	w.mu.Lock()
	defer w.mu.Unlock()
	var others [][]byte
	for _, other := range w.all {
		if p.inView(other) {
			link(p, other)
			other.send(p.appearance()...)
			others = append(others, other.appearance()...)
		}
	}
	p.send(others...)
	w.all[p.add.VID] = p
	if p.isPlayer() {
		w.byName[strings.ToLower(p.info.Name)] = p
	}
}

// move keeps where the character stands, brings those who came into view and drops those
// who left it, on both sides, then shows the move to every player in view: someone just
// met gets the character before its move. A moving player gets all its changes in one
// send.
func (w *presence) move(vid uint32, m protocol.Move) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[vid]
	if p.dead() {
		return // a body lies where it fell
	}
	// Kept as the client draws it, so whoever meets the character later sees it where it
	// went and facing the way it last moved: the move counts in steps of 5 degrees.
	p.add.X, p.add.Y, p.add.Rotation = m.X, m.Y, float32(m.Rotation)*5
	var mine [][]byte
	for _, other := range w.all {
		if other == p {
			continue
		}
		_, seen := p.sees[other.add.VID]
		switch near := p.inView(other); {
		case near && !seen:
			link(p, other)
			other.send(p.appearance()...)
			mine = append(mine, other.appearance()...)
		case !near && seen:
			unlink(p, other)
			other.send(protocol.EncodeCharacterDelete(vid))
			mine = append(mine, protocol.EncodeCharacterDelete(other.add.VID))
		}
	}
	p.send(mine...)
	toViewers(p, protocol.EncodeMove(vid, m))
}

// say passes a packet from the player on to everyone who sees them.
func (w *presence) say(vid uint32, packet []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	toViewers(w.all[vid], packet)
}

// leave takes the player out of the world, and out of the view of everyone who sees them.
func (w *presence) leave(vid uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.all[vid]
	delete(w.all, vid)
	if p.isPlayer() {
		delete(w.byName, strings.ToLower(p.info.Name)) // a mob's name may be a player's too
	}
	toViewers(p, protocol.EncodeCharacterDelete(vid))
	for _, other := range p.sees {
		unlink(p, other)
	}
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
func (w *presence) whisper(name string, packet []byte) (uint32, whisperOutcome) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.byName[strings.ToLower(name)]
	switch {
	case !ok:
		return 0, whisperNotFound
	case p.blocks.Has(account.BlockWhisper):
		return p.character, whisperRefused
	}
	_ = p.out.Send(packet)
	return p.character, whisperDelivered
}

// setBlocks records what the player now refuses, for whispers sent to them.
func (w *presence) setBlocks(vid uint32, blocks account.Blocks) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.all[vid].blocks = blocks
}

func toViewers(p *present, packet []byte) {
	for _, other := range p.sees {
		other.send(packet)
	}
}
