package game

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// maxWhisper is the longest line the client sends: it refuses 255 bytes or more.
const maxWhisper = 254

var errWhisperTooLong = errors.New("game: whisper longer than the client sends")

// notSaved is what the sender reads when their whisper was delivered but not kept.
const notSaved = "Not saved."

// onWhisper passes a line to the player named, wherever they are in the world, and keeps
// it. The sender's client shows its own line as it sends it, so nothing is echoed, and a
// line to oneself is left at that.
func (s *session) onWhisper(body []byte) error {
	w, err := protocol.DecodeWhisper(body)
	if err != nil {
		return err
	}
	if len(w.Text) > maxWhisper {
		return errWhisperTooLong
	}
	p := s.playing
	if w.Text == "" || strings.EqualFold(w.To, p.Name) {
		return nil
	}
	// The line's time is taken before it can reach the recipient, so a reply to it is
	// later, whichever session saves first.
	now := time.Now()
	// A whisper over the limit gets no reply at all, so a flood cannot be turned into
	// replies.
	if !s.flood.allow(now) {
		s.conn.Log().Debug("whisper dropped: talking too fast", "character", p.Name)
		return nil
	}
	if p.Blocks.Has(account.BlockWhisper) {
		return s.conn.Send(protocol.EncodeWhisper(protocol.WhisperSenderBlocked, w.To, ""))
	}
	recipient, outcome := s.svc.presence.whisper(w.To, protocol.EncodeWhisper(protocol.WhisperLine, p.Name, w.Text))
	switch outcome {
	case whisperNotFound:
		return s.conn.Send(protocol.EncodeWhisper(protocol.WhisperNotOnline, w.To, ""))
	case whisperRefused:
		return s.conn.Send(protocol.EncodeWhisper(protocol.WhisperTargetBlocked, w.To, ""))
	}
	// The line is delivered, so it is kept even if the server is stopping: the save has
	// its own time, as a position's does.
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := s.svc.whispers.Save(ctx, p.ID, recipient, textFromClient(w.Text), now); err != nil {
		s.conn.Log().Error("whisper not saved", "from", p.Name, "to", w.To, "err", err)
		return s.conn.Send(protocol.EncodeWhisper(protocol.WhisperError, w.To, notSaved))
	}
	return nil
}
