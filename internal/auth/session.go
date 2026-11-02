// Package auth is the login port. A client syncs its clock, sends its name and
// password, and receives a key to present on the game port; then it disconnects.
package auth

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// maxAttempts is how many wrong passwords one connection may send before it is closed.
const maxAttempts = 3

// Service is what every auth connection shares.
type Service struct {
	store account.Store
	keys  *account.Keys
	clock handshake.Clock
	// checks bounds the password checks running at once, so a flood of logins queues
	// instead of taking every CPU.
	checks chan struct{}
	// decoy is checked for names that do not exist, so they take as long to refuse as a
	// wrong password and the response time does not reveal which accounts exist.
	decoy []byte
}

func NewService(store account.Store, keys *account.Keys, clock handshake.Clock) (*Service, error) {
	decoy, err := bcrypt.GenerateFromPassword([]byte("decoy"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	return &Service{store: store, keys: keys, clock: clock, checks: make(chan struct{}, runtime.NumCPU()), decoy: decoy}, nil
}

func (s *Service) NewSession(conn *network.Conn) network.Session {
	return &session{svc: s, conn: conn, sync: handshake.NewSync(s.clock), phase: protocol.PhaseHandshake}
}

type session struct {
	svc      *Service
	conn     *network.Conn
	sync     *handshake.Sync
	phase    protocol.Phase
	failures int
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
	acc, err := s.findAccount(ctx, login.Name)
	if err != nil {
		return err
	}
	hash := acc.PasswordHash
	if hash == nil {
		hash = s.svc.decoy
	}
	ok, err := s.checkPassword(ctx, hash, login.Password)
	if err != nil {
		return err
	}
	// An unknown name and a wrong password get the same answer after the same work, so
	// the login screen cannot be used to find out which accounts exist.
	if !ok || acc.PasswordHash == nil {
		return s.refuse(login.Name, protocol.LoginWrongPassword)
	}
	key, issued := s.svc.keys.Issue(acc.ID, acc.Name)
	if !issued {
		return s.refuse(login.Name, protocol.LoginAlreadyOnline)
	}
	s.conn.Log().Info("login accepted", "account", acc.Name)
	return s.conn.Send(protocol.EncodeAuthSuccess(key))
}

// findAccount returns the zero Account for a name no account has, including one that is
// not valid UTF-8 and so could not be stored.
func (s *session) findAccount(ctx context.Context, name string) (account.Account, error) {
	if !utf8.ValidString(name) {
		return account.Account{}, nil
	}
	acc, err := s.svc.store.AccountByName(ctx, name)
	if errors.Is(err, account.ErrNotFound) {
		return account.Account{}, nil
	}
	return acc, err
}

func (s *session) checkPassword(ctx context.Context, hash []byte, password string) (bool, error) {
	select {
	case s.svc.checks <- struct{}{}:
	case <-ctx.Done():
		return false, ctx.Err()
	}
	defer func() { <-s.svc.checks }()
	return bcrypt.CompareHashAndPassword(hash, []byte(password)) == nil, nil
}

func (s *session) refuse(name string, status protocol.LoginStatus) error {
	s.conn.Log().Info("login refused", "account", name, "status", status)
	if err := s.conn.Send(protocol.EncodeLoginFailure(status)); err != nil {
		return err
	}
	if s.failures++; s.failures >= maxAttempts {
		return fmt.Errorf("auth: %d failed logins on one connection", s.failures)
	}
	return nil
}
