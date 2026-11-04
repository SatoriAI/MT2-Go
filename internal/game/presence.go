package game

import (
	"strings"
	"sync"

	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// viewRange is how far a player sees other characters: open-mt2's choice, since the
// client takes any. It draws characters up to 20000 away, so all of these are drawn.
const viewRange = 8000

// presence is who stands in the world, and who sees whom: two characters on the same map
// within viewRange of each other. Packets go out under the lock, so every player is told
// of the same things in the same order: a character always appears before its moves.
// Sending only queues, so the lock is never held across a slow client. A send that fails
// is left: that connection closes itself.
type presence struct {
	mu      sync.Mutex
	players map[uint32]*present // by vid
	byName  map[string]*present // by lower-cased name: names are unique regardless of case
}

// present is one player in the world, as the others see them.
type present struct {
	conn      *network.Conn
	character uint32 // the stored character's id; the vid lasts one login
	world     string // the map's name; "" for a position on no known map, which those share
	add       protocol.CharacterAdd
	info      protocol.CharacterInfo
	// sees is who this player has been shown, by vid. Seeing is mutual, so both sides
	// change together, and leaving tells exactly these.
	sees map[uint32]*present
}

// appearance is what shows the character to another client. The client builds a player
// only when its info follows its add, so the two go out as one send.
func (p *present) appearance() [][]byte {
	return [][]byte{protocol.EncodeCharacterAdd(p.add), protocol.EncodeCharacterInfo(p.info)}
}

func (p *present) inView(o *present) bool {
	dx, dy := int64(p.add.X)-int64(o.add.X), int64(p.add.Y)-int64(o.add.Y)
	return p.world == o.world && dx*dx+dy*dy <= viewRange*viewRange
}

func link(a, b *present) {
	a.sees[b.add.VID] = b
	b.sees[a.add.VID] = a
}

func unlink(a, b *present) {
	delete(a.sees, b.add.VID)
	delete(b.sees, a.add.VID)
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
	for _, other := range w.players {
		if p.inView(other) {
			link(p, other)
			_ = other.conn.Send(p.appearance()...)
			others = append(others, other.appearance()...)
		}
	}
	if others != nil {
		_ = p.conn.Send(others...)
	}
	w.players[p.add.VID] = p
	w.byName[strings.ToLower(p.info.Name)] = p
}

// move keeps where the player stands, brings those who came into view and drops those who
// left it, on both sides, then shows the move to everyone in view: someone just met gets
// the character before its move. The mover gets all its changes in one send.
func (w *presence) move(vid uint32, m protocol.Move) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[vid]
	p.add.X, p.add.Y = m.X, m.Y
	var mine [][]byte
	for _, other := range w.players {
		if other == p {
			continue
		}
		_, seen := p.sees[other.add.VID]
		switch near := p.inView(other); {
		case near && !seen:
			link(p, other)
			_ = other.conn.Send(p.appearance()...)
			mine = append(mine, other.appearance()...)
		case !near && seen:
			unlink(p, other)
			_ = other.conn.Send(protocol.EncodeCharacterDelete(vid))
			mine = append(mine, protocol.EncodeCharacterDelete(other.add.VID))
		}
	}
	if mine != nil {
		_ = p.conn.Send(mine...)
	}
	toViewers(p, protocol.EncodeMove(vid, m))
}

// say passes a packet from the player on to everyone who sees them.
func (w *presence) say(vid uint32, packet []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	toViewers(w.players[vid], packet)
}

// leave takes the player out of the world, and out of the view of everyone who sees them.
func (w *presence) leave(vid uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[vid]
	delete(w.players, vid)
	delete(w.byName, strings.ToLower(p.info.Name))
	toViewers(p, protocol.EncodeCharacterDelete(vid))
	for _, other := range p.sees {
		unlink(p, other)
	}
}

// whisper sends the player named a packet, wherever they are in the world, and returns
// their character's id: false if nobody of that name is in the world.
func (w *presence) whisper(name string, packet []byte) (uint32, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p, ok := w.byName[strings.ToLower(name)]
	if !ok {
		return 0, false
	}
	_ = p.conn.Send(packet)
	return p.character, true
}

func toViewers(p *present, packet []byte) {
	for _, other := range p.sees {
		_ = other.conn.Send(packet)
	}
}
