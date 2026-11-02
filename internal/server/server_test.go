package server

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/SatoriAI/MT2-Go/internal/account"
	"github.com/SatoriAI/MT2-Go/internal/config"
	"github.com/SatoriAI/MT2-Go/internal/protocol"
	"github.com/SatoriAI/MT2-Go/internal/store"
)

// These tests drive both ports over real sockets with the bytes the game client sends,
// against a real database, so they prove the flow rather than the parts.

const sequenceByte = 0xAF // the first entry of the client's sequence table

const password = "secret" // every test account's

type harness struct {
	t        *testing.T
	accounts store.Accounts
	auth     string
	game     string
}

func start(t *testing.T) *harness {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; run `make db` and use .env")
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool, err := store.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE accounts, characters RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	authLn, _ := net.Listen("tcp", "127.0.0.1:0")
	gameLn, _ := net.Listen("tcp", "127.0.0.1:0")
	cfg := config.Config{GamePublic: netip.MustParseAddrPort(gameLn.Addr().String())}
	accounts := store.Accounts{Pool: pool}
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, Ports{Auth: authLn, Game: gameLn}, accounts, cfg, slog.New(slog.DiscardHandler))
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
		pool.Close()
	})
	return &harness{t: t, accounts: accounts, auth: authLn.Addr().String(), game: gameLn.Addr().String()}
}

func (h *harness) createAccount(name string) int64 {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	id, err := h.accounts.CreateAccount(context.Background(), name, hash)
	if err != nil {
		h.t.Fatal(err)
	}
	return id
}

// exec runs SQL the server has no query for yet, such as creating a character.
func (h *harness) exec(sql string, args ...any) {
	h.t.Helper()
	if _, err := h.accounts.Pool.Exec(context.Background(), sql, args...); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) waitForEmpire(name string, want account.Empire) {
	h.t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if acc, err := h.accounts.AccountByName(context.Background(), name); err == nil && acc.Empire == want {
			return
		}
	}
	h.t.Fatalf("kingdom %d was never stored for %s", want, name)
}

// client plays the game client's side of one connection.
type client struct {
	t    *testing.T
	conn net.Conn
}

func (h *harness) dial(addr string) *client {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		h.t.Fatal(err)
	}
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	h.t.Cleanup(func() { _ = conn.Close() })
	return &client{t: h.t, conn: conn}
}

func (c *client) read(n int) []byte {
	c.t.Helper()
	b := make([]byte, n)
	if _, err := io.ReadFull(c.conn, b); err != nil {
		c.t.Fatalf("reading %d bytes: %v", n, err)
	}
	return b
}

func (c *client) expect(want []byte) {
	c.t.Helper()
	if got := c.read(len(want)); !bytes.Equal(got, want) {
		c.t.Fatalf("got %x, want %x", got, want)
	}
}

func (c *client) send(parts ...[]byte) {
	c.t.Helper()
	if _, err := c.conn.Write(bytes.Join(parts, nil)); err != nil {
		c.t.Fatal(err)
	}
}

// handshake answers the greeting the way the client does, then expects the next phase.
func (c *client) handshake(next protocol.Phase) {
	c.t.Helper()
	c.expect(protocol.EncodePhase(protocol.PhaseHandshake))
	greeting := c.read(13)
	c.send(greeting[:9], []byte{0, 0, 0, 0}) // same id and time, delta 0
	c.expect(protocol.EncodePhase(next))
}

func field(s string, size int) []byte {
	b := make([]byte, size)
	copy(b, s)
	return b
}

func (c *client) login(name, password string) {
	c.send([]byte{byte(protocol.ClientLogin)}, field(name, 31), field(password, 17), make([]byte, 16), []byte{sequenceByte})
}

func (c *client) loginByKey(name string, key uint32) {
	c.send([]byte{byte(protocol.ClientLoginByKey)}, field(name, 31), binary.LittleEndian.AppendUint32(nil, key),
		make([]byte, 16), []byte{sequenceByte})
}

func (c *client) chooseEmpire(e account.Empire, sequence byte) {
	c.send([]byte{byte(protocol.ClientEmpire), byte(e), sequence})
}

// expectClosed waits for the server to end the connection.
func (c *client) expectClosed() {
	c.t.Helper()
	if n, err := c.conn.Read(make([]byte, 1)); !errors.Is(err, io.EOF) {
		c.t.Fatalf("expected the server to close the connection, read %d bytes, err %v", n, err)
	}
}

// enterSelection logs in with a key on a new game connection, up to the select phase
// and anything sent before it.
func (h *harness) enterSelection(name string, key uint32) *client {
	game := h.dial(h.game)
	game.handshake(protocol.PhaseLogin)
	game.loginByKey(name, key)
	return game
}

