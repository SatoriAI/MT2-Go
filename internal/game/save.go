package game

import (
	"context"
	"log/slog"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// saveTimeout bounds one save of a character: its position and progress.
const saveTimeout = 5 * time.Second

// SavePlayers saves every player in the world every so often until ctx ends, so a crash
// loses at most that much of their play. Leaving the world saves too, and it runs as the
// connection closes, so nothing is lost when ctx ends at shutdown.
func (s *Service) SavePlayers(ctx context.Context, every time.Duration, log *slog.Logger) error {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			for _, p := range s.presence.players() {
				s.saveInWorld(p, log)
			}
		}
	}
}

// saveInWorld saves a player as they stand now, if they are still in the world; one who
// left was saved on leaving. Holding p.saving keeps this save and the one on leaving in
// order, so an older snapshot never lands after the last.
func (s *Service) saveInWorld(p *present, log *slog.Logger) {
	p.saving.Lock()
	defer p.saving.Unlock()
	if c, ok := s.presence.snapshot(p); ok {
		s.save(c, log)
	}
}

// leaveAndSave takes the player out of the world, after which nobody else touches the
// character, and saves it once any save of it already under way has landed.
func (s *Service) leaveAndSave(c *character, log *slog.Logger) {
	p := s.presence.leave(c.vid)
	p.saving.Lock()
	defer p.saving.Unlock()
	s.save(c.Character, log)
}

// save keeps the character's progress, and where it stands if that is on a map the server
// knows; otherwise the last good position stays. It has its own context, since at
// shutdown it runs after the server's is cancelled.
func (s *Service) save(c account.Character, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if _, ok := world.MapAt(c.X, c.Y); !ok {
		log.Warn("position not saved: on no known map", "character", c.Name, "x", c.X, "y", c.Y)
	} else if err := s.store.SavePosition(ctx, c.ID, c.X, c.Y); err != nil {
		log.Error("position not saved", "character", c.Name, "err", err)
	}
	if err := s.store.SaveProgress(ctx, c); err != nil {
		log.Error("progress not saved", "character", c.Name, "err", err)
	}
}
