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
	// ChooseEmpire sets the account's kingdom while it has no character, and reports
	// whether the account now has that kingdom.
	ChooseEmpire(ctx context.Context, id int64, empire account.Empire) (bool, error)
	Characters(ctx context.Context, accountID int64) ([]account.Character, error)
	// CreateCharacter stores c and returns its id, while the account's kingdom is still
	// empire: account.ErrKingdomChanged, ErrNameTaken or ErrSlotTaken otherwise.
	CreateCharacter(ctx context.Context, accountID int64, empire account.Empire, c account.Character) (uint32, error)
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
	accountID int64          // set once the login key is redeemed
	empire    account.Empire // the account's kingdom as this connection last knew it
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
	case s.phase == protocol.PhaseSelect && f.Header == protocol.ClientCreateCharacter:
		return s.onCreateCharacter(ctx, f.Body)
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
	s.empire = acc.Empire
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

// onEmpire stores the kingdom the player picked. The client sends it from the kingdom
// screen, and again from the screen it shows before the first character is created, where
// the player may pick another; nothing is sent back. Once the account has a character,
// only the same kingdom again is accepted.
func (s *session) onEmpire(ctx context.Context, body []byte) error {
	raw, err := protocol.DecodeEmpire(body)
	if err != nil {
		return err
	}
	empire, ok := empireFromNumber(raw)
	if !ok {
		return fmt.Errorf("game: kingdom %d does not exist", raw)
	}
	if empire == s.empire {
		return nil // a repeat, for example a double click; nothing to store
	}
	chosen, err := s.svc.store.ChooseEmpire(ctx, s.accountID, empire)
	if err != nil {
		return err
	}
	if !chosen {
		return fmt.Errorf("game: kingdom %q refused: the account has characters in another", empire)
	}
	s.empire = empire
	return nil
}

// onCreateCharacter creates a character in an empty slot. A request the rules refuse gets
// the failure reply the create screen shows, and the connection stays open.
func (s *session) onCreateCharacter(ctx context.Context, body []byte) error {
	req, err := protocol.DecodeCreateCharacter(body)
	if err != nil {
		return err
	}
	if s.empire == account.EmpireNone {
		return errors.New("game: a character needs a kingdom first")
	}
	class, sex, ok := raceFromNumber(req.Race)
	if !ok || req.Slot >= protocol.CharacterSlots || req.Shape > 1 || !account.ValidCharacterName(req.Name) {
		s.conn.Log().Info("character refused", "name", req.Name, "race", req.Race, "slot", req.Slot, "shape", req.Shape)
		return s.conn.Send(protocol.EncodeCreateFailure(protocol.CreateRefused))
	}
	c := account.Character{
		Slot:  int(req.Slot),
		Name:  req.Name,
		Class: class,
		Sex:   sex,
		Level: 1,
		Stats: class.StartingStats(),
		Shape: req.Shape,
	}
	c.X, c.Y = s.empire.StartPosition()

	c.ID, err = s.svc.store.CreateCharacter(ctx, s.accountID, s.empire, c)
	switch {
	case errors.Is(err, account.ErrNameTaken):
		return s.conn.Send(protocol.EncodeCreateFailure(protocol.CreateNameTaken))
	case errors.Is(err, account.ErrSlotTaken):
		return s.conn.Send(protocol.EncodeCreateFailure(protocol.CreateRefused))
	case errors.Is(err, account.ErrKingdomChanged):
		// Another connection changed it. This one's screens are out of date, and logging in
		// again shows the account as it is.
		return fmt.Errorf("game: the kingdom changed while %q was being created", c.Name)
	case err != nil:
		return err
	}
	s.conn.Log().Info("character created", "name", c.Name, "class", c.Class, "sex", c.Sex)
	return s.conn.Send(protocol.EncodeCharacterCreated(req.Slot, summary(c), s.svc.public))
}
