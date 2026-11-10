package game

import (
	"context"
	"errors"
	"fmt"
	"slices"

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
	c, err := s.svc.accounts.Character(ctx, s.accountID, int(slot))
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
		if c, err = s.svc.accounts.Character(ctx, s.accountID, int(slot)); err != nil {
			return err
		}
	}
	if err := s.loadInventory(ctx, c.ID); err != nil {
		return err
	}
	if err := s.loadTaskbar(ctx, c.ID); err != nil {
		return err
	}
	if c.Skills, c.SkillReads, err = s.svc.skills.Learnt(ctx, c.ID); err != nil {
		return err
	}
	wear(&c, s.inventory)
	points, ok := c.Points()
	if !ok {
		return fmt.Errorf("game: no points for class %q", c.Class)
	}
	c.Enter(points)
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
		protocol.EncodePoints(pointsOf(s.playing, s.gold)),
		protocol.EncodeSkillLevels(skillLevels(c.Skills)),
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
		out:  s.conn,
		char: &p.Character,
		add: protocol.CharacterAdd{
			VID:         p.vid,
			X:           p.X,
			Y:           p.Y,
			Type:        protocol.EntityPlayer,
			Race:        uint16(raceNumber(p.Class, p.Sex)),
			MoveSpeed:   moveSpeed(&p.Character),
			AttackSpeed: attackSpeed(&p.Character),
		},
		blocks: p.Blocks,
		info: protocol.CharacterInfo{
			VID:        p.vid,
			Name:       p.Name,
			BodyPart:   bodyPart(p.Character),
			WeaponPart: weaponPart(p.Character),
			HairPart:   p.Hair,
			Empire:     empireNumbers[s.empire],
			Level:      uint32(p.Level),
		},
	}
	err := s.conn.Send(slices.Concat(
		[][]byte{protocol.EncodePhase(protocol.PhaseGame), protocol.EncodeTime(s.svc.clock.Now()), protocol.EncodeChannel(1)},
		me.appearance(), [][]byte{s.blockMode()}, s.inventoryShown(), s.taskbarShown(),
	)...)
	if err != nil {
		return err
	}
	// Only now is the player in the game phase: leaving the world takes them out of the
	// record, so they must be in it.
	me.tether, p.UnseenUntil = tether{x: p.X, y: p.Y}, s.svc.now().Add(unseenFor)
	s.svc.record.enter(me)
	s.phase = protocol.PhaseGame
	s.conn.Log().Info("entered the world", "character", p.Name, "x", p.X, "y", p.Y)
	return nil
}

// onMove keeps where the player's character is, to be saved when the connection closes,
// and shows the move to the others; the client moves its own character by itself. A move
// further than the character can have gone is not taken (walk).
func (s *session) onMove(body []byte) error {
	m, err := protocol.DecodeMove(body)
	if err != nil {
		return err
	}
	s.playing.X, s.playing.Y = s.svc.record.walk(s.playing.vid, m, s.svc.now())
	s.conn.Log().Debug("moved", "character", s.playing.Name, "type", m.Type, "x", m.X, "y", m.Y)
	return nil
}

// pointsOf fills the client's points array for the character, with the hit points and
// mana it has now, and its yang.
func pointsOf(p *character, gold uint32) protocol.CharacterPoints {
	return protocol.CharacterPoints{
		Level:        uint32(p.Level),
		Exp:          p.Exp,
		NextExp:      p.points.NextExp,
		StatPoints:   uint32(p.StatPoints),
		HP:           p.HP,
		MaxHP:        p.points.MaxHP,
		SP:           p.SP,
		MaxSP:        p.points.MaxSP,
		Stamina:      p.points.MaxStamina,
		MaxStamina:   p.points.MaxStamina,
		ST:           uint32(p.Strength),
		HT:           uint32(p.Vitality),
		DX:           uint32(p.Dexterity),
		IQ:           uint32(p.Intelligence),
		AttackSpeed:  uint32(attackSpeed(&p.Character)),
		MoveSpeed:    uint32(moveSpeed(&p.Character)),
		Defence:      defence(&p.Character),
		Gold:         gold,
		SkillPoints:  skillPoints(&p.Character),
		CastingSpeed: castingSpeed,
	}
}

// leaveWorld takes the character in play out of the others' view and saves it, if it
// entered the world, and lets go of it.
func (s *session) leaveWorld() {
	if s.claim == nil {
		return
	}
	if s.phase == protocol.PhaseGame {
		s.leaveTrade()
		s.svc.leaveAndSave(s.playing, s.empire, s.conn.Log())
	}
	s.svc.release(s.claim)
	s.claim, s.playing, s.shop, s.talking, s.upgrading = nil, nil, 0, nil, nil
	s.flyTarget, s.flyAdds, s.volley = 0, nil, volley{} // the next character's arrows are their own
}

// inventoryShown fills the inventory window, an item to a packet.
func (s *session) inventoryShown() [][]byte {
	packets := make([][]byte, 0, len(s.inventory))
	for _, it := range s.inventory {
		packets = append(packets, itemSet(it))
	}
	return packets
}
