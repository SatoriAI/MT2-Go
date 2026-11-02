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

// Accounts is what the game port needs from storage.
type Accounts interface {
	AccountByID(ctx context.Context, id int64) (account.Account, error)
	// ChooseEmpire sets the kingdom of an account that has none, and reports whether it
	// did: a kingdom, once chosen, stays.
	ChooseEmpire(ctx context.Context, id int64, empire account.Empire) (bool, error)
	Characters(ctx context.Context, accountID int64) ([]account.Character, error)
}

// Service is what every game connection shares.
type Service struct {
	store Accounts
	keys  *account.Keys
	clock handshake.Clock
	// public is the address the client is told to connect to for this game server. It
	// has to be reachable from the client, so it is never 0.0.0.0.
	public netip.AddrPort
}

func NewService(store Accounts, keys *account.Keys, clock handshake.Clock, public netip.AddrPort) *Service {
	return &Service{store: store, keys: keys, clock: clock, public: public}
}

func (s *Service) NewSession(conn *network.Conn) network.Session {
	return &session{svc: s, conn: conn, sync: handshake.NewSync(s.clock), phase: protocol.PhaseHandshake}
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
		s.svc.keys.Release(s.accountID)
	}
}

func (s *session) Handle(ctx context.Context, f protocol.Frame) error {
	switch {
	case f.Header == protocol.ClientStateChecker:
		return s.conn.Send(protocol.EncodeChannelStatus([]protocol.ChannelStatus{
			{Port: s.svc.public.Port(), Status: protocol.ChannelNormal},
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
	id, ok := s.svc.keys.Redeem(login.Key, login.Name)
	if !ok {
		return fmt.Errorf("game: login key refused for %q", login.Name)
	}
	s.accountID = id
	s.conn.KeepOpen() // the player may sit on the selection screens for as long as they like

	acc, err := s.svc.store.AccountByID(ctx, id)
	if err != nil {
		return err
	}
	var packets [][]byte
	if acc.Empire != account.EmpireNone {
		characters, err := s.svc.store.Characters(ctx, id)
		if err != nil {
			return err
		}
		packets = append(packets, protocol.EncodeEmpire(empireNumbers[acc.Empire]),
			protocol.EncodeCharacterList(slots(characters), s.svc.public))
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
	empire, ok := empireFromNumber(raw)
	if !ok {
		return fmt.Errorf("game: kingdom %d does not exist", raw)
	}
	chosen, err := s.svc.store.ChooseEmpire(ctx, s.accountID, empire)
	if err != nil {
		return err
	}
	if !chosen {
		return errors.New("game: the account already has a kingdom")
	}
	return nil
}
