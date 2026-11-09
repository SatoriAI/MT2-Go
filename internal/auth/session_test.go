package auth

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/handshake"
	"github.com/SatoriAI/MT2-Go/internal/network"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
)

// accounts are the accounts the store holds, by lower-cased name; looked counts the
// lookups.
type accounts struct {
	byName map[string]account.Account
	looked int
}

func (a *accounts) AccountByName(_ context.Context, name string) (account.Account, error) {
	a.looked++
	if acc, ok := a.byName[strings.ToLower(name)]; ok {
		return acc, nil
	}
	return account.Account{}, account.ErrNotFound
}

// idle is a session that does nothing, for the server side of a test connection.
type idle struct{}

func (idle) Open() error                                  { return nil }
func (idle) Close()                                       {}
func (idle) Handle(context.Context, protocol.Frame) error { return nil }

// loggingIn is a session past its handshake, on a connection whose client end the test
// reads, against a store holding Hendra with the password "sekret".
func loggingIn(t *testing.T) (*session, *accounts, *account.Keys, net.Conn) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	accepted := make(chan *network.Conn, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- network.Serve(ctx, ln, func(c *network.Conn) network.Session {
			accepted <- c
			return idle{}
		}, network.Timeouts{}, slog.New(slog.DiscardHandler))
	}()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = client.Close()
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	hash, err := bcrypt.GenerateFromPassword([]byte("sekret"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	store := &accounts{byName: map[string]account.Account{"hendra": {ID: 7, Name: "Hendra", PasswordHash: hash}}}
	keys := account.NewKeys(time.Now)
	svc := NewService(store, keys, handshake.NewClock())
	s := &session{svc: svc, conn: <-accepted, phase: protocol.PhaseAuth}
	return s, store, keys, client
}

// login is LOGIN3's body as the client sends it: the name, the password and the client's
// key, which is not read.
func login(name, password string) protocol.Frame {
	body := make([]byte, 31+17+16)
	copy(body, name)
	copy(body[31:], password)
	return protocol.Frame{Header: protocol.ClientLogin, Body: body}
}

func expect(t *testing.T, c net.Conn, want []byte) {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(time.Second))
	got := make([]byte, len(want))
	if _, err := io.ReadFull(c, got); err != nil || !bytes.Equal(got, want) {
		t.Fatalf("got %x, %v; want %x", got, err, want)
	}
}

// The right password hands out a key for the account, which the game port then redeems.
// The name's case is the store's to ignore.
func TestTheRightPasswordHandsOutAKey(t *testing.T) {
	s, _, keys, client := loggingIn(t)

	if err := s.Handle(context.Background(), login("hendra", "sekret")); err != nil {
		t.Fatal(err)
	}

	got := make([]byte, 6)
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := io.ReadFull(client, got); err != nil || got[0] != byte(protocol.ServerAuthSuccess) {
		t.Fatalf("got %x, %v", got, err)
	}
	r := protocol.NewReader(got[1:])
	key, result := r.U32(), r.U8()
	if id, ok := keys.Redeem(key, "Hendra"); result != 1 || !ok || id != 7 {
		t.Fatalf("result %d, redeemed %d, %v", result, id, ok)
	}
}

// A wrong password and an unknown name get the same answer, so the login screen cannot be
// used to find out which accounts exist; an unknown name is refused even with the password
// its decoy check is made from.
func TestAWrongPasswordAndAnUnknownNameAreRefusedAlike(t *testing.T) {
	for _, f := range []protocol.Frame{login("hendra", "zgadnij"), login("nikt", "sekret"), login("nikt", "decoy")} {
		s, _, _, client := loggingIn(t)
		if err := s.Handle(context.Background(), f); err != nil {
			t.Fatal(err)
		}
		expect(t, client, protocol.EncodeLoginFailure(protocol.LoginWrongPassword))
	}
}

// An account in the game, its key redeemed by a game connection still open, is refused as
// already online.
func TestAnAccountInTheGameIsRefused(t *testing.T) {
	s, _, keys, client := loggingIn(t)
	key, _ := keys.Issue(7, "Hendra")
	keys.Redeem(key, "Hendra")

	if err := s.Handle(context.Background(), login("Hendra", "sekret")); err != nil {
		t.Fatal(err)
	}

	expect(t, client, protocol.EncodeLoginFailure(protocol.LoginAlreadyOnline))
}

// A name that is not valid UTF-8 could not be stored, so it is refused as wrong without
// asking the store.
func TestANameThatIsNotUTF8IsRefusedWithoutALookup(t *testing.T) {
	s, store, _, client := loggingIn(t)

	if err := s.Handle(context.Background(), login("\xff\xfe", "sekret")); err != nil {
		t.Fatal(err)
	}

	expect(t, client, protocol.EncodeLoginFailure(protocol.LoginWrongPassword))
	if store.looked != 0 {
		t.Fatalf("the store was asked %d times", store.looked)
	}
}

// Three wrong passwords on one connection end it, each refusal told first.
func TestThreeWrongPasswordsEndTheConnection(t *testing.T) {
	s, _, _, client := loggingIn(t)
	for i := range 3 {
		err := s.Handle(context.Background(), login("hendra", "zgadnij"))
		expect(t, client, protocol.EncodeLoginFailure(protocol.LoginWrongPassword))
		if (err != nil) != (i == 2) {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
}

// A login before the handshake is done is out of its phase, and ends the connection.
func TestALoginBeforeTheHandshakeEndsTheConnection(t *testing.T) {
	s, store, _, _ := loggingIn(t)
	s.phase = protocol.PhaseHandshake

	if err := s.Handle(context.Background(), login("hendra", "sekret")); err == nil || store.looked != 0 {
		t.Fatalf("got %v, the store asked %d times", err, store.looked)
	}
}

// A store that fails ends the connection with its error, telling the client nothing.
func TestAStoreThatFailsEndsTheConnection(t *testing.T) {
	s, _, _, _ := loggingIn(t)
	failing := errors.New("the database is gone")
	s.svc = NewService(failingAccounts{failing}, account.NewKeys(time.Now), handshake.NewClock())

	if err := s.Handle(context.Background(), login("hendra", "sekret")); !errors.Is(err, failing) {
		t.Fatalf("got %v", err)
	}
}

type failingAccounts struct{ err error }

func (f failingAccounts) AccountByName(context.Context, string) (account.Account, error) {
	return account.Account{}, f.err
}
