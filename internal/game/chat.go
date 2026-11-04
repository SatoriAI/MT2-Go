package game

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// maxChat is the longest line the client sends: it refuses 512 bytes or more.
const maxChat = 511

var errChatTooLong = errors.New("game: chat line longer than the client sends")

// onChat answers a line typed in the world, or a command a button or menu sent as one. A
// line goes to its speaker and everyone who sees them; the client shows nothing it types
// until the server sends it back.
func (s *session) onChat(body []byte) error {
	c, err := protocol.DecodeChat(body)
	if err != nil {
		return err
	}
	s.conn.Log().Debug("chat", "type", c.Type, "text", c.Text, "bytes", fmt.Sprintf("% x", c.Text))
	if len(c.Text) > maxChat {
		return errChatTooLong
	}
	if command, ok := strings.CutPrefix(c.Text, "/"); ok {
		return s.onCommand(command)
	}
	if c.Text == "" {
		return nil
	}
	if !s.flood.allow(time.Now()) {
		s.conn.Log().Debug("line dropped: talking too fast", "character", s.playing.Name)
		return nil
	}
	p := s.playing
	line := protocol.EncodeChat(protocol.ChatNormal, p.vid, empireNumbers[s.empire], p.Name+" : "+c.Text)
	s.svc.presence.say(p.vid, line)
	return s.conn.Send(line)
}

// onCommand carries out the commands the system menu sends: exit, log out and change
// character, and what the options window blocks. Leaving acts at once; the original's
// countdown guards against leaving a fight, and there is no fighting yet. Every other
// command, and the buttons send many, is ignored.
func (s *session) onCommand(command string) error {
	switch name, arg, _ := strings.Cut(command, " "); name {
	case "quit":
		// The client closes itself when told; the session saves as the connection ends.
		return s.conn.Send(protocol.EncodeChat(protocol.ChatCommand, s.playing.vid, empireNumbers[s.empire], "quit"))
	case "logout":
		if err := s.conn.Send(protocol.EncodePhase(protocol.PhaseClose)); err != nil {
			return err
		}
		return network.ErrHangUp
	case "setblockmode":
		return s.onSetBlockMode(arg)
	case "stat":
		s.svc.presence.spendStatPoint(s.playing.vid, arg)
		return nil
	case "phase_select":
		// The phase alone: the client, still in its game window, ignores a kingdom and
		// list sent ahead of it, and keeps the list it has.
		s.leaveWorld()
		s.leftWorld = true
		s.phase = protocol.PhaseSelect
		return s.conn.Send(protocol.EncodePhase(protocol.PhaseSelect))
	default:
		s.conn.Log().Debug("command ignored", "command", name)
		return nil
	}
}