// authenticate logs in on the auth port and returns the key for the game port.
func (h *harness) authenticate(name string) uint32 {
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)
	auth.login(name, password)
	reply := auth.read(6)
	if reply[0] != byte(protocol.ServerAuthSuccess) || reply[5] != 1 {
		h.t.Fatalf("login refused: %x", reply)
	}
	return binary.LittleEndian.Uint32(reply[1:5])
}

func TestANewAccountLogsInAndIsAskedForAKingdom(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	key := h.authenticate("admin")

	game := h.enterSelection("admin", key)

	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
}

func TestTheNameLogsInWhateverItsCase(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	key := h.authenticate("ADMIN")

	game := h.enterSelection("ADMIN", key)

	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
}

func TestTheChosenKingdomIsStoredAndTheNextLoginShowsTheCharacterList(t *testing.T) {
	h := start(t)
	id := h.createAccount("admin")
	key := h.authenticate("admin")
	first := h.enterSelection("admin", key)
	first.expect(protocol.EncodePhase(protocol.PhaseSelect))

	first.chooseEmpire(account.EmpireYellow, 0xCA)
	h.waitForEmpire("admin", account.EmpireYellow) // the server sends nothing back to wait on
	_ = first.conn.Close()
	h.exec(`INSERT INTO characters (account_id, slot, name, class, sex, level, st, ht, dx, iq, hair, map, x, y)
		VALUES ($1, 2, 'Hendra', 2, 1, 21, 6, 4, 3, 2, 3, 1, 469300, 964200)`, id)

	second := h.enterSelection("admin", key) // the client reuses its key for a second game connection
	second.expect(protocol.EncodeEmpire(uint8(account.EmpireYellow)))
	var slots [protocol.CharacterSlots]protocol.CharacterSummary
	slots[2] = protocol.CharacterSummary{ID: 1, Name: "Hendra", Job: 6, Level: 21, ST: 6, HT: 4, DX: 3, IQ: 2,
		HairPart: 3, X: 469300, Y: 964200}
	second.expect(protocol.EncodeCharacterList(slots, netip.MustParseAddrPort(h.game)))
	second.expect(protocol.EncodePhase(protocol.PhaseSelect))
}

func TestAKingdomOnceChosenCannotBeChanged(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	key := h.authenticate("admin")
	game := h.enterSelection("admin", key)
	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
	game.chooseEmpire(account.EmpireRed, 0xCA)
	h.waitForEmpire("admin", account.EmpireRed)

	game.chooseEmpire(account.EmpireBlue, 0x8A)

	game.expectClosed()
	h.waitForEmpire("admin", account.EmpireRed)
}

func TestASecondLoginIsRefusedWhileTheAccountIsInTheGame(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.enterSelection("admin", h.authenticate("admin"))
	game.expect(protocol.EncodePhase(protocol.PhaseSelect))
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)

	auth.login("admin", password)

	auth.expect(protocol.EncodeLoginFailure(protocol.LoginAlreadyOnline))
}

func TestAKeyIsRefusedForAnotherName(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	h.createAccount("other")
	key := h.authenticate("admin")

	game := h.enterSelection("other", key)

	game.expectClosed()
}

func TestAPacketOutOfItsPhaseEndsTheConnection(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	game := h.dial(h.game)
	game.handshake(protocol.PhaseLogin)

	game.chooseEmpire(account.EmpireRed, sequenceByte) // before logging in

	game.expectClosed()
}

func TestAWrongPasswordIsRefusedWithTheStatusTheClientShows(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)

	auth.login("admin", "wrong")

	auth.expect(protocol.EncodeLoginFailure(protocol.LoginWrongPassword))
}

func TestAnUnknownNameGetsTheSameAnswerAsAWrongPassword(t *testing.T) {
	h := start(t)
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)

	auth.login("nobody", password)

	auth.expect(protocol.EncodeLoginFailure(protocol.LoginWrongPassword))
}

func TestTheConnectionClosesAfterThreeWrongPasswords(t *testing.T) {
	h := start(t)
	h.createAccount("admin")
	auth := h.dial(h.auth)
	auth.handshake(protocol.PhaseAuth)

	for range 3 {
		auth.login("admin", "wrong")
		auth.expect(protocol.EncodeLoginFailure(protocol.LoginWrongPassword))
	}

	auth.expectClosed()
}

func TestTheLoginScreenSeesTheChannelAsOpen(t *testing.T) {
	h := start(t)
	game := h.dial(h.game)
	game.expect(protocol.EncodePhase(protocol.PhaseHandshake))
	game.read(13)
	port := netip.MustParseAddrPort(h.game).Port()

	game.send([]byte{byte(protocol.ClientStateChecker)})

	game.expect(protocol.EncodeChannelStatus([]protocol.ChannelStatus{{Port: port, Status: protocol.ChannelNormal}}))
}
