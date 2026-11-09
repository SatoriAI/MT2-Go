package game

import (
	"context"
	"errors"
	"time"
)

// releaseWait bounds how long Start waits for a displaced connection to save and let go.
// That connection only starts saving once it is closed, and may first wait for a periodic
// save of the character under way, so the wait must outlast two saves.
const releaseWait = 2*saveTimeout + time.Second

// claim is one connection's hold on a character in play. done closes once that connection
// has let go of the character, and never before every claim it displaced has: a closed
// done means the saved position is the latest.
type claim struct {
	character uint32
	stop      func() // closes the holding connection
	done      chan struct{}
}

// claimCharacter gives the character to the connection stop closes, displacing any
// connection that held it: that one is stopped, and claimCharacter returns once it has
// saved, reporting whether there was one, so the caller knows to read the character
// again. Each character is in play on one connection at most.
func (svc *Service) claimCharacter(ctx context.Context, id uint32, stop func()) (mine *claim, displaced bool, err error) {
	mine = &claim{character: id, stop: stop, done: make(chan struct{})}
	svc.mu.Lock()
	previous := svc.claims[id]
	svc.claims[id] = mine
	svc.mu.Unlock()
	if previous == nil {
		return mine, false, nil
	}
	previous.stop()
	select {
	case <-previous.done:
		return mine, true, nil
	case <-ctx.Done():
		err = ctx.Err()
	case <-time.After(releaseWait):
		err = errors.New("game: the character's previous connection did not let go")
	}
	// Giving up must not let a later claim past the one still saving.
	given := mine
	go func() {
		<-previous.done
		svc.release(given)
	}()
	return nil, false, err
}

// release lets go of the character, unless another connection has claimed it since.
func (svc *Service) release(c *claim) {
	svc.mu.Lock()
	if svc.claims[c.character] == c {
		delete(svc.claims, c.character)
	}
	svc.mu.Unlock()
	close(c.done)
}
