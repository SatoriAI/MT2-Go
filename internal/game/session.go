// Package game is the game port. So far it covers what happens before a character
// enters the world: the login screen's channel check, the login by key, the kingdom
// choice and the character list.
package game

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Service is what every game connection shares.
type Service struct {
	Store account.Store
	Keys  *account.Keys
	Clock handshake.Clock
	// Public is the address the client is told to connect to for this game server. It
	// has to be reachable from the client, so it is never 0.0.0.0.
	Public netip.AddrPort
}

func (s *Service) NewSession(conn *network.Conn) network.Session {
	return &session{svc: s, conn: conn, sync: handshake.NewSync(s.Clock), phase: protocol.PhaseHandshake}
}

type session struct {
	svc       *Service
	conn      *network.Conn
	sync      *handshake.Sync
	phase     protocol.Phase
	accountID int64 // set once the login key is redeemed
}

func (s *session) Open() error { return s.conn.Send(s.sync.Greeting()...) }

func (s *session) Close() {
	if s.accountID != 0 {
		s.svc.Keys.Release(s.accountID)
	}
}

func (s *session) Handle(ctx context.Context, f protocol.Frame) error {
	switch {
	case f.Header == protocol.ClientStateChecker:
		return s.conn.Send(protocol.EncodeChannelStatus([]protocol.ChannelStatus{
			{Port: s.svc.Public.Port(), Status: protocol.ChannelNormal},
		}))
	case f.Header == protocol.ClientMarkLogin:
		// Guild marks are not served yet; the client copes with a closed mark socket.
		return network.ErrHangUp
	case s.phase == protocol.PhaseHandshake && f.Header == protocol.ClientHandshake:
		return s.onHandshake(f.Body)
	case s.phase == protocol.PhaseLogin && f.Header == protocol.ClientLoginByKey:
		return s.onLoginByKey(ctx, f.Body)
	case s.phase == protocol.PhaseSelect && f.Header == protocol.ClientEmpire:
		return s.onEmpire(ctx, f.Body)
	}
	return fmt.Errorf("game: header 0x%02X not expected in phase %d", byte(f.Header), s.phase)
}

func (s *session) onHandshake(body []byte) error {
	done, resend, err := s.sync.Reply(body)
	if err != nil {
		return err
	}
	if !done {
		return s.conn.Send(resend)
	}
	s.phase = protocol.PhaseLogin
	return s.conn.Send(protocol.EncodePhase(protocol.PhaseLogin))
}

// onLoginByKey sends what the selection screens need. The client picks its screen when
// the select phase arrives: with a kingdom known it shows the characters, without one it
// asks for a kingdom first. So the kingdom and the list must come before the phase.
func (s *session) onLoginByKey(ctx context.Context, body []byte) error {
	login, err := protocol.DecodeLoginByKey(body)
	if err != nil {
		return err
	}
	id, ok := s.svc.Keys.Redeem(login.Key, login.Name)
	if !ok {
		return fmt.Errorf("game: login key refused for %q", login.Name)
	}
	s.accountID = id
	s.conn.SetIdleTimeout(0) // the player may sit on the selection screens for as long as they like

	acc, err := s.svc.Store.AccountByID(ctx, id)
	if err != nil {
		return err
	}
	var packets [][]byte
	if acc.Empire != account.EmpireNone {
		characters, err := s.svc.Store.Characters(ctx, id)
		if err != nil {
			return err
		}
		packets = append(packets, protocol.EncodeEmpire(uint8(acc.Empire)),
			protocol.EncodeCharacterList(slots(characters), s.svc.Public))
	}
	s.phase = protocol.PhaseSelect
	s.conn.Log().Info("entered selection", "account", acc.Name, "empire", acc.Empire)
	return s.conn.Send(append(packets, protocol.EncodePhase(protocol.PhaseSelect))...)
}

// onEmpire stores the kingdom a new account picked. The client moves on to the character
// screen by itself, so nothing is sent back.
func (s *session) onEmpire(ctx context.Context, body []byte) error {
	raw, err := protocol.DecodeEmpire(body)
	if err != nil {
		return err
	}
	empire := account.Empire(raw)
	if !empire.Valid() {
		return fmt.Errorf("game: kingdom %d does not exist", empire)
	}
	chosen, err := s.svc.Store.ChooseEmpire(ctx, s.accountID, empire)
	if err != nil {
		return err
	}
	if !chosen {
		return errors.New("game: the account already has a kingdom")
	}
	return nil
}

// slots lays the characters out as the selection screen shows them.
func slots(characters []account.Character) [protocol.CharacterSlots]protocol.CharacterSummary {
	var out [protocol.CharacterSlots]protocol.CharacterSummary
	for _, c := range characters {
		out[c.Slot] = protocol.CharacterSummary{
			ID:          c.ID,
			Name:        c.Name,
			Job:         job(c.Class, c.Sex),
			Level:       c.Level,
			PlayMinutes: c.PlayMinutes,
			ST:          c.ST, HT: c.HT, DX: c.DX, IQ: c.IQ,
			HairPart: c.Hair,
			X:        c.X, Y: c.Y,
			SkillGroup: c.SkillGroup,
		}
	}
	return out
}

// job is the client's single number for class and sex: 0 to 3 are the male warrior,
// ninja, sura and shaman, 4 to 7 the female ones. The body part stays 0, the bare body, until
// characters wear armour.
func job(c account.Class, s account.Sex) uint8 {
	return uint8(c) + 4*uint8(s)
}
