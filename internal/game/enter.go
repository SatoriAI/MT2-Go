package game

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/world"
)

// saveTimeout bounds saving a character when its connection closes.
const saveTimeout = 5 * time.Second

// character is the one this connection plays, once Start was pressed.
type character struct {
	account.Character
	vid    uint32
	points account.Points
}

// onSelectCharacter starts loading the world for the character in the slot the player
// pressed Start on. The client asks only for a slot it shows a character in, so an empty
// one ends the connection.
func (s *session) onSelectCharacter(ctx context.Context, body []byte) error {
	slot, err := protocol.DecodeSelectCharacter(body)
	if err != nil {
		return err
	}
	c, err := s.svc.store.Character(ctx, s.accountID, int(slot))
	if errors.Is(err, account.ErrNotFound) {
		return fmt.Errorf("game: no character in slot %d", slot)
	}
	if err != nil {
		return err
	}
	claim, displaced, err := s.svc.claimCharacter(ctx, c.ID, s.conn.Close)
	if err != nil {
		return err
	}
	s.claim = claim
	s.leftWorld = false
	if displaced {
		s.conn.Log().Info("took the character over from another connection", "character", c.Name)
		// The displaced connection has just saved where the character stood.
		if c, err = s.svc.store.Character(ctx, s.accountID, int(slot)); err != nil {
			return err
		}
	}
	points, ok := c.Points()
	if !ok {
		return fmt.Errorf("game: no points for class %q", c.Class)
	}
	s.playing = &character{Character: c, vid: s.svc.lastVID.Add(1), points: points}
	s.phase = protocol.PhaseLoading
	s.conn.Log().Info("loading", "character", c.Name, "vid", s.playing.vid)
	return s.conn.Send(
		protocol.EncodePhase(protocol.PhaseLoading),
		protocol.EncodeMainCharacter(protocol.MainCharacter{
			VID:        s.playing.vid,
			Race:       uint16(raceNumber(c.Class, c.Sex)),
			Name:       c.Name,
			X:          c.X,
			Y:          c.Y,
			Empire:     empireNumbers[s.empire],
			SkillGroup: skillGroupNumber(c.Class, c.SkillGroup),
		}),
		protocol.EncodePoints(pointsOf(s.playing)),
		protocol.EncodeSkillLevels(),
	)
}

// onSelectAgain accepts the second request for the same character, which the loading
// screen always sends (introloading.py); any other character then is not something the
// client does.
func (s *session) onSelectAgain(body []byte) error {
	slot, err := protocol.DecodeSelectCharacter(body)
	if err != nil {
		return err
	}
	if int(slot) != s.playing.Slot {
		return fmt.Errorf("game: slot %d selected while loading slot %d", slot, s.playing.Slot)
	}
	return nil
}

// onVersion records which client build is loading; nothing depends on it yet.
func (s *session) onVersion(body []byte) error {
	v, err := protocol.DecodeVersion(body)
	if err != nil {
		return err
	}
	s.conn.Log().Info("client version", "name", v.Name, "timestamp", v.Timestamp)
	return nil
}

// onEnterGame puts the player in the world once the client has loaded the map: the game
// phase, the clock and channel, then the player's own character and what they block, then
// the others in view. The client drops every other character when its own is added, so
// it goes first.
func (s *session) onEnterGame() error {
	p := s.playing
	me := &present{
		out:       s.conn,
		character: p.ID,
		add: protocol.CharacterAdd{
			VID:         p.vid,
			X:           p.X,
			Y:           p.Y,
			Type:        protocol.EntityPlayer,
			Race:        uint16(raceNumber(p.Class, p.Sex)),
			MoveSpeed:   p.points.MoveSpeed,
			AttackSpeed: p.points.AttackSpeed,
		},
		blocks: p.Blocks,
		info: protocol.CharacterInfo{
			VID:      p.vid,
			Name:     p.Name,
			BodyPart: uint16(p.Shape),
			HairPart: p.Hair,
			Empire:   empireNumbers[s.empire],
			Level:    uint32(p.Level),
		},
	}
	err := s.conn.Send(append([][]byte{
		protocol.EncodePhase(protocol.PhaseGame),
		protocol.EncodeTime(s.svc.clock.Now()),
		protocol.EncodeChannel(1),
	}, append(me.appearance(), s.blockMode())...)...)
	if err != nil {
		return err
	}
	// Only now is the player in the game phase: leaving the world takes them out of the
	// record, so they must be in it.
	s.svc.presence.enter(me)
	s.phase = protocol.PhaseGame
	s.conn.Log().Info("entered the world", "character", p.Name, "x", p.X, "y", p.Y)
	return nil
}

// onMove keeps where the player's character is, to be saved when the connection closes,
// and shows the move to the others; the client moves its own character by itself.
func (s *session) onMove(body []byte) error {
	m, err := protocol.DecodeMove(body)
	if err != nil {
		return err
	}
	s.playing.X, s.playing.Y = m.X, m.Y
	s.svc.presence.move(s.playing.vid, m)
	s.conn.Log().Debug("moved", "character", s.playing.Name, "type", m.Type, "x", m.X, "y", m.Y)
	return nil
}

// pointsOf fills the client's points array for the character, at full hit points and
// mana.
func pointsOf(p *character) protocol.CharacterPoints {
	return protocol.CharacterPoints{
		Level:       uint32(p.Level),
		NextExp:     p.points.NextExp,
		HP:          p.points.MaxHP,
		MaxHP:       p.points.MaxHP,
		SP:          p.points.MaxSP,
		MaxSP:       p.points.MaxSP,
		Stamina:     p.points.MaxStamina,
		MaxStamina:  p.points.MaxStamina,
		ST:          uint32(p.Strength),
		HT:          uint32(p.Vitality),
		DX:          uint32(p.Dexterity),
		IQ:          uint32(p.Intelligence),
		AttackSpeed: uint32(p.points.AttackSpeed),
		MoveSpeed:   uint32(p.points.MoveSpeed),
	}
}

// savePosition keeps where the character stood for its next login, if that is on a map the
// server knows; otherwise the last good position stays. It runs as the connection closes,
// which at shutdown is after the server's context is cancelled, so the save has its own.
func (s *session) savePosition() {
	p := s.playing
	if _, ok := world.MapAt(p.X, p.Y); !ok {
		s.conn.Log().Warn("position not saved: on no known map", "character", p.Name, "x", p.X, "y", p.Y)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), saveTimeout)
	defer cancel()
	if err := s.svc.store.SavePosition(ctx, p.ID, p.X, p.Y); err != nil {
		s.conn.Log().Error("position not saved", "character", p.Name, "err", err)
	}
}

// leaveWorld takes the character in play out of the others' view and saves it, if it
// entered the world, and lets go of it.
func (s *session) leaveWorld() {
	if s.claim == nil {
		return
	}
	if s.phase == protocol.PhaseGame {
		s.svc.presence.leave(s.playing.vid)
		s.savePosition()
	}
	s.svc.release(s.claim)
	s.claim = nil
	s.playing = nil
}
