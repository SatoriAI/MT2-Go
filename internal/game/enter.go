package game

import (
	"context"
	"errors"
	"fmt"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

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

// onSelectAgain tolerates the client asking for the same character a second time while it
// loads; any other character then is not something the client does.
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
// phase, the clock and channel, then the player's own character.
func (s *session) onEnterGame() error {
	p := s.playing
	s.phase = protocol.PhaseGame
	s.conn.Log().Info("entered the world", "character", p.Name, "x", p.X, "y", p.Y)
	return s.conn.Send(
		protocol.EncodePhase(protocol.PhaseGame),
		protocol.EncodeTime(s.svc.clock.Now()),
		protocol.EncodeChannel(1),
		protocol.EncodeCharacterAdd(protocol.CharacterAdd{
			VID:         p.vid,
			X:           p.X,
			Y:           p.Y,
			Type:        protocol.EntityPlayer,
			Race:        uint16(raceNumber(p.Class, p.Sex)),
			MoveSpeed:   p.points.MoveSpeed,
			AttackSpeed: p.points.AttackSpeed,
		}),
		protocol.EncodeCharacterInfo(protocol.CharacterInfo{
			VID:      p.vid,
			Name:     p.Name,
			BodyPart: uint16(p.Shape),
			HairPart: p.Hair,
			Empire:   empireNumbers[s.empire],
			Level:    uint32(p.Level),
		}),
	)
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
