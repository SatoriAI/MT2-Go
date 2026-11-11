package game

import (
	"context"
	"log/slog"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// saveTimeout bounds one save of a character: its position, progress and hit points.
const saveTimeout = 5 * time.Second

// SavePlayers saves every player in the world every so often until ctx ends, so a crash
// loses at most that much of their play. Leaving the world saves too, and it runs as the
// connection closes, so nothing is lost when ctx ends at shutdown.
func (svc *Service) SavePlayers(ctx context.Context, every time.Duration, log *slog.Logger) error {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			for _, p := range svc.record.players() {
				if ctx.Err() != nil {
					return nil // at shutdown each player is saved on leaving, without waiting on this round
				}
				svc.saveInWorld(p, log)
			}
		}
	}
}

// saveInWorld saves a player as they stand now, if they are still in the world; one who
// left was saved on leaving. One lying dead is not saved until they restart or leave.
// Holding p.saving keeps this save and the one on leaving in order, so an older snapshot
// never lands after the last.
func (svc *Service) saveInWorld(p *present, log *slog.Logger) {
	p.saving.Lock()
	defer p.saving.Unlock()
	if c, ok := svc.record.snapshot(p); ok {
		svc.save(c, log, slog.LevelDebug) // warned once, on leaving, not every minute
	}
}

// leaveAndSave takes the player out of the world by out, after which nobody else touches the
// character, and saves it once any save of it already under way has landed. The position
// is the world record's, as the periodic save's is, so the two never disagree. A player
// who leaves dead is saved as a restart in town would leave them: at their kingdom's
// start, with all their hit points and mana.
func (svc *Service) leaveAndSave(out func(vid uint32) *present, c *character, empire account.Empire, log *slog.Logger) {
	p := out(c.vid)
	p.saving.Lock()
	defer p.saving.Unlock()
	saved := c.Character
	saved.X, saved.Y = p.add.X, p.add.Y
	if p.dead() {
		if x, y, ok := empire.StartPosition(); ok {
			saved.X, saved.Y = x, y
		}
		points, _ := saved.Points()
		saved.HP, saved.SP = points.MaxHP, points.MaxSP
	}
	svc.save(saved, log, slog.LevelWarn)
}

// save keeps the character's progress, and where it stands if that is on a map the server
// knows; otherwise the last good position stays, logged at offMap. It has its own
// context, since at shutdown it runs after the server's is cancelled.
func (svc *Service) save(c account.Character, log *slog.Logger, offMap slog.Level) {
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if _, ok := world.MapAt(c.X, c.Y); !ok {
		log.Log(ctx, offMap, "position not saved: on no known map", "character", c.Name, "x", c.X, "y", c.Y)
	} else if err := svc.accounts.SavePosition(ctx, c.ID, c.X, c.Y); err != nil {
		log.Error("position not saved", "character", c.Name, "err", err)
	}
	if err := svc.accounts.SaveProgress(ctx, c); err != nil {
		log.Error("progress not saved", "character", c.Name, "err", err)
	}
}
