package game

import (
	"context"
	"strings"

	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// onChat answers a line typed in the world, or a command a button or menu sent as one.
// Nobody else is in the world yet, so a line goes back to its speaker only.
func (s *session) onChat(ctx context.Context, body []byte) error {
	c, err := protocol.DecodeChat(body)
	if err != nil {
		return err
	}
	if command, ok := strings.CutPrefix(c.Text, "/"); ok {
		return s.onCommand(ctx, command)
	}
	p := s.playing
	return s.conn.Send(protocol.EncodeChat(protocol.ChatNormal, p.vid, empireNumbers[s.empire], p.Name+" : "+c.Text))
}

// onCommand carries out the commands the system menu sends: exit, log out and change
// character. They act at once; the original's countdown guards against leaving a fight,
// and there is no fighting yet. Every other command, and the buttons send many, is ignored.
func (s *session) onCommand(ctx context.Context, command string) error {
	switch name, _, _ := strings.Cut(command, " "); name {
	case "quit":
		// The client closes itself when told; the session saves as the connection ends.
		return s.conn.Send(protocol.EncodeChat(protocol.ChatCommand, s.playing.vid, empireNumbers[s.empire], "quit"))
	case "logout":
		if err := s.conn.Send(protocol.EncodePhase(protocol.PhaseClose)); err != nil {
			return err
		}
		return network.ErrHangUp
	case "phase_select":
		s.leaveWorld()
		return s.enterSelection(ctx)
	default:
		s.conn.Log().Debug("command ignored", "command", name)
		return nil
	}
}
