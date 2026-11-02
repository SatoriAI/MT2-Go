// Package game is the game port. So far it covers what happens before a character
// enters the world: the login screen's channel check, the login by key, the kingdom
// choice and the character list.
package game

import (
	"context"
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
		s.conn.Close()
		return nil
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

	acc, err := s.svc.Store.AccountByID(ctx, id)
	if err != nil {
		return err
	}
	var packets [][]byte
	if acc.Empire != protocol.EmpireNone {
		slots, err := s.svc.Store.Characters(ctx, id)
		if err != nil {
			return err
		}
		packets = append(packets, protocol.EncodeEmpire(acc.Empire), protocol.EncodeCharacterList(slots, s.svc.Public))
	}
	s.phase = protocol.PhaseSelect
	s.conn.Log().Info("entered selection", "account", acc.Name, "empire", acc.Empire)
	return s.conn.Send(append(packets, protocol.EncodePhase(protocol.PhaseSelect))...)
}

// onEmpire stores the kingdom a new account picked. The client moves on to the character
// screen by itself, so nothing is sent back.
func (s *session) onEmpire(ctx context.Context, body []byte) error {
	empire, err := protocol.DecodeEmpire(body)
	if err != nil {
		return err
	}
	if !empire.Valid() {
		return fmt.Errorf("game: kingdom %d does not exist", empire)
	}
	return s.svc.Store.SetEmpire(ctx, s.accountID, empire)
}
