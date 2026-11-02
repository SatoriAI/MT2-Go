// Package auth is the login port. A client syncs its clock, sends its name and
// password, and receives a key to present on the game port; then it disconnects.
package auth

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// Service is what every auth connection shares.
type Service struct {
	Store account.Store
	Keys  *account.Keys
	Clock handshake.Clock
}

func (s *Service) NewSession(conn *network.Conn) network.Session {
	return &session{svc: s, conn: conn, sync: handshake.NewSync(s.Clock), phase: protocol.PhaseHandshake}
}

type session struct {
	svc   *Service
	conn  *network.Conn
	sync  *handshake.Sync
	phase protocol.Phase
}

func (s *session) Open() error { return s.conn.Send(s.sync.Greeting()...) }

func (s *session) Close() {}

func (s *session) Handle(ctx context.Context, f protocol.Frame) error {
	switch {
	case s.phase == protocol.PhaseHandshake && f.Header == protocol.ClientHandshake:
		return s.onHandshake(f.Body)
	case s.phase == protocol.PhaseAuth && f.Header == protocol.ClientLogin:
		return s.onLogin(ctx, f.Body)
	}
	return fmt.Errorf("auth: header 0x%02X not expected in phase %d", byte(f.Header), s.phase)
}

func (s *session) onHandshake(body []byte) error {
	done, resend, err := s.sync.Reply(body)
	if err != nil {
		return err
	}
	if !done {
		return s.conn.Send(resend)
	}
	s.phase = protocol.PhaseAuth
	return s.conn.Send(protocol.EncodePhase(protocol.PhaseAuth))
}

func (s *session) onLogin(ctx context.Context, body []byte) error {
	login, err := protocol.DecodeLogin(body)
	if err != nil {
		return err
	}
	acc, err := s.svc.Store.AccountByName(ctx, login.Name)
	if errors.Is(err, account.ErrNotFound) {
		return s.refuse(login.Name, protocol.LoginWrongPassword)
	}
	if err != nil {
		return err
	}
	// An unknown name and a wrong password get the same answer, so the login screen
	// cannot be used to find out which accounts exist.
	if bcrypt.CompareHashAndPassword(acc.PasswordHash, []byte(login.Password)) != nil {
		return s.refuse(login.Name, protocol.LoginWrongPassword)
	}
	if s.svc.Keys.Online(acc.ID) {
		return s.refuse(login.Name, protocol.LoginAlreadyOnline)
	}
	s.conn.Log().Info("login accepted", "account", acc.Name)
	return s.conn.Send(protocol.EncodeAuthSuccess(s.svc.Keys.Issue(acc.ID, acc.Name)))
}

func (s *session) refuse(name string, status protocol.LoginStatus) error {
	s.conn.Log().Info("login refused", "account", name, "status", status)
	return s.conn.Send(protocol.EncodeLoginFailure(status))
}
