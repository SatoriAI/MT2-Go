package game

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// onSetBlockMode keeps what the options window now blocks, the client's whole mask, and
// tells the client, which shows the buttons pressed only from the reply. A mask that is
// not a number is ignored, as an unknown command is.
func (s *session) onSetBlockMode(arg string) error {
	mask, err := strconv.ParseUint(arg, 10, 32)
	if err != nil {
		s.conn.Log().Debug("block mode ignored", "arg", arg)
		return nil
	}
	// Unlike leaving, this writes to the database, so it counts against the flood limit;
	// a change dropped leaves the buttons as they were.
	if !s.flood.allow(time.Now()) {
		s.conn.Log().Debug("block mode dropped: too fast", "character", s.playing.Name)
		return nil
	}
	p := s.playing
	blocks := blocksFromMask(uint32(mask))
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := s.svc.store.SaveBlocks(ctx, p.ID, blocks); err != nil {
		// The buttons go back to what is kept.
		s.conn.Log().Error("blocks not saved", "character", p.Name, "err", err)
		return s.conn.Send(s.blockMode())
	}
	p.Blocks = blocks
	s.svc.presence.setBlocks(p.vid, blocks)
	return s.conn.Send(s.blockMode())
}

// blockMode tells the client what the player blocks, as the command its options window
// reads.
func (s *session) blockMode() []byte {
	text := fmt.Sprintf("setblockmode %d", blockMask(s.playing.Blocks))
	return protocol.EncodeChat(protocol.ChatCommand, s.playing.vid, empireNumbers[s.empire], text)
}
