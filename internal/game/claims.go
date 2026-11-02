package game

import (
	"errors"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/network"
)

// releaseWait bounds how long Start waits for a displaced connection to save and let go.
const releaseWait = 5 * time.Second

// claim is one connection's hold on a character in play. done closes once that
// connection has saved the character and let go of it.
type claim struct {
	character uint32
	conn      *network.Conn
	done      chan struct{}
}

// claimCharacter gives conn the character, displacing any connection that held it: that
// one is closed, and claimCharacter returns once it has saved, so what is read next is
// what it left. Each character is in play on one connection at most.
func (s *Service) claimCharacter(id uint32, conn *network.Conn) (*claim, error) {
	mine := &claim{character: id, conn: conn, done: make(chan struct{})}
	s.mu.Lock()
	previous := s.claims[id]
	s.claims[id] = mine
	s.mu.Unlock()
	if previous == nil {
		return mine, nil
	}
	previous.conn.Log().Info("closing: the character was started on another connection")
	previous.conn.Close()
	select {
	case <-previous.done:
		return mine, nil
	case <-time.After(releaseWait):
		s.release(mine)
		return nil, errors.New("game: the character's previous connection did not let go")
	}
}

// release lets go of the character, unless another connection has claimed it since.
func (s *Service) release(c *claim) {
	s.mu.Lock()
	if s.claims[c.character] == c {
		delete(s.claims, c.character)
	}
	s.mu.Unlock()
	close(c.done)
}
