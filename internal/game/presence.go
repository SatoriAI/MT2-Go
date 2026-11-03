package game

import (
	"sync"

	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// presence is who stands in the world. Everyone on a map sees everyone else there; a view
// range comes later. Packets go out under the lock, so every player is told of the same
// things in the same order: a character always appears before its moves. Sending only
// queues, so the lock is never held across a slow client. A send that fails is left: that
// connection closes itself.
type presence struct {
	mu      sync.Mutex
	players map[uint32]*present // by vid
}

// present is one player in the world, as the others see them.
type present struct {
	conn  *network.Conn
	world string // the map's name; "" for a position on no known map
	add   protocol.CharacterAdd
	info  protocol.CharacterInfo
}

// appearance is what shows the character to another client. The client builds a player
// only when its info follows its add, so the two go out as one send.
func (p *present) appearance() [][]byte {
	return [][]byte{protocol.EncodeCharacterAdd(p.add), protocol.EncodeCharacterInfo(p.info)}
}

// enter puts p in the world, and shows p and the others on its map to each other.
func (w *presence) enter(p *present) {
	if m, ok := world.MapAt(p.add.X, p.add.Y); ok {
		p.world = m.Name
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, other := range w.players {
		if other.world == p.world {
			_ = other.conn.Send(p.appearance()...)
			_ = p.conn.Send(other.appearance()...)
		}
	}
	w.players[p.add.VID] = p
}

// move keeps where the player stands, for those who arrive later, and passes the move on.
func (w *presence) move(vid uint32, m protocol.Move) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[vid]
	p.add.X, p.add.Y = m.X, m.Y
	w.toOthers(p, protocol.EncodeMove(vid, m))
}

// say passes a packet from the player on to everyone who sees them.
func (w *presence) say(vid uint32, packet []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.toOthers(w.players[vid], packet)
}

// leave takes the player out of the world, and out of the others' view.
func (w *presence) leave(vid uint32) {
	w.mu.Lock()
	defer w.mu.Unlock()
	p := w.players[vid]
	delete(w.players, vid)
	w.toOthers(p, protocol.EncodeCharacterDelete(vid))
}

func (w *presence) toOthers(p *present, packet []byte) {
	for _, other := range w.players {
		if other != p && other.world == p.world {
			_ = other.conn.Send(packet)
		}
	}
}
