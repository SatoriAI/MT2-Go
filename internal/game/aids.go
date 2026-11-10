package game

import (
	"context"
	"errors"
	"slices"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/item"
)

// aidHeld is what a player using a second of an aid held is told.
const aidHeld = "Ten efekt jest już aktywny."

// takeAid uses one of the item up for the aid it leaves, kept on the character and its
// icon shown. A second of an aid held is refused with a line, and the item kept; one the
// store holds but the world record did not is taken from the store, and shown.
func (s *session) takeAid(ctx context.Context, it item.Item, aid account.Aid) error {
	if s.svc.record.hasAid(s.playing.vid, aid) {
		return s.conn.Send(infoLine(aidHeld))
	}
	left := it
	left.Count--
	switch err := s.svc.skills.TakeAid(ctx, s.playing.ID, aid, left); {
	case errors.Is(err, account.ErrAidHeld): // the store holds it, and the store is right
		s.svc.record.aided(s.playing.vid, aid)
		return s.conn.Send(aidShown(aid), infoLine(aidHeld))
	case err != nil:
		s.conn.Log().Error("aid not kept", "item", it.ID, "err", err)
		return s.conn.Send(itemSet(it))
	}
	s.svc.record.aided(s.playing.vid, aid)
	return s.showUsed(it, left, aidShown(aid))
}

// aidsShown are the icons of the aids the character holds, for entering the world.
func (s *session) aidsShown() [][]byte {
	shown := make([][]byte, 0, len(s.playing.Aids))
	for _, aid := range s.playing.Aids {
		shown = append(shown, aidShown(aid))
	}
	return shown
}

// hasAid is whether the player holds the aid.
func (w *record) hasAid(vid uint32, aid account.Aid) bool {
	l, unlock := w.lock()
	defer unlock()
	return l.all[vid].char.Aids.Has(aid)
}

// aided gives the player the aid.
func (w *record) aided(vid uint32, aid account.Aid) {
	l, unlock := w.lock()
	defer unlock()
	c := l.all[vid].char
	c.Aids = append(c.Aids, aid)
}

// spend takes the aids a read spent off the player.
func (l locked) spend(c *account.Character, spent account.Aids) {
	c.Aids = slices.DeleteFunc(c.Aids, spent.Has)
}
